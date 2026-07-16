						package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/collectors"
)

type tickMsg struct{}

type App struct {
	store    *core.StateStore
	cfg      *config.Config
	snapshot core.Snapshot
	view     View
	cursor   int
	filter   string
	filterOn bool
	filterInput string

	helpOn      bool
	helpScroll  int

	selectedProject *core.Project
	tab             Tab
	tabCursor       int
	gitFocus        gitFocus
	gitSubview      gitSubview
	gitBranchCursor int
	gitBranchScroll int
	gitCommitCursor int
	gitCommitScroll int
	gitFileCursor   int
	gitFileScroll   int
	gitViewBranch   string
	gitBranchCommits []core.GitCommit
	gitBranchLoading bool
	gitSelectedCommit core.GitCommit
	gitCommitFiles []core.GitCommitFileChange
	gitCommitFilesLoading bool
	gitCommitFileCursor int
	gitCommitFileScroll int
	gitBranchFilterOn   bool
	gitBranchFilterInput string
	gitBranchFilter     string
	gitSelectedCommits  map[string]bool
	gitCommitSelectAnchor int
	gitCherryPickBuffer []string
	gitCherryPickMarked map[string]bool
	gitCherryPickActive bool
	gitCherryPickSourceBranch string
	gitStatusMsg        string
	gitActionLoading    bool
	gitPromptOn         bool
	gitPromptKind       gitPromptKind
	gitPromptInput      string
	gitPromptBranch     string
	gitConfirmOn        bool
	gitConfirmAction    string
	gitConfirmBranch    string
	gitBranchLoadGen    int
	gitRenderCache      *core.GitInfo
	gitMarkedBranch     string
	gitBranches         []core.GitBranch
	gitBranchDenylist   map[string]struct{}
	dashboardScroll     int
	dashboardSubview    dashboardSubview
	projectShellExitErr string
	gitCommitFullMsg    string
	gitCommitMsgScroll  int
	gitCommitMsgCursor  int
	gitCommitDetailFocus gitCommitDetailFocus
	containerSubview      containerSubview
	containerScroll       int
	containerStatusMsg    string
	containerActions      map[string]string
	containerShellExitErr string
	containerDetailTab         containerDetailTab
	containerDetailID          string
	containerDetailName        string
	containerDetailProjectPath string
	containerDetailScroll      int
	containerDetailContent     string
	containerDetailLoading     bool
	containerDetailCache       map[containerDetailTab]string
	fuzzyOn               bool
	fuzzyInput            string
	deployConfirm         bool
	containerConfirmRemove bool
	projectLogs           string
	projectLogsLoading    bool
	projectLogsFollow     bool
	projectLogsPaused     bool
	projectLogContainerID string
	projectLogSource      string
	statusMsg             string
	width           int
	height          int
	now             time.Time
	quitting        bool
}

func NewApp(store *core.StateStore, cfg *config.Config) *App {
	InitTheme(cfg.UI.Theme)
	a := &App{
		store:    store,
		cfg:      cfg,
		snapshot: store.Get(),
		view:     ViewDashboard,
		tab:      TabOverview,
		now:      time.Now(),
	}
	a.openProjectFromCwd()
	return a
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} }),
	)
}

func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if a.helpOn {
			return a.updateHelp(msg)
		}
		if a.containerSubview == containerSubviewShellReturn {
			switch msg.String() {
			case "enter", "esc":
				cmd := a.dismissContainerShellReturn()
				return a, cmd
			case "q", "ctrl+c":
				a.quitting = true
				return a, tea.Quit
			}
			return a, nil
		}
		if a.gitPromptOn {
			return a.updateGitPrompt(msg)
		}
		if a.gitConfirmOn {
			return a.updateGitConfirm(msg)
		}
		if a.gitBranchFilterOn {
			return a.updateGitBranchFilter(msg)
		}
		if a.fuzzyOn {
			return a.updateFuzzy(msg)
		}
		if a.filterOn {
			return a.updateFilter(msg)
		}
		if a.deployConfirm {
			return a.updateDeployConfirm(msg)
		}
		if a.containerConfirmRemove {
			return a.updateContainerRemoveConfirm(msg)
		}
		return a.updateKey(msg)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.view == ViewDashboard {
			projects := filterNestedProjects(sortProjects(a.filteredProjects()))
			a.syncDashboardScroll(len(projects))
		}
		return a, nil

	case tickMsg:
		a.snapshot = a.store.Get()
		a.now = time.Now()
		if a.view == ViewProject && a.tab == TabGit {
			if p := a.currentProject(); p != nil && p.Git != nil && p.Git.IsRepo {
				a.syncGitBranchesFrom(p)
			}
		}
		return a, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} })

	case gitCommitsLoadedMsg:
		a.handleGitCommitsLoaded(msg)
		return a, nil

	case gitCommitDetailLoadedMsg:
		a.handleGitCommitDetailLoaded(msg)
		return a, nil

	case gitActionDoneMsg:
		a.handleGitActionDone(msg)
		if msg.err == nil && needsGitBranchCommitsReload(msg.action) {
			branch := a.gitViewBranch
			if msg.action == "rename-branch" && msg.newBranch != "" {
				branch = msg.newBranch
			}
			if branch == "" {
				if p := a.currentProject(); p != nil && p.Git != nil {
					branch = p.Git.Branch
				}
			}
			if branch != "" {
				return a, a.requestGitBranchCommits(msg.path, branch)
			}
		}
		return a, nil

	case containerDetailLoadedMsg:
		a.handleContainerDetailLoaded(msg)
		return a, nil

	case projectLogFollowMsg:
		if a.projectLogContainerID == msg.id && a.projectLogsFollow && !a.projectLogsPaused {
			if msg.logs != "" {
				a.projectLogs += msg.logs
			}
		}
		if a.projectLogsFollow && a.projectLogContainerID != "" && !a.projectLogsPaused {
			return a, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				logs, _ := collectors.DockerLogsSince(a.projectLogContainerID, 2, 80)
				return projectLogFollowMsg{id: a.projectLogContainerID, logs: logs}
			})
		}
		return a, nil

	case deployDoneMsg:
		if msg.err != nil {
			a.statusMsg = "deploy: " + msg.err.Error()
		} else {
			a.statusMsg = "deploy concluído ✓"
		}
		a.deployConfirm = false
		a.snapshot = a.store.Get()
		return a, nil

	case lazyGitDoneMsg:
		a.snapshot = a.store.Get()
		if msg.err != nil {
			a.statusMsg = "lazygit: " + msg.err.Error()
		}
		return a, nil

	case containerActionDoneMsg:
		a.handleContainerActionDone(msg)
		return a, nil

	case containerShellDoneMsg:
		cmd := a.handleContainerShellDone(msg)
		return a, cmd

	case dockerRefreshedMsg:
		a.snapshot = a.store.Get()
		containers := a.currentProjectContainers()
		if len(containers) > 0 {
			a.tabCursor = clampCursor(a.tabCursor, len(containers))
			a.syncContainerScroll(len(containers))
		}
		return a, nil

	case composeDoneMsg:
		a.handleComposeDone(msg)
		return a, nil

	case projectLogsLoadedMsg:
		a.handleProjectLogsLoaded(msg)
		return a, nil

	case projectShellDoneMsg:
		a.handleProjectShellDone(msg)
		return a, nil

	case containerShellFallbackMsg:
		return a, a.containerExecShellFallback(msg)

	case tea.QuitMsg:
		a.quitting = true
		return a, tea.Quit
	}
	return a, nil
}

func (a *App) updateGitBranchFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.gitBranchFilterOn = false
		a.gitBranchFilterInput = ""
		a.gitBranchFilter = ""
		if p := a.currentProject(); p != nil && p.Git != nil {
			a.syncGitBranchCursor(p.Git.Branches)
		}
	case "enter":
		a.gitBranchFilterOn = false
		a.gitBranchFilter = a.gitBranchFilterInput
		a.gitBranchFilterInput = ""
		if p := a.currentProject(); p != nil && p.Git != nil {
			a.syncGitBranchCursor(p.Git.Branches)
		}
	case "backspace":
		if len(a.gitBranchFilterInput) > 0 {
			a.gitBranchFilterInput = a.gitBranchFilterInput[:len(a.gitBranchFilterInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			a.gitBranchFilterInput += msg.String()
		}
	}
	return a, nil
}

func (a *App) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.filterOn = false
		a.filterInput = ""
		a.filter = ""
	case "enter":
		a.filterOn = false
		a.filter = a.filterInput
		a.filterInput = ""
		a.dashboardScroll = 0
	case "backspace":
		if len(a.filterInput) > 0 {
			a.filterInput = a.filterInput[:len(a.filterInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			a.filterInput += msg.String()
		}
	}
	return a, nil
}

func (a *App) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		a.quitting = true
		return a, tea.Quit

	case msg.String() == "?":
		a.helpOn = true
		a.helpScroll = 0
		return a, nil

	case msg.String() == "/":
		a.filterOn = true
		a.filterInput = ""
		return a, nil

	case msg.String() == "ctrl+p":
		a.fuzzyOn = true
		a.fuzzyInput = a.filter
		return a, nil
	}

	switch a.view {
	case ViewDashboard:
		return a.updateDashboard(msg)
	case ViewProject:
		return a.updateProject(msg)
	}
	return a, nil
}

func (a *App) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	helpLines := strings.Split(strings.TrimSpace(getHelpText()), "\n")
	viewport := 12 // altura do conteúdo na telinha de ajuda
	maxScroll := len(helpLines) - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.String() {
	case "esc", "?":
		a.helpOn = false
		a.helpScroll = 0
	case "up", "k":
		if a.helpScroll > 0 {
			a.helpScroll--
		}
	case "down", "j":
		if a.helpScroll < maxScroll {
			a.helpScroll++
		}
	case "pgup":
		a.helpScroll -= viewport
		if a.helpScroll < 0 {
			a.helpScroll = 0
		}
	case "pgdown":
		a.helpScroll += viewport
		if a.helpScroll > maxScroll {
			a.helpScroll = maxScroll
		}
	}
	return a, nil
}

func (a *App) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.dashboardSubview == dashboardSubviewShellReturn {
		switch msg.String() {
		case "enter", "esc":
			a.dismissProjectShellReturn()
		case "q", "ctrl+c":
			a.quitting = true
			return a, tea.Quit
		}
		return a, nil
	}

	projects := filterNestedProjects(sortProjects(a.filteredProjects()))

	switch msg.String() {
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
			a.syncDashboardScroll(len(projects))
		}
	case "down", "j":
		if a.cursor < len(projects)-1 {
			a.cursor++
			a.syncDashboardScroll(len(projects))
		}
	case "enter":
		if len(projects) > 0 && a.cursor < len(projects) {
			a.openProject(projects[a.cursor], TabOverview)
		}
	case "E", "shift+e":
		if len(projects) > 0 && a.cursor < len(projects) {
			return a, a.projectExecShell(projects[a.cursor].Path)
		}
	case "g":
		if len(projects) > 0 && a.cursor < len(projects) {
			a.openProject(projects[a.cursor], TabGit)
		}
	case "c":
		if len(projects) > 0 && a.cursor < len(projects) {
			a.openProject(projects[a.cursor], TabContainers)
		}
	case "r":
		a.snapshot = a.store.Get()
	}
	return a, nil
}

func (a *App) openProject(p core.Project, tab Tab) tea.Cmd {
	cp := p
	a.selectedProject = &cp
	a.view = ViewProject
	a.tab = tab
	a.tabCursor = 0
	if tab == TabGit {
		a.initGitTab(&cp)
	}
	if tab == TabContainers {
		a.initContainersTab()
	}
	if tab == TabLogs {
		return a.initLogsTab(&cp)
	}
	return nil
}

func (a *App) updateProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.containerSubview == containerSubviewShellReturn {
		switch msg.String() {
		case "enter", "esc":
			cmd := a.dismissContainerShellReturn()
			return a, cmd
		}
		return a, nil
	}

	p := a.currentProject()
	if p == nil {
		return a, nil
	}

	if a.tab == TabContainers && a.containerSubview == containerSubviewDetail {
		return a.handleContainerDetailKeys(msg, p)
	}

	switch msg.String() {
	case "esc":
		if a.tab == TabGit && a.gitSubview == gitSubviewCommit {
			a.gitSubview = gitSubviewMain
			return a, nil
		}
		if a.tab == TabContainers && a.containerSubview == containerSubviewDetail {
			a.containerSubview = containerSubviewList
			a.containerDetailCache = nil
			return a, nil
		}
		a.view = ViewDashboard
		a.selectedProject = nil
		a.gitRenderCache = nil
		return a, nil
	case "tab":
		if a.tab == TabGit && a.gitSubview == gitSubviewCommit {
			if a.gitCommitDetailFocus == gitCommitFocusMessage {
				a.gitCommitDetailFocus = gitCommitFocusFiles
			} else {
				a.gitCommitDetailFocus = gitCommitFocusMessage
			}
			return a, nil
		}
		a.tab = Tab((int(a.tab) + 1) % len(AllTabs))
		a.tabCursor = 0
		if a.tab == TabGit {
			if p := a.currentProject(); p != nil {
				a.initGitTab(p)
			}
		}
		if a.tab == TabContainers {
			a.initContainersTab()
		}
	case "shift+tab":
		i := int(a.tab) - 1
		if i < 0 {
			i = len(AllTabs) - 1
		}
		a.tab = Tab(i)
		a.tabCursor = 0
		if a.tab == TabGit {
			if p := a.currentProject(); p != nil {
				a.initGitTab(p)
			}
		}
		if a.tab == TabContainers {
			a.initContainersTab()
		}
	case "1":
		a.tab = TabOverview
		a.tabCursor = 0
	case "2":
		a.tab = TabGit
		a.tabCursor = 0
		if p := a.currentProject(); p != nil {
			a.initGitTab(p)
		}
	case "3":
		a.tab = TabContainers
		a.tabCursor = 0
		a.initContainersTab()
	case "4":
		a.tab = TabHealth
		a.tabCursor = 0
	case "5":
		a.tab = TabLogs
		a.tabCursor = 0
		if cmd := a.initLogsTab(p); cmd != nil {
			return a, cmd
		}
	case "6":
		a.tab = TabMetrics
		a.tabCursor = 0
	case "L":
		return a, a.openLazyGit(p.Path)
	case "o", "O":
		if a.gitTabReady(p) {
			a.gitOpenPullRequest(p)
			return a, nil
		}
		a.openProjectURL(p)
	case "D":
		if a.gitTabReady(p) {
			a.gitToggleMarkedBranch(p)
			return a, nil
		}
		if p.DeployScript != "" {
			a.deployConfirm = true
			a.statusMsg = "confirmar deploy (" + p.DeployScript + ")? y/n"
		} else {
			a.statusMsg = "nenhum script de deploy detectado"
		}
	case "E", "shift+e":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerExecShell(c)
			}
		}
		return a, a.projectExecShell(p.Path)
	case "shift+u", "U":
		if p.HasDockerCompose || collectors.ComposeFile(p.Path) != "" {
			return a, a.composeUp(p.Path)
		}
		a.statusMsg = "docker-compose não encontrado"
	case "shift+d":
		if p.HasDockerCompose || collectors.ComposeFile(p.Path) != "" {
			return a, a.composeDown(p.Path)
		}
		a.statusMsg = "docker-compose não encontrado"
	case "R":
		if a.gitTabReady(p) {
			a.startGitRenameBranch(p)
			return a, nil
		}
		if p.HasDockerCompose || collectors.ComposeFile(p.Path) != "" {
			return a, a.composeRestart(p.Path)
		}
	case "d":
		if a.gitTabReady(p) {
			a.startGitDeleteBranch(p)
			return a, nil
		}
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				a.containerConfirmRemove = true
				a.containerStatusMsg = "remover " + c.Name + "? y/n"
			}
		}
	case "f":
		if a.tab == TabLogs {
			return a, a.startProjectLogsFollow()
		}
	case "r":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerStartOrRestart(c)
			}
		}
		if a.tab == TabLogs {
			a.projectLogsLoading = true
			return a, a.loadProjectLogs(p)
		}
		if a.tab == TabLogs && a.projectLogsFollow {
			a.projectLogsPaused = !a.projectLogsPaused
			if !a.projectLogsPaused {
				return a, a.startProjectLogsFollow()
			}
			return a, nil
		}
	case "p":
		if a.gitTabReady(p) {
			return a, a.gitPull(p)
		}
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerPause(c)
			}
		}
		if a.tab == TabLogs && a.projectLogsFollow {
			a.projectLogsPaused = !a.projectLogsPaused
			if !a.projectLogsPaused {
				return a, a.startProjectLogsFollow()
			}
			return a, nil
		}
	case "m":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.openContainerDetail(c, p.Path)
			}
		}
	case "n", "N":
		if a.gitTabReady(p) {
			a.startGitNewBranch(p)
			return a, nil
		}
	case "shift+r", "shift+R":
		if a.gitTabReady(p) {
			a.startGitRenameBranch(p)
			return a, nil
		}
	case "shift+m", "shift+M", "M":
		if a.gitTabReady(p) {
			a.startGitMerge(p)
			return a, nil
		}
	case "shift+p", "shift+P", "P":
		if a.gitTabReady(p) {
			return a, a.gitPush(p)
		}
	case "s":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerStop(c)
			}
		}
	case "x":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo && a.gitFocus == gitFocusCommits {
			a.toggleGitCommitSelection(p)
		}
	case "b":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo {
			a.gitBranchFilterOn = true
			a.gitBranchFilterInput = a.gitBranchFilter
			a.gitFocus = gitFocusBranches
			return a, nil
		}
	case " ":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo {
			return a, a.gitSpaceAction(p)
		}
	case "shift+c", "shift+C", "C":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo {
			a.gitCherryPickCopy(p)
		}
	case "shift+v", "shift+V", "V":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo {
			return a, a.gitCherryPickPaste(p)
		}
	case "up", "k", "shift+up", "shift+k":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			shift := strings.HasPrefix(msg.String(), "shift+")
			return a, a.updateGitCursor(-1, p, shift)
		}
		if cmd := a.tabNav(-1, p); cmd != nil {
			return a, cmd
		}
	case "down", "j", "shift+down", "shift+j":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			shift := strings.HasPrefix(msg.String(), "shift+")
			return a, a.updateGitCursor(1, p, shift)
		}
		if cmd := a.tabNav(1, p); cmd != nil {
			return a, cmd
		}
	case "left":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			a.gitFocusPrev()
		}
	case "h", "H":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			a.gitFocusPrev()
		} else if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.tab = TabHealth
			a.tabCursor = 0
		}
	case "right":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			a.gitFocusNext()
		}
	case "l":
		if a.tab == TabGit && a.gitSubview != gitSubviewCommit {
			a.gitFocusNext()
		} else if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.tab = TabLogs
			a.tabCursor = 0
			if cmd := a.initLogsTab(p); cmd != nil {
				return a, cmd
			}
		}
	case "enter":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.openContainerDetail(c, p.Path)
			}
		}
		if a.tab == TabGit && p.Git != nil && p.Git.IsRepo {
			if a.gitSubview == gitSubviewCommit {
				return a, nil
			}
			if a.gitFocus == gitFocusBranches {
				branches := a.filteredGitBranches(p.Git.Branches)
				if a.gitBranchCursor < len(branches) {
					branch := branches[a.gitBranchCursor].Name
					if cmd := a.selectGitBranch(p, branch); cmd != nil {
						a.gitFocus = gitFocusCommits
						return a, cmd
					}
					a.gitFocus = gitFocusCommits
				}
			}
			if a.gitFocus == gitFocusCommits {
				commits := a.gitDisplayedCommits()
				if a.gitCommitCursor < len(commits) {
					return a, a.openGitCommitDetail(p, commits[a.gitCommitCursor])
				}
			}
		}
	}
	return a, nil
}

func (a *App) tabNav(delta int, p *core.Project) tea.Cmd {
	switch a.tab {
	case TabContainers:
		a.updateContainerCursor(delta, p)
	}
	return nil
}

func (a *App) currentProject() *core.Project {
	if a.selectedProject == nil {
		return nil
	}
	for _, p := range a.snapshot.Projects {
		if p.ID == a.selectedProject.ID {
			cp := p
			a.selectedProject = &cp
			return &cp
		}
	}
	for _, p := range a.snapshot.Projects {
		if p.Path == a.selectedProject.Path {
			cp := p
			a.selectedProject = &cp
			return &cp
		}
	}
	return a.selectedProject
}

func (a *App) filteredProjects() []core.Project {
	if a.filter == "" {
		return a.snapshot.Projects
	}
	f := strings.ToLower(a.filter)
	var result []core.Project
	for _, p := range a.snapshot.Projects {
		if strings.Contains(strings.ToLower(p.Name), f) ||
			strings.Contains(strings.ToLower(p.Framework.Name), f) ||
			strings.Contains(strings.ToLower(p.Path), f) {
			result = append(result, p)
			continue
		}
		if p.Git != nil && strings.Contains(strings.ToLower(p.Git.Branch), f) {
			result = append(result, p)
			continue
		}
		for _, d := range p.Domains {
			if strings.Contains(strings.ToLower(d.Host), f) {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

func (a *App) View() string {
	if a.quitting {
		return ""
	}

	if a.helpOn {
		return a.renderHelpPopup()
	}

	if a.fuzzyOn {
		return a.renderFuzzyPrompt()
	}

	if a.filterOn {
		return a.renderFilterPrompt()
	}

	if a.gitBranchFilterOn {
		return a.renderGitBranchFilterPrompt()
	}

	if a.gitPromptOn {
		return a.renderGitPrompt()
	}

	if a.dashboardSubview == dashboardSubviewShellReturn && a.view == ViewDashboard {
		return a.renderFullShellReturn(a.projectShellExitErr)
	}

	if a.containerSubview == containerSubviewShellReturn {
		return a.renderFullShellReturn(a.containerShellExitErr)
	}

	switch a.view {
	case ViewProject:
		return a.renderProject()
	default:
		return a.renderDashboard()
	}
}

func (a *App) renderGitBranchFilterPrompt() string {
	p := a.currentProject()
	content := a.renderProject()
	if p == nil {
		return content
	}
	prompt := StylePanel.Render("Buscar branch: " + a.gitBranchFilterInput + "█")
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		prompt,
		a.renderStatusBar("type to filter branches | enter confirm | esc cancel"),
	)
}

func (a *App) renderFilterPrompt() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderHeader(),
		"",
		StylePanel.Render("Filter: "+a.filterInput+"█"),
		a.renderStatusBar("type to filter | enter confirm | esc cancel"),
	)
}

func (a *App) renderHeader() string {
	m := a.snapshot.HostMetrics
	title := StyleTitle.Render("DevScope")
	metrics := StyleMetric.Render(fmt.Sprintf(
		"CPU %.0f%%  RAM %.0f%%  DISK %.0f%%",
		m.CPUPercent, m.MemoryPercent, m.DiskPercent,
	))
	line := strings.Repeat("─", maxInt(a.width-2, 40))
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", metrics),
		StyleMuted.Render(line),
	)
}

func (a *App) renderProject() string {
	p := a.currentProject()
	if p == nil {
		return a.renderDashboard()
	}

	tabs := a.renderTabs()
	content := a.renderTabContent(p)
	hints := "tab switch  esc back  q quit"
	if a.tab == TabGit {
		if a.gitSubview == gitSubviewCommit {
			hints = "↑↓ scroll  tab message/files  esc back  " + hints
		} else {
			hints = "space checkout  shift+↑↓ range  x toggle  shift+c copy  shift+v paste  b filter  " + hints
		}
	}
	if a.tab == TabContainers {
		if a.containerSubview == containerSubviewDetail {
			hints = "←→ tabs  ↑↓ scroll  esc back  " + hints
		} else {
			hints = "↑↓ navigate  enter detalhe  shift+e shell  s stop  r start/restart  p pause  d remove  shift+u up  shift+d down  " + hints
		}
	}

	header := lipgloss.JoinVertical(lipgloss.Left,
		a.renderHeader(),
		"",
		StyleTitle.Render(p.Name),
		StyleMuted.Render(fmt.Sprintf("%s  •  %s  •  %d containers",
			p.Status, p.Health, p.ContainerCount)),
		"",
		tabs,
		"",
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		"",
		a.renderStatusBar(hints),
	)
}

func (a *App) renderTabs() string {
	var parts []string
	for _, t := range AllTabs {
		label := t.String()
		if t == TabContainers && a.selectedProject != nil {
			if n := a.containersCount(a.currentProject()); n > 0 {
				label = fmt.Sprintf("%s (%d)", label, n)
			}
		}
		if t == a.tab {
			parts = append(parts, StyleTabActive.Render(label))
		} else {
			parts = append(parts, StyleTab.Render(label))
		}
	}
	return strings.Join(parts, " │ ")
}

func (a *App) renderTabContent(p *core.Project) string {
	switch a.tab {
	case TabGit:
		return a.renderGitTab(p)
	case TabContainers:
		return a.renderContainersTab(p)
	case TabMetrics:
		return a.renderMetricsTab(p)
	case TabHealth:
		return a.renderHealthTab(p)
	case TabLogs:
		return a.renderLogsTab(p)
	default:
		return a.renderOverviewTab(p)
	}
}

func (a *App) renderOverviewTab(p *core.Project) string {
	lines := []string{
		fmt.Sprintf("Path:         %s", p.Path),
		fmt.Sprintf("Status:       %s", p.Status),
		fmt.Sprintf("Health:       %s", p.Health),
		"",
		"Frameworks:",
	}

	frameworks := p.Frameworks
	if len(frameworks) == 0 && p.Framework.Name != "" && p.Framework.Name != "Unknown" {
		frameworks = []core.FrameworkInfo{p.Framework}
	}
	if len(frameworks) == 0 {
		lines = append(lines, "  (nenhum detectado ainda — aguarde o scan)")
	} else {
		for _, fw := range frameworks {
			line := fmt.Sprintf("  • %s (%s)", fw.Name, fw.Language)
			if fw.Version != "" {
				line += fmt.Sprintf("  v%s", fw.Version)
			}
			lines = append(lines, line)
		}
	}

	if p.HasDockerCompose {
		lines = append(lines, "", "Docker:       docker-compose detectado")
	}
	if p.HasDockerfile {
		lines = append(lines, "Docker:       Dockerfile detectado")
	}
	if p.ContainerCount > 0 {
		lines = append(lines, fmt.Sprintf("Containers:   %d vinculados", p.ContainerCount))
	}
	if p.WorkerCount > 0 {
		lines = append(lines, fmt.Sprintf("PM2 Workers:  %d", p.WorkerCount))
		for _, w := range p.Workers {
			lines = append(lines, fmt.Sprintf("  • %s [%s] CPU %.1f%%", w.Name, w.Status, w.CPU))
		}
	}
	if p.Metrics.CPUPercent > 0 || p.Metrics.MemoryMB > 0 {
		lines = append(lines, fmt.Sprintf("Metrics:      CPU %.1f%%  RAM %d MB", p.Metrics.CPUPercent, p.Metrics.MemoryMB))
	}
	if len(p.Ports) > 0 {
		lines = append(lines, fmt.Sprintf("Ports:        %s", collectors.FormatPortsShort(p.Ports, 5)))
	}
	if p.DeployScript != "" {
		lines = append(lines, fmt.Sprintf("Deploy:       %s (D)", p.DeployScript))
	}

	if len(p.Modules) > 0 {
		lines = append(lines, "", "Modules:")
		for _, m := range p.Modules {
			lines = append(lines, fmt.Sprintf("  • %s [%s] — %s", m.Name, m.Role, m.Path))
		}
	}

	if p.Git != nil && p.Git.IsRepo {
		lines = append(lines, "",
			fmt.Sprintf("Git Branch:   %s", p.Git.Branch),
			fmt.Sprintf("Last Commit:  %s — %s", p.Git.LastCommit, p.Git.LastCommitMsg),
		)
	}

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderMetricsTab(p *core.Project) string {
	m := a.snapshot.HostMetrics
	lines := []string{
		"Host Metrics:",
		fmt.Sprintf("  CPU:    %.1f%%", m.CPUPercent),
		fmt.Sprintf("  RAM:    %.1f%% (%d / %d MB)", m.MemoryPercent, m.MemoryUsedMB, m.MemoryTotalMB),
		fmt.Sprintf("  Disk:   %.1f%% (%.1f / %.1f GB)", m.DiskPercent, m.DiskUsedGB, m.DiskTotalGB),
		fmt.Sprintf("  Swap:   %.1f%%", m.SwapPercent),
		"",
		"Project Metrics:",
		fmt.Sprintf("  CPU:        %.1f%%", p.Metrics.CPUPercent),
		fmt.Sprintf("  Memory:     %d MB", p.Metrics.MemoryMB),
		fmt.Sprintf("  Containers: %d", p.ContainerCount),
		fmt.Sprintf("  Workers:    %d", p.WorkerCount),
	}
	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderHelpPopup() string {
	helpLines := strings.Split(strings.TrimSpace(getHelpText()), "\n")
	viewport := 12 // Altura visível de conteúdo

	maxScroll := len(helpLines) - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.helpScroll > maxScroll {
		a.helpScroll = maxScroll
	}
	if a.helpScroll < 0 {
		a.helpScroll = 0
	}

	var visibleLines []string
	start := a.helpScroll
	end := minInt(start+viewport, len(helpLines))

	if start > 0 {
		visibleLines = append(visibleLines, StyleMuted.Render(fmt.Sprintf("  ↑ %d comandos acima", start)))
	} else {
		visibleLines = append(visibleLines, "")
	}

	for i := start; i < end; i++ {
		visibleLines = append(visibleLines, helpLines[i])
	}

	for len(visibleLines) < viewport+1 {
		visibleLines = append(visibleLines, "")
	}

	if end < len(helpLines) {
		visibleLines = append(visibleLines, StyleMuted.Render(fmt.Sprintf("  ↓ %d comandos abaixo", len(helpLines)-end)))
	} else {
		visibleLines = append(visibleLines, "")
	}

	title := StyleSection.Render("Ajuda — Atalhos do DevScope")
	footer := StyleMuted.Render("↑/↓ scroll  │  esc ou ? fechar")

	helpBox := StylePanel.Render(title + "\n\n" + strings.Join(visibleLines, "\n") + "\n\n" + footer)

	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderHeader(),
		"",
		helpBox,
		a.renderStatusBar("ajuda aberta | esc ou ? para fechar"),
	)
}

func getHelpText() string {
	return `Navigation:
  ↑/k, ↓/j     Navegar na lista
  Enter        Abrir projeto / Ver detalhes
  Esc          Voltar / Fechar
  Tab          Próxima aba (na view de projeto)
  /            Filtrar projetos
  ctrl+p       Filtro fuzzy de projetos
  ?            Alternar exibição de ajuda
  q            Sair do DevScope

Dashboard:
  shift+e      Abrir terminal no diretório do projeto
  g            Abrir direto na aba Git
  c            Abrir direto na aba Containers
  r            Forçar atualização rápida

Abas de Projeto:
  1-6          Overview, Git, Containers, Health, Logs, Metrics
  h            Ir para aba Health
  l            Ir para aba Logs
  L            Abrir LazyGit no projeto
  D            Executar Deploy script (confirmação y/n)
  shift+u      Docker compose up -d
  shift+d      Docker compose down
  R            Docker compose restart
  o            Abrir URL do projeto no navegador

Aba Git:
  space        Checkout de branch (ou toggle commit)
  shift+↑/↓    Selecionar range de commits
  x            Toggle de seleção de commit individual
  shift+c      Copiar commits selecionados (cherry-pick)
  shift+v      Colar commits (cherry-pick) na branch destino
  b            Filtrar lista de branches
  enter        Ver detalhes do commit
  n            Criar nova branch
  d            Apagar branch (confirmação y/esc)
  D            Marcar branch de origem
  shift+R / R  Renomear branch
  o            Abrir Pull Request no GitHub
  shift+m / M  Mesclar branch na atual (confirmação y/esc)
  p            Pull origin da branch pai
  shift+P / P  Push
  ←/→ or h/l   Alternar foco entre colunas (Branches / Commits)

Aba Containers:
  enter / m    Monitoramento de detalhes do container
  shift+e      Abrir shell interativo dentro do container
  s            Parar container (stop)
  r            Iniciar/Reiniciar container
  p            Pausar/Retomar container
  d            Remover container (confirmação y/n)
  shift+u      Docker compose up -d
  shift+d      Docker compose down

Detalhes do Container:
  ←/→          Alternar abas (Logs, Stats, Env, Config, etc.)
  ↑/↓          Rolar conteúdo do log / stats
  esc          Voltar para a lista de containers

CLI & Configuração:
  devscope scan --json
  devscope watch
  Configuração em: ~/.config/devscope/config.yaml`
}

func (a *App) renderStatusBar(hints string) string {
	scanInfo := ""
	if !a.snapshot.ScannedAt.IsZero() {
		scanInfo = fmt.Sprintf("scanned %s ago | ", time.Since(a.snapshot.ScannedAt).Round(time.Second))
	}
	return StyleStatusBar.Render(scanInfo + hints)
}
