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
		{Prefix: "* ", Hash: "aaaa", Short: "aaa", Author: "igor", Date: "2024-01-17", Subject: "first"},
		{Prefix: "| ", Hash: "bbbb", Short: "bbb", Author: "igor", Date: "2024-01-16", Subject: "second", Refs: "main"},
		{Prefix: "|/"},
	}
	out := a.renderGitGraph(p)
	if out == "" || !strings.Contains(out, "COMMITS") || !strings.Contains(out, "CHANGED FILES") {
		t.Fatalf("expected commits/changed-files panels, got:\n%s", out)
	}

	a.gitGraphDetailHash = "aaaa"
	a.gitGraphDetailMsg = "first\n\nlonger body"
	a.gitGraphDetailFiles = []collectors.GitCommitFileStat{{Path: "main.go", Insertions: 3, Deletions: 1}}
	out = a.renderGitGraph(p)
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected changed file to render, got:\n%s", out)
	}
}

func TestHandleGitGraphKeysSkipsConnectorRows(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.gitGraphRows = []collectors.GitGraphRow{
		{Prefix: "* ", Hash: "aaaa", Short: "aaa", Subject: "first"},
		{Prefix: "|/"}, // connector-only, must be skipped by cursor movement
		{Prefix: "* ", Hash: "bbbb", Short: "bbb", Subject: "second"},
	}

	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyDown}, p)
	if a.gitGraphCursor != 2 {
		t.Fatalf("down should land on the next commit row (index 2), got %d", a.gitGraphCursor)
	}
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyUp}, p)
	if a.gitGraphCursor != 0 {
		t.Fatalf("up should land back on the first commit row (index 0), got %d", a.gitGraphCursor)
	}
	if _, cmd := a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEsc}, p); cmd != nil {
		t.Fatal("esc should not return a cmd")
	}
	if a.gitSubview != gitSubviewMain {
		t.Fatal("esc should return to the main git subview")
	}
}

func TestGitGraphEnterOpensCommitDetailAndEscReturnsToGraph(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p, gitSubview: gitSubviewGraph}
	a.gitGraphRows = []collectors.GitGraphRow{
		{Prefix: "* ", Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Short: "aaaaaaa", Author: "igor", Date: "2024-01-17", Subject: "first"},
	}

	if _, cmd := a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); cmd == nil {
		t.Fatal("enter on a commit row should return a load cmd")
	}
	if a.gitSubview != gitSubviewCommit {
		t.Fatalf("enter should open the commit detail subview, got %d", a.gitSubview)
	}
	if a.gitSelectedCommit.Hash != a.gitGraphRows[0].Hash {
		t.Fatalf("commit detail should target the selected row's hash, got %q", a.gitSelectedCommit.Hash)
	}
	if a.gitCommitReturnTo != gitSubviewGraph {
		t.Fatalf("should remember to return to the graph, got %d", a.gitCommitReturnTo)
	}

	a.handleGitDedicatedKeys(tea.KeyMsg{Type: tea.KeyEsc}, p)
	if a.gitSubview != gitSubviewGraph {
		t.Fatalf("esc from commit detail opened via the graph should return to the graph, got %d", a.gitSubview)
	}
}
