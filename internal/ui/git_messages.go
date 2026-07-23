package ui

import (
	"fmt"
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
	if len(p.Git.Files) == 0 {
		a.gitFileCursor = 0
		a.gitFileScroll = 0
		return
	}
	if a.gitFileCursor >= len(p.Git.Files) {
		a.gitFileCursor = len(p.Git.Files) - 1
	}
	a.gitFileScroll = ensureVisible(a.gitFileCursor, a.gitFileScroll, a.gitFilesViewport(), len(p.Git.Files))
}

func (a *App) requestGitWorkingTreeDiff(path, file string) tea.Cmd {
	if path == "" || file == "" {
		return nil
	}
	a.gitWTDiffFile = file
	return func() tea.Msg {
		return gitWTDiffMsg{file: file, diff: collectors.CollectWorkingTreeDiff(path, file)}
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
	if len(p.Git.Files) == 0 || a.gitFileCursor >= len(p.Git.Files) {
		a.gitStatusMsg = "nenhum arquivo para stage"
		return nil
	}
	f := p.Git.Files[a.gitFileCursor]
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
		err := collectors.GitCheckout(path, branch)
		return gitActionDoneMsg{path: path, action: "checkout", branch: branch, err: err}
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
		current := collectors.GitCurrentBranch(path)
		if branch != current && target != current {
			if err := collectors.GitCheckout(path, target); err != nil {
				return gitActionDoneMsg{path: path, action: "merge", branch: branch, err: err}
			}
		}
		err := collectors.GitMerge(path, branch)
		return gitActionDoneMsg{path: path, action: "merge", branch: branch, err: err}
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
		err := collectors.GitPullOrigin(path, source)
		return gitActionDoneMsg{path: path, action: "pull", branch: source, err: err}
	}
}

func (a *App) gitPush(p *core.Project) tea.Cmd {
	a.gitActionLoading = true
	a.gitStatusMsg = "push..."
	path := p.Path
	return func() tea.Msg {
		err := collectors.GitPush(path)
		return gitActionDoneMsg{path: path, action: "push", err: err}
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
		current := collectors.GitCurrentBranch(path)
		if target != "" && target != current {
			if err := collectors.GitCheckout(path, target); err != nil {
				return gitActionDoneMsg{path: path, action: "cherry-pick", branch: target, count: count, err: err}
			}
		}
		err := collectors.GitCherryPick(path, hashes)
		return gitActionDoneMsg{path: path, action: "cherry-pick", branch: target, count: count, err: err}
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
	a.gitCommitFileScroll = 0
	if len(msg.files) == 0 {
		a.gitCommitDiff = "(nenhum arquivo alterado)"
		a.gitCommitDiffLoading = false
		return nil
	}
	return a.requestGitCommitFileDiff(msg.path, msg.hash, msg.files[0].Path)
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
	case "checkout", "cherry-pick", "create-branch", "rename-branch", "commit":
		return true
	default:
		return false
	}
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
	case "merge":
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "merge " + msg.branch + " ✓"
	case "pull":
		a.gitViewBranch = p.Git.Branch
		a.gitBranchCommits = p.Git.Commits
		a.gitBranchLoading = false
		a.syncGitBranchCursor(p.Git.Branches)
		a.gitStatusMsg = "pull origin " + msg.branch + " ✓"
	case "push":
		a.gitBranchLoading = false
		a.gitStatusMsg = "push ✓"
	case "add":
		a.gitStatusMsg = "staged " + msg.branch + " ✓"
	case "add-all":
		a.gitStatusMsg = "todos os arquivos em stage ✓"
	case "unstage":
		a.gitStatusMsg = "unstage " + msg.branch + " ✓"
	case "unstage-all":
		a.gitStatusMsg = "todos removidos do stage ✓"
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
