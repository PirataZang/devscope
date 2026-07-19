//go:build windows

package ui

import (
	"os/exec"
	"syscall"
)

// configureDBCommand detaches the child process from the TUI console so
// CreateProcess does not fail with "fork/exec ... docker.exe: invalid argument"
// while bubbletea has the console in raw mode.
func configureDBCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
