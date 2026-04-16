//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

func setDetachedProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
