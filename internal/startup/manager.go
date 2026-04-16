package startup

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const label = "io.github.yinming.frp-helper"

type Manager struct {
	goos          string
	userHomeDir   func() (string, error)
	userConfigDir func() (string, error)
	getenv        func(string) string
}

func NewManager() *Manager {
	return &Manager{
		goos:          runtime.GOOS,
		userHomeDir:   os.UserHomeDir,
		userConfigDir: os.UserConfigDir,
		getenv:        os.Getenv,
	}
}

type Status struct {
	Enabled  bool
	Location string
}

type EnableOptions struct {
	ExecutablePath string
	HomeDir        string
	LogsDir        string
}

func (m *Manager) Enable(opts EnableOptions) (string, error) {
	location, err := m.location()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(location), 0o700); err != nil {
		return "", fmt.Errorf("create startup directory: %w", err)
	}

	content, perm, err := m.render(opts)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(location, []byte(content), perm); err != nil {
		return "", fmt.Errorf("write startup item %s: %w", location, err)
	}
	return location, nil
}

func (m *Manager) Disable() (string, error) {
	location, err := m.location()
	if err != nil {
		return "", err
	}
	if err := os.Remove(location); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove startup item %s: %w", location, err)
	}
	return location, nil
}

func (m *Manager) GetStatus() (Status, error) {
	location, err := m.location()
	if err != nil {
		return Status{}, err
	}
	info, err := os.Stat(location)
	if err == nil && !info.IsDir() {
		return Status{Enabled: true, Location: location}, nil
	}
	if os.IsNotExist(err) {
		return Status{Enabled: false, Location: location}, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("stat startup item %s: %w", location, err)
	}
	return Status{Enabled: false, Location: location}, nil
}

func (m *Manager) location() (string, error) {
	switch m.goos {
	case "darwin":
		home, err := m.userHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
	case "linux":
		configDir, err := m.userConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve user config directory: %w", err)
		}
		return filepath.Join(configDir, "autostart", "frp-helper.desktop"), nil
	case "windows":
		appData := strings.TrimSpace(m.getenv("APPDATA"))
		if appData == "" {
			return "", fmt.Errorf("APPDATA is not set")
		}
		return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "frp-helper-start.cmd"), nil
	default:
		return "", fmt.Errorf("startup integration is not supported on %s", m.goos)
	}
}

func (m *Manager) render(opts EnableOptions) (string, os.FileMode, error) {
	executablePath, err := filepath.Abs(opts.ExecutablePath)
	if err != nil {
		return "", 0, fmt.Errorf("resolve executable path: %w", err)
	}
	homeDir, err := filepath.Abs(opts.HomeDir)
	if err != nil {
		return "", 0, fmt.Errorf("resolve home directory: %w", err)
	}
	logsDir, err := filepath.Abs(opts.LogsDir)
	if err != nil {
		return "", 0, fmt.Errorf("resolve logs directory: %w", err)
	}

	switch m.goos {
	case "darwin":
		return renderLaunchAgent(executablePath, homeDir, logsDir), 0o644, nil
	case "linux":
		return renderDesktopEntry(executablePath, homeDir), 0o644, nil
	case "windows":
		return renderStartupCmd(executablePath, homeDir), 0o644, nil
	default:
		return "", 0, fmt.Errorf("startup integration is not supported on %s", m.goos)
	}
}

func renderLaunchAgent(executablePath, homeDir, logsDir string) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString("<dict>\n")
	b.WriteString("  <key>Label</key>\n")
	b.WriteString("  <string>" + label + "</string>\n")
	b.WriteString("  <key>ProgramArguments</key>\n")
	b.WriteString("  <array>\n")
	b.WriteString("    <string>" + xmlEscape(executablePath) + "</string>\n")
	b.WriteString("    <string>run</string>\n")
	b.WriteString("  </array>\n")
	b.WriteString("  <key>RunAtLoad</key>\n")
	b.WriteString("  <true/>\n")
	b.WriteString("  <key>EnvironmentVariables</key>\n")
	b.WriteString("  <dict>\n")
	b.WriteString("    <key>FRP_HELPER_HOME</key>\n")
	b.WriteString("    <string>" + xmlEscape(homeDir) + "</string>\n")
	b.WriteString("  </dict>\n")
	b.WriteString("  <key>StandardOutPath</key>\n")
	b.WriteString("  <string>" + xmlEscape(filepath.Join(logsDir, "startup-stdout.log")) + "</string>\n")
	b.WriteString("  <key>StandardErrorPath</key>\n")
	b.WriteString("  <string>" + xmlEscape(filepath.Join(logsDir, "startup-stderr.log")) + "</string>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func renderDesktopEntry(executablePath, homeDir string) string {
	cmd := fmt.Sprintf("/bin/sh -lc %s", shellQuote(fmt.Sprintf("export FRP_HELPER_HOME=%s; %s run", shellQuote(homeDir), shellQuote(executablePath))))
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=FRP Helper",
		"Comment=Start FRP Helper on login",
		"Exec=" + cmd,
		"Terminal=false",
		"X-GNOME-Autostart-enabled=true",
		"",
	}, "\n")
}

func renderStartupCmd(executablePath, homeDir string) string {
	return strings.Join([]string{
		"@echo off",
		fmt.Sprintf("set \"FRP_HELPER_HOME=%s\"", homeDir),
		fmt.Sprintf("start \"\" /min \"%s\" run", executablePath),
		"",
	}, "\r\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
