package startup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestManagerEnableDisableStatus(t *testing.T) {
	tempHome := t.TempDir()
	tempConfig := filepath.Join(tempHome, ".config")
	tempAppData := filepath.Join(tempHome, "AppData", "Roaming")

	manager := &Manager{
		goos: runtime.GOOS,
		userHomeDir: func() (string, error) {
			return tempHome, nil
		},
		userConfigDir: func() (string, error) {
			return tempConfig, nil
		},
		getenv: func(key string) string {
			if key == "APPDATA" {
				return tempAppData
			}
			return ""
		},
	}

	location, err := manager.Enable(EnableOptions{
		ExecutablePath: filepath.Join(tempHome, "frp-helper"),
		HomeDir:        filepath.Join(tempHome, ".frp-helper"),
		LogsDir:        filepath.Join(tempHome, ".frp-helper", "logs"),
	})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if _, err := os.Stat(location); err != nil {
		t.Fatalf("startup item not written: %v", err)
	}

	status, err := manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus enabled: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected startup enabled")
	}

	if _, err := manager.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	status, err = manager.GetStatus()
	if err != nil {
		t.Fatalf("GetStatus disabled: %v", err)
	}
	if status.Enabled {
		t.Fatalf("expected startup disabled")
	}
}
