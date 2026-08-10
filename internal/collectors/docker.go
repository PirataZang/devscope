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
	ComposeService string
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
const dockerPSFormat = "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}\t{{.Status}}\t{{.Ports}}\t{{.Label \"com.docker.compose.project\"}}\t{{.Label \"com.docker.compose.project.working_dir\"}}\t{{.Label \"com.docker.compose.project.config_files\"}}\t{{.Label \"com.docker.compose.service\"}}"

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
	applyRestartPolicies(containers)
	return containers, meta, nil
}

// applyRestartPolicies fills Container.Restart via one batch inspect.
func applyRestartPolicies(containers []core.Container) {
	if len(containers) == 0 {
		return
	}
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	args := append([]string{"inspect", "-f", "{{.Id}}\t{{.HostConfig.RestartPolicy.Name}}"}, ids...)
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		return
	}
	byID := make(map[string]string, len(ids))
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		id := parts[0]
		short := id
		if len(short) > 12 {
			short = short[:12]
		}
		policy := strings.TrimSpace(parts[1])
		byID[id] = policy
		byID[short] = policy
	}
	for i := range containers {
		if p, ok := byID[containers[i].ID]; ok {
			containers[i].Restart = p
		}
	}
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
	if len(parts) > 9 {
		m.ComposeService = parts[9]
	}
	name := strings.TrimPrefix(parts[1], "/")
	return core.Container{
		ID:          id,
		Name:        name,
		Image:       parts[2],
		Status:      strings.ToLower(parts[3]),
		State:       parts[4], // human status ("Up 2 hours" / "Exited (0) ...")
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
// Returns containers that matched no scanned project (still in docker ps -a).
func AssignContainersToProjects(projects []core.Project, containers []core.Container, meta map[string]containerMeta) []core.Container {
	for i := range projects {
		projects[i].Containers = nil
		projects[i].ContainerCount = 0
	}

	var orphans []core.Container
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
		} else {
			orphans = append(orphans, c)
		}
	}
	orphans = reclaimOrphansByComposeName(projects, orphans, meta)
	EnrichProjectsWithMissingComposeServices(projects)
	return orphans
}

// reclaimOrphansByComposeName pulls unassigned containers into a project when the
// name/service matches a compose service — surfaces stopped containers that would
// conflict on `compose up`.
func reclaimOrphansByComposeName(projects []core.Project, orphans []core.Container, meta map[string]containerMeta) []core.Container {
	if len(orphans) == 0 || len(projects) == 0 {
		return orphans
	}
	services := make([][]string, len(projects))
	hasAny := false
	for i := range projects {
		services[i] = ListComposeServiceNames(projects[i].Path)
		if len(services[i]) > 0 {
			hasAny = true
		}
	}
	if !hasAny {
		return orphans
	}
	var still []core.Container
	for _, c := range orphans {
		m := lookupMeta(c.ID, meta)
		claimed := -1
		for i := range projects {
			if len(services[i]) == 0 {
				continue
			}
			base := filepath.Base(projects[i].Path)
			if m.ComposeService != "" {
				cp := strings.ToLower(m.ComposeProject)
				if cp == "" || cp == strings.ToLower(base) || cp == strings.ToLower(projects[i].Name) {
					for _, svc := range services[i] {
						if strings.EqualFold(m.ComposeService, svc) {
							claimed = i
							break
						}
					}
				}
			}
			if claimed < 0 {
				for _, svc := range services[i] {
					if containerMatchesComposeService(c.Name, projects[i].Name, base, svc) {
						claimed = i
						break
					}
				}
			}
			if claimed >= 0 {
				break
			}
		}
		if claimed >= 0 {
			projects[claimed].Containers = append(projects[claimed].Containers, c)
			projects[claimed].ContainerCount = len(projects[claimed].Containers)
			continue
		}
		still = append(still, c)
	}
	return still
}

func containerMatchesComposeService(containerName, projectName, projectBase, service string) bool {
	n := strings.ToLower(strings.TrimPrefix(containerName, "/"))
	s := strings.ToLower(service)
	if s == "" || n == "" {
		return false
	}
	if n == s {
		return true
	}
	// compose default: {project}-{service}-{replica}
	for _, p := range []string{strings.ToLower(projectName), strings.ToLower(projectBase), strings.ToLower(strings.ReplaceAll(projectBase, "_", "-"))} {
		if p == "" {
			continue
		}
		if strings.HasPrefix(n, p+"-"+s+"-") || n == p+"-"+s {
			return true
		}
	}
	if strings.Contains(n, "-"+s+"-") || strings.HasSuffix(n, "-"+s) {
		return true
	}
	return false
}

// EnrichProjectsWithMissingComposeServices adds synthetic rows for compose services
// with no container yet (Status=missing).
func EnrichProjectsWithMissingComposeServices(projects []core.Project) {
	for i := range projects {
		services := ListComposeServiceNames(projects[i].Path)
		if len(services) == 0 {
			continue
		}
		present := make(map[string]bool, len(projects[i].Containers))
		for _, c := range projects[i].Containers {
			for _, svc := range services {
				if strings.EqualFold(c.Name, svc) ||
					containerMatchesComposeService(c.Name, projects[i].Name, filepath.Base(projects[i].Path), svc) {
					present[strings.ToLower(svc)] = true
				}
			}
		}
		for _, svc := range services {
			if present[strings.ToLower(svc)] {
				continue
			}
			projects[i].Containers = append(projects[i].Containers, core.Container{
				Name:        svc,
				Image:       "compose",
				Status:      "missing",
				State:       "not created",
				ProjectPath: projects[i].Path,
			})
		}
		projects[i].ContainerCount = len(projects[i].Containers)
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
