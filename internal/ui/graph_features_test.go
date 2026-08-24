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

// mergeFixtureCommits is a small DAG with a real 2-parent merge — enough to
// exercise branch/merge cells (╭╮╰╯), not just a straight line of pipes.
// Newest first, as git log --topo-order would return it:
//
//	head   (merge of feature + main-tip)
//	feat1  (on the feature branch)
//	main-tip
//	base   (fork point: both feature and main-tip branch from here)
func mergeFixtureCommits() []collectors.DAGCommit {
	return []collectors.DAGCommit{
		{Hash: "head0000", Short: "head000", Subject: "Merge feature", Parents: []string{"maintip0", "feat1000"}, Author: "igor", Date: "2024-01-18", IsHead: true},
		{Hash: "feat1000", Short: "feat1000", Subject: "feat: add thing", Parents: []string{"base0000"}, Author: "igor", Date: "2024-01-17"},
		{Hash: "maintip0", Short: "maintip0", Subject: "main tip commit", Parents: []string{"base0000"}, Author: "igor", Date: "2024-01-16", Refs: []string{"main"}},
		{Hash: "base0000", Short: "base0000", Subject: "base commit", Author: "igor", Date: "2024-01-15"},
	}
}

func TestBuildGraphLayoutRowsShareAUniformWidth(t *testing.T) {
	layout := buildGraphLayout(mergeFixtureCommits())
	if len(layout.nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	want := (layout.maxLane + 1) * 2
	for i, n := range layout.nodes {
		if len(n.cells) != want {
			t.Fatalf("node %d: expected %d cells (uniform width), got %d", i, want, len(n.cells))
		}
	}
	// The merge commit itself should show at least one rounded merge glyph,
	// not a plain straight line — this is the whole point of computing our
	// own layout instead of reusing git's diagonal /\ output.
	head := layout.nodes[0]
	sawCurve := false
	for _, c := range head.cells {
		if c.kind == cellMergeLeft || c.kind == cellMergeRight || c.kind == cellBranchLeft || c.kind == cellBranchRight {
			sawCurve = true
		}
	}
	if !sawCurve {
		t.Fatalf("expected a rounded branch/merge cell on the merge commit's row, got cells: %+v", head.cells)
	}
}

func TestRenderGitGraphDoesNotPanic(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	a.gitGraphLayout = buildGraphLayout(mergeFixtureCommits())
	a.gitGraphCursor = 0

	out := a.renderGitGraph(p)
	if out == "" || !strings.Contains(out, "COMMITS") || !strings.Contains(out, "CHANGED FILES") {
		t.Fatalf("expected commits/changed-files panels, got:\n%s", out)
	}
	if !strings.Contains(out, "[main]") {
		t.Fatalf("expected the main branch badge to render, got:\n%s", out)
	}

	a.gitGraphDetailHash = "head0000"
	a.gitGraphDetailMsg = "Merge feature\n\nlonger body"
	a.gitGraphDetailFiles = []collectors.GitCommitFileStat{{Path: "main.go", Insertions: 3, Deletions: 1}}
	out = a.renderGitGraph(p)
	if !strings.Contains(out, "main.go") {
		t.Fatalf("expected changed file to render, got:\n%s", out)
	}
}

func TestHandleGitGraphKeysSkipsConnectorRows(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	// Two commits forking from the same base produces a connector-only fork
	// row ahead of "base"'s own commit row — cursor movement must skip it.
	a.gitGraphLayout = buildGraphLayout([]collectors.DAGCommit{
		{Hash: "c1", Short: "c1", Subject: "c1", Parents: []string{"base"}},
		{Hash: "c2", Short: "c2", Subject: "c2", Parents: []string{"base"}},
		{Hash: "base", Short: "base", Subject: "base"},
	})
	var connectorRows int
	for _, n := range a.gitGraphLayout.nodes {
		if n.commit == nil {
			connectorRows++
		}
	}
	if connectorRows == 0 {
		t.Fatal("fixture should produce at least one connector-only fork row")
	}

	from := -1
	for {
		i := a.nextGitGraphCommitRow(from)
		if i < 0 {
			break
		}
		a.gitGraphCursor, from = i, i
		if node, ok := a.selectedGitGraphNode(); !ok || node.commit == nil {
			t.Fatalf("cursor landed on a non-commit row at index %d", a.gitGraphCursor)
		}
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
	a.gitGraphLayout = buildGraphLayout(mergeFixtureCommits())
	a.gitGraphCursor = 0

	wantHash := a.gitGraphLayout.nodes[0].commit.Hash
	if _, cmd := a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); cmd == nil {
		t.Fatal("enter on a commit row should return a load cmd")
	}
	if a.gitSubview != gitSubviewCommit {
		t.Fatalf("enter should open the commit detail subview, got %d", a.gitSubview)
	}
	if a.gitSelectedCommit.Hash != wantHash {
		t.Fatalf("commit detail should target the selected row's hash, got %q want %q", a.gitSelectedCommit.Hash, wantHash)
	}
	if a.gitCommitReturnTo != gitSubviewGraph {
		t.Fatalf("should remember to return to the graph, got %d", a.gitCommitReturnTo)
	}

	a.handleGitDedicatedKeys(tea.KeyMsg{Type: tea.KeyEsc}, p)
	if a.gitSubview != gitSubviewGraph {
		t.Fatalf("esc from commit detail opened via the graph should return to the graph, got %d", a.gitSubview)
	}
}
