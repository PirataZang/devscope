package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// graphAppFixture is a graph screen sitting on a real layout with a loaded
// commit detail, so the pane scroll/focus keys have something to move.
func graphAppFixture() (*App, *core.Project) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p, gitSubview: gitSubviewGraph}
	a.gitGraphLayout = buildGraphLayout(mergeFixtureCommits())
	a.gitGraphCursor = 0
	a.gitGraphDetailHash = a.gitGraphLayout.nodes[0].commit.Hash
	a.gitGraphDetailMsg = strings.Repeat("linha bem comprida de mensagem de commit\n", 40)
	a.gitGraphDetailFiles = []collectors.GitCommitFileStat{
		{Path: strings.Repeat("dir/", 30) + "arquivo.go", Insertions: 12, Deletions: 3},
	}
	return a, p
}

func TestGitGraphTabCyclesPanesAndScrollsThem(t *testing.T) {
	a, p := graphAppFixture()

	if a.gitGraphFocus != gitGraphFocusCommits {
		t.Fatalf("graph should start focused on the commit list, got %d", a.gitGraphFocus)
	}

	// tab → COMMIT DETAIL: arrows now scroll the pane, not the cursor.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyTab}, p)
	if a.gitGraphFocus != gitGraphFocusDetail {
		t.Fatalf("tab should focus the detail pane, got %d", a.gitGraphFocus)
	}
	cursorBefore := a.gitGraphCursor
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyDown}, p)
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if a.gitGraphDetailScroll == 0 || a.gitGraphDetailHScroll == 0 {
		t.Fatalf("detail pane should scroll both axes, got v=%d h=%d", a.gitGraphDetailScroll, a.gitGraphDetailHScroll)
	}
	if a.gitGraphCursor != cursorBefore {
		t.Fatal("scrolling a pane must not move the commit cursor")
	}

	// tab → CHANGED FILES.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyTab}, p)
	if a.gitGraphFocus != gitGraphFocusFiles {
		t.Fatalf("tab should focus the files pane, got %d", a.gitGraphFocus)
	}
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if a.gitGraphFilesHScroll == 0 {
		t.Fatal("files pane should scroll sideways")
	}

	// tab wraps back to the list, where arrows move the cursor again.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyTab}, p)
	if a.gitGraphFocus != gitGraphFocusCommits {
		t.Fatalf("tab should wrap back to the commit list, got %d", a.gitGraphFocus)
	}

	// The render pass clamps offsets to what actually fits.
	a.gitGraphDetailScroll, a.gitGraphDetailHScroll = 9999, 9999
	a.gitGraphFilesScroll, a.gitGraphFilesHScroll = 9999, 9999
	view := a.renderGitGraph(p)
	if !strings.Contains(stripANSI(view), "COMMIT DETAIL") {
		t.Fatal("graph view should still draw the detail pane")
	}
	if a.gitGraphDetailScroll >= 9999 || a.gitGraphDetailHScroll >= 9999 {
		t.Fatalf("render should clamp the detail offsets, got v=%d h=%d", a.gitGraphDetailScroll, a.gitGraphDetailHScroll)
	}
	// One file line, one short-ish path: nothing to page vertically.
	if a.gitGraphFilesScroll != 0 {
		t.Fatalf("a single file row has nowhere to scroll down, got %d", a.gitGraphFilesScroll)
	}
	if a.gitGraphFilesHScroll >= 9999 {
		t.Fatalf("render should clamp the files h-offset, got %d", a.gitGraphFilesHScroll)
	}
}

func TestGitGraphBranchPickerFiltersTheGraph(t *testing.T) {
	a, p := graphAppFixture()

	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}}, p)
	if !a.gitGraphBranchPicker {
		t.Fatal("B should open the branch picker")
	}
	if len(a.gitGraphBranchOpts) < 2 || a.gitGraphBranchOpts[0] != gitGraphAllBranches {
		t.Fatalf("picker should offer 'all' plus the refs drawn in the graph, got %v", a.gitGraphBranchOpts)
	}
	if a.gitGraphBranchOpts[1] != "main" {
		t.Fatalf("the fixture's only ref is main, got %v", a.gitGraphBranchOpts)
	}

	// Selecting a branch stores the ref; the reload walks only that branch.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyDown}, p)
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitGraphBranchPicker {
		t.Fatal("enter should close the picker")
	}
	if a.gitGraphBranchRef != "main" {
		t.Fatalf("enter should apply the selected branch, got %q", a.gitGraphBranchRef)
	}
	if !strings.Contains(stripANSI(a.renderGitGraphList(120, 20)), "main") {
		t.Fatal("the commits pane title should name the active branch filter")
	}

	// Back to "all" clears the filter.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}}, p)
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyUp}, p)
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitGraphBranchRef != "" {
		t.Fatalf("picking 'all' should clear the filter, got %q", a.gitGraphBranchRef)
	}
}

func TestGitGraphBranchPickerTypingNarrowsTheList(t *testing.T) {
	a, p := graphAppFixture()
	a.gitBranches = []core.GitBranch{{Name: "main"}, {Name: "feat/graph"}, {Name: "feat/api"}}

	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}}, p)
	if len(a.gitGraphBranchList()) != 4 {
		t.Fatalf("picker should start with 'all' plus 3 branches, got %v", a.gitGraphBranchList())
	}

	for _, r := range "api" {
		a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, p)
	}
	got := a.gitGraphBranchList()
	if len(got) != 2 || got[1] != "feat/api" {
		t.Fatalf("typing should narrow to the matching branch, got %v", got)
	}

	// "ap" also matches gr-ap-h, so backspace widens the list back out.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyBackspace}, p)
	if len(a.gitGraphBranchList()) != 3 {
		t.Fatalf("backspace should widen back to both feat branches, got %v", a.gitGraphBranchList())
	}

	// Enter applies the highlighted entry from the *filtered* list.
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyDown}, p)
	a.handleGitGraphKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitGraphBranchRef != "feat/api" {
		t.Fatalf("enter should apply the filtered selection, got %q", a.gitGraphBranchRef)
	}
}

func TestDockerImageRowColorsProjectMembership(t *testing.T) {
	p := testProjectWithContainer()
	a := &App{width: 120, height: 40, selectedProject: p}
	cols := a.imageColumns()

	mine := core.Image{ID: "aaa", Repository: repoOf(p.Containers[0].Image), Tag: "latest", Created: "1d", Size: "10MB"}
	mine.Tag = strings.TrimPrefix(p.Containers[0].Image, mine.Repository+":")
	if !imageBelongsToProject(mine, p) {
		t.Fatalf("fixture image %s:%s should match the project container", mine.Repository, mine.Tag)
	}
	other := core.Image{ID: "bbb", Repository: "alguem/outro", Tag: "latest", Created: "1d", Size: "10MB"}

	dangling := core.Image{ID: "ccc", Repository: "<none>", Tag: "<none>", Created: "1d", Size: "10MB"}

	for _, tc := range []struct {
		name  string
		img   core.Image
		want  lipgloss.TerminalColor
		glyph string
	}{
		{"projeto", mine, StyleAccent.GetForeground(), "●"},
		{"outros", other, StyleWarning.GetForeground(), "·"},
		{"dangling", dangling, StyleMuted.GetForeground(), "·"},
	} {
		style, glyph := imageOwnerStyle(tc.img, p)
		if style.GetForeground() != tc.want {
			t.Fatalf("%s: wrong colour, got %v want %v", tc.name, style.GetForeground(), tc.want)
		}
		if glyph != tc.glyph {
			t.Fatalf("%s: wrong indicator, got %q want %q", tc.name, glyph, tc.glyph)
		}
		if !strings.Contains(a.renderImageRow(tc.img, cols, false, p), tc.glyph) {
			t.Fatalf("%s: row should carry the %q indicator", tc.name, tc.glyph)
		}
	}
}
