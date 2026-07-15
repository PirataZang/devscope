package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestFindProjectForCwd(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "digiliza")
	sub := filepath.Join(proj, "src", "app")
	for _, d := range []string{proj, sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	projects := []core.Project{
		{Path: root, Name: "root"},
		{Path: proj, Name: "digiliza"},
	}

	t.Chdir(sub)
	got := findProjectForCwd(projects)
	if got == nil || got.Name != "digiliza" {
		t.Fatalf("expected digiliza project, got %+v", got)
	}
}
