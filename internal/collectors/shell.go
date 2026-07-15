package collectors

import (
	"os"
	"os/exec"
)

func ProjectShell(path string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = path
	return cmd
}
