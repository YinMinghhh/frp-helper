package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"frp-helper/internal/frpc"
	"frp-helper/internal/model"
	"frp-helper/internal/store"
)

type ApplyMode string

const (
	ApplyReplace ApplyMode = "replace"
	ApplyMerge   ApplyMode = "merge"
)

type InstallOptions struct {
	Version     string
	ArchivePath string
	BaseURL     string
}

type App struct {
	store     *store.Store
	installer *frpc.Installer
	stdout    io.Writer
	stderr    io.Writer
	now       func() time.Time
}

func New(stdout, stderr io.Writer) (*App, error) {
	s, err := store.New()
	if err != nil {
		return nil, err
	}
	return &App{
		store:     s,
		installer: frpc.NewInstaller(s),
		stdout:    stdout,
		stderr:    stderr,
		now:       time.Now,
	}, nil
}

func (a *App) Apply(ctx context.Context, manifestPath string, mode ApplyMode) error {
	incoming, err := loadManifestFile(manifestPath)
	if err != nil {
		return err
	}
	if err := incoming.Validate(); err != nil {
		return err
	}

	finalManifest := incoming
	if mode == ApplyMerge {
		if existing, err := a.store.ReadManifest(); err == nil {
			finalManifest = mergeManifest(existing, incoming)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := finalManifest.Validate(); err != nil {
		return err
	}

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	version := a.selectedVersion(state)
	if _, err := a.ensureInstalled(ctx, version, "", ""); err != nil {
		return err
	}

	if err := a.persistManifestState(finalManifest, &state); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "Applied %d service(s).\n", len(finalManifest.Services))
	fmt.Fprintf(a.stdout, "Manifest: %s\n", a.store.ManifestPath())
	fmt.Fprintf(a.stdout, "Config: %s\n", a.store.ConfigPath())
	return nil
}

func (a *App) Install(ctx context.Context, opts InstallOptions) error {
	version := opts.Version
	if strings.TrimSpace(version) == "" {
		version = model.DefaultFRPCVersion
	}
	frpcPath, err := a.ensureInstalled(ctx, version, opts.ArchivePath, opts.BaseURL)
	if err != nil {
		return err
	}

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	state.SelectedVersion = normalizeVersion(version)
	state.FRPCPath = frpcPath
	state.ConfigPath = a.store.ConfigPath()
	state.ManifestPath = a.store.ManifestPath()
	if err := a.store.WriteRuntime(state); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "Installed frpc %s\n", normalizeVersion(version))
	fmt.Fprintf(a.stdout, "Binary: %s\n", frpcPath)
	return nil
}

func (a *App) Start(ctx context.Context) error {
	manifest, err := a.store.ReadManifest()
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return err
	}

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	if state.Running && frpc.ProcessRunning(state.PID) {
		return fmt.Errorf("frpc is already running with pid %d", state.PID)
	}

	version := a.selectedVersion(state)
	frpcPath, err := a.ensureInstalled(ctx, version, "", "")
	if err != nil {
		return err
	}
	if err := a.persistManifestState(manifest, &state); err != nil {
		return err
	}

	if err := preflightPorts(manifest.EnabledServices()); err != nil {
		return err
	}

	verifyOutput, err := frpc.VerifyConfig(ctx, frpcPath, a.store.ConfigPath())
	verifyOutput = model.Sanitize(verifyOutput, manifest.Secrets())
	if err != nil {
		message := frpc.MapRuntimeError(verifyOutput)
		if message == "" {
			message = err.Error()
		}
		state.LastError = message
		state.Running = false
		state.PID = 0
		state.LastExitTime = timePtr(a.now())
		if writeErr := a.store.WriteRuntime(state); writeErr != nil {
			return writeErr
		}
		return fmt.Errorf("%s", message)
	}

	logFile, err := os.OpenFile(a.store.LogPath(), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	writer := frpc.NewRedactingWriter([]io.Writer{a.stdout, logFile}, manifest.Secrets())
	defer writer.Close()

	cmd := exec.CommandContext(ctx, frpcPath, "-c", a.store.ConfigPath())
	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start frpc: %w", err)
	}

	startedAt := a.now()
	state.SelectedVersion = version
	state.FRPCPath = frpcPath
	state.ConfigPath = a.store.ConfigPath()
	state.ManifestPath = a.store.ManifestPath()
	state.PID = cmd.Process.Pid
	state.Running = true
	state.LastStartTime = &startedAt
	state.LastError = ""
	state.Services = buildServiceRuntimeMap(manifest, state.Services)
	markServices(state.Services, manifest, "starting", "waiting for local listener")
	if err := a.store.WriteRuntime(state); err != nil {
		_ = cmd.Process.Kill()
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	probeErr := a.probeServices(manifest, &state, writer)
	if probeErr != nil {
		_ = cmd.Process.Kill()
		waitErr := <-waitCh
		if waitErr != nil {
			raw := writer.RecentText()
			message, actionable := frpc.InterpretRuntimeError(raw)
			if !actionable {
				currentState, readErr := a.store.ReadRuntime()
				explicitlyStopped := readErr == nil && !currentState.Running
				if explicitlyStopped || isExpectedStop(waitErr) {
					return a.finishExitedProcess(state, "", false)
				}
				message = ""
			}
			if message == "" {
				message = probeErr.Error()
			}
			return a.finishExitedProcess(state, message, true)
		}
		return a.finishExitedProcess(state, probeErr.Error(), true)
	}

	rows := endpointRows(manifest, state)
	a.printEndpointTable(rows)
	fmt.Fprintln(a.stdout, "Press Ctrl+C to stop.")

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	select {
	case <-signals:
		fmt.Fprintln(a.stdout, "Stopping frpc...")
		_ = cmd.Process.Kill()
		<-waitCh
		return a.finishExitedProcess(state, "", false)
	case waitErr := <-waitCh:
		raw := writer.RecentText()
		message, actionable := frpc.InterpretRuntimeError(raw)
		if !actionable {
			message = ""
		}
		currentState, readErr := a.store.ReadRuntime()
		explicitlyStopped := readErr == nil && !currentState.Running
		if explicitlyStopped && !actionable {
			return a.finishExitedProcess(state, "", false)
		}
		if waitErr != nil && isExpectedStop(waitErr) && !actionable {
			return a.finishExitedProcess(state, "", false)
		}
		if waitErr != nil && message == "" {
			message = waitErr.Error()
		}
		return a.finishExitedProcess(state, message, waitErr != nil)
	}
}

func (a *App) Stop(ctx context.Context) error {
	_ = ctx

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	if state.PID == 0 {
		fmt.Fprintln(a.stdout, "frpc is not running.")
		return nil
	}
	if !frpc.ProcessRunning(state.PID) {
		state.Running = false
		state.PID = 0
		now := a.now()
		state.LastStopTime = &now
		if err := a.store.WriteRuntime(state); err != nil {
			return err
		}
		fmt.Fprintln(a.stdout, "frpc was not running; cleared stale state.")
		return nil
	}

	pid := state.PID
	state.Running = false
	state.PID = 0
	now := a.now()
	state.LastStopTime = &now
	if err := a.store.WriteRuntime(state); err != nil {
		return err
	}
	if err := frpc.KillProcess(pid); err != nil {
		return fmt.Errorf("stop frpc pid %d: %w", pid, err)
	}

	fmt.Fprintln(a.stdout, "frpc stopped.")
	return nil
}

func (a *App) Restart(ctx context.Context) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}
	return a.Start(ctx)
}

func (a *App) Status(ctx context.Context) error {
	_ = ctx

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	manifest, manifestErr := a.store.ReadManifest()
	if manifestErr != nil && !errors.Is(manifestErr, os.ErrNotExist) {
		return manifestErr
	}

	if state.Running && !frpc.ProcessRunning(state.PID) {
		state.Running = false
		state.PID = 0
		now := a.now()
		state.LastExitTime = &now
		if err := a.store.WriteRuntime(state); err != nil {
			return err
		}
	}

	fmt.Fprintf(a.stdout, "Home: %s\n", a.store.Layout.Home)
	fmt.Fprintf(a.stdout, "frpc version: %s\n", blankIfEmpty(state.SelectedVersion, "(not selected)"))
	fmt.Fprintf(a.stdout, "frpc path: %s\n", blankIfEmpty(state.FRPCPath, "(not installed)"))
	fmt.Fprintf(a.stdout, "config path: %s\n", blankIfEmpty(state.ConfigPath, a.store.ConfigPath()))
	fmt.Fprintf(a.stdout, "running: %t\n", state.Running)
	if state.PID != 0 {
		fmt.Fprintf(a.stdout, "pid: %d\n", state.PID)
	}
	if state.LastStartTime != nil {
		fmt.Fprintf(a.stdout, "last start: %s\n", state.LastStartTime.Format(time.RFC3339))
	}
	if state.LastStopTime != nil {
		fmt.Fprintf(a.stdout, "last stop: %s\n", state.LastStopTime.Format(time.RFC3339))
	}
	if state.LastExitTime != nil {
		fmt.Fprintf(a.stdout, "last exit: %s\n", state.LastExitTime.Format(time.RFC3339))
	}
	if state.LastError != "" {
		fmt.Fprintf(a.stdout, "last error: %s\n", state.LastError)
	}

	if manifestErr == nil {
		a.printEndpointTable(endpointRows(manifest, state))
	} else {
		fmt.Fprintln(a.stdout, "No manifest imported yet.")
	}
	return nil
}

func (a *App) Endpoints(ctx context.Context) error {
	_ = ctx

	manifest, err := a.store.ReadManifest()
	if err != nil {
		return err
	}
	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	a.printEndpointTable(endpointRows(manifest, state))
	return nil
}

func (a *App) Purge(ctx context.Context, withBin bool) error {
	if err := a.Stop(ctx); err != nil {
		return err
	}

	paths := []string{
		a.store.ManifestPath(),
		a.store.ConfigPath(),
		a.store.RuntimePath(),
		a.store.LogPath(),
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	if withBin {
		if err := os.RemoveAll(filepath.Join(a.store.Layout.BinRoot, "frpc")); err != nil {
			return fmt.Errorf("remove installed frpc binaries: %w", err)
		}
	}
	fmt.Fprintln(a.stdout, "Local configuration purged.")
	return nil
}

func (a *App) ServiceList(ctx context.Context) error {
	_ = ctx

	manifest, err := a.store.ReadManifest()
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(a.stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SERVICE KEY\tNAME\tSTATUS\tPORT\tPROTOCOL")
	for _, svc := range manifest.SortedServices() {
		status := "enabled"
		if svc.Disabled {
			status = "disabled"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", svc.Key(manifest.User), svc.Name, status, svc.BindPort, blankIfEmpty(svc.NormalizedProtocol(), "-"))
	}
	return tw.Flush()
}

func (a *App) SetServiceDisabled(ctx context.Context, serviceKey string, disabled bool) error {
	_ = ctx

	manifest, err := a.store.ReadManifest()
	if err != nil {
		return err
	}

	found := false
	for idx, svc := range manifest.Services {
		if svc.Key(manifest.User) != serviceKey {
			continue
		}
		manifest.Services[idx].Disabled = disabled
		found = true
		break
	}
	if !found {
		return fmt.Errorf("service %s not found", serviceKey)
	}

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	if err := a.persistManifestState(manifest, &state); err != nil {
		return err
	}

	status := "enabled"
	if disabled {
		status = "disabled"
	}
	fmt.Fprintf(a.stdout, "Service %s %s.\n", serviceKey, status)
	return nil
}

func (a *App) RemoveService(ctx context.Context, serviceKey string) error {
	_ = ctx

	manifest, err := a.store.ReadManifest()
	if err != nil {
		return err
	}

	filtered := manifest.Services[:0]
	found := false
	for _, svc := range manifest.Services {
		if svc.Key(manifest.User) == serviceKey {
			found = true
			continue
		}
		filtered = append(filtered, svc)
	}
	if !found {
		return fmt.Errorf("service %s not found", serviceKey)
	}
	manifest.Services = filtered

	state, err := a.store.ReadRuntime()
	if err != nil {
		return err
	}
	if err := a.persistManifestState(manifest, &state); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "Service %s removed.\n", serviceKey)
	return nil
}

func (a *App) ensureInstalled(ctx context.Context, version, archivePath, baseURL string) (string, error) {
	version = normalizeVersion(version)
	path := a.store.FRPCPath(version)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return a.installer.Install(ctx, version, archivePath, baseURL)
}

func (a *App) selectedVersion(state model.RuntimeState) string {
	if strings.TrimSpace(state.SelectedVersion) != "" {
		return normalizeVersion(state.SelectedVersion)
	}
	return model.DefaultFRPCVersion
}

func (a *App) persistManifestState(manifest model.Manifest, state *model.RuntimeState) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	if err := a.store.WriteManifest(manifest); err != nil {
		return err
	}
	if err := a.store.WriteConfig(model.RenderTOML(manifest)); err != nil {
		return err
	}

	state.ConfigPath = a.store.ConfigPath()
	state.ManifestPath = a.store.ManifestPath()
	state.Services = buildServiceRuntimeMap(manifest, state.Services)
	if !state.Running {
		for _, svc := range manifest.SortedServices() {
			status := "stopped"
			msg := "ready to start"
			if svc.Disabled {
				status = "disabled"
				msg = "service disabled"
			}
			updateServiceState(state.Services, manifest, svc, status, msg)
		}
	}
	state.LastError = model.Sanitize(state.LastError, manifest.Secrets())
	return a.store.WriteRuntime(*state)
}

func (a *App) probeServices(manifest model.Manifest, state *model.RuntimeState, writer *frpc.RedactingWriter) error {
	deadline := time.Now().Add(8 * time.Second)
	for _, svc := range manifest.SortedServices() {
		if svc.Disabled {
			updateServiceState(state.Services, manifest, svc, "disabled", "service disabled")
			continue
		}

		address := fmt.Sprintf("%s:%d", svc.BindAddress(), svc.BindPort)
		for {
			conn, err := net.DialTimeout("tcp", address, 250*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				time.Sleep(250 * time.Millisecond)
				recent := writer.RecentText()
				if mapped, actionable := frpc.InterpretRuntimeError(recent); actionable {
					updateServiceState(state.Services, manifest, svc, "error", mapped)
					_ = a.store.WriteRuntime(*state)
					return fmt.Errorf("%s", mapped)
				}
				updateServiceState(state.Services, manifest, svc, "listening", "listener ready")
				if err := a.store.WriteRuntime(*state); err != nil {
					return err
				}
				break
			}

			if state.PID != 0 && !frpc.ProcessRunning(state.PID) {
				recent := writer.RecentText()
				mapped, actionable := frpc.InterpretRuntimeError(recent)
				if !actionable {
					mapped = ""
				}
				if mapped == "" {
					mapped = fmt.Sprintf("service %s failed to start local listener", svc.Key(manifest.User))
				}
				updateServiceState(state.Services, manifest, svc, "error", mapped)
				_ = a.store.WriteRuntime(*state)
				return fmt.Errorf("%s", mapped)
			}

			if time.Now().After(deadline) {
				message := frpc.MapRuntimeError(writer.RecentText())
				if message == "" {
					message = fmt.Sprintf("service %s did not open local listener on %s", svc.Key(manifest.User), address)
				}
				updateServiceState(state.Services, manifest, svc, "error", message)
				_ = a.store.WriteRuntime(*state)
				return fmt.Errorf("%s", message)
			}

			time.Sleep(200 * time.Millisecond)
		}
	}
	return nil
}

func (a *App) finishExitedProcess(state model.RuntimeState, message string, returnError bool) error {
	now := a.now()
	state.Running = false
	state.PID = 0
	state.LastExitTime = &now
	if !returnError {
		state.LastStopTime = &now
	}
	if message != "" {
		state.LastError = message
	}
	for key, svc := range state.Services {
		if svc.Status != "disabled" {
			svc.Status = "stopped"
			svc.Message = blankIfEmpty(message, "frpc stopped")
			svc.UpdatedAt = &now
			state.Services[key] = svc
		}
	}
	if err := a.store.WriteRuntime(state); err != nil {
		return err
	}
	if returnError && message != "" {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func (a *App) printEndpointTable(rows []model.EndpointRow) {
	tw := tabwriter.NewWriter(a.stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SERVICE KEY\tNAME\tADDRESS\tPROTOCOL\tSTATUS\tACCESS")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s:%d\t%s\t%s\t%s\n",
			row.ServiceKey,
			row.Name,
			row.BindAddr,
			row.BindPort,
			blankIfEmpty(row.Protocol, "-"),
			blankIfEmpty(row.Status, "unknown"),
			row.Command,
		)
	}
	_ = tw.Flush()
}

func loadManifestFile(path string) (model.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("open manifest %s: %w", path, err)
	}
	defer f.Close()

	var manifest model.Manifest
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return model.Manifest{}, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	return manifest, nil
}

func mergeManifest(existing, incoming model.Manifest) model.Manifest {
	merged := existing
	if strings.TrimSpace(incoming.ServerAddr) != "" {
		merged.ServerAddr = incoming.ServerAddr
	}
	if incoming.ServerPort != 0 {
		merged.ServerPort = incoming.ServerPort
	}
	if strings.TrimSpace(incoming.AuthToken) != "" {
		merged.AuthToken = incoming.AuthToken
	}
	if strings.TrimSpace(incoming.User) != "" {
		merged.User = incoming.User
	}

	incomingByKey := map[string]model.Service{}
	for _, svc := range incoming.Services {
		incomingByKey[svc.Key(merged.User)] = svc
	}

	services := make([]model.Service, 0, len(existing.Services)+len(incoming.Services))
	for _, svc := range existing.Services {
		key := svc.Key(merged.User)
		if replacement, ok := incomingByKey[key]; ok {
			services = append(services, replacement)
			delete(incomingByKey, key)
			continue
		}
		services = append(services, svc)
	}

	leftovers := make([]model.Service, 0, len(incomingByKey))
	for _, svc := range incomingByKey {
		leftovers = append(leftovers, svc)
	}
	slices.SortFunc(leftovers, func(a, b model.Service) int {
		return strings.Compare(a.Key(merged.User), b.Key(merged.User))
	})
	services = append(services, leftovers...)
	merged.Services = services
	return merged
}

func buildServiceRuntimeMap(manifest model.Manifest, existing map[string]model.ServiceRuntime) map[string]model.ServiceRuntime {
	result := map[string]model.ServiceRuntime{}
	for key, svc := range existing {
		result[key] = svc
	}

	for _, svc := range manifest.SortedServices() {
		key := svc.Key(manifest.User)
		current := result[key]
		current.ServiceKey = key
		current.Name = svc.Name
		current.BindAddr = svc.BindAddress()
		current.BindPort = svc.BindPort
		current.Protocol = svc.NormalizedProtocol()
		current.Endpoint = svc.AccessCommand()
		if current.Status == "" {
			if svc.Disabled {
				current.Status = "disabled"
				current.Message = "service disabled"
			} else {
				current.Status = "stopped"
				current.Message = "ready to start"
			}
		}
		result[key] = current
	}

	for key := range result {
		if !hasService(manifest, key) {
			delete(result, key)
		}
	}
	return result
}

func markServices(serviceStates map[string]model.ServiceRuntime, manifest model.Manifest, status, message string) {
	for _, svc := range manifest.SortedServices() {
		if svc.Disabled {
			updateServiceState(serviceStates, manifest, svc, "disabled", "service disabled")
			continue
		}
		updateServiceState(serviceStates, manifest, svc, status, message)
	}
}

func updateServiceState(states map[string]model.ServiceRuntime, manifest model.Manifest, svc model.Service, status, message string) {
	key := svc.Key(manifest.User)
	current := states[key]
	now := time.Now()
	current.ServiceKey = key
	current.Name = svc.Name
	current.BindAddr = svc.BindAddress()
	current.BindPort = svc.BindPort
	current.Protocol = svc.NormalizedProtocol()
	current.Endpoint = svc.AccessCommand()
	current.Status = status
	current.Message = message
	current.UpdatedAt = &now
	states[key] = current
}

func endpointRows(manifest model.Manifest, state model.RuntimeState) []model.EndpointRow {
	rows := make([]model.EndpointRow, 0, len(manifest.Services))
	for _, svc := range manifest.SortedServices() {
		key := svc.Key(manifest.User)
		status := "stopped"
		if svc.Disabled {
			status = "disabled"
		}
		if runtimeSvc, ok := state.Services[key]; ok && runtimeSvc.Status != "" {
			status = runtimeSvc.Status
		}
		rows = append(rows, model.EndpointRow{
			ServiceKey: key,
			Name:       svc.Name,
			BindAddr:   svc.BindAddress(),
			BindPort:   svc.BindPort,
			Protocol:   svc.NormalizedProtocol(),
			Status:     status,
			Command:    svc.AccessCommand(),
		})
	}
	return rows
}

func hasService(manifest model.Manifest, serviceKey string) bool {
	for _, svc := range manifest.Services {
		if svc.Key(manifest.User) == serviceKey {
			return true
		}
	}
	return false
}

func preflightPorts(services []model.Service) error {
	for _, svc := range services {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", svc.BindAddress(), svc.BindPort))
		if err != nil {
			return fmt.Errorf("bindPort %d is unavailable on %s: %w", svc.BindPort, svc.BindAddress(), err)
		}
		_ = ln.Close()
	}
	return nil
}

func blankIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return model.DefaultFRPCVersion
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func isExpectedStop(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "signal: killed") || strings.Contains(text, "signal: interrupt")
}
