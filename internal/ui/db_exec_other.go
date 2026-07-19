//go:build !windows

package ui

import "os/exec"

func configureDBCommand(cmd *exec.Cmd) {}
