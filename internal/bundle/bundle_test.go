package bundle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteSupportFiles(t *testing.T) {
	outputDir := t.TempDir()

	if err := WriteSupportFiles(Options{
		OutputDir:      outputDir,
		ExecutableName: executableNameForTest(),
		EnableStartup:  true,
	}); err != nil {
		t.Fatalf("WriteSupportFiles: %v", err)
	}

	startScript := filepath.Join(outputDir, startScriptNameForTest())
	data, err := os.ReadFile(startScript)
	if err != nil {
		t.Fatalf("ReadFile start script: %v", err)
	}
	if !strings.Contains(string(data), "run --enable-startup") {
		t.Fatalf("expected startup flag in start script, got %q", string(data))
	}

	if _, err := os.Stat(filepath.Join(outputDir, "README-bundle.txt")); err != nil {
		t.Fatalf("expected bundle readme: %v", err)
	}
}

func executableNameForTest() string {
	if runtime.GOOS == "windows" {
		return "frp-helper.exe"
	}
	return "frp-helper"
}

func startScriptNameForTest() string {
	switch runtime.GOOS {
	case "windows":
		return "start.bat"
	case "darwin":
		return "start.command"
	default:
		return "start.sh"
	}
}
