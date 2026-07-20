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

// ProjectOpenCode launches OpenCode with cwd set to the project path.
func ProjectOpenCode(path string) (*exec.Cmd, error) {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin)
	cmd.Dir = path
	return cmd, nil
}
