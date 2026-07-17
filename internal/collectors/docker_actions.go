package collectors

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

func DockerPause(id string) error {
	return dockerAction(id, "pause")
}

func DockerUnpause(id string) error {
	return dockerAction(id, "unpause")
}

func DockerStart(id string) error {
	return dockerAction(id, "start")
}

func DockerStop(id string) error {
	return dockerAction(id, "stop")
}

func DockerRestart(id string) error {
	return dockerAction(id, "restart")
}

func DockerRemove(id string) error {
	return dockerAction(id, "rm", "-f")
}

func DockerLogs(id string, tail int) (string, error) {
	if tail <= 0 {
		tail = 300
	}
	out, err := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", tail), id).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return string(out), err
		}
		return "", err
	}
	return string(out), nil
}

// DockerLogsSince returns logs since N seconds ago (for follow polling).
func DockerLogsSince(id string, sinceSec int, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}
	args := []string{"logs", "--tail", fmt.Sprintf("%d", tail), "--since", fmt.Sprintf("%ds", sinceSec), id}
	out, err := exec.Command("docker", args...).CombinedOutput()
	return string(out), err
}

// DockerExecTarget prefers container name (docker accepts it) over short ID.
func DockerExecTarget(c core.Container) string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID
}

func DockerExecShell(target string) *exec.Cmd {
	// lazydocker: login shell from /etc/passwd
	script := `eval $(grep ^$(id -un): /etc/passwd | cut -d : -f 7-)`
	return exec.Command("docker", "exec", "-it", target, "/bin/sh", "-c", script)
}

func DockerExecShellBash(target string) *exec.Cmd {
	return exec.Command("docker", "exec", "-it", target, "/bin/bash")
}

func DockerExecShellFallback(target string) *exec.Cmd {
	return exec.Command("docker", "exec", "-it", target, "/bin/sh")
}

func dockerAction(id string, args ...string) error {
	cmd := append([]string{args[0], id}, args[1:]...)
	return exec.Command("docker", cmd...).Run()
}

// RefreshProjectsDocker re-links containers to a project after an action.
func RefreshProjectsDocker(store *core.StateStore, projectPath string, healthCfg config.HealthConfig) {
	RefreshProjectDocker(store, projectPath, healthCfg)
}

func IsContainerRunning(c core.Container) bool {
	return strings.EqualFold(c.Status, "running")
}

func IsContainerStopped(c core.Container) bool {
	switch strings.ToLower(c.Status) {
	case "exited", "stopped", "created", "dead":
		return true
	default:
		return false
	}
}
