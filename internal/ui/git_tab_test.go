package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestRenderGitMainShowsBottomBoxes(t *testing.T) {
	project := core.Project{
		Path: "/tmp/repo",
		Name: "repo",
		Git: &core.GitInfo{
			IsRepo:     true,
			Branch:     "DES-2834",
			Ahead:      3,
			Modified:   2,
			Staged:     1,
			Untracked:  1,
			StashCount: 2,
			LastCommit: "dc23f83a",
			Files: []core.GitFileStatus{
				{Path: "app.go", Staging: " ", Worktree: "M"},
				{Path: "new.go", Staging: "A", Worktree: " "},
			},
			Branches: []core.GitBranch{{Name: "DES-2834", Current: true}, {Name: "develop"}},
			Commits:  []core.GitCommit{{Hash: "dc23f83a", Message: "fix", Date: "2h ago"}},
			Stashes:  []core.GitStash{{Ref: "stash@{0}", Message: "wip"}},
			Remotes:  []core.GitRemote{{Name: "origin", URL: "git@github.com:org/repo.git"}},
		},
	}
	a := &App{
		width:            120,
		height:           40,
		view:             ViewProject,
		tab:              TabGit,
		gitSubview:       gitSubviewMain,
		gitFocus:         gitFocusBranches,
		gitViewBranch:    "DES-2834",
		gitBranches:      project.Git.Branches,
		gitBranchCommits: project.Git.Commits,
		gitActivity:      []string{"14:30 Checkout DES-2834"},
		gitWTDiff:        "@@ -1 +1 @@\n-old\n+new\n",
		gitWTDiffFile:    "app.go",
		selectedProject:  &project,
		snapshot:         core.Snapshot{Projects: []core.Project{project}},
	}
	got := stripANSI(a.renderGitTab(&project))
	for _, want := range []string{
		"BRANCHES", "COMMITS", "MODIFIED FILES", "DIFF",
		"RECENT ACTIVITY", "STASHES", "REMOTES",
		"DES-2834", "stash@{0}", "origin",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("git main missing %q in:\n%s", want, got)
		}
	}
}

func TestOpenGitFileDiffAllowsScroll(t *testing.T) {
	project := core.Project{
		Path: "/tmp/repo",
		Git: &core.GitInfo{
			IsRepo: true,
			Branch: "main",
			Files: []core.GitFileStatus{
				{Path: "src/Hero.astro", Staging: " ", Worktree: "M"},
			},
		},
	}
	diff := "diff --git a/src/Hero.astro b/src/Hero.astro\n@@ -1,2 +1,2 @@\n-old line that is quite long " + strings.Repeat("x", 80) + "\n+new line that is also long " + strings.Repeat("y", 80) + "\n"
	a := &App{
		width:           80,
		height:          24,
		view:            ViewProject,
		tab:             TabGit,
		gitSubview:      gitSubviewMain,
		gitFocus:        gitFocusFiles,
		gitViewBranch:   "main",
		gitFileCursor:   0,
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
		gitWTDiff:       diff,
		gitWTDiffFile:   "src/Hero.astro",
	}
	cmd := a.openGitFileDiff(&project)
	if cmd != nil {
		t.Fatal("cached diff should not reload")
	}
	if a.gitSubview != gitSubviewFileDiff {
		t.Fatalf("expected file diff subview, got %v", a.gitSubview)
	}
	got := stripANSI(a.renderGitFileDiff(&project))
	if !strings.Contains(got, "DIFF") || !strings.Contains(got, "Hero.astro") {
		t.Fatalf("file diff missing header: %q", got)
	}
	if !strings.Contains(got, "old line") && !strings.Contains(got, "new line") {
		t.Fatalf("file diff missing content: %q", got)
	}
	a.gitWTDiffHScrollBy(10)
	if a.gitWTDiffHScroll == 0 {
		t.Fatal("horizontal scroll should move")
	}
	a.gitWTDiffScrollBy(1)
	scrolled := stripANSI(a.renderGitFileDiff(&project))
	if scrolled == "" {
		t.Fatal("empty after scroll")
	}
	_, _ = a.handleGitDedicatedKeys(tea.KeyMsg{Type: tea.KeyEsc}, &project)
	if a.gitSubview != gitSubviewMain || a.gitFocus != gitFocusFiles {
		t.Fatalf("esc should return to files focus, got sub=%v focus=%v", a.gitSubview, a.gitFocus)
	}
}

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

func TestGitPromptEditsAtCursor(t *testing.T) {
	a := &App{gitPromptInput: "siteV4", gitPromptCursor: 6}
	a.updateGitPrompt(tea.KeyMsg{Type: tea.KeyLeft})
	a.updateGitPrompt(tea.KeyMsg{Type: tea.KeyLeft})
	a.updateGitPrompt(tea.KeyMsg{Type: tea.KeyDelete})
	a.updateGitPrompt(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})

	if a.gitPromptInput != "sitev4" || a.gitPromptCursor != 5 {
		t.Fatalf("unexpected prompt state: %q at %d", a.gitPromptInput, a.gitPromptCursor)
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

func TestOpenGitBranchHistoryDoesNotOpenCommit(t *testing.T) {
	project := core.Project{
		Path: "/tmp/repo",
		Git: &core.GitInfo{
			IsRepo: true,
			Branch: "main",
			Branches: []core.GitBranch{
				{Name: "main", Current: true},
				{Name: "feature"},
			},
		},
	}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabGit,
		gitSubview:      gitSubviewMain,
		gitFocus:        gitFocusBranches,
		gitBranchCursor: 1,
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
		gitBranches:     project.Git.Branches,
		gitBranchCommits: []core.GitCommit{
			{Hash: "aaa", Message: "latest", Author: "dev"},
		},
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if a.gitSubview != gitSubviewBranch {
		t.Fatalf("expected branch history subview, got %v", a.gitSubview)
	}
	if a.gitSubview == gitSubviewCommit {
		t.Fatal("enter on branch must not open commit detail")
	}
	got := stripANSI(a.renderGitTab(&project))
	if strings.Contains(got, "SCOPE") {
		t.Fatal("branch history must be a dedicated full-width screen")
	}
	for _, want := range []string{"feature", "latest", "COMMITS", "DETALHES", "AUTHORS"} {
		if !strings.Contains(got, want) {
			t.Fatalf("branch history missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderGitDiffLineColors(t *testing.T) {
	add := renderGitDiffLine(gitDiffLine{kind: "add", newNum: 2, text: "+added line"}, 40, 0, false, false)
	remove := renderGitDiffLine(gitDiffLine{kind: "remove", oldNum: 1, text: "-removed line"}, 40, 0, false, false)
	hunk := renderGitDiffLine(gitDiffLine{kind: "hunk", text: "@@ -1,2 +1,2 @@"}, 40, 0, false, false)
	meta := renderGitDiffLine(gitDiffLine{kind: "meta", text: "--- a/file"}, 40, 0, false, false)

	if !strings.Contains(add, "added line") || !strings.Contains(remove, "removed line") {
		t.Fatal("diff lines lost content")
	}
	if add == remove || add == hunk || remove == meta {
		t.Fatal("diff line styles should differ by prefix")
	}
	if !strings.Contains(add, "│") || !strings.Contains(remove, "│") {
		t.Fatal("diff lines should show line number gutter")
	}
}

func TestGitDiffHorizontalScrollRevealsTail(t *testing.T) {
	long := "+" + strings.Repeat("abcdefghi ", 20)
	line := gitDiffLine{kind: "add", newNum: 1, text: long}
	start := renderGitDiffLine(line, 40, 0, false, false)
	scrolled := renderGitDiffLine(line, 40, 30, false, false)
	if start == scrolled {
		t.Fatal("horizontal scroll should change visible diff window")
	}
	if !strings.Contains(scrolled, "abcdefghi") {
		t.Fatal("scrolled diff should still show content")
	}
}

func TestGitCommitDetailShowsSidebarAndDiff(t *testing.T) {
	project := core.Project{Path: "/tmp/repo", Name: "repo", Git: &core.GitInfo{IsRepo: true, Branch: "main"}}
	a := &App{
		width:               100,
		height:              30,
		view:                ViewProject,
		tab:                 TabGit,
		gitSubview:          gitSubviewCommit,
		gitSelectedCommit:   core.GitCommit{Hash: "abc1234", Message: "fix things", Author: "dev", Date: "1 hour ago"},
		gitCommitFullMsg:    "fix things\n\nbody",
		gitCommitFiles:      []core.GitCommitFileChange{{Status: "M", Path: "app/main.go"}, {Status: "A", Path: "app/new.go"}},
		gitCommitFileCursor: 0,
		gitCommitDiff:       "--- a/app/main.go\n+++ b/app/main.go\n@@ -1 +1 @@\n-old\n+new\n",
		selectedProject:     &project,
		snapshot:            core.Snapshot{Projects: []core.Project{project}},
	}

	got := a.renderProject()
	if strings.Contains(stripANSI(got), "SCOPE") {
		t.Fatal("commit detail must hide project sidebar")
	}
	if !strings.Contains(got, "Arquivos") || !strings.Contains(got, "main.go") || !strings.Contains(got, "+new") || !strings.Contains(got, "-old") {
		t.Fatalf("commit detail missing sidebar/diff: %q", got)
	}
}

func TestGitCommitMessageExpandToggle(t *testing.T) {
	a := &App{
		width:             100,
		height:            30,
		gitSubview:        gitSubviewCommit,
		gitSelectedCommit: core.GitCommit{Hash: "abc", Message: "title", Author: "dev"},
		gitCommitFullMsg:  "title\n\nlong body line",
	}
	collapsed := strings.Join(a.renderGitCommitHeaderLines(80), "\n")
	if !strings.Contains(collapsed, "m+") || strings.Contains(collapsed, "long body line") {
		t.Fatalf("collapsed message unexpected: %q", collapsed)
	}
	a.gitCommitMsgExpanded = true
	expanded := strings.Join(a.renderGitCommitHeaderLines(80), "\n")
	if !strings.Contains(expanded, "long body line") {
		t.Fatalf("expanded message missing body: %q", expanded)
	}
}

func TestGitCommitDetailKeepsFileColumnClean(t *testing.T) {
	project := core.Project{Path: "/tmp/repo", Git: &core.GitInfo{IsRepo: true}}
	a := &App{
		width:                100,
		height:               28,
		view:                 ViewProject,
		tab:                  TabGit,
		gitSubview:           gitSubviewCommit,
		gitCommitDetailFocus: gitCommitFocusDiff,
		gitSelectedCommit:    core.GitCommit{Hash: "abc", Message: "msg", Author: "dev"},
		gitCommitFiles:       []core.GitCommitFileChange{{Status: "M", Path: "app/Services/VeryLongServiceName.php"}},
		gitCommitFileCursor:  0,
		gitCommitDiff:        "@@ -1 +1 @@\n+$camposIntegraJson = something very long that used to leak\n+$permiteDocIntegra = another long line\n",
		selectedProject:      &project,
		snapshot:             core.Snapshot{Projects: []core.Project{project}},
	}
	got := a.renderGitCommitDetail(&project)
	// File column should show the filename, not raw diff variable fragments as fake files.
	if strings.Count(got, "Arquivos") != 1 {
		t.Fatal("expected a single Arquivos header")
	}
	if !strings.Contains(got, "VeryLongServiceName.php") {
		t.Fatal("expected file name in sidebar")
	}
}

func TestGitDiffSearchJumpsToMatch(t *testing.T) {
	a := &App{
		width:              100,
		height:             30,
		gitSubview:         gitSubviewCommit,
		gitSelectedCommit:  core.GitCommit{Hash: "abc", Message: "msg"},
		gitCommitDiff:      "@@ -1 +1 @@\n context\n-old value\n+new search-target\n",
		gitDiffSearchQuery: "search-target",
	}
	a.jumpGitDiffSearch(0)
	matches := a.gitDiffSearchMatches()
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if a.gitCommitDiffScroll > matches[0] {
		t.Fatalf("scroll %d should keep match %d visible", a.gitCommitDiffScroll, matches[0])
	}
}

func TestSwitchGitCommitFileUsesCache(t *testing.T) {
	project := core.Project{Path: "/tmp/repo", Git: &core.GitInfo{IsRepo: true}}
	a := &App{
		selectedProject:   &project,
		gitSelectedCommit: core.GitCommit{Hash: "abc"},
		gitCommitFiles: []core.GitCommitFileChange{
			{Status: "M", Path: "a.go"},
			{Status: "M", Path: "b.go"},
		},
		gitCommitFileCursor: 0,
		gitCommitDiffCache: map[string]string{
			"a.go": "diff a",
			"b.go": "diff b",
		},
	}

	cmd := a.switchGitCommitFile(1)
	if cmd != nil {
		t.Fatal("cached file switch should not schedule a load")
	}
	if a.gitCommitFileCursor != 1 || a.gitCommitDiff != "diff b" {
		t.Fatalf("unexpected file/diff state: cursor=%d diff=%q", a.gitCommitFileCursor, a.gitCommitDiff)
	}
}

func TestGitCommitDiffScrollReachesEnd(t *testing.T) {
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	a := &App{
		width:             100,
		height:            28,
		gitSubview:        gitSubviewCommit,
		gitSelectedCommit: core.GitCommit{Hash: "abc", Message: "msg", Author: "dev"},
		gitCommitDiff:     strings.Join(lines, "\n"),
	}
	a.gitCommitDiffScrollBy(100)
	got := a.renderGitCommitDiffBody(a.gitCommitDiffViewport())
	if !strings.Contains(got, "line 40") {
		t.Fatal("diff scroll must reach the last line")
	}
}
