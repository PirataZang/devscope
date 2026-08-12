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
		"BRANCHES", "COMMITS", "MODIFIED FILES", "COMMAND LOG",
		"ACTIVITY", "STASHES", "REMOTES", "AÇÕES",
		"DES-2834", "stash@{0}", "origin",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("git main missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "DIFF ·") || strings.Contains(got, "\nDIFF\n") {
		t.Fatal("main git view should not show inline DIFF panel")
	}
}

func TestExtractGitURL(t *testing.T) {
	got := extractGitURL("  https://github.com/acme/app/compare/main...feat?expand=1")
	if got != "https://github.com/acme/app/compare/main...feat?expand=1" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderGitCommandLogShowsOutput(t *testing.T) {
	a := &App{
		width: 100, height: 40,
		gitFocus: gitFocusCmdLog,
		gitCommandLog: []gitCmdLogEntry{
			{Title: "Push", Cmdline: "git push", Output: "Create a pull request:\n  https://github.com/acme/app/pull/new/feat"},
		},
	}
	got := stripANSI(a.renderGitCommandLog(60, 10))
	if !strings.Contains(got, "COMMAND LOG") || !strings.Contains(got, "git push") || !strings.Contains(got, "https://github.com") {
		t.Fatalf("%q", got)
	}
}

func TestGitFileLinesShowsStagedBadge(t *testing.T) {
	g := &core.GitInfo{
		IsRepo: true, Branch: "main", Staged: 1,
		Files: []core.GitFileStatus{
			{Path: "dirty.go", Staging: " ", Worktree: "M"},
			{Path: "ready.go", Staging: "A", Worktree: " "},
		},
	}
	a := &App{gitFocus: gitFocusFiles, gitFileCursor: 1, gitFileTreeCursor: 1, gitViewBranch: "main"}
	got := stripANSI(strings.Join(a.gitFileLines(g, "main", 10), "\n"))
	if !strings.Contains(got, "ready.go") || !strings.Contains(got, "● staged") {
		t.Fatalf("expected staged badge, got %q", got)
	}
	if strings.Count(got, "● staged") != 1 {
		t.Fatalf("only staged file should show badge: %q", got)
	}
	if !gitFileStaged(g.Files[1]) || gitFileStaged(g.Files[0]) {
		t.Fatal("worktree-only M must not count as staged")
	}
}

func TestMoveWTFileTreeCursorSkipsFolders(t *testing.T) {
	files := []core.GitFileStatus{
		{Path: "app/Http/Controllers/SetorController.php", Staging: " ", Worktree: "M"},
		{Path: "resources/views/setor/alterar.blade.php", Staging: " ", Worktree: "M"},
		{Path: "features/foo.json", Staging: "?", Worktree: "?"},
	}
	rows := wtFileTreeFrom(files)
	// start snapped off leading folder onto first file
	cur := moveWTFileTreeCursor(rows, 0, 0)
	if rows[cur].isDir || rows[cur].label != "SetorController.php" {
		t.Fatalf("start=%+v", rows[cur])
	}
	cur = moveWTFileTreeCursor(rows, cur, 1)
	if rows[cur].label != "alterar.blade.php" {
		t.Fatalf("down should skip folders, got %+v", rows[cur])
	}
	cur = moveWTFileTreeCursor(rows, cur, 1)
	if rows[cur].label != "foo.json" {
		t.Fatalf("down again=%+v", rows[cur])
	}
	cur = moveWTFileTreeCursor(rows, cur, -1)
	if rows[cur].label != "alterar.blade.php" {
		t.Fatalf("up should skip folders, got %+v", rows[cur])
	}
}

func TestGitFileLinesShowsTree(t *testing.T) {
	g := &core.GitInfo{
		IsRepo: true, Branch: "main",
		Files: []core.GitFileStatus{
			{Path: "app/Http/Controllers/SetorController.php", Staging: " ", Worktree: "M"},
			{Path: "resources/views/setor/alterar.blade.php", Staging: " ", Worktree: "M"},
			{Path: "features/foo.json", Staging: "?", Worktree: "?"},
		},
	}
	a := &App{gitFocus: gitFocusFiles, gitViewBranch: "main"}
	got := stripANSI(strings.Join(a.gitFileLines(g, "main", 20), "\n"))
	for _, want := range []string{"▾ app/Http/Controllers", "SetorController.php", "▾ resources/views/setor", "alterar.blade.php", "▾ features", "foo.json"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in tree: %q", want, got)
		}
	}
	if strings.Contains(got, "app/Http/Controllers/SetorController.php") {
		t.Fatal("should show basename under folder, not full flat path")
	}
}

func TestGitConflictFileLinesOnlyUnmerged(t *testing.T) {
	g := &core.GitInfo{
		IsRepo: true, Branch: "feat",
		Files: []core.GitFileStatus{
			{Path: "ok.php", Staging: "M", Worktree: " "},
			{Path: "app/Conflict.php", Staging: "U", Worktree: "U"},
			{Path: "other/Also.php", Staging: "U", Worktree: "U"},
			{Path: "clean.js", Staging: "A", Worktree: " "},
		},
	}
	a := &App{
		gitFocus: gitFocusFiles, gitViewBranch: "feat", gitConflictOn: true,
		gitConflictOurs: "feat", gitConflictTheirs: "develop", width: 100,
	}
	got := stripANSI(strings.Join(a.gitFileLines(g, "feat", 20), "\n"))
	if !strings.Contains(got, "CONFLICT") || !strings.Contains(got, "o=feat") || !strings.Contains(got, "t=develop") {
		t.Fatalf("expected conflict legend, got %q", got)
	}
	if !strings.Contains(got, "app/Conflict.php") || !strings.Contains(got, "other/Also.php") {
		t.Fatalf("expected only conflict paths, got %q", got)
	}
	if strings.Contains(got, "ok.php") || strings.Contains(got, "clean.js") || strings.Contains(got, "staged") {
		t.Fatalf("clean merges must be hidden during conflict: %q", got)
	}
	title := stripANSI(a.renderGitWorkingRow(g, "feat", 60, 12))
	if !strings.Contains(title, "CONFLICTS · 2 left") {
		t.Fatalf("title=%q", title)
	}
}

func TestGitConflictDiffScreen(t *testing.T) {
	p := core.Project{
		Name: "demo", Path: "/p",
		Git: &core.GitInfo{
			IsRepo: true, Branch: "feat",
			Files: []core.GitFileStatus{{Path: "app/Conflict.php", Staging: "U", Worktree: "U"}},
		},
	}
	a := &App{
		width: 100, height: 30, view: ViewProject, tab: TabGit,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		gitConflictOn: true, gitConflictOurs: "feat", gitConflictTheirs: "develop",
		gitSubview: gitSubviewFileDiff, gitWTDiffConflict: true,
		gitWTDiffFile: "app/Conflict.php",
		gitWTDiff: "CONFLICT  o=feat  (−)  vs  t=develop  (+)\n" +
			"--- a/app/Conflict.php  (feat / ours)\n" +
			"+++ b/app/Conflict.php  (develop / theirs)\n" +
			"@@ -1 +1 @@\n-mine\n+theirs\n",
		gitViewBranch: "feat", gitFocus: gitFocusFiles,
	}
	got := stripANSI(a.renderGitFileDiff(&p))
	for _, want := range []string{"CONFLICT DIFF", "feat", "develop", "o=", "t=", "b=ambas", "-mine", "+theirs"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestGitAddFileTogglesUnstage(t *testing.T) {
	p := core.Project{
		Path: "/tmp/repo", Name: "repo",
		Git: &core.GitInfo{
			IsRepo: true, Branch: "main",
			Files: []core.GitFileStatus{{Path: "a.go", Staging: "A", Worktree: " "}},
		},
	}
	a := &App{gitFocus: gitFocusFiles, selectedProject: &p}
	if cmd := a.gitAddFile(&p); cmd == nil {
		t.Fatal("expected unstage cmd")
	}
	if a.gitStatusMsg != "unstage a.go…" {
		t.Fatalf("status=%q", a.gitStatusMsg)
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
		gitFocus:          gitFocusFiles,
		gitViewBranch:     "main",
		gitFileCursor:     0,
		gitFileTreeCursor: 1, // file under ▾ src
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

func TestGitBranchFilterLiveInline(t *testing.T) {
	a := &App{
		view: ViewProject,
		tab:  TabGit,
		selectedProject: &core.Project{
			Path: "/tmp/repo",
			Git: &core.GitInfo{
				IsRepo: true,
				Branches: []core.GitBranch{
					{Name: "develop"},
					{Name: "feat/kanban"},
					{Name: "master"},
				},
			},
		},
		gitBranchFilterOn: true,
	}
	a.gitBranches = a.selectedProject.Git.Branches
	for _, ch := range "feat" {
		_, _ = a.updateGitBranchFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if a.gitBranchFilter != "feat" || !a.gitBranchFilterOn {
		t.Fatalf("live filter: on=%v filter=%q", a.gitBranchFilterOn, a.gitBranchFilter)
	}
	got := a.filteredGitBranches(a.gitBranches)
	if len(got) != 1 || got[0].Name != "feat/kanban" {
		t.Fatalf("live filter result: %+v", got)
	}
	line := stripANSI(a.renderGitBranchFilterLine(80))
	if !strings.Contains(line, "filter") || !strings.Contains(line, "feat") {
		t.Fatalf("inline filter line: %q", line)
	}
	_, _ = a.updateGitBranchFilter(tea.KeyMsg{Type: tea.KeyEnter})
	if a.gitBranchFilterOn || a.gitBranchFilter != "feat" {
		t.Fatalf("after enter: on=%v filter=%q", a.gitBranchFilterOn, a.gitBranchFilter)
	}
}

func TestGitDiffSearchInlineLive(t *testing.T) {
	a := &App{
		gitSubview:         gitSubviewCommit,
		gitDiffSearchOn:    true,
		gitCommitDiff:      "@@ -1 +1 @@\n context\n-old value\n+new search-target\n",
		gitDiffSearchInput: "",
	}
	for _, ch := range "search-target" {
		_, _ = a.updateGitDiffSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if a.gitDiffSearchQuery != "search-target" || !a.gitDiffSearchOn {
		t.Fatalf("live search: on=%v q=%q", a.gitDiffSearchOn, a.gitDiffSearchQuery)
	}
	if len(a.gitDiffSearchMatches()) == 0 {
		t.Fatal("expected matches while typing")
	}
	line := stripANSI(a.renderGitDiffSearchLine(80))
	if !strings.Contains(line, "search") || !strings.Contains(line, "search-target") {
		t.Fatalf("inline search line: %q", line)
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

func TestBuildCommitFileTreeCompactsFolders(t *testing.T) {
	files := []core.GitCommitFileChange{
		{Status: "M", Path: "src/components/HomeCtaSection.astro"},
		{Status: "M", Path: "src/components/HomeFAQSection.astro"},
		{Status: "M", Path: "astro.config.mjs"},
	}
	rows := buildCommitFileTree(files, nil)
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.isDir {
			got = append(got, fmt.Sprintf("dir:%d:%s", r.depth, r.label))
		} else {
			got = append(got, fmt.Sprintf("file:%d:%s:%s:%d", r.depth, r.status, r.label, r.fileIdx))
		}
	}
	want := []string{
		"dir:0:src/components",
		"file:1:M:HomeCtaSection.astro:0",
		"file:1:M:HomeFAQSection.astro:1",
		"file:0:M:astro.config.mjs:2",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("tree=%v want=%v", got, want)
	}

	collapsed := buildCommitFileTree(files, map[string]bool{"src/components": true})
	if len(collapsed) != 2 || !collapsed[0].isDir || collapsed[0].label != "src/components" || collapsed[1].label != "astro.config.mjs" {
		t.Fatalf("collapsed tree unexpected: %+v", collapsed)
	}
}

func TestGitCommitTreeKeysOpenCollapse(t *testing.T) {
	project := core.Project{Path: "/tmp/repo", Git: &core.GitInfo{IsRepo: true}}
	a := &App{
		width:                100,
		height:               30,
		selectedProject:      &project,
		gitSubview:           gitSubviewCommit,
		gitCommitDetailFocus: gitCommitFocusFiles,
		gitSelectedCommit:    core.GitCommit{Hash: "abc"},
		gitCommitFiles: []core.GitCommitFileChange{
			{Status: "M", Path: "src/components/A.astro"},
			{Status: "M", Path: "src/components/B.astro"},
		},
		gitCommitTreeCursor: 0,
		gitCommitDiffCache:  map[string]string{"src/components/A.astro": "diff a"},
	}

	a.collapseGitCommitTree()
	if !a.gitCommitCollapsed["src/components"] {
		t.Fatal("left should collapse folder")
	}
	a.expandGitCommitTree()
	if a.gitCommitCollapsed["src/components"] {
		t.Fatal("right should expand folder")
	}

	a.gitCommitTreeCursor = 1 // first file under folder
	cmd := a.previewGitCommitTreeSelection()
	if cmd != nil {
		t.Fatal("cached preview should not schedule load")
	}
	if a.gitCommitFileOpen || a.gitCommitDetailFocus != gitCommitFocusFiles || a.gitCommitDiff != "diff a" {
		t.Fatalf("preview should load sideways without access: open=%v focus=%v diff=%q", a.gitCommitFileOpen, a.gitCommitDetailFocus, a.gitCommitDiff)
	}

	cmd = a.openSelectedGitCommitFile()
	if cmd != nil {
		t.Fatal("enter on already previewed file should not reload")
	}
	if !a.gitCommitFileOpen || a.gitCommitDetailFocus != gitCommitFocusDiff {
		t.Fatalf("enter should access diff panel: open=%v focus=%v", a.gitCommitFileOpen, a.gitCommitDetailFocus)
	}

	_, _ = a.handleGitDedicatedKeys(tea.KeyMsg{Type: tea.KeyEsc}, &project)
	if a.gitCommitFileOpen || a.gitCommitDetailFocus != gitCommitFocusFiles || a.gitSubview != gitSubviewCommit || a.gitCommitDiff != "diff a" {
		t.Fatalf("esc should leave access but keep preview: open=%v focus=%v sub=%v diff=%q", a.gitCommitFileOpen, a.gitCommitDetailFocus, a.gitSubview, a.gitCommitDiff)
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
		gitCommitTreeCursor: 1,
		gitCommitFileOpen:   true,
		gitCommitDiff:       "--- a/app/main.go\n+++ b/app/main.go\n@@ -1 +1 @@\n-old\n+new\n",
		selectedProject:     &project,
		snapshot:            core.Snapshot{Projects: []core.Project{project}},
	}

	got := a.renderProject()
	if strings.Contains(stripANSI(got), "SCOPE") {
		t.Fatal("commit detail must hide project sidebar")
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "Arquivos") || !strings.Contains(plain, "▾ app") || !strings.Contains(plain, "main.go") || !strings.Contains(got, "+new") || !strings.Contains(got, "-old") {
		t.Fatalf("commit detail missing sidebar/diff: %q", plain)
	}
}

func TestCommitFileChangeCounts(t *testing.T) {
	files := []core.GitCommitFileChange{
		{Status: "A", Path: "new.go"},
		{Status: "A", Path: "new2.go"},
		{Status: "M", Path: "edit.go"},
		{Status: "R", Path: "renamed.go"},
		{Status: "D", Path: "gone.go"},
	}
	add, mod, del := commitFileChangeCounts(files)
	if add != 2 || mod != 2 || del != 1 {
		t.Fatalf("counts A=%d M=%d D=%d", add, mod, del)
	}
}

func TestGitCommitDetailShowsFileStats(t *testing.T) {
	project := core.Project{Path: "/tmp/repo", Name: "repo", Git: &core.GitInfo{IsRepo: true}}
	a := &App{
		width:  100,
		height: 30,
		view:   ViewProject,
		tab:    TabGit,
		gitSubview: gitSubviewCommit,
		gitSelectedCommit: core.GitCommit{Hash: "abc1234", Message: "stats", Author: "dev", Date: "now"},
		gitCommitFiles: []core.GitCommitFileChange{
			{Status: "A", Path: "a.go"},
			{Status: "M", Path: "b.go"},
			{Status: "D", Path: "c.go"},
		},
		selectedProject: &project,
		snapshot:        core.Snapshot{Projects: []core.Project{project}},
	}
	plain := stripANSI(a.renderGitCommitDetail(&project))
	for _, want := range []string{"A 1", "novos", "M 1", "alterados", "D 1", "deletados", "3 arquivos", "Arquivos (3)"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, truncate(plain, 500))
		}
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
	if !strings.Contains(stripANSI(got), "VeryLongServiceName") {
		t.Fatalf("expected file name in sidebar: %q", stripANSI(got))
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
		gitCommitFileOpen:    true,
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

func TestGitComposeModalMultiline(t *testing.T) {
	p := core.Project{
		Path: "/tmp/repo",
		Name: "repo",
		Git: &core.GitInfo{
			IsRepo: true, Branch: "main", Staged: 2, Modified: 1,
			Branches: []core.GitBranch{{Name: "main", Current: true}},
		},
	}
	a := &App{
		width: 100, height: 30,
		view: ViewProject, tab: TabGit, gitSubview: gitSubviewMain,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.startGitCompose(&p)
	if !a.gitComposeOn {
		t.Fatal("compose not open")
	}
	_, _ = a.updateGitCompose(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("fix")})
	_, _ = a.updateGitCompose(tea.KeyMsg{Type: tea.KeyEnter})
	_, _ = a.updateGitCompose(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("body")})
	if !strings.Contains(a.gitComposeMsg, "fix\nbody") {
		t.Fatalf("multiline msg=%q", a.gitComposeMsg)
	}
	got := stripANSI(a.renderGitCompose())
	for _, want := range []string{"GIT", "Novo commit", "main", "STAGED", "MODIFIED", "preview", "Commitar", "Cancelar"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, truncate(got, 300))
		}
	}
	// enter in editor must NOT commit
	if !a.gitComposeOn {
		t.Fatal("enter should stay in editor")
	}
	_, _ = a.updateGitCompose(tea.KeyMsg{Type: tea.KeyTab})
	if a.gitComposeFocus != gitComposeFocusCommit {
		t.Fatalf("tab should focus commit button, got %v", a.gitComposeFocus)
	}
	_, cmd := a.updateGitCompose(tea.KeyMsg{Type: tea.KeyEnter})
	if a.gitComposeOn {
		t.Fatal("enter on Commitar should submit")
	}
	if cmd == nil {
		t.Fatal("expected commit cmd")
	}
}

func TestGitNewBranchPromptIsModal(t *testing.T) {
	p := core.Project{
		Path: "/tmp/repo",
		Name: "repo",
		Git:  &core.GitInfo{IsRepo: true, Branch: "main", Branches: []core.GitBranch{{Name: "main", Current: true}}},
	}
	a := &App{
		width: 100, height: 30,
		view: ViewProject, tab: TabGit, gitSubview: gitSubviewMain,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.startGitNewBranch(&p)
	if !a.gitPromptOn || a.gitPromptKind != gitPromptNewBranch {
		t.Fatal("prompt not started")
	}
	got := stripANSI(a.renderGitPrompt())
	for _, want := range []string{"Nova branch", "a partir de", "main", "enter cria"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, truncate(got, 250))
		}
	}
}
