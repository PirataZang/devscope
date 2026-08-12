package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// A view taller/wider than the terminal wraps and scrolls the top of the UI
// off screen (VS Code split terminals). Every tab must fit the box exactly.
func TestViewFitsSmallTerminal(t *testing.T) {
	project := core.Project{
		Path: "/tmp/repo", Name: "repo",
		Git: &core.GitInfo{
			IsRepo: true, Branch: "DES-2875", Ahead: 1, Modified: 2, Staged: 1, Untracked: 1, StashCount: 1,
			Files:    []core.GitFileStatus{{Path: "app.go", Staging: " ", Worktree: "M"}},
			Branches: []core.GitBranch{{Name: "DES-2875", Current: true}, {Name: "DES-2850"}},
			Commits:  []core.GitCommit{{Hash: "7d6fb1", Message: "refactor", Date: "2h"}},
			Stashes:  []core.GitStash{{Ref: "stash@{0}", Message: "wip"}},
			Remotes:  []core.GitRemote{{Name: "origin", URL: "git@github.com:o/r.git"}},
		},
	}
	for _, sz := range [][2]int{{100, 20}, {80, 24}, {60, 15}, {120, 30}} {
		for _, view := range []View{ViewDashboard, ViewProject} {
			for _, tab := range AllTabs {
				a := &App{
					width: sz[0], height: sz[1], view: view, tab: tab,
					gitSubview: gitSubviewMain, gitFocus: gitFocusBranches, gitViewBranch: "DES-2875",
					gitBranches: project.Git.Branches, gitBranchCommits: project.Git.Commits,
					selectedProject: &project,
					snapshot:        core.Snapshot{Projects: []core.Project{project}},
				}
				out := a.View()
				if w, h := lipgloss.Width(out), lipgloss.Height(out); w != sz[0] || h != sz[1] {
					t.Errorf("%dx%d view=%d tab=%s rendered %dx%d", sz[0], sz[1], view, tab, w, h)
				}
			}
		}
	}
}

// The active module must stay visible when the rail is taller than the terminal.
func TestSidebarKeepsActiveTabVisible(t *testing.T) {
	rows := []string{"a", "b", "c", "d", "e", "f"}
	got := sidebarWindow(rows, 5, 3)
	if len(got) != 3 || got[2] != "f" {
		t.Fatalf("focus at end: %v", got)
	}
	if got := sidebarWindow(rows, 0, 3); got[0] != "a" {
		t.Fatalf("focus at start: %v", got)
	}
	if got := sidebarWindow(rows, 2, 10); len(got) != 6 {
		t.Fatalf("no window needed: %v", got)
	}
}
