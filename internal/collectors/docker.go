package collectors

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

type containerMeta struct {
	ComposeProject string
	WorkingDir     string
	ConfigFiles    string
	Mounts         []string
	Health         string
}

func (m containerMeta) composeRoot() string {
	if m.WorkingDir != "" {
		return filepath.Clean(m.WorkingDir)
	}
	if m.ConfigFiles != "" {
		for _, f := range strings.Split(m.ConfigFiles, ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				return filepath.Clean(filepath.Dir(f))
			}
		}
	}
	return ""
}

func CollectDocker(ctx context.Context) ([]core.Container, map[string]containerMeta, error) {
	containers, meta, err := CollectDockerPS(ctx)
	if err != nil || len(containers) == 0 {
		return containers, meta, err
	}
	// Full inspect adds mount paths for better project matching (CLI / deep scan).
	if full := inspectContainerMeta(ctx); len(full) > 0 {
		meta = full
		for i := range containers {
			m := lookupMeta(containers[i].ID, meta)
			containers[i].ProjectPath = m.composeRoot()
			if m.Health != "" {
				containers[i].Health = m.Health
			}
		}
	}
	return containers, meta, nil
}

// dockerPSFormat uses tabs — JSON templates break docker's Label quoting.
const dockerPSFormat = "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}\t{{.Ports}}\t{{.Label \"com.docker.compose.project\"}}\t{{.Label \"com.docker.compose.project.working_dir\"}}\t{{.Label \"com.docker.compose.project.config_files\"}}"

// CollectDockerPS lists containers via docker ps only (no inspect) — fast path for the dashboard.
func CollectDockerPS(ctx context.Context) ([]core.Container, map[string]containerMeta, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, nil, nil
	}

	out, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", dockerPSFormat).Output()
	if err != nil {
		return nil, nil, err
	}

	meta := make(map[string]containerMeta)
	var containers []core.Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		c, m, ok := parseDockerPSLine(line)
		if !ok {
			continue
		}
		meta[c.ID] = m
		containers = append(containers, c)
	}
	return containers, meta, nil
}

func parseDockerPSLine(line string) (core.Container, containerMeta, bool) {
	parts := strings.Split(line, "\t")
	if len(parts) < 6 {
		return core.Container{}, containerMeta{}, false
	}
	id := parts[0]
	if len(id) > 12 {
		id = id[:12]
	}
	m := containerMeta{}
	if len(parts) > 6 {
		m.ComposeProject = parts[6]
	}
	if len(parts) > 7 {
		m.WorkingDir = parts[7]
	}
	if len(parts) > 8 {
		m.ConfigFiles = parts[8]
	}
	name := strings.TrimPrefix(parts[1], "/")
	return core.Container{
		ID:          id,
		Name:        name,
		Image:       parts[2],
		Status:      strings.ToLower(parts[3]),
		State:       parts[3],
		Health:      parseHealthFromStatus(parts[4]),
		Ports:       parts[5],
		ProjectPath: m.composeRoot(),
	}, m, true
}

func inspectContainerMeta(ctx context.Context) map[string]containerMeta {
	out, err := exec.CommandContext(ctx, "docker", "ps", "-aq").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}
	ids := strings.Fields(string(out))
	args := append([]string{"inspect", "-f",
		"{{.Id}}\t{{index .Config.Labels \"com.docker.compose.project\"}}\t{{index .Config.Labels \"com.docker.compose.project.working_dir\"}}\t{{index .Config.Labels \"com.docker.compose.project.config_files\"}}\t{{range .Mounts}}{{.Source}};{{end}}",
	}, ids...)
	out, err = exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil
	}

	result := make(map[string]containerMeta)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 2 {
			continue
		}
		id := parts[0]
		shortID := id
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		m := containerMeta{ComposeProject: parts[1]}
		if len(parts) > 2 {
			m.WorkingDir = parts[2]
		}
		if len(parts) > 3 {
			m.ConfigFiles = parts[3]
		}
		if len(parts) > 4 {
			for _, mount := range strings.Split(parts[4], ";") {
				mount = strings.TrimSpace(mount)
				if mount != "" {
					m.Mounts = append(m.Mounts, mount)
				}
			}
		}
		result[id] = m
		result[shortID] = m
	}
	return result
}

func lookupMeta(id string, meta map[string]containerMeta) containerMeta {
	if m, ok := meta[id]; ok {
		return m
	}
	for k, v := range meta {
		if strings.HasPrefix(k, id) || strings.HasPrefix(id, k) {
			return v
		}
	}
	return containerMeta{}
}

// AssignContainersToProjects links each container to at most one project (best match).
func AssignContainersToProjects(projects []core.Project, containers []core.Container, meta map[string]containerMeta) {
	for i := range projects {
		projects[i].Containers = nil
		projects[i].ContainerCount = 0
	}

	for _, c := range containers {
		m := lookupMeta(c.ID, meta)
		bestIdx := -1
		bestScore := 0
		for i, p := range projects {
			if score := matchScore(p.Path, m); score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx < 0 && c.ProjectPath != "" {
			for i, p := range projects {
				if filepath.Clean(p.Path) == filepath.Clean(c.ProjectPath) {
					bestIdx = i
					break
				}
			}
		}
		if bestIdx >= 0 {
			projects[bestIdx].Containers = append(projects[bestIdx].Containers, c)
			projects[bestIdx].ContainerCount = len(projects[bestIdx].Containers)
		}
	}
}

func matchScore(projectPath string, m containerMeta) int {
	projectPath = filepath.Clean(projectPath)
	if projectPath == "" || projectPath == "/" {
		return 0
	}

	// Strongest: compose working dir / config file root
	if root := m.composeRoot(); root != "" {
		if root == projectPath {
			return 10000 + len(projectPath)
		}
		if strings.HasPrefix(root, projectPath+string(filepath.Separator)) {
			return 9000 + len(projectPath)
		}
	}

	// Mount is inside project directory
	for _, mount := range m.Mounts {
		mount = filepath.Clean(mount)
		if mount == "" {
			continue
		}
		if mount == projectPath {
			return 8000 + len(projectPath)
		}
		if strings.HasPrefix(mount, projectPath+string(filepath.Separator)) {
			return 7000 + len(mount)
		}
	}

	// Compose project name equals folder name (exact)
	if m.ComposeProject != "" {
		base := strings.ToLower(filepath.Base(projectPath))
		compose := strings.ToLower(m.ComposeProject)
		if compose == base {
			return 5000 + len(projectPath)
		}
	}

	return 0
}

func ProjectRunning(containers []core.Container) bool {
	for _, c := range containers {
		if c.Status == "running" {
			return true
		}
	}
	return false
}

func parseHealthFromStatus(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "(healthy)"):
		return "healthy"
	case strings.Contains(lower, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(lower, "(health: starting)"):
		return "starting"
	default:
		return ""
	}
}
