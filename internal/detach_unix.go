//go:build !windows

package internal

import (
	"os/exec"
	"syscall"
)

// detach starts cmd in its own session, so it outlives initializeCommand
// and isn't stopped with its process group.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
