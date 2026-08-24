package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func testProjectWithContainer() *core.Project {
	return &core.Project{
		Path: "/apps/demo", Name: "demo",
		Containers: []core.Container{
			{ID: "c1", Name: "demo-dev-1", Image: "demo-dev:latest", Status: "running", ProjectPath: "/apps/demo"},
		},
		Git: &core.GitInfo{
			IsRepo: true,
			Branch: "main",
			Branches: []core.GitBranch{
				{Name: "main", Current: true},
				{Name: "feature/x"},
			},
		},
	}
}

func TestRenderContainerImagesDoesNotPanic(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.imageAll = []core.Image{
		{ID: "abc123", Repository: "demo-dev", Tag: "latest", Created: "1h", Size: "10MB", ComposeProject: "demo", ComposeService: "dev"},
		{ID: "def456", Repository: "<none>", Tag: "<none>", Created: "2h", Size: "9MB", ComposeProject: "demo", ComposeService: "dev"},
		{ID: "ghi789", Repository: "nginx", Tag: "latest", Created: "3d", Size: "40MB"},
	}
	for _, scope := range []imageScope{imageScopeContainer, imageScopeProject, imageScopeAll} {
		a.imageScope = scope
		a.imageContainerRepo = "demo-dev"
		out := a.renderContainerImages(p)
		if out == "" {
			t.Fatalf("scope %d: empty render", scope)
		}
	}
	a.imageScope = imageScopeProject
	if got := len(a.currentImageList(p)); got != 2 {
		t.Fatalf("project scope should include the dangling rebuild via compose label, got %d images", got)
	}
	a.imageScope = imageScopeAll
	if got := len(a.currentImageList(p)); got != 3 {
		t.Fatalf("all scope should list every image, got %d", got)
	}

	a.imageConfirmRemove = true
	a.imageConfirmCursor = 1
	if out := a.renderContainerImages(p); !strings.Contains(out, "Remover") {
		t.Fatalf("remove modal should render its options, got:\n%s", out)
	}
}

func TestHandleContainerImagesKeysNavigatesAndCycles(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.imageAll = []core.Image{
		{ID: "abc123", Repository: "demo-dev", Tag: "latest"},
		{ID: "ghi789", Repository: "nginx", Tag: "latest"},
	}
	a.imageScope = imageScopeContainer
	a.imageContainerRepo = "demo-dev"

	if _, cmd := a.handleContainerImagesKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")}, p); cmd != nil {
		t.Fatalf("cycling scope should not return a cmd")
	}
	if a.imageScope != imageScopeProject {
		t.Fatalf("expected scope to advance to project, got %d", a.imageScope)
	}

	if _, cmd := a.handleContainerImagesKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}, p); cmd != nil {
		t.Fatalf("opening the modal should not return a cmd")
	}
	if !a.imageConfirmRemove {
		t.Fatalf("D should open the remove modal")
	}
	if _, cmd := a.handleContainerImagesKeys(tea.KeyMsg{Type: tea.KeyEsc}, p); cmd != nil {
		t.Fatalf("esc on modal should not return a cmd")
	}
	if a.imageConfirmRemove {
		t.Fatalf("esc should close the modal")
	}
}

func TestRenderContainerDepsBuildsForestAndDoesNotPanic(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.containerDeps = []collectors.ComposeDependency{
		{Service: "web", DependsOn: []string{"api"}},
		{Service: "api", DependsOn: []string{"db", "cache"}},
		{Service: "db"},
		{Service: "cache"},
	}
	out := a.renderContainerDeps(p)
	if out == "" {
		t.Fatal("empty render")
	}
	forest := buildDepForest(a.containerDeps)
	if len(forest) != 1 || forest[0].Name != "web" {
		t.Fatalf("expected single root 'web' (nothing else depends on it), got %+v", forest)
	}
	lines := flattenDepTree(forest)
	if len(lines) != 4 {
		t.Fatalf("expected 4 flattened lines (web, api, db, cache), got %d: %+v", len(lines), lines)
	}
}

func TestRenderGitGraphDoesNotPanic(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.gitGraphRows = []collectors.GitGraphRow{
		{Prefix: "* ", Hash: "aaaa", Short: "aaa", Author: "igor", When: "1h", Subject: "first"},
		{Prefix: "| ", Hash: "bbbb", Short: "bbb", Author: "igor", When: "2h", Subject: "second", Refs: "main"},
		{Prefix: "|/"},
	}
	out := a.renderGitGraph(p)
	if out == "" || !strings.Contains(out, "BRANCHES") || !strings.Contains(out, "COMMITS") {
		t.Fatalf("expected branches/commits panels, got:\n%s", out)
	}

	a.gitGraphBranchCursor = 1 // "main" (index 0 is the synthetic "Todas")
	a.gitGraphReachable = map[string]bool{"aaaa": true}
	out = a.renderGitGraph(p)
	if out == "" {
		t.Fatal("empty render with a highlight filter active")
	}
}

func TestHandleGitGraphKeysTogglesFocusAndSelection(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.gitGraphRows = []collectors.GitGraphRow{{Prefix: "* ", Hash: "aaaa", Short: "aaa", Subject: "first"}}

	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if !a.gitGraphFocusGraph {
		t.Fatal("right should focus the graph pane")
	}
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyLeft}, p)
	if a.gitGraphFocusGraph {
		t.Fatal("left should focus the branch list")
	}
	if _, cmd := a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEsc}, p); cmd != nil {
		t.Fatal("esc should not return a cmd")
	}
	if a.gitSubview != gitSubviewMain {
		t.Fatal("esc should return to the main git subview")
	}
}
