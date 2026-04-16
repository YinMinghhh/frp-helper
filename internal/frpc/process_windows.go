//go:build windows

package frpc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	text := strings.TrimSpace(string(output))
	if text == "" || strings.Contains(text, "No tasks are running") {
		return false
	}
	return strings.Contains(text, fmt.Sprintf(",\"%d\",", pid))
}

func KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
