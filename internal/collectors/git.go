package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devscope/devscope/internal/core"
)

func Collect(path string) *core.GitInfo {
	return CollectAt(path)
}

// CollectGitSummary reads only .git/HEAD for the dashboard.
func CollectGitSummary(path string) *core.GitInfo {
	root := gitRepoRoot(path)
	if root == "" {
		return &core.GitInfo{IsRepo: false}
	}
	head, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return &core.GitInfo{IsRepo: true}
	}
	branch := strings.TrimSpace(string(head))
	if strings.HasPrefix(branch, "ref: refs/heads/") {
		branch = strings.TrimPrefix(branch, "ref: refs/heads/")
	} else {
		branch = "HEAD"
	}
	return &core.GitInfo{IsRepo: true, Branch: branch}
}

func CollectAt(path string) *core.GitInfo {
	if !isGitRepo(path) {
		return &core.GitInfo{IsRepo: false}
	}

	info := &core.GitInfo{IsRepo: true}

	var (
		branch     string
		commitData string
		remote     string
		files      []core.GitFileStatus
		branches   []core.GitBranch
		stash      string
		upstream   string
	)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() {
		defer wg.Done()
		branch = strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", "HEAD"))
	}()
	go func() {
		defer wg.Done()
		commitData = strings.TrimSpace(gitOutput(path, "log", "-1", "--pretty=format:%h|%s|%an|%ci"))
	}()
	go func() {
		defer wg.Done()
		remote = strings.TrimSpace(gitOutput(path, "config", "--get", "remote.origin.url"))
	}()
	go func() {
		defer wg.Done()
		files = collectGitFiles(path)
	}()
	go func() {
		defer wg.Done()
		branches = collectGitBranches(path)
	}()
	go func() {
		defer wg.Done()
		stash = strings.TrimSpace(gitOutput(path, "stash", "list"))
	}()
	wg.Wait()

	info.Branch = branch
	if commitData != "" {
		parts := strings.SplitN(commitData, "|", 4)
		if len(parts) == 4 {
			info.LastCommit = parts[0]
			info.LastCommitMsg = parts[1]
			info.Author = parts[2]
			if t, err := time.Parse("2006-01-02 15:04:05 -0700", parts[3]); err == nil {
				info.LastCommitDate = t
			}
		}
	}
	info.Remote = remote
	info.Files = files
	info.Modified = 0
	info.Staged = 0
	info.Untracked = 0
	for _, f := range info.Files {
		if f.Staging == "?" || f.Worktree == "?" {
			info.Untracked++
			continue
		}
		if f.Staging != " " && f.Staging != "" {
			info.Staged++
		}
		if f.Worktree != " " && f.Worktree != "" {
			info.Modified++
		}
	}
	info.Branches = branches
	info.Commits = collectGitCommits(path, info.Branch, 20)
	info.Stashes = parseGitStashes(stash)
	info.StashCount = len(info.Stashes)
	info.Remotes = collectGitRemotes(path)
	if info.Remote == "" && len(info.Remotes) > 0 {
		info.Remote = info.Remotes[0].URL
	}

	upstream = strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", "@{upstream}"))
	if upstream != "" && !strings.Contains(upstream, "fatal:") && !strings.Contains(upstream, "@") {
		aheadBehind := gitOutput(path, "rev-list", "--left-right", "--count", "HEAD..."+upstream)
		parts := strings.Fields(aheadBehind)
		if len(parts) == 2 {
			info.Ahead, _ = strconv.Atoi(parts[0])
			info.Behind, _ = strconv.Atoi(parts[1])
		}
	}

	return info
}

func parseGitStashes(stashList string) []core.GitStash {
	stashList = strings.TrimSpace(stashList)
	if stashList == "" {
		return nil
	}
	var out []core.GitStash
	for _, line := range strings.Split(stashList, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// stash@{0}: WIP on main: abc message
		ref := line
		msg := ""
		if i := strings.Index(line, ":"); i >= 0 {
			ref = strings.TrimSpace(line[:i])
			msg = strings.TrimSpace(line[i+1:])
		}
		out = append(out, core.GitStash{Ref: ref, Message: msg})
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func collectGitRemotes(path string) []core.GitRemote {
	out := strings.TrimSpace(gitOutput(path, "remote", "-v"))
	if out == "" {
		return nil
	}
	seen := map[string]bool{}
	var remotes []core.GitRemote
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		name, url := f[0], f[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		remotes = append(remotes, core.GitRemote{Name: name, URL: url})
	}
	return remotes
}

// CollectWorkingTreeDiff returns unstaged/staged/HEAD diff for a path.
func CollectWorkingTreeDiff(repo, file string) string {
	if repo == "" || file == "" {
		return ""
	}
	file = strings.TrimSpace(file)
	if diff := strings.TrimSpace(gitDiffOutput(repo, "diff", "--", file)); diff != "" {
		return diff
	}
	if diff := strings.TrimSpace(gitDiffOutput(repo, "diff", "--cached", "--", file)); diff != "" {
		return diff
	}
	if diff := strings.TrimSpace(gitDiffOutput(repo, "diff", "HEAD", "--", file)); diff != "" {
		return diff
	}

	st := strings.TrimSpace(gitOutput(repo, "status", "--porcelain", "--", file))
	if st == "" {
		return "(sem alterações neste arquivo)"
	}
	xy := st
	if len(xy) >= 2 {
		xy = xy[:2]
	}
	// Untracked file or directory
	if strings.HasPrefix(xy, "??") || strings.HasPrefix(xy, "A ") {
		if strings.HasSuffix(file, "/") {
			return "(diretório untracked — abra um arquivo dentro)"
		}
		if diff := strings.TrimSpace(gitDiffOutput(repo, "diff", "--no-index", "/dev/null", file)); diff != "" {
			return diff
		}
		return "(untracked — sem conteúdo para diff)"
	}
	return "(sem diff textual — binário ou mudança de modo)"
}

func collectGitFiles(path string) []core.GitFileStatus {
	status := gitOutput(path, "status", "--porcelain")
	if status == "" {
		return nil
	}
	var files []core.GitFileStatus
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		if len(line) < 3 {
			continue
		}
		staging, worktree := string(line[0]), string(line[1])
		rest := strings.TrimSpace(line[2:])
		// renames: "old -> new"
		if i := strings.Index(rest, " -> "); i >= 0 {
			rest = strings.TrimSpace(rest[i+4:])
		}
		rest = strings.Trim(rest, `"`)
		if rest == "" {
			continue
		}
		files = append(files, core.GitFileStatus{
			Staging:  staging,
			Worktree: worktree,
			Path:     rest,
		})
	}
	return files
}

func CollectCommitsAt(path, branch string, limit int) []core.GitCommit {
	if branch == "" {
		branch = "HEAD"
	}
	if limit <= 0 {
		limit = 20
	}
	return collectGitCommits(path, branch, limit)
}

func CollectCommitFiles(path, hash string) []core.GitCommitFileChange {
	full := strings.TrimSpace(gitOutput(path, "rev-parse", hash))
	if full == "" {
		full = hash
	}
	out := gitOutput(path, "show", "--name-status", "--pretty=format:", full)
	if out == "" {
		out = gitOutput(path, "diff-tree", "--root", "--no-commit-id", "--name-status", "-r", full)
	}
	return parseCommitFileChanges(out)
}

func CollectCommitFullMessage(path, hash string) string {
	full := strings.TrimSpace(gitOutput(path, "rev-parse", hash))
	if full == "" {
		full = hash
	}
	msg := strings.TrimSpace(gitOutput(path, "log", "-1", "--format=%B", full))
	if msg == "" {
		msg = strings.TrimSpace(gitOutput(path, "log", "-1", "--format=%s", full))
	}
	return msg
}

func CollectCommitFileDiff(path, hash, filePath string) string {
	if filePath == "" {
		return ""
	}
	full := strings.TrimSpace(gitOutput(path, "rev-parse", hash))
	if full == "" {
		full = hash
	}
	return gitOutput(path, "show", "--format=", "--no-ext-diff", full, "--", filePath)
}

func parseCommitFileChanges(out string) []core.GitCommitFileChange {
	if out == "" {
		return nil
	}
	var files []core.GitCommitFileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		files = append(files, core.GitCommitFileChange{
			Status: parts[0],
			Path:   parts[len(parts)-1],
		})
	}
	return files
}

func collectGitCommits(path, branch string, limit int) []core.GitCommit {
	logRef := branchLogRef(path, branch)
	out := gitOutput(path, "log", logRef, fmt.Sprintf("-%d", limit), "--pretty=format:%h|%s|%an|%cr")
	if out == "" {
		return nil
	}
	var commits []core.GitCommit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, core.GitCommit{
			Hash:    parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits
}

// branchLogRef returns the branch name directly to show its full history.
func branchLogRef(path, branch string) string {
	if branch == "" || branch == "HEAD" {
		return "HEAD"
	}
	return branch
}

func isTrunkBranch(path, branch string) bool {
	for _, trunk := range []string{"main", "master", "develop"} {
		if branch != trunk {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", branch)) != "" {
			return true
		}
	}
	return false
}

func findBranchBase(path, branch string) string {
	upstream := strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", branch+"@{upstream}"))
	if upstream != "" && upstream != branch {
		if mb := strings.TrimSpace(gitOutput(path, "merge-base", branch, upstream)); mb != "" {
			return mb
		}
	}
	for _, candidate := range []string{"main", "master", "develop"} {
		if candidate == branch {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", candidate)) == "" {
			continue
		}
		if mb := strings.TrimSpace(gitOutput(path, "merge-base", branch, candidate)); mb != "" {
			return mb
		}
	}
	return ""
}

func collectGitBranches(path string) []core.GitBranch {
	out := gitOutput(path, "for-each-ref", "refs/heads/", "--format=%(committerdate:unix)|%(creatordate:unix)|%(refname:short)|%(HEAD)")
	if out == "" {
		return nil
	}
	type branchEntry struct {
		committer int64
		created   int64
		branch    core.GitBranch
	}
	var entries []branchEntry
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 3 || parts[2] == "" {
			continue
		}
		committer, _ := strconv.ParseInt(parts[0], 10, 64)
		created, _ := strconv.ParseInt(parts[1], 10, 64)
		entries = append(entries, branchEntry{
			committer: committer,
			created:   created,
			branch: core.GitBranch{
				Name:    parts[2],
				Current: len(parts) > 3 && parts[3] == "*",
			},
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].committer != entries[j].committer {
			return entries[i].committer > entries[j].committer
		}
		return entries[i].created > entries[j].created
	})
	branches := make([]core.GitBranch, len(entries))
	for i, e := range entries {
		branches[i] = e.branch
	}
	return branches
}

func gitOutput(path string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// gitDiffOutput keeps stdout even when git exits 1 (differences found / --no-index).
func gitDiffOutput(path string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok && len(out) > 0 {
			return string(out)
		}
		return ""
	}
	return string(out)
}

func gitRun(path string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func GitCheckout(path, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch vazia")
	}
	return gitRun(path, "checkout", branch)
}

func GitCherryPick(path string, hashes []string) error {
	if len(hashes) == 0 {
		return fmt.Errorf("nenhum commit para cherry-pick")
	}
	args := append([]string{"cherry-pick"}, hashes...)
	return gitRun(path, args...)
}

func GitResolveHash(path, ref string) string {
	full := strings.TrimSpace(gitOutput(path, "rev-parse", ref))
	if full != "" {
		return full
	}
	return ref
}

func GitCurrentBranch(path string) string {
	return strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", "HEAD"))
}

func GitBranchCreate(path, name, from string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("nome da branch vazio")
	}
	args := []string{"checkout", "-b", name}
	if from != "" && from != GitCurrentBranch(path) {
		args = append(args, from)
	}
	return gitRun(path, args...)
}

func GitBranchDelete(path, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch vazia")
	}
	if branch == GitCurrentBranch(path) {
		return fmt.Errorf("não é possível apagar a branch atual")
	}
	if err := gitRun(path, "branch", "-d", branch); err != nil {
		return gitRun(path, "branch", "-D", branch)
	}
	return nil
}

func GitBranchRename(path, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("nome inválido")
	}
	if oldName == GitCurrentBranch(path) {
		return gitRun(path, "branch", "-m", newName)
	}
	return gitRun(path, "branch", "-m", oldName, newName)
}

func GitPull(path string) error {
	return gitRun(path, "pull", "--ff-only")
}

// GitBranchOrigin returns the branch this one likely originated from (e.g. develop).
func GitBranchOrigin(path, branch string) string {
	if branch == "" {
		branch = GitCurrentBranch(path)
	}
	if branch == "" || branch == "HEAD" {
		return ""
	}
	if isTrunkBranch(path, branch) {
		upstream := strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", branch+"@{upstream}"))
		if upstream != "" {
			if idx := strings.Index(upstream, "/"); idx >= 0 {
				return upstream[idx+1:]
			}
			return upstream
		}
		return branch
	}
	for _, candidate := range []string{"develop", "main", "master"} {
		if candidate == branch {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", candidate)) == "" {
			continue
		}
		base := strings.TrimSpace(gitOutput(path, "merge-base", branch, candidate))
		if base == "" {
			continue
		}
		tip := strings.TrimSpace(gitOutput(path, "rev-parse", candidate))
		if base == tip {
			return candidate
		}
	}
	upstream := strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", branch+"@{upstream}"))
	if upstream != "" {
		if idx := strings.Index(upstream, "/"); idx >= 0 {
			return upstream[idx+1:]
		}
	}
	for _, candidate := range []string{"develop", "main", "master"} {
		if candidate == branch {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", candidate)) == "" {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "merge-base", branch, candidate)) != "" {
			return candidate
		}
	}
	return ""
}

func GitPullOrigin(path, sourceBranch string) error {
	sourceBranch = strings.TrimSpace(sourceBranch)
	if sourceBranch == "" {
		return fmt.Errorf("branch de origem não detectada")
	}
	return gitRun(path, "pull", "origin", sourceBranch, "--ff-only")
}

func GitPush(path string) error {
	branch := GitCurrentBranch(path)
	if branch == "" || branch == "HEAD" {
		return fmt.Errorf("branch atual inválida")
	}
	upstream := strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", branch+"@{upstream}"))
	if upstream == "" {
		return gitRun(path, "push", "-u", "origin", branch)
	}
	return gitRun(path, "push")
}

func GitMerge(path, branch string) error {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return fmt.Errorf("branch vazia")
	}
	current := GitCurrentBranch(path)
	if branch == current {
		return fmt.Errorf("não é possível mesclar a branch atual nela mesma")
	}
	return gitRun(path, "merge", branch)
}

func ParseGitHubRepo(remote string) (owner, repo string, ok bool) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	if strings.HasPrefix(remote, "git@github.com:") {
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@github.com:"), "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], parts[1], true
		}
	}
	if idx := strings.Index(remote, "github.com/"); idx >= 0 {
		rest := remote[idx+len("github.com/"):]
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			return parts[0], strings.TrimSuffix(parts[1], ".git"), true
		}
	}
	return "", "", false
}

func GitDefaultPRBase(path, branch string) string {
	upstream := strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", branch+"@{upstream}"))
	if upstream != "" {
		if idx := strings.Index(upstream, "/"); idx >= 0 {
			return upstream[idx+1:]
		}
	}
	for _, candidate := range []string{"main", "master", "develop"} {
		if candidate == branch {
			continue
		}
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", candidate)) != "" {
			return candidate
		}
	}
	return "main"
}

func GitHubCompareURL(remote, base, head string) string {
	owner, repo, ok := ParseGitHubRepo(remote)
	if !ok || head == "" {
		return ""
	}
	if base == "" {
		base = "main"
	}
	return fmt.Sprintf("https://github.com/%s/%s/compare/%s...%s?expand=1", owner, repo, base, head)
}

func GitWorkTreeRoot(path string) string {
	return strings.TrimSpace(gitOutput(path, "rev-parse", "--show-toplevel"))
}

func RefreshGitBranches(path string, prev *core.GitInfo) *core.GitInfo {
	if prev == nil {
		return CollectAt(path)
	}
	if !isGitRepo(path) {
		return &core.GitInfo{IsRepo: false}
	}
	copy := *prev
	copy.Branch = strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", "HEAD"))
	copy.Branches = collectGitBranches(path)
	copy.Commits = collectGitCommits(path, copy.Branch, 20)
	return &copy
}

func RefreshProjectGit(store *core.StateStore, projectPath string) {
	projectPath = filepath.Clean(projectPath)
	root := gitRepoRoot(projectPath)
	if root == "" {
		store.UpdateProjectGit(projectPath, core.GitInfo{IsRepo: false})
		return
	}
	info := CollectAt(root)
	if info != nil {
		store.UpdateProjectGit(projectPath, *info)
	}
}

func preserveGitForProjects(store *core.StateStore, projects []core.Project) []core.Project {
	snap := store.Get()
	prev := make(map[string]*core.GitInfo, len(snap.Projects))
	for i := range snap.Projects {
		if snap.Projects[i].Git != nil {
			prev[snap.Projects[i].Path] = snap.Projects[i].Git
		}
	}
	for i := range projects {
		var base *core.GitInfo
		if projects[i].Git != nil {
			base = projects[i].Git
		} else if git, ok := prev[projects[i].Path]; ok {
			base = git
		}
		if base != nil {
			projects[i].Git = RefreshGitBranches(projects[i].Path, base)
		}
	}
	return projects
}

func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	fi, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	return fi.IsDir() || fi.Mode().IsRegular()
}

func gitRepoRoot(path string) string {
	path = filepath.Clean(path)
	for {
		if isGitRepo(path) {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}
