package ui

import (
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestFilterGitBranchesDenylist(t *testing.T) {
	branches := []core.GitBranch{
		{Name: "develop"},
		{Name: "feat/x"},
		{Name: "old"},
	}
	deny := map[string]struct{}{"old": {}}
	got := filterGitBranches(branches, deny)
	if len(got) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(got))
	}
	for _, b := range got {
		if b.Name == "old" {
			t.Fatal("denylisted branch should be filtered out")
		}
	}
}

func TestPruneGitBranch(t *testing.T) {
	a := &App{
		gitBranches: []core.GitBranch{
			{Name: "develop"},
			{Name: "gone"},
		},
		gitMarkedBranch: "gone",
	}
	a.pruneGitBranch("gone")
	if len(a.gitBranches) != 1 || a.gitBranches[0].Name != "develop" {
		t.Fatalf("unexpected branches: %+v", a.gitBranches)
	}
	if a.gitMarkedBranch != "" {
		t.Fatal("marked branch should be cleared")
	}
	if _, ok := a.gitBranchDenylist["gone"]; !ok {
		t.Fatal("deleted branch should be denylisted")
	}
}
