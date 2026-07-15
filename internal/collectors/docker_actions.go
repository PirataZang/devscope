package collectors

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/scanner"
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

// RefreshProjectsDocker re-links containers to projects after an action.
func RefreshProjectsDocker(store *core.StateStore) {
	ctx := context.Background()
	snap := store.Get()
	if len(snap.Projects) == 0 {
		return
	}
	projects := make([]core.Project, len(snap.Projects))
	copy(projects, snap.Projects)

	containers, meta, err := CollectDocker(ctx)
	if err != nil {
		return
	}
	AssignContainersToProjects(projects, containers, meta)
	stats := CollectDockerStats(ctx)
	ApplyDockerStats(projects, stats)
	pm2Apps := CollectPM2(ctx)
	AssignWorkersToProjects(projects, pm2Apps)
	AssignPortsToProjects(projects, ReadListeningPorts())
	for i := range projects {
		p := &projects[i]
		pm2Roots := scanner.DiscoverRunningRoots(ctx)
		switch {
		case ProjectRunning(p.Containers):
			p.Status = core.StatusRunning
		case PM2ProjectRunning(p.Workers) || pm2Roots[p.Path]:
			p.Status = core.StatusRunning
		case p.HasDockerCompose || p.HasDockerfile:
			p.Status = core.StatusStopped
		}
	}
	store.SetProjects(projects)
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
