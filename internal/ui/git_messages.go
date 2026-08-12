package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type gitCommitsLoadedMsg struct {
	path    string
	branch  string
	commits []core.GitCommit
	gen     int
}

type gitCommitDetailLoadedMsg struct {
	path        string
	hash        string
	files       []core.GitCommitFileChange
	fullMessage string
}

type gitCommitDiffLoadedMsg struct {
	path string
	hash string
	file string
	diff string
	gen  int
}

type gitActionDoneMsg struct {
	path      string
	action    string
	branch    string
	newBranch string
	count     int
	cmdline   string
	output    string
	err       error
}

type gitWTDiffMsg struct {
	file string
	diff string
}

type gitWTRefreshedMsg struct {
	path string
	gen  int
}

func (a *App) maybeRefreshGitWorkingTree() tea.Cmd {
	if a.view != ViewProject || a.tab != TabGit || a.gitSubview != gitSubviewMain {
		return nil
	}
	if a.gitActionLoading || a.gitComposeOn || a.gitPromptOn || a.gitConfirmOn || a.gitWTRefreshing {
		return nil
	}
	if time.Since(a.gitLastWTRefresh) < time.Second {
		return nil
	}
	p := a.currentProject()
	if p == nil || p.Git == nil || !p.Git.IsRepo {
		return nil
	}
	a.gitLastWTRefresh = time.Now()
	a.gitWTRefreshing = true
	a.gitWTRefreshGen++
	gen := a.gitWTRefreshGen
	path := p.Path
	store := a.store
	return func() tea.Msg {
		collectors.RefreshProjectGitFiles(store, path)
		return gitWTRefreshedMsg{path: path, gen: gen}
	}
}

func (a *App) handleGitWTRefreshed(msg gitWTRefreshedMsg) {
	a.gitWTRefreshing = false
	if msg.gen != a.gitWTRefreshGen {
		return
	}
	if a.selectedProject == nil || !pathsMatch(a.selectedProject.Path, msg.path) {
		return
	}
	a.snapshot = a.store.Get()
	p := a.currentProject()
	if p == nil || p.Git == nil {
		return
	}
	a.syncGitBranchesFrom(p)
	rows := a.wtRowsForUI(p.Git.Files)
	if len(rows) == 0 {
		a.gitFileCursor = 0
		a.gitFileTreeCursor = 0
		a.gitFileScroll = 0
		return
	}
	viewport := a.gitFilesViewport()
	if a.gitConflictOn {
		viewport = maxInt(1, viewport-2)
	}
	a.gitFileTreeCursor = snapWTFileTreeCursor(rows, a.gitFileTreeCursor)
	a.gitFileScroll = ensureVisible(a.gitFileTreeCursor, a.gitFileScroll, viewport, len(rows))
	if r := rows[a.gitFileTreeCursor]; !r.isDir && r.fileIdx >= 0 {
		a.gitFileCursor = r.fileIdx
	} else {
		a.gitFileCursor = clampCursor(a.gitFileCursor, len(p.Git.Files))
	}
}

func (a *App) requestGitWorkingTreeDiff(path, file string) tea.Cmd {
	if path == "" || file == "" {
		return nil
	}
	a.gitWTDiffFile = file
	a.gitWTDiffConflict = false
	return func() tea.Msg {
		return gitWTDiffMsg{file: file, diff: collectors.CollectWorkingTreeDiff(path, file)}
	}
}

func (a *App) requestGitConflictDiff(path, file string) tea.Cmd {
	if path == "" || file == "" {
		return nil
	}
	a.gitWTDiffFile = file
	a.gitWTDiffConflict = true
	a.gitWTDiff = ""
	ours := a.gitConflictOurs
	theirs := a.gitConflictTheirs
	return func() tea.Msg {
		return gitWTDiffMsg{file: file, diff: collectors.CollectConflictDiff(path, file, ours, theirs)}
	}
}

func (a *App) pushGitActivity(msg gitActionDoneMsg) {
	if msg.err != nil {
		return
	}
	label := msg.action
	if msg.branch != "" {
		label += " " + msg.branch
	}
	if msg.action == "checkout" {
		label = "Checkout " + msg.branch
	}
	entry := timeNowHHMM() + " " + label
	a.gitActivity = append([]string{entry}, a.gitActivity...)
	if len(a.gitActivity) > 20 {
		a.gitActivity = a.gitActivity[:20]
	}
}

func (a *App) appendGitCommandLog(msg gitActionDoneMsg) {
	title := msg.action
	if title != "" {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	switch msg.action {
	case "pull":
		title = "Pull"
	case "pull-merge":
		title = "Pull merge"
	case "pull-rebase":
		title = "Pull rebase"
	case "continue":
		title = "Continue"
	case "abort":
		title = "Abort"
	case "ours", "theirs":
		title = strings.ToUpper(msg.action[:1]) + msg.action[1:]
	case "both":
		title = "Ambas"
	case "push":
		title = "Push"
	case "checkout":
		title = "Checkout"
	}
	cmdline := msg.cmdline
	if cmdline == "" {
		cmdline = "git " + msg.action
		if msg.branch != "" {
			cmdline += " " + msg.branch
		}
	}
	out := strings.TrimSpace(msg.output)
	if out == "" {
		if msg.err != nil {
			out = msg.err.Error()
		} else {
			out = "ok"
		}
	}
	a.gitCommandLog = append(a.gitCommandLog, gitCmdLogEntry{
		Title:   title,
		Cmdline: cmdline,
		Output:  out,
		Time:    timeNowHHMM(),
	})
	if len(a.gitCommandLog) > 40 {
		a.gitCommandLog = a.gitCommandLog[len(a.gitCommandLog)-40:]
	}
	a.gitCmdLogScroll = 0
}

func timeNowHHMM() string {
	return time.Now().Format("15:04")
}

func loadGitBranchCommits(path, branch string, gen int) tea.Cmd {
	return func() tea.Msg {
		commits := collectors.CollectCommitsAt(path, branch, 80)
		return gitCommitsLoadedMsg{path: path, branch: branch, commits: commits, gen: gen}
	}
}

func (a *App) requestGitBranchCommits(path, branch string) tea.Cmd {
	a.gitBranchLoadGen++
	gen := a.gitBranchLoadGen
	a.gitBranchLoading = true
	return loadGitBranchCommits(path, branch, gen)
}

func loadGitCommitDetail(path, hash string) tea.Cmd {
	return func() tea.Msg {
		return gitCommitDetailLoadedMsg{
			path:        path,
			hash:        hash,
			files:       collectors.CollectCommitFiles(path, hash),
			fullMessage: collectors.CollectCommitFullMessage(path, hash),
		}
	}
}

func loadGitCommitFileDiff(path, hash, file string, gen int) tea.Cmd {
	return func() tea.Msg {
		diff := collectors.CollectCommitFileDiff(path, hash, file)
		if diff == "" {
			diff = "(sem diff para este arquivo)"
		}
		return gitCommitDiffLoadedMsg{path: path, hash: hash, file: file, diff: diff, gen: gen}
	}
}

func (a *App) gitAddFile(p *core.Project) tea.Cmd {
	if p == nil || p.Git == nil {
		return nil
	}
	if a.gitViewBranch != "" && a.gitViewBranch != p.Git.Branch {
		a.gitStatusMsg = "checkout da branch para stage"
		return nil
	}
	f, ok := a.selectedWTFile(p.Git)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo para stage"
		return nil
	}
	file := f.Path
	path := p.Path
	a.gitActionLoading = true
	if gitFileStaged(f) {
		a.gitStatusMsg = "unstage " + file + "…"
		return func() tea.Msg {
			err := collectors.GitUnstage(path, file)
			return gitActionDoneMsg{path: path, action: "unstage", branch: file, err: err}
		}
	}
	a.gitStatusMsg = "git add " + file + "…"
	return func() tea.Msg {
		err := collectors.GitAdd(path, file)
		return gitActionDoneMsg{path: path, action: "add", branch: file, err: err}
	}
}

func (a *App) gitAddAll(p *core.Project) tea.Cmd {
	if p == nil || p.Git == nil {
		return nil
	}
	if a.gitViewBranch != "" && a.gitViewBranch != p.Git.Branch {
		a.gitStatusMsg = "checkout da branch para stage"
		return nil
	}
	if len(p.Git.Files) == 0 {
		a.gitStatusMsg = "nada para stage"
		return nil
	}
	path := p.Path
	allStaged := true
	for _, f := range p.Git.Files {
		if !gitFileStaged(f) {
			allStaged = false
			break
		}
	}
	a.gitActionLoading = true
	if allStaged {
		a.gitStatusMsg = "unstage all…"
		return func() tea.Msg {
			err := collectors.GitUnstage(path)
			return gitActionDoneMsg{path: path, action: "unstage-all", err: err}
		}
	}
	a.gitStatusMsg = "git add -A…"
	return func() tea.Msg {
		err := collectors.GitAdd(path)
		return gitActionDoneMsg{path: path, action: "add-all", err: err}
	}
}

func (a *App) gitCheckoutBranch(p *core.Project, branch string) tea.Cmd {
	if p == nil || p.Git == nil || branch == "" || branch == p.Git.Branch {
		return nil
	}
	a.gitActionLoading = true
	a.gitStatusMsg = "checkout " + branch + "..."
	path := p.Path
	return func() tea.Msg {
		out, err := collectors.GitCheckout(path, branch)
		return gitActionDoneMsg{path: path, action: "checkout", branch: branch, cmdline: "git checkout " + branch, output: out, err: err}
	}
}

func (a *App) gitCreateBranch(p *core.Project, name, from string) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "criando branch " + name + "..."
	path := p.Path
	return func() tea.Msg {
		err := collectors.GitBranchCreate(path, name, from)
		return gitActionDoneMsg{path: path, action: "create-branch", branch: name, err: err}
	}
}

func (a *App) gitRenameBranch(p *core.Project, oldName, newName string) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "renomeando " + oldName + " → " + newName + "..."
	path := p.Path
	return func() tea.Msg {
		err := collectors.GitBranchRename(path, oldName, newName)
		return gitActionDoneMsg{path: path, action: "rename-branch", branch: oldName, newBranch: newName, err: err}
	}
}

func (a *App) gitDeleteBranch(p *core.Project, branch string) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "apagando " + branch + "..."
	path := p.Path
	return func() tea.Msg {
		err := collectors.GitBranchDelete(path, branch)
		return gitActionDoneMsg{path: path, action: "delete-branch", branch: branch, err: err}
	}
}

func (a *App) gitMergeBranch(p *core.Project, branch string) tea.Cmd {
	a.gitActionLoading = true
	target := p.Git.Branch
	a.gitStatusMsg = "mesclando " + branch + " em " + target + "..."
	path := p.Path
	return func() tea.Msg {
		var parts []string
		current := collectors.GitCurrentBranch(path)
		if branch != current && target != current {
			out, err := collectors.GitCheckout(path, target)
			if out != "" {
				parts = append(parts, out)
			}
			if err != nil {
				return gitActionDoneMsg{path: path, action: "merge", branch: branch, cmdline: "git merge " + branch, output: strings.Join(parts, "\n"), err: err}
			}
		}
		err := collectors.GitMerge(path, branch)
		return gitActionDoneMsg{path: path, action: "merge", branch: branch, cmdline: "git merge " + branch, output: strings.Join(parts, "\n"), err: err}
	}
}

func (a *App) gitPull(p *core.Project) tea.Cmd {
	source := a.gitPullSourceBranch(p)
	if source == "" {
		a.gitStatusMsg = "origem não detectada — marque com D na branch pai"
		return nil
	}
	a.gitActionLoading = true
	a.gitStatusMsg = "pull origin " + source + "..."
	path := p.Path
	return func() tea.Msg {
		out, err := collectors.GitPullOrigin(path, source)
		return gitActionDoneMsg{path: path, action: "pull", branch: source, cmdline: "git pull origin " + source + " --ff-only", output: out, err: err}
	}
}

func (a *App) gitPullMerge(p *core.Project, source string) tea.Cmd {
	if source == "" {
		a.gitStatusMsg = "origem não detectada"
		return nil
	}
	a.gitActionLoading = true
	a.gitStatusMsg = "pull --no-ff origin " + source + "..."
	path := p.Path
	return func() tea.Msg {
		out, err := collectors.GitPullOriginMerge(path, source)
		return gitActionDoneMsg{path: path, action: "pull-merge", branch: source, cmdline: "git pull origin " + source + " --no-ff", output: out, err: err}
	}
}

func (a *App) gitPullRebase(p *core.Project, source string) tea.Cmd {
	if source == "" {
		a.gitStatusMsg = "origem não detectada"
		return nil
	}
	a.gitActionLoading = true
	a.gitStatusMsg = "pull --rebase origin " + source + "..."
	path := p.Path
	return func() tea.Msg {
		out, err := collectors.GitPullOriginRebase(path, source)
		return gitActionDoneMsg{path: path, action: "pull-rebase", branch: source, cmdline: "git pull --rebase origin " + source, output: out, err: err}
	}
}

func (a *App) gitPush(p *core.Project) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "push..."
	path := p.Path
	remote := ""
	if p.Git != nil {
		remote = p.Git.Remote
	}
	return func() tea.Msg {
		out, err := collectors.GitPush(path)
		if err == nil {
			head := collectors.GitCurrentBranch(path)
			base := collectors.GitDefaultPRBase(path, head)
			if url := collectors.GitHubCompareURL(remote, base, head); url != "" {
				if out != "" {
					out += "\n"
				}
				out += "Create a pull request for '" + head + "' on GitHub by visiting:\n  " + url
			}
		}
		return gitActionDoneMsg{path: path, action: "push", cmdline: "git push", output: out, err: err}
	}
}

func (a *App) gitCherryPickPaste(p *core.Project) tea.Cmd {
	if p == nil || p.Git == nil || len(a.gitCherryPickBuffer) == 0 {
		a.gitStatusMsg = "nenhum commit no buffer — use shift+c"
		return nil
	}
	target := a.gitViewBranch
	if target == "" {
		target = p.Git.Branch
	}
	a.gitActionLoading = true
	a.gitStatusMsg = fmt.Sprintf("cherry-pick em %s...", target)
	path := p.Path
	hashes := append([]string(nil), a.gitCherryPickBuffer...)
	count := len(hashes)
	return func() tea.Msg {
		var parts []string
		current := collectors.GitCurrentBranch(path)
		if target != "" && target != current {
			out, err := collectors.GitCheckout(path, target)
			if out != "" {
				parts = append(parts, out)
			}
			if err != nil {
				return gitActionDoneMsg{path: path, action: "cherry-pick", branch: target, count: count, cmdline: "git cherry-pick", output: strings.Join(parts, "\n"), err: err}
			}
		}
		err := collectors.GitCherryPick(path, hashes)
		return gitActionDoneMsg{path: path, action: "cherry-pick", branch: target, count: count, cmdline: "git cherry-pick " + strings.Join(hashes, " "), output: strings.Join(parts, "\n"), err: err}
	}
}

func (a *App) handleGitCommitsLoaded(msg gitCommitsLoadedMsg) {
	if a.selectedProject == nil || msg.path != a.selectedProject.Path {
		return
	}
	if msg.gen != a.gitBranchLoadGen {
		return
	}
	a.gitBranchLoading = false
	if msg.branch != a.gitViewBranch {
		return
	}
	a.gitBranchCommits = msg.commits
}

func (a *App) handleGitCommitDetailLoaded(msg gitCommitDetailLoadedMsg) tea.Cmd {
	if a.selectedProject == nil || msg.path != a.selectedProject.Path {
		return nil
	}
	if msg.hash != a.gitSelectedCommit.Hash {
		return nil
	}
	a.gitCommitFiles = msg.files
	a.gitCommitFullMsg = msg.fullMessage
	a.gitCommitFilesLoading = false
	a.gitCommitFileCursor = 0
	a.gitCommitTreeCursor = 0
	a.gitCommitFileScroll = 0
	a.gitCommitCollapsed = nil
	a.gitCommitFileOpen = false
	a.gitCommitDetailFocus = gitCommitFocusFiles
	a.gitCommitDiffLoading = false
	if len(msg.files) == 0 {
		a.gitCommitDiff = "(nenhum arquivo alterado)"
		return nil
	}
	// Preview the first file under the tree cursor (stay on Arquivos).
	rows := a.commitFileTreeRows()
	for i, r := range rows {
		if r.isDir {
			continue
		}
		a.gitCommitTreeCursor = i
		a.gitCommitFileCursor = r.fileIdx
		return a.requestGitCommitFileDiff(msg.path, msg.hash, msg.files[r.fileIdx].Path)
	}
	a.gitCommitDiff = ""
	return nil
}

func (a *App) requestGitCommitFileDiff(path, hash, file string) tea.Cmd {
	if file == "" {
		a.gitCommitDiff = "(nenhum arquivo)"
		a.gitCommitDiffLoading = false
		return nil
	}
	if a.gitCommitDiffCache != nil {
		if diff, ok := a.gitCommitDiffCache[file]; ok {
			a.gitCommitDiff = diff
			a.gitCommitDiffLoading = false
			a.gitCommitDiffScroll = 0
			a.gitCommitDiffHScroll = 0
			return nil
		}
	}
	a.gitCommitDiffGen++
	gen := a.gitCommitDiffGen
	a.gitCommitDiffLoading = true
	a.gitCommitDiff = ""
	a.gitCommitDiffScroll = 0
	a.gitCommitDiffHScroll = 0
	return loadGitCommitFileDiff(path, hash, file, gen)
}

func (a *App) handleGitCommitDiffLoaded(msg gitCommitDiffLoadedMsg) {
	if a.selectedProject == nil || msg.path != a.selectedProject.Path {
		return
	}
	if msg.hash != a.gitSelectedCommit.Hash || msg.gen != a.gitCommitDiffGen {
		return
	}
	if a.gitCommitFileCursor < len(a.gitCommitFiles) && a.gitCommitFiles[a.gitCommitFileCursor].Path != msg.file {
		return
	}
	a.gitCommitDiff = msg.diff
	a.gitCommitDiffLoading = false
	if a.gitCommitDiffCache == nil {
		a.gitCommitDiffCache = make(map[string]string)
	}
	a.gitCommitDiffCache[msg.file] = msg.diff
}

func needsGitBranchCommitsReload(action string) bool {
	switch action {
	case "checkout", "cherry-pick", "create-branch", "rename-branch", "commit",
		"pull", "pull-merge", "pull-rebase", "merge", "continue", "abort":
		return true
	default:
		return false
	}
}

func (a *App) conflictStatusMsg(kind string) string {
	if kind == "" {
		kind = "git"
	}
	n := 0
	if p := a.currentProject(); p != nil && p.Git != nil {
		n = collectors.GitUnmergedCount(p.Git.Files)
	}
	ours := firstNonEmpty(a.gitConflictOurs, "HEAD")
	theirs := firstNonEmpty(a.gitConflictTheirs, "incoming")
	if n == 0 {
		return fmt.Sprintf("CONFLITO (%s) resolvido — c continue  x abort", kind)
	}
	return fmt.Sprintf("CONFLITO (%s) · %d arquivo(s) · o=%s  t=%s", kind, n, ours, theirs)
}

func (a *App) enterGitConflictMode(path, hintKind string) bool {
	kind := collectors.GitOpInProgress(path)
	if kind == "" {
		kind = hintKind
	}
	if kind == "" {
		return false
	}
	a.gitConflictOn = true
	a.gitConflictKind = kind
	a.gitConflictOurs, a.gitConflictTheirs = collectors.GitConflictSides(path)
	if p := a.currentProject(); p != nil && p.Git != nil {
		a.focusFirstConflict(p.Git)
	} else {
		a.gitFocus = gitFocusFiles
	}
	a.gitStatusMsg = a.conflictStatusMsg(kind)
	return true
}

func (a *App) clearGitConflictMode() {
	a.gitConflictOn = false
	a.gitConflictKind = ""
	a.gitConflictOurs = ""
	a.gitConflictTheirs = ""
}

func (a *App) handleGitActionDone(msg gitActionDoneMsg) {
	a.gitActionLoading = false
	if a.selectedProject == nil || msg.path != a.selectedProject.Path {
		return
	}

	collectors.RefreshProjectGit(a.store, msg.path)
	a.snapshot = a.store.Get()

	p := a.currentProject()
	if p == nil || p.Git == nil {
		return
	}
	a.syncGitBranchesFrom(p)

	if msg.err != nil {
		out := msg.output
		if out == "" {
			out = msg.err.Error()
		}
		if msg.action == "pull" && collectors.GitIsFFImpossible(msg.err, out) {
			a.openPullStrategyModal(msg.branch)
			return
		}
		hint := ""
		switch msg.action {
		case "pull-merge", "merge":
			hint = "merge"
		case "pull-rebase":
			hint = "rebase"
		case "cherry-pick":
			hint = "cherry-pick"
		}
		if collectors.GitIsConflictError(msg.err, out) || collectors.GitOpInProgress(msg.path) != "" {
			if a.enterGitConflictMode(msg.path, hint) {
				return
			}
		}
		a.gitStatusMsg = gitCompactError(msg.action, msg.err.Error())
		return
	}

	switch msg.action {
	case "checkout":
		a.gitViewBranch = msg.branch
		a.gitSelectedCommits = nil
		a.gitCommitSelectAnchor = -1
		a.gitBranchLoading = true
		a.gitBranchCommits = nil
		a.gitCommitCursor = 0
		a.gitCommitScroll = 0
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "checkout " + msg.branch + " ✓"
	case "cherry-pick":
		a.clearGitConflictMode()
		a.gitCherryPickBuffer = nil
		a.gitCherryPickMarked = nil
		a.gitCherryPickActive = false
		a.gitCherryPickSourceBranch = ""
		a.clearGitCommitSelection()
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = fmt.Sprintf("cherry-pick em %s ✓ (%d commits)", msg.branch, msg.count)
	case "commit":
		a.gitViewBranch = p.Git.Branch
		a.gitBranchLoading = true
		a.gitBranchCommits = nil
		a.gitCommitCursor = 0
		a.gitCommitScroll = 0
		a.clearGitCommitSelection()
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "commit ✓"
	case "create-branch":
		a.allowGitBranchName(msg.branch)
		a.gitViewBranch = msg.branch
		a.gitBranchLoading = true
		a.gitBranchCommits = nil
		a.gitCommitCursor = 0
		a.gitCommitScroll = 0
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "branch " + msg.branch + " criada ✓"
	case "rename-branch":
		a.allowGitBranchName(msg.newBranch)
		if a.gitBranchDenylist != nil {
			delete(a.gitBranchDenylist, msg.branch)
		}
		if a.gitMarkedBranch == msg.branch {
			a.gitMarkedBranch = msg.newBranch
		}
		if a.gitViewBranch == msg.branch {
			a.gitViewBranch = msg.newBranch
		}
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitBranchLoading = true
		a.gitBranchCommits = nil
		a.gitStatusMsg = msg.branch + " → " + msg.newBranch + " ✓"
	case "delete-branch":
		a.pruneGitBranch(msg.branch)
		if a.gitViewBranch == msg.branch {
			a.gitViewBranch = p.Git.Branch
		}
		a.syncGitBranchCursor(a.gitBranchesForUI())
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.gitStatusMsg = "branch " + msg.branch + " apagada ✓"
	case "merge", "pull-merge":
		a.clearGitConflictMode()
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		if msg.action == "pull-merge" {
			a.gitStatusMsg = "pull merge origin " + msg.branch + " ✓"
		} else {
			a.gitStatusMsg = "merge " + msg.branch + " ✓"
		}
	case "pull", "pull-rebase":
		a.clearGitConflictMode()
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		if msg.action == "pull-rebase" {
			a.gitStatusMsg = "pull rebase origin " + msg.branch + " ✓"
		} else {
			a.gitStatusMsg = "pull origin " + msg.branch + " ✓"
		}
	case "continue":
		a.clearGitConflictMode()
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "continue ✓"
	case "abort":
		a.clearGitConflictMode()
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "abort ✓"
	case "ours", "theirs", "both":
		a.gitSubview = gitSubviewMain
		a.gitWTDiffConflict = false
		a.gitWTDiff = ""
		if collectors.GitOpInProgress(msg.path) != "" {
			a.gitConflictOn = true
			a.focusFirstConflict(p.Git)
			label := msg.action
			if label == "both" {
				label = "ambas"
			}
			a.gitStatusMsg = label + " ✓ · " + a.conflictStatusMsg(a.gitConflictKind)
		} else {
			a.clearGitConflictMode()
			a.gitStatusMsg = msg.action + " " + msg.branch + " ✓"
		}
	case "edit":
		a.gitStatusMsg = a.conflictStatusMsg(a.gitConflictKind)
	case "push":
		a.gitBranchLoading = false
		a.gitStatusMsg = "push ✓"
	case "add":
		a.gitStatusMsg = "staged " + msg.branch + " ✓"
		if a.gitConflictOn {
			a.gitStatusMsg = a.conflictStatusMsg(a.gitConflictKind)
		}
	case "add-all":
		a.gitStatusMsg = "todos os arquivos em stage ✓"
	case "unstage":
		a.gitStatusMsg = "unstage " + msg.branch + " ✓"
	case "unstage-all":
		a.gitStatusMsg = "todos removidos do stage ✓"
	}

	if a.gitConflictOn && collectors.GitOpInProgress(msg.path) == "" {
		a.clearGitConflictMode()
	}
}

func (a *App) gitCherryPickCopy(p *core.Project) {
	if p == nil {
		return
	}
	commits := a.gitDisplayedCommits()
	var hashes []string
	a.gitCherryPickMarked = make(map[string]bool)

	if len(a.gitSelectedCommits) == 0 {
		if a.gitCommitCursor < len(commits) {
			c := commits[a.gitCommitCursor]
			hashes = []string{collectors.GitResolveHash(p.Path, c.Hash)}
			a.gitCherryPickMarked[c.Hash] = true
		}
	} else {
		for i := len(commits) - 1; i >= 0; i-- {
			c := commits[i]
			if !a.gitSelectedCommits[c.Hash] {
				continue
			}
			hashes = append(hashes, collectors.GitResolveHash(p.Path, c.Hash))
			a.gitCherryPickMarked[c.Hash] = true
		}
	}
	if len(hashes) == 0 {
		a.gitStatusMsg = "selecione commits (x ou shift+↑↓) e pressione shift+c"
		return
	}
	a.gitCherryPickBuffer = hashes
	a.gitCherryPickActive = true
	a.gitCherryPickSourceBranch = a.gitViewBranch
	a.clearGitCommitSelection()
	a.gitStatusMsg = fmt.Sprintf("🍒 %d commit(s) copiados de %s — vá à branch destino e shift+v", len(hashes), a.gitCherryPickSourceBranch)
}

func (a *App) toggleGitCommitSelection(p *core.Project) {
	commits := a.gitDisplayedCommits()
	if a.gitCommitCursor >= len(commits) {
		return
	}
	hash := commits[a.gitCommitCursor].Hash
	if a.gitSelectedCommits == nil {
		a.gitSelectedCommits = make(map[string]bool)
	}
	if a.gitSelectedCommits[hash] {
		delete(a.gitSelectedCommits, hash)
	} else {
		a.gitSelectedCommits[hash] = true
	}
	a.gitCommitSelectAnchor = a.gitCommitCursor
}

func (a *App) gitSelectedCommitCount() int {
	return len(a.gitSelectedCommits)
}

func (a *App) isGitCommitSelected(hash string) bool {
	return a.gitSelectedCommits != nil && a.gitSelectedCommits[hash]
}

func (a *App) clearGitCommitSelection() {
	a.gitSelectedCommits = nil
	a.gitCommitSelectAnchor = -1
}

func (a *App) isGitCommitInCherryBuffer(hash string) bool {
	if a.gitCherryPickMarked != nil && a.gitCherryPickMarked[hash] {
		return true
	}
	for _, h := range a.gitCherryPickBuffer {
		if h == hash || strings.HasPrefix(h, hash) {
			return true
		}
	}
	return false
}

func (a *App) gitCherryPickSummary() string {
	if !a.gitCherryPickActive || len(a.gitCherryPickBuffer) == 0 {
		return ""
	}
	parts := make([]string, 0, minInt(3, len(a.gitCherryPickBuffer)))
	for i, h := range a.gitCherryPickBuffer {
		if i >= 3 {
			parts = append(parts, "...")
			break
		}
		parts = append(parts, shortGitHash(h))
	}
	return strings.Join(parts, " → ")
}

func shortGitHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func (a *App) updateGitConflict(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := a.currentProject()
	if p == nil || p.Git == nil {
		a.clearGitConflictMode()
		return a, nil
	}
	if a.gitActionLoading {
		return a, nil
	}
	switch msg.String() {
	case "o", "O":
		return a, a.gitResolveOurs(p)
	case "t", "T":
		return a, a.gitResolveTheirs(p)
	case "b", "B":
		return a, a.gitResolveBoth(p)
	case "e", "E", "enter":
		return a, a.openGitConflictDiff(p)
	case "a", "A":
		return a, a.gitAddFile(p)
	case "c", "C":
		return a, a.gitConflictContinue(p)
	case "x", "X":
		return a, a.gitConflictAbort(p)
	case "L", "shift+l", "shift+L":
		return a, a.openLazyGit(p.Path)
	case "esc":
		a.gitStatusMsg = a.conflictStatusMsg(a.gitConflictKind)
	}
	// Allow panel navigation while resolving.
	switch msg.String() {
	case "up", "k", "down", "j", "left", "h", "right", "l", "tab", "shift+tab":
		return a.updateProject(msg)
	}
	return a, nil
}

func (a *App) gitConflictSelectedFile(p *core.Project) (string, bool) {
	if p == nil || p.Git == nil {
		return "", false
	}
	f, ok := a.selectedWTFile(p.Git)
	if !ok {
		return "", false
	}
	return f.Path, true
}

func (a *App) gitResolveOurs(p *core.Project) tea.Cmd {
	file, ok := a.gitConflictSelectedFile(p)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo em conflito"
		return nil
	}
	path := p.Path
	a.gitActionLoading = true
	a.gitStatusMsg = "ours " + file + "…"
	return func() tea.Msg {
		err := collectors.GitCheckoutOurs(path, file)
		return gitActionDoneMsg{path: path, action: "ours", branch: file, cmdline: "git checkout --ours -- " + file, err: err}
	}
}

func (a *App) gitResolveTheirs(p *core.Project) tea.Cmd {
	file, ok := a.gitConflictSelectedFile(p)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo em conflito"
		return nil
	}
	path := p.Path
	a.gitActionLoading = true
	a.gitStatusMsg = "theirs " + file + "…"
	return func() tea.Msg {
		err := collectors.GitCheckoutTheirs(path, file)
		return gitActionDoneMsg{path: path, action: "theirs", branch: file, cmdline: "git checkout --theirs -- " + file, err: err}
	}
}

func (a *App) gitResolveBoth(p *core.Project) tea.Cmd {
	file, ok := a.gitConflictSelectedFile(p)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo em conflito"
		return nil
	}
	path := p.Path
	a.gitActionLoading = true
	a.gitStatusMsg = "ambas " + file + "…"
	return func() tea.Msg {
		err := collectors.GitResolveBoth(path, file)
		return gitActionDoneMsg{path: path, action: "both", branch: file, cmdline: "resolve both -- " + file, err: err}
	}
}

func (a *App) openGitConflictDiff(p *core.Project) tea.Cmd {
	if p == nil || p.Git == nil {
		return nil
	}
	f, ok := a.selectedWTFile(p.Git)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo em conflito"
		return nil
	}
	a.gitSubview = gitSubviewFileDiff
	a.gitFocus = gitFocusFiles
	a.gitWTDiffScroll = 0
	a.gitWTDiffHScroll = 0
	a.gitWTDiff = ""
	a.gitStatusMsg = "diff do conflito — o/t/b escolhe lado"
	return a.requestGitConflictDiff(p.Path, f.Path)
}

func (a *App) updateGitConflictDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := a.currentProject()
	if p == nil {
		return a, nil
	}
	if a.gitActionLoading {
		return a, nil
	}
	switch msg.String() {
	case "o", "O":
		return a, a.gitResolveOurs(p)
	case "t", "T":
		return a, a.gitResolveTheirs(p)
	case "b", "B":
		return a, a.gitResolveBoth(p)
	case "e", "E":
		// External editor still available from the conflict diff screen.
		return a, a.gitConflictEdit(p)
	case "esc":
		a.gitSubview = gitSubviewMain
		a.gitFocus = gitFocusFiles
		a.gitWTDiffConflict = false
		a.gitStatusMsg = a.conflictStatusMsg(a.gitConflictKind)
		return a, nil
	}
	return a.handleGitDedicatedKeys(msg, p)
}

func (a *App) gitConflictContinue(p *core.Project) tea.Cmd {
	path := p.Path
	a.gitActionLoading = true
	a.gitStatusMsg = "continue…"
	return func() tea.Msg {
		out, err := collectors.GitContinue(path)
		return gitActionDoneMsg{path: path, action: "continue", cmdline: "git … --continue", output: out, err: err}
	}
}

func (a *App) gitConflictAbort(p *core.Project) tea.Cmd {
	path := p.Path
	a.gitActionLoading = true
	a.gitStatusMsg = "abort…"
	return func() tea.Msg {
		out, err := collectors.GitAbort(path)
		return gitActionDoneMsg{path: path, action: "abort", cmdline: "git … --abort", output: out, err: err}
	}
}

func (a *App) gitConflictEdit(p *core.Project) tea.Cmd {
	file, ok := a.gitConflictSelectedFile(p)
	if !ok {
		a.gitStatusMsg = "selecione um arquivo em conflito"
		return nil
	}
	abs := filepath.Join(p.Path, file)
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	args := append(append([]string{}, parts[1:]...), abs)
	cmd := exec.Command(parts[0], args...)
	cmd.Dir = p.Path
	path := p.Path
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return gitActionDoneMsg{path: path, action: "edit", branch: file, cmdline: editor + " " + file, err: err}
	})
}

// gitCompactError converte mensagens de erro multi-linha do git em uma única linha
// compacta para exibição na notifLine da aba Git.
func gitCompactError(action, errText string) string {
	lines := strings.Split(strings.TrimSpace(errText), "\n")
	if len(lines) <= 1 {
		return action + ": " + errText
	}

	// Conta arquivos com alterações locais (indentados com tab pelo git)
	fileCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "\t") {
			fileCount++
		}
	}
	if fileCount > 0 {
		return fmt.Sprintf("%s: %d arquivo(s) com alterações locais — faça commit ou stash antes", action, fileCount)
	}

	// Pega a última linha não-vazia e não genérica como resumo
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" && l != "Aborting" && l != "error" {
			return action + ": " + l
		}
	}
	return action + ": " + strings.TrimSpace(lines[0])
}
