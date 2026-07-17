package collectors

import (
	"context"
	"log"
	"path/filepath"

	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/scanner"
)

func findProjectByPath(projects []core.Project, path string) (core.Project, bool) {
	path = filepath.Clean(path)
	for _, p := range projects {
		if filepath.Clean(p.Path) == path {
			return p, true
		}
	}
	return core.Project{}, false
}

// EnrichProjectDockerDetail adds stats, workers and health for one project only.
func EnrichProjectDockerDetail(ctx context.Context, store *core.StateStore, projectPath string, healthCfg config.HealthConfig) {
	snap := store.Get()
	project, found := findProjectByPath(snap.Projects, projectPath)
	if !found {
		return
	}

	if len(project.Containers) == 0 {
		containers, meta, err := CollectDockerPS(ctx)
		if err == nil {
			assignContainersToProject(&project, snap.Projects, containers, meta)
			store.UpdateProjectRuntime(projectPath, project)
		}
	}

	ids := make([]string, 0, len(project.Containers))
	for _, c := range project.Containers {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}

	projects := []core.Project{project}
	stats := CollectDockerStatsForIDs(ctx, ids)
	ApplyDockerStats(projects, stats)
	pm2Apps := CollectPM2(ctx)
	AssignWorkersToProjects(projects, pm2Apps)
	CollectHealth(ctx, projects, healthCfg)
	ApplyProjectStatus(projects, nil)

	store.UpdateProjectRuntime(projectPath, projects[0])
}

// RefreshProjectDocker re-links containers and stats for a single project after an action.
func RefreshProjectDocker(store *core.StateStore, projectPath string, healthCfg config.HealthConfig) {
	ctx := context.Background()
	snap := store.Get()
	project, found := findProjectByPath(snap.Projects, projectPath)
	if !found {
		return
	}

	containers, meta, err := CollectDockerPS(ctx)
	if err == nil {
		assignContainersToProject(&project, snap.Projects, containers, meta)
	}

	projects := []core.Project{project}
	ids := make([]string, 0, len(project.Containers))
	for _, c := range project.Containers {
		if c.ID != "" {
			ids = append(ids, c.ID)
		}
	}
	stats := CollectDockerStatsForIDs(ctx, ids)
	ApplyDockerStats(projects, stats)
	pm2Apps := CollectPM2(ctx)
	AssignWorkersToProjects(projects, pm2Apps)
	ApplyProjectStatus(projects, nil)

	store.UpdateProjectRuntime(projectPath, projects[0])
}

func assignContainersToProject(p *core.Project, allProjects []core.Project, containers []core.Container, meta map[string]containerMeta) {
	p.Containers = nil
	p.ContainerCount = 0

	for _, c := range containers {
		m := lookupMeta(c.ID, meta)
		bestIdx := -1
		bestScore := 0
		for i, proj := range allProjects {
			if score := matchScore(proj.Path, m); score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx < 0 && c.ProjectPath != "" {
			for i, proj := range allProjects {
				if filepath.Clean(proj.Path) == filepath.Clean(c.ProjectPath) {
					bestIdx = i
					break
				}
			}
		}
		if bestIdx >= 0 && filepath.Clean(allProjects[bestIdx].Path) == filepath.Clean(p.Path) {
			p.Containers = append(p.Containers, c)
		}
	}
	p.ContainerCount = len(p.Containers)
}

// enrichProjectsFull runs docker, stats, health and git for all projects (CLI scan).
func enrichProjectsFull(ctx context.Context, projects []core.Project, m *Manager) {
	containers, meta, err := CollectDocker(ctx)
	if err != nil {
		log.Printf("docker collector error: %v", err)
	}

	pm2Roots := scanner.DiscoverRunningRoots(ctx)
	pm2Apps := CollectPM2(ctx)

	AssignContainersToProjects(projects, containers, meta)
	stats := CollectDockerStats(ctx)
	ApplyDockerStats(projects, stats)
	AssignWorkersToProjects(projects, pm2Apps)
	AssignPortsToProjects(projects, ReadListeningPorts())
	AssignDomainsToProjects(projects, m.nginxDomains)
	AssignSSLToProjects(projects, m.sslCerts)
	AssignDeployScripts(projects)
	ApplyProjectStatus(projects, pm2Roots)
	CollectHealth(ctx, projects, m.cfg.Health)
	ApplyProjectStatus(projects, pm2Roots)
	for i := range projects {
		root := gitRepoRoot(projects[i].Path)
		if root == "" {
			continue
		}
		if info := CollectAt(root); info != nil {
			projects[i].Git = info
		}
	}
}
