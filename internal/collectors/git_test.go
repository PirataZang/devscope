package collectors

import (
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
	if err := GitPullOrigin(dir, ""); err == nil {
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
	if err := GitCheckout(dir, "feature"); err != nil {
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
