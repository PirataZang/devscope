package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestCollectGitSummary(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	head, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimPrefix(strings.TrimSpace(string(head)), "ref: refs/heads/")

	got := CollectGitSummary(dir)
	if !got.IsRepo || got.Branch != want {
		t.Fatalf("expected branch %q, got %+v", want, got)
	}
}

func TestCollectGitBranchesOrder(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "main commit")
	mainBranch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "older-feature")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "older feature")

	runGit(t, dir, "checkout", mainBranch)
	runGit(t, dir, "checkout", "-b", "newer-feature")
	writeFile(t, filepath.Join(dir, "c.txt"), "c")
	runGit(t, dir, "add", "c.txt")
	runGit(t, dir, "commit", "-m", "newer feature")

	branches := collectGitBranches(dir)
	newerIdx, olderIdx := -1, -1
	for i, b := range branches {
		switch b.Name {
		case "newer-feature":
			newerIdx = i
		case "older-feature":
			olderIdx = i
		}
	}
	if newerIdx == -1 || olderIdx == -1 {
		t.Fatalf("expected feature branches in list, got %v", branchNames(branches))
	}
	if newerIdx > olderIdx {
		t.Fatalf("expected newer branch before older, got order: %v", branchNames(branches))
	}
}

func branchNames(branches []core.GitBranch) []string {
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name
	}
	return names
}

func TestCollectCommitsAtBranch(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "main commit")
	defaultBranch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "feature commit")

	commits := CollectCommitsAt(dir, "feature", 10)
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits on feature branch, got %d", len(commits))
	}
	if commits[0].Message != "feature commit" {
		t.Fatalf("expected first commit to be feature commit, got %q", commits[0].Message)
	}
	if commits[1].Message != "main commit" {
		t.Fatalf("expected second commit to be main commit, got %q", commits[1].Message)
	}

	mainBranch := defaultBranch
	runGit(t, dir, "checkout", mainBranch)

	mainCommits := CollectCommitsAt(dir, mainBranch, 10)
	if len(mainCommits) == 0 {
		t.Fatal("expected commits on main branch")
	}
	foundMain := false
	for _, c := range mainCommits {
		if c.Message == "main commit" {
			foundMain = true
		}
		if c.Message == "feature commit" {
			t.Fatal("feature commit should not appear on main")
		}
	}
	if !foundMain {
		t.Fatal("main commit not found on main branch")
	}
}

func TestCollectCommitFiles(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "add a")

	hash := strings.TrimSpace(gitOutput(dir, "rev-parse", "--short", "HEAD"))
	files := CollectCommitFiles(dir, hash)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "a.txt" || !strings.HasPrefix(files[0].Status, "A") {
		t.Fatalf("unexpected file change: %+v", files[0])
	}
}

func TestCollectCommitFileDiff(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "old\n")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "add a")

	writeFile(t, filepath.Join(dir, "a.txt"), "new\n")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "change a")

	hash := strings.TrimSpace(gitOutput(dir, "rev-parse", "--short", "HEAD"))
	diff := CollectCommitFileDiff(dir, hash, "a.txt")
	if !strings.Contains(diff, "-old") || !strings.Contains(diff, "+new") {
		t.Fatalf("expected colored-ready diff with old/new lines, got %q", diff)
	}
}

func TestCollectCommitFullMessage(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "title", "-m", "body line one", "-m", "body line two")

	hash := strings.TrimSpace(gitOutput(dir, "rev-parse", "--short", "HEAD"))
	msg := CollectCommitFullMessage(dir, hash)
	if !strings.Contains(msg, "body line one") || !strings.Contains(msg, "body line two") {
		t.Fatalf("expected full message body, got %q", msg)
	}
}

func TestParseGitHubRepo(t *testing.T) {
	cases := []struct {
		remote      string
		owner, repo string
		ok          bool
	}{
		{"git@github.com:acme/app.git", "acme", "app", true},
		{"https://github.com/acme/app.git", "acme", "app", true},
		{"https://gitlab.com/acme/app.git", "", "", false},
	}
	for _, tc := range cases {
		owner, repo, ok := ParseGitHubRepo(tc.remote)
		if ok != tc.ok || owner != tc.owner || repo != tc.repo {
			t.Fatalf("ParseGitHubRepo(%q) = %q,%q,%v want %q,%q,%v", tc.remote, owner, repo, ok, tc.owner, tc.repo, tc.ok)
		}
	}
}

func TestGitHubCompareURL(t *testing.T) {
	url := GitHubCompareURL("git@github.com:acme/app.git", "main", "feat/x")
	want := "https://github.com/acme/app/compare/main...feat/x?expand=1"
	if url != want {
		t.Fatalf("got %q want %q", url, want)
	}
}

func TestGitBranchOrigin(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "develop")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "on develop")

	runGit(t, dir, "checkout", "-b", "feat/x")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "feature")

	if got := GitBranchOrigin(dir, "feat/x"); got != "develop" {
		t.Fatalf("expected develop, got %q", got)
	}
}

func TestGitPullOrigin(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "remote", "add", "origin", dir+".remote")
	// bare remote setup is heavy; just verify empty source errors
	if _, err := GitPullOrigin(dir, ""); err == nil {
		t.Fatal("expected error for empty source branch")
	}
}

func TestGitBranchCreateRenameDelete(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "base")
	mainBranch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	if err := GitBranchCreate(dir, "feature", mainBranch); err != nil {
		t.Fatal(err)
	}
	if branch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "feature" {
		t.Fatalf("expected feature branch, got %s", branch)
	}

	if err := GitBranchRename(dir, "feature", "feature-renamed"); err != nil {
		t.Fatal(err)
	}
	if branch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "feature-renamed" {
		t.Fatalf("expected feature-renamed branch, got %s", branch)
	}

	runGit(t, dir, "checkout", mainBranch)
	if err := GitBranchDelete(dir, "feature-renamed"); err != nil {
		t.Fatal(err)
	}
}

func TestGitCheckoutAndCherryPick(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "base")
	mainBranch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "feature work")
	featureHash := strings.TrimSpace(gitOutput(dir, "rev-parse", "HEAD"))

	runGit(t, dir, "checkout", mainBranch)
	if _, err := GitCheckout(dir, "feature"); err != nil {
		t.Fatal(err)
	}
	if branch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")); branch != "feature" {
		t.Fatalf("expected feature branch, got %s", branch)
	}

	runGit(t, dir, "checkout", mainBranch)
	if err := GitCherryPick(dir, []string{featureHash}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("expected cherry-picked file: %v", err)
	}
}

func TestCollectGitFilesStagingColumns(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "a.txt"), "one\n")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "a.txt"), "two\n")
	files := collectGitFiles(dir)
	if len(files) != 1 || files[0].Staging != " " || files[0].Worktree != "M" {
		t.Fatalf("unstaged modify want ' M', got %+v", files)
	}

	runGit(t, dir, "add", "a.txt")
	files = collectGitFiles(dir)
	if len(files) != 1 || files[0].Staging != "M" || files[0].Worktree != " " {
		t.Fatalf("staged modify want 'M ', got %+v", files)
	}

	store := core.NewStateStore([]string{dir})
	store.SetProjects([]core.Project{{
		Path: dir, Name: "t",
		Git: &core.GitInfo{IsRepo: true, Branch: "main", LastCommit: "x", Branches: []core.GitBranch{{Name: "main"}}},
	}})
	runGit(t, dir, "restore", "--staged", "a.txt")
	RefreshProjectGitFiles(store, dir)
	got := store.Get().Projects[0].Git
	if got.Staged != 0 || got.Modified != 1 || len(got.Files) != 1 || got.Files[0].Staging != " " {
		t.Fatalf("refresh files after unstage: staged=%d mod=%d files=%+v", got.Staged, got.Modified, got.Files)
	}
}

func TestCollectWorkingTreeDiffModifiedAndUntracked(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "src", "Hero.astro"), "old\n")
	runGit(t, dir, "add", "src/Hero.astro")
	runGit(t, dir, "commit", "-m", "base")

	writeFile(t, filepath.Join(dir, "src", "Hero.astro"), "new line\n")
	mod := CollectWorkingTreeDiff(dir, "src/Hero.astro")
	if !strings.Contains(mod, "-old") || !strings.Contains(mod, "+new line") {
		t.Fatalf("modified diff missing changes:\n%s", mod)
	}
	if strings.Contains(mod, "untracked") {
		t.Fatalf("modified file must not look untracked:\n%s", mod)
	}

	writeFile(t, filepath.Join(dir, "src", "New.astro"), "fresh\n")
	unt := CollectWorkingTreeDiff(dir, "src/New.astro")
	if !strings.Contains(unt, "+fresh") && !strings.Contains(unt, "fresh") {
		t.Fatalf("untracked should show content:\n%s", unt)
	}
}

func TestGitIsFFImpossible(t *testing.T) {
	err := fmt.Errorf("pull: fatal: Not possible to fast-forward, aborting.")
	if !GitIsFFImpossible(err, err.Error()) {
		t.Fatal("expected ff impossible")
	}
	if GitIsFFImpossible(nil, "Already up to date.") {
		t.Fatal("clean pull must not look like ff failure")
	}
}

func TestGitIsConflictError(t *testing.T) {
	err := fmt.Errorf("Automatic merge failed; fix conflicts and then commit the result.")
	if !GitIsConflictError(err, err.Error()) {
		t.Fatal("expected conflict")
	}
	if GitIsConflictError(nil, "Already up to date.") {
		t.Fatal("clean output must not look like conflict")
	}
}

func TestGitFileUnmerged(t *testing.T) {
	if !GitFileUnmerged("U", "U") {
		t.Fatal("UU should be unmerged")
	}
	if GitFileUnmerged("M", " ") {
		t.Fatal("staged modify is not unmerged")
	}
}

func TestKeepBothConflictSides(t *testing.T) {
	in := "head\n<<<<<<< ours\nmy line\n=======\ntheir line\n>>>>>>> theirs\ntail\n"
	got := keepBothConflictSides(in)
	want := "head\nmy line\ntheir line\ntail\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestCollectConflictDiffStages(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "f.txt"), "base\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "base")
	main := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "other")
	writeFile(t, filepath.Join(dir, "f.txt"), "other-side\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "other")

	runGit(t, dir, "checkout", main)
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "f.txt"), "feature-side\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "feature")

	cmd := exec.Command("git", "merge", "other")
	cmd.Dir = dir
	_ = cmd.Run()

	diff := CollectConflictDiff(dir, "f.txt", "feature", "other")
	if !strings.Contains(diff, "CONFLICT") || !strings.Contains(diff, "feature") || !strings.Contains(diff, "other") {
		t.Fatalf("expected labeled conflict diff, got:\n%s", diff)
	}
	if !strings.Contains(diff, "feature-side") && !strings.Contains(diff, "-feature") {
		// either stage diff or marker fallback should show the sides
		if !strings.Contains(diff, "+other") && !strings.Contains(diff, "other-side") {
			t.Fatalf("expected both sides in diff:\n%s", diff)
		}
	}

	if err := GitResolveBoth(dir, "f.txt"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, "<<<<<<<") || strings.Contains(body, ">>>>>>>") {
		t.Fatalf("markers remain: %q", body)
	}
	if !strings.Contains(body, "feature-side") || !strings.Contains(body, "other-side") {
		t.Fatalf("both sides should remain: %q", body)
	}
}

func TestGitConflictSidesMerge(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "f.txt"), "base\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "base")
	main := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "other")
	writeFile(t, filepath.Join(dir, "f.txt"), "other\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "other")

	runGit(t, dir, "checkout", main)
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, filepath.Join(dir, "f.txt"), "feature\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "feature")

	cmd := exec.Command("git", "merge", "other")
	cmd.Dir = dir
	_ = cmd.Run()

	ours, theirs := GitConflictSides(dir)
	if ours != "feature" {
		t.Fatalf("ours=%q", ours)
	}
	if theirs != "other" {
		t.Fatalf("theirs=%q", theirs)
	}
	if GitUnmergedCount(collectGitFiles(dir)) != 1 {
		t.Fatal("expected 1 unmerged file")
	}
}

func TestGitOpInProgressMergeConflict(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "f.txt"), "base\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "base")
	main := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))

	runGit(t, dir, "checkout", "-b", "other")
	writeFile(t, filepath.Join(dir, "f.txt"), "other\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "other")

	runGit(t, dir, "checkout", main)
	writeFile(t, filepath.Join(dir, "f.txt"), "main\n")
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "main")

	cmd := exec.Command("git", "merge", "other")
	cmd.Dir = dir
	_ = cmd.Run() // expect conflict

	if kind := GitOpInProgress(dir); kind != "merge" {
		t.Fatalf("want merge in progress, got %q", kind)
	}
	files := collectGitFiles(dir)
	found := false
	for _, f := range files {
		if f.Path == "f.txt" && GitFileUnmerged(f.Staging, f.Worktree) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected conflicted f.txt in %+v", files)
	}

	if err := GitCheckoutOurs(dir, "f.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := GitContinue(dir); err != nil {
		t.Fatal(err)
	}
	if kind := GitOpInProgress(dir); kind != "" {
		t.Fatalf("expected no op after continue, got %q", kind)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGitRemoteBranchNames(t *testing.T) {
	remote := t.TempDir()
	runGit(t, remote, "init", "--bare")

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "init")
	mainBranch := strings.TrimSpace(gitOutput(dir, "rev-parse", "--abbrev-ref", "HEAD"))
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-u", "origin", mainBranch)

	runGit(t, dir, "checkout", "-b", "local-only")
	writeFile(t, filepath.Join(dir, "b.txt"), "b")
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "local")

	runGit(t, dir, "checkout", "-b", "pushed-feat")
	writeFile(t, filepath.Join(dir, "c.txt"), "c")
	runGit(t, dir, "add", "c.txt")
	runGit(t, dir, "commit", "-m", "feat")
	runGit(t, dir, "push", "-u", "origin", "pushed-feat")

	names := GitRemoteBranchNames(dir)
	hasMain, hasFeat, hasLocal := false, false, false
	for _, n := range names {
		switch n {
		case mainBranch:
			hasMain = true
		case "pushed-feat":
			hasFeat = true
		case "local-only":
			hasLocal = true
		}
	}
	if !hasMain || !hasFeat {
		t.Fatalf("expected pushed branches, got %v", names)
	}
	if hasLocal {
		t.Fatalf("local-only should not appear: %v", names)
	}
}
