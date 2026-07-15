package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/scanner"
)

func findProjectForCwd(projects []core.Project) *core.Project {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	cwd = filepath.Clean(cwd)

	if root := scanner.ResolveProjectRoot(cwd); root != "" {
		if p := projectByPath(projects, root); p != nil {
			return p
		}
	}

	var best *core.Project
	bestLen := -1
	for i := range projects {
		pp := filepath.Clean(projects[i].Path)
		if cwd == pp || strings.HasPrefix(cwd, pp+string(filepath.Separator)) {
			if len(pp) > bestLen {
				cp := projects[i]
				best = &cp
				bestLen = len(pp)
			}
		}
	}
	return best
}

func projectByPath(projects []core.Project, path string) *core.Project {
	path = filepath.Clean(path)
	for i := range projects {
		if filepath.Clean(projects[i].Path) == path {
			cp := projects[i]
			return &cp
		}
	}
	return nil
}

func (a *App) openProjectFromCwd() {
	p := findProjectForCwd(a.snapshot.Projects)
	if p == nil {
		return
	}
	cp := *p
	a.selectedProject = &cp
	a.view = ViewProject
	a.tab = TabOverview
	a.tabCursor = 0
}
