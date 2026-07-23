package ui

import (
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestRestoreContainerCursorKeepsShowAllSelection(t *testing.T) {
	p1 := core.Project{
		Path: "/apps/one", Name: "alpha",
		Containers: []core.Container{
			{ID: "a1", Name: "alpha-1", Status: "running", ProjectPath: "/apps/one"},
			{ID: "a2", Name: "alpha-2", Status: "running", ProjectPath: "/apps/one"},
		},
	}
	p2 := core.Project{
		Path: "/apps/two", Name: "beta",
		Containers: []core.Container{
			{ID: "b1", Name: "beta-1", Status: "running", ProjectPath: "/apps/two"},
		},
	}
	a := &App{
		containerShowAll: true,
		selectedProject:  &p1,
		snapshot:         core.Snapshot{Projects: []core.Project{p1, p2}},
		containerPreviewID: "b1",
		tabCursor:          0, // wrong index after a bad clamp
	}
	a.restoreContainerCursor("b1")
	if a.tabCursor != 2 {
		t.Fatalf("expected cursor on beta-1 (index 2), got %d", a.tabCursor)
	}
	// Simulates old bug: clamp against current project only.
	a.tabCursor = clampCursor(99, len(p1.Containers))
	if a.tabCursor != 1 {
		t.Fatalf("precondition: clamp to current project last=%d", a.tabCursor)
	}
	a.restoreContainerCursor("b1")
	if a.tabCursor != 2 {
		t.Fatalf("restore should recover show-all selection, got %d", a.tabCursor)
	}
}

func TestContainerShowAllIncludesProjectColumn(t *testing.T) {
	p1 := core.Project{
		Path: "/apps/one", Name: "alpha-app",
		Containers: []core.Container{
			// ProjectPath is compose cwd (subdir), not the project root — common in docker ps.
			{ID: "1", Name: "one-web", Image: "nginx", Status: "running", ProjectPath: "/apps/one/docker"},
		},
	}
	p2 := core.Project{
		Path: "/apps/two", Name: "beta-app",
		Containers: []core.Container{
			{ID: "2", Name: "two-db", Image: "postgres", Status: "running", ProjectPath: "/apps/two/compose"},
		},
	}
	a := &App{
		width: 120, height: 40,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		containerShowAll: true,
		selectedProject:  &p1,
		snapshot:         core.Snapshot{Projects: []core.Project{p1, p2}},
	}
	got := stripANSI(a.renderContainerList(&p1))
	for _, want := range []string{"TODOS OS PROJETOS", "PROJECT", "alpha-app", "beta-app", "one-web", "two-db"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, truncate(got, 400))
		}
	}
	if strings.Contains(got, "/apps/one") || strings.Contains(got, "docker") && strings.Contains(got, "PROJECT") {
		// path must not appear as the PROJECT cell value
		if strings.Contains(got, "/apps/") {
			t.Fatalf("PROJECT column should show names, not paths:\n%s", truncate(got, 400))
		}
	}
	a.containerShowAll = false
	only := stripANSI(a.renderContainerList(&p1))
	if strings.Contains(only, "two-db") {
		t.Fatal("project filter should hide other project containers")
	}
	if strings.Contains(only, "PROJECT") {
		t.Fatal("project column only in show-all mode")
	}
}
