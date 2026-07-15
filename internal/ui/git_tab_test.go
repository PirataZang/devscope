package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestFilteredGitBranches(t *testing.T) {
	a := &App{}
	branches := []core.GitBranch{
		{Name: "develop"},
		{Name: "feat/kanban"},
		{Name: "master"},
	}
	a.gitBranchFilter = "feat"
	got := a.filteredGitBranches(branches)
	if len(got) != 1 || got[0].Name != "feat/kanban" {
		t.Fatalf("unexpected filter result: %+v", got)
	}
}

func TestSyncGitBranchCursor(t *testing.T) {
	a := &App{gitViewBranch: "develop"}
	branches := []core.GitBranch{
		{Name: "master"},
		{Name: "develop"},
	}
	a.syncGitBranchCursor(branches)
	if a.gitBranchCursor != 1 {
		t.Fatalf("expected cursor at develop (1), got %d", a.gitBranchCursor)
	}
}

func TestGitSelectCommitRange(t *testing.T) {
	a := &App{}
	commits := []core.GitCommit{
		{Hash: "aaa"},
		{Hash: "bbb"},
		{Hash: "ccc"},
		{Hash: "ddd"},
	}
	a.gitSelectCommitRange(commits, 1, 3)
	if len(a.gitSelectedCommits) != 3 {
		t.Fatalf("expected 3 selected, got %d", len(a.gitSelectedCommits))
	}
	for _, h := range []string{"bbb", "ccc", "ddd"} {
		if !a.gitSelectedCommits[h] {
			t.Fatalf("expected %s selected", h)
		}
	}
}

func TestFitGitPanelLines(t *testing.T) {
	got := fitGitPanelLines("a\nb", 4)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}
}

func TestIsGitCommitInCherryBufferMarked(t *testing.T) {
	a := &App{
		gitCherryPickMarked: map[string]bool{"abc1234": true},
		gitCherryPickBuffer: []string{"abc1234deadbeef"},
	}
	if !a.isGitCommitInCherryBuffer("abc1234") {
		t.Fatal("expected marked commit in cherry buffer")
	}
}
