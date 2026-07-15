package ui

import "github.com/devscope/devscope/internal/core"

// syncGitBranchesFrom updates the UI branch list from live store data,
// ignoring branches the user already deleted in this session.
func (a *App) syncGitBranchesFrom(p *core.Project) {
	if p == nil || p.Git == nil || !p.Git.IsRepo {
		return
	}
	a.gitBranches = filterGitBranches(p.Git.Branches, a.gitBranchDenylist)
	if len(a.gitBranches) > 0 {
		a.gitRenderCache = p.Git
	}
}

func filterGitBranches(branches []core.GitBranch, deny map[string]struct{}) []core.GitBranch {
	if len(branches) == 0 {
		return nil
	}
	if len(deny) == 0 {
		return append([]core.GitBranch(nil), branches...)
	}
	out := make([]core.GitBranch, 0, len(branches))
	for _, b := range branches {
		if _, banned := deny[b.Name]; banned {
			continue
		}
		out = append(out, b)
	}
	return out
}

func (a *App) gitBranchesForUI() []core.GitBranch {
	return a.gitBranches
}

func (a *App) pruneGitBranch(name string) {
	if name == "" {
		return
	}
	if a.gitBranchDenylist == nil {
		a.gitBranchDenylist = make(map[string]struct{})
	}
	a.gitBranchDenylist[name] = struct{}{}

	if len(a.gitBranches) == 0 {
		return
	}
	out := a.gitBranches[:0]
	for _, b := range a.gitBranches {
		if b.Name != name {
			out = append(out, b)
		}
	}
	a.gitBranches = out
	if a.gitMarkedBranch == name {
		a.gitMarkedBranch = ""
	}
}

func (a *App) allowGitBranchName(name string) {
	if a.gitBranchDenylist == nil || name == "" {
		return
	}
	delete(a.gitBranchDenylist, name)
}

// projectGitInfo returns git metadata for rendering. Branch lists come from
// gitBranchesForUI() to avoid flashing deleted branches from stale store/cache.
func (a *App) projectGitInfo(p *core.Project) *core.GitInfo {
	if p != nil && p.Git != nil && p.Git.IsRepo {
		a.gitRenderCache = p.Git
		return p.Git
	}
	if a.gitRenderCache != nil && a.gitRenderCache.IsRepo && len(a.gitBranches) > 0 {
		return a.gitRenderCache
	}
	return nil
}
