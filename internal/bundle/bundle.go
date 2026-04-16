package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Options struct {
	OutputDir      string
	ExecutableName string
	EnableStartup  bool
}

func WriteSupportFiles(opts Options) error {
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create bundle directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return writeWindowsFiles(opts)
	case "darwin":
		return writeUnixFiles(opts, "start.command", "stop.command")
	default:
		return writeUnixFiles(opts, "start.sh", "stop.sh")
	}
}

func writeUnixFiles(opts Options, startName, stopName string) error {
	startArgs := "run"
	if opts.EnableStartup {
		startArgs += " --enable-startup"
	}

	startContent := strings.Join([]string{
		"#!/bin/sh",
		`DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"`,
		`export FRP_HELPER_HOME="$DIR/.frp-helper"`,
		fmt.Sprintf(`"$DIR/%s" %s`, opts.ExecutableName, startArgs),
		"",
	}, "\n")

	stopContent := strings.Join([]string{
		"#!/bin/sh",
		`DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"`,
		`export FRP_HELPER_HOME="$DIR/.frp-helper"`,
		fmt.Sprintf(`"$DIR/%s" stop`, opts.ExecutableName),
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(opts.OutputDir, startName), []byte(startContent), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", startName, err)
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, stopName), []byte(stopContent), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", stopName, err)
	}

	return writeReadme(opts, startName, stopName)
}

func writeWindowsFiles(opts Options) error {
	startArgs := "run"
	if opts.EnableStartup {
		startArgs += " --enable-startup"
	}

	startContent := strings.Join([]string{
		"@echo off",
		"set \"DIR=%~dp0\"",
		`set "FRP_HELPER_HOME=%DIR%.frp-helper"`,
		fmt.Sprintf(`"%%DIR%%%s" %s`, opts.ExecutableName, startArgs),
		"",
	}, "\r\n")

	stopContent := strings.Join([]string{
		"@echo off",
		"set \"DIR=%~dp0\"",
		`set "FRP_HELPER_HOME=%DIR%.frp-helper"`,
		fmt.Sprintf(`"%%DIR%%%s" stop`, opts.ExecutableName),
		"",
	}, "\r\n")

	if err := os.WriteFile(filepath.Join(opts.OutputDir, "start.bat"), []byte(startContent), 0o755); err != nil {
		return fmt.Errorf("write start.bat: %w", err)
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "stop.bat"), []byte(stopContent), 0o755); err != nil {
		return fmt.Errorf("write stop.bat: %w", err)
	}

	return writeReadme(opts, "start.bat", "stop.bat")
}

func writeReadme(opts Options, startScript, stopScript string) error {
	startupNote := "enabled on first run"
	if !opts.EnableStartup {
		startupNote = "disabled by default"
	}
	content := strings.Join([]string{
		"FRP Helper Bundle",
		"",
		"Files:",
		"- access.json: bundled FRP manifest",
		"- " + opts.ExecutableName + ": helper executable",
		"- " + startScript + ": start now (" + startupNote + ")",
		"- " + stopScript + ": stop current frpc process",
		"",
		"Usage:",
		"1. Double-click the start script.",
		"2. It starts frpc in the background and returns immediately.",
		"3. Use the printed local access command or run the helper with status/endpoints.",
		"4. Double-click the stop script when needed.",
		"",
	}, "\n")

	return os.WriteFile(filepath.Join(opts.OutputDir, "README-bundle.txt"), []byte(content), 0o644)
}
