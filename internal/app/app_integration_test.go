package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"frp-helper/internal/model"
)

var (
	stubBuildOnce sync.Once
	stubBinary    string
	stubBuildErr  error
)

func TestApplyMergeAndServiceMutations(t *testing.T) {
	app, stdout, _ := newTestApp(t)
	installStubFRPC(t, app)

	initial := writeManifest(t, manifestFixture("frps.example.com", []model.Service{
		{Name: "ssh-1", ServerName: "ssh_1", SecretKey: "secret-1", BindPort: 6000, ProtocolHint: "ssh"},
	}))
	if err := app.Apply(context.Background(), initial, ApplyReplace); err != nil {
		t.Fatalf("Apply replace: %v", err)
	}

	mergePath := writeManifest(t, manifestFixture("frps.example.com", []model.Service{
		{Name: "ssh-2", ServerName: "ssh_2", SecretKey: "secret-2", BindPort: 6001, ProtocolHint: "ssh"},
	}))
	if err := app.Apply(context.Background(), mergePath, ApplyMerge); err != nil {
		t.Fatalf("Apply merge: %v", err)
	}

	manifest, err := app.store.ReadManifest()
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if got, want := len(manifest.Services), 2; got != want {
		t.Fatalf("got %d services want %d", got, want)
	}

	if err := app.SetServiceDisabled(context.Background(), "ops:ssh_2", true); err != nil {
		t.Fatalf("SetServiceDisabled: %v", err)
	}
	if err := app.RemoveService(context.Background(), "ops:ssh_1"); err != nil {
		t.Fatalf("RemoveService: %v", err)
	}

	rendered, err := os.ReadFile(app.store.ConfigPath())
	if err != nil {
		t.Fatalf("ReadFile config: %v", err)
	}
	text := string(rendered)
	if strings.Contains(text, "ssh_1") {
		t.Fatalf("removed service still in config: %s", text)
	}
	if strings.Contains(text, "ssh_2") {
		t.Fatalf("disabled service should not be rendered: %s", text)
	}
	if !strings.Contains(stdout.String(), "Applied 1 service(s).") {
		t.Fatalf("expected apply output, got %q", stdout.String())
	}
}

func TestStartStatusAndStop(t *testing.T) {
	app, stdout, _ := newTestApp(t)
	installStubFRPC(t, app)

	manifestPath := writeManifest(t, manifestFixture("frps.example.com", []model.Service{
		{Name: "ssh-1", ServerName: "ssh_1", SecretKey: "secret-1", BindPort: 6100, ProtocolHint: "ssh", AccessUser: "alice"},
		{Name: "web-1", ServerName: "web_1", SecretKey: "secret-2", BindPort: 6101, ProtocolHint: "http"},
	}))
	if err := app.Apply(context.Background(), manifestPath, ApplyReplace); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Start(context.Background())
	}()

	waitFor(t, 5*time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:6100", 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	})

	if err := app.Status(context.Background()); err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(stdout.String(), "running: true") {
		t.Fatalf("expected running status, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "ssh -p 6100 alice@127.0.0.1") {
		t.Fatalf("expected ssh endpoint in output, got %q", stdout.String())
	}

	if err := app.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned error after stop: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for start to exit")
	}

	state, err := app.store.ReadRuntime()
	if err != nil {
		t.Fatalf("ReadRuntime: %v", err)
	}
	if state.Running {
		t.Fatalf("expected runtime to be stopped: %+v", state)
	}
}

func TestStartVerifyFailure(t *testing.T) {
	app, _, _ := newTestApp(t)
	installStubFRPC(t, app)

	manifestPath := writeManifest(t, manifestFixture("verify-fail.local", []model.Service{
		{Name: "ssh-1", ServerName: "ssh_1", SecretKey: "secret-1", BindPort: 6200, ProtocolHint: "ssh"},
	}))
	if err := app.Apply(context.Background(), manifestPath, ApplyReplace); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("expected start failure")
	}
	if !strings.Contains(err.Error(), "frpc configuration is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartAuthFailureMapsError(t *testing.T) {
	app, _, _ := newTestApp(t)
	installStubFRPC(t, app)

	manifestPath := writeManifest(t, manifestFixture("auth-fail.local", []model.Service{
		{Name: "ssh-1", ServerName: "ssh_1", SecretKey: "secret-1", BindPort: 6300, ProtocolHint: "ssh"},
	}))
	if err := app.Apply(context.Background(), manifestPath, ApplyReplace); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	err := app.Start(context.Background())
	if err == nil {
		t.Fatal("expected start failure")
	}
	if !strings.Contains(err.Error(), "auth.token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("FRP_HELPER_HOME", home)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app, err := New(stdout, stderr)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app, stdout, stderr
}

func installStubFRPC(t *testing.T, app *App) {
	t.Helper()

	binaryPath := buildStubFRPC(t)
	destPath := app.store.FRPCPath(model.DefaultFRPCVersion)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := os.WriteFile(destPath, data, 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func buildStubFRPC(t *testing.T) string {
	t.Helper()

	stubBuildOnce.Do(func() {
		root := repoRoot(t)
		tempRoot, err := os.MkdirTemp("", "frpc-stub-*")
		if err != nil {
			stubBuildErr = fmt.Errorf("create temp dir for stub frpc: %w", err)
			return
		}
		output := filepath.Join(tempRoot, "frpc-stub")
		if runtime.GOOS == "windows" {
			output += ".exe"
		}

		cmd := exec.Command("go", "build", "-o", output, "./testdata/frpcstub")
		cmd.Dir = root
		cmd.Env = os.Environ()
		buildOutput, err := cmd.CombinedOutput()
		if err != nil {
			stubBuildErr = fmt.Errorf("build stub frpc: %w: %s", err, string(buildOutput))
			return
		}
		stubBinary = output
	})

	if stubBuildErr != nil {
		t.Fatal(stubBuildErr)
	}
	return stubBinary
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func manifestFixture(serverAddr string, services []model.Service) model.Manifest {
	return model.Manifest{
		ServerAddr: serverAddr,
		ServerPort: 7000,
		AuthToken:  "token-123",
		User:       "ops",
		Services:   services,
	}
}

func writeManifest(t *testing.T, manifest model.Manifest) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "access.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
