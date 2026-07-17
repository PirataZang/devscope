package ui

import (
	"context"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
)

type projectGitLoadedMsg struct {
	path string
	gen  int
}

type projectDockerLoadedMsg struct {
	path string
	gen  int
}

func (a *App) startProjectLoad(path string) tea.Cmd {
	path = filepath.Clean(path)
	a.snapshot = a.store.Get()
	a.projectGitLoading = true
	a.projectLoadGen++
	gen := a.projectLoadGen

	hasContainers := false
	for _, p := range a.snapshot.Projects {
		if pathsMatch(p.Path, path) && len(p.Containers) > 0 {
			hasContainers = true
			break
		}
	}
	a.projectDockerLoading = !hasContainers

	// Git first — docker detail runs after git finishes (was blocking git when batched).
	return loadProjectGit(path, gen, a.store)
}

func loadProjectGit(path string, gen int, store *core.StateStore) tea.Cmd {
	return func() tea.Msg {
		collectors.RefreshProjectGit(store, path)
		return projectGitLoadedMsg{path: path, gen: gen}
	}
}

func loadProjectDockerDetail(path string, gen int, store *core.StateStore, healthCfg config.HealthConfig) tea.Cmd {
	return func() tea.Msg {
		collectors.EnrichProjectDockerDetail(context.Background(), store, path, healthCfg)
		return projectDockerLoadedMsg{path: path, gen: gen}
	}
}

func (a *App) handleProjectGitLoaded(msg projectGitLoadedMsg) tea.Cmd {
	if msg.gen != a.projectLoadGen || a.selectedProject == nil || !pathsMatch(a.selectedProject.Path, msg.path) {
		return nil
	}
	a.projectGitLoading = false
	a.snapshot = a.store.Get()
	if p := a.currentProject(); p != nil {
		cp := *p
		a.selectedProject = &cp
		if p.Git != nil && p.Git.IsRepo {
			a.initGitTab(p)
		}
	}
	return loadProjectDockerDetail(msg.path, msg.gen, a.store, a.cfg.Health)
}

func (a *App) handleProjectDockerLoaded(msg projectDockerLoadedMsg) {
	if msg.gen != a.projectLoadGen || a.selectedProject == nil || !pathsMatch(a.selectedProject.Path, msg.path) {
		return
	}
	a.projectDockerLoading = false
	a.snapshot = a.store.Get()
	if cp := a.currentProject(); cp != nil {
		p := *cp
		a.selectedProject = &p
	}
}
