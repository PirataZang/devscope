package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
	"github.com/devscope/devscope/internal/core"
	"github.com/mattn/go-runewidth"
)

type tickMsg struct{}

type App struct {
	store       *core.StateStore
	cfg         *config.Config
	snapshot    core.Snapshot
	view        View
	cursor      int
	filter      string
	filterOn    bool
	filterInput string

	helpOn     bool
	helpScroll int

	selectedProject             *core.Project
	tab                         Tab
	tabCursor                   int
	gitFocus                    gitFocus
	gitSubview                  gitSubview
	gitBranchCursor             int
	gitBranchScroll             int
	gitCommitCursor             int
	gitCommitScroll             int
	gitFileCursor               int
	gitFileScroll               int
	gitViewBranch               string
	gitBranchCommits            []core.GitCommit
	gitBranchLoading            bool
	gitSelectedCommit           core.GitCommit
	gitCommitFiles              []core.GitCommitFileChange
	gitCommitFilesLoading       bool
	gitCommitFileCursor         int
	gitCommitFileScroll         int
	gitBranchFilterOn           bool
	gitBranchFilterInput        string
	gitBranchFilter             string
	gitSelectedCommits          map[string]bool
	gitCommitSelectAnchor       int
	gitCherryPickBuffer         []string
	gitCherryPickMarked         map[string]bool
	gitCherryPickActive         bool
	gitCherryPickSourceBranch   string
	gitStatusMsg                string
	gitActionLoading            bool
	gitPromptOn                 bool
	gitPromptKind               gitPromptKind
	gitPromptInput              string
	gitPromptCursor             int
	gitPromptBranch             string
	gitConfirmOn                bool
	gitConfirmAction            string
	gitConfirmBranch            string
	gitBranchLoadGen            int
	gitRenderCache              *core.GitInfo
	gitMarkedBranch             string
	gitBranches                 []core.GitBranch
	gitBranchDenylist           map[string]struct{}
	dashboardScroll             int
	dashboardSubview            dashboardSubview
	projectShellExitErr         string
	gitCommitFullMsg            string
	gitCommitMsgScroll          int
	gitCommitMsgCursor          int
	gitCommitDetailFocus        gitCommitDetailFocus
	gitCommitDiff               string
	gitCommitDiffLoading        bool
	gitCommitDiffScroll         int
	gitCommitDiffHScroll        int
	gitCommitDiffCache          map[string]string
	gitCommitDiffGen            int
	gitCommitMsgExpanded        bool
	gitDiffSearchOn             bool
	gitDiffSearchInput          string
	gitDiffSearchQuery          string
	gitDiffSearchIdx            int
	containerSubview            containerSubview
	containerScroll             int
	containerStatusMsg          string
	containerActions            map[string]string
	containerShellExitErr       string
	containerDetailTab          containerDetailTab
	containerDetailID           string
	containerDetailName         string
	containerDetailProjectPath  string
	containerDetailScroll       int
	containerDetailHScroll      int
	containerDetailContent      string
	containerDetailLoading      bool
	containerDetailCache        map[containerDetailTab]string
	containerDetailFollow       bool
	containerDetailFollowPaused bool
	containerDetailFollowGen    int
	containerDetailSearchOn     bool
	containerDetailSearchInput  string
	containerDetailSearchQuery  string
	containerDetailSearchIdx    int
	apiMethod                   string
	apiURL                      string
	apiHeaders                  string
	apiAuthType                 apiAuthType
	apiAuthToken                string
	apiAuthUser                 string
	apiAuthPass                 string
	apiAuthEditPass             bool
	apiBody                     string
	apiBlock                    apiBlock
	apiRightTab                 apiRightTab
	apiMethodCursor             int
	apiEditing                  bool
	apiOpen                     bool // true = fullscreen API client; false = tab 7 landing
	apiEditorCursor             int
	apiEditorAnchor             int // selection anchor; -1 = none
	apiEditorScroll             int
	apiResponseScroll           int
	apiHScroll                  int
	apiLoading                  bool
	apiResponseStatus           string
	apiResponseCode             int
	apiResponseTime             time.Duration
	apiResponseHeaders          string
	apiResponseBody             string
	apiResponseErr              string
	apiShowResponseHeaders      bool
	apiHistory                  []apiHistoryItem
	apiPortIndex                int
	apiSearchOn                 bool
	apiSearchInput              string
	apiSearchQuery              string
	apiSearchIdx                int
	dbOpen                      bool
	dbEngine                    dbEngine
	dbHost                      string
	dbPort                      int
	dbUser                      string
	dbPassword                  string
	dbDatabase                  string
	dbContainer                 string
	dbQuery                     string
	dbTargets                   []dbTarget
	dbTargetCursor              int
	dbTargetScroll              int
	dbConnField                 int // 0 host, 1 port, 2 database, 3 user, 4 pass
	dbBlock                     dbBlock
	dbRightTab                  dbRightTab
	dbEditing                   bool
	dbAuthEditPass              bool
	dbEditorCursor              int
	dbEditorAnchor              int
	dbEditorScroll              int
	dbResultScroll              int
	dbHScroll                   int
	dbLoading                   bool
	dbResultBody                string
	dbResultErr                 string
	dbResultHint                string
	dbResultTime                time.Duration
	dbHistory                   []dbHistoryItem
	dbSearchOn                  bool
	dbSearchInput               string
	dbSearchQuery               string
	dbSearchIdx                 int
	dbTables                    []string
	dbTableCursor               int
	dbTableScroll               int
	dbColumns                   []dbColumnInfo
	dbColumnScroll              int
	dbColHScroll                int
	dbSchemaLoading             bool
	dbSchemaErr                 string
	dbSchemaTable               string
	dbColumnsLoading            bool
	fuzzyOn                     bool
	fuzzyInput                  string
	deployConfirm               bool
	containerConfirmRemove      bool
	projectLogs                 string
	projectLogsLoading          bool
	projectLogsFollow           bool
	projectLogsPaused           bool
	projectLogContainerID       string
	projectLogSource            string
	statusMsg                   string
	projectGitLoading           bool
	projectDockerLoading        bool
	projectLoadGen              int
	projectContentScroll        int
	projectContentTab           Tab
	width                       int
	height                      int
	now                         time.Time
	quitting                    bool
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
	cmds := []tea.Cmd{
		tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} }),
	}
	if a.selectedProject != nil {
		cmds = append(cmds, a.startProjectLoad(a.selectedProject.Path))
	}
	return tea.Batch(cmds...)
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
		if a.gitDiffSearchOn {
			return a.updateGitDiffSearch(msg)
		}
		if a.containerDetailSearchOn {
			return a.updateContainerDetailSearch(msg)
		}
		if a.apiSearchOn {
			return a.updateApiSearch(msg)
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
		if a.view == ViewProject {
			if p := a.currentProject(); p != nil {
				if a.projectGitLoading && p.Git != nil {
					a.projectGitLoading = false
					if p.Git.IsRepo {
						a.initGitTab(p)
					}
				}
				if a.tab == TabGit && p.Git != nil && p.Git.IsRepo {
					a.syncGitBranchesFrom(p)
				}
			}
		}
		return a, tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg{} })

	case gitCommitsLoadedMsg:
		a.handleGitCommitsLoaded(msg)
		return a, nil

	case gitCommitDetailLoadedMsg:
		return a, a.handleGitCommitDetailLoaded(msg)

	case gitCommitDiffLoadedMsg:
		a.handleGitCommitDiffLoaded(msg)
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
		cmd := a.handleContainerDetailLoaded(msg)
		return a, cmd

	case containerDetailFollowMsg:
		return a, a.handleContainerDetailFollow(msg)

	case apiResponseMsg:
		a.handleApiResponse(msg)
		return a, nil

	case dbResultMsg:
		return a, a.handleDbResult(msg)

	case dbSchemaMsg:
		return a, a.handleDbSchema(msg)

	case dbColumnsMsg:
		a.handleDbColumns(msg)
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

	case opencodeDoneMsg:
		a.snapshot = a.store.Get()
		if msg.err != nil {
			a.statusMsg = "opencode: " + msg.err.Error()
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

	case projectGitLoadedMsg:
		cmd := a.handleProjectGitLoaded(msg)
		return a, cmd

	case projectDockerLoadedMsg:
		a.handleProjectDockerLoaded(msg)
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
	// API/DB field editing must receive every key (incl. q/k/?//) — like a normal text editor.
	if a.view == ViewProject && a.tab == TabAPI && a.apiOpen && a.apiEditing {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabDB && a.dbOpen && a.dbEditing {
		return a.updateProject(msg)
	}

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
	viewport := a.helpViewport()
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
			return a, a.openProject(projects[a.cursor], TabOverview)
		}
	case "E", "shift+e":
		if len(projects) > 0 && a.cursor < len(projects) {
			return a, a.projectExecShell(projects[a.cursor].Path)
		}
	case "O", "shift+o":
		if len(projects) > 0 && a.cursor < len(projects) {
			return a, a.openOpencode(projects[a.cursor].Path)
		}
	case "g":
		if len(projects) > 0 && a.cursor < len(projects) {
			return a, a.openProject(projects[a.cursor], TabGit)
		}
	case "c":
		if len(projects) > 0 && a.cursor < len(projects) {
			return a, a.openProject(projects[a.cursor], TabContainers)
		}
	case "r":
		a.snapshot = a.store.Get()
	}
	return a, nil
}

func (a *App) openProject(p core.Project, tab Tab) tea.Cmd {
	a.snapshot = a.store.Get()
	cp := p
	for _, sp := range a.snapshot.Projects {
		if pathsMatch(sp.Path, p.Path) {
			cp = sp
			break
		}
	}
	a.selectedProject = &cp
	a.view = ViewProject
	a.tab = tab
	a.projectContentTab = tab
	a.projectContentScroll = 0
	a.tabCursor = 0
	if tab == TabGit {
		a.initGitTab(&cp)
	}
	if tab == TabContainers {
		a.initContainersTab()
	}
	if tab == TabAPI {
		a.apiOpen = false
	}
	if tab == TabDB {
		a.dbOpen = false
	}
	var cmds []tea.Cmd
	cmds = append(cmds, a.startProjectLoad(cp.Path))
	if tab == TabLogs {
		cmds = append(cmds, a.initLogsTab(&cp))
	}
	return tea.Batch(cmds...)
}

func (a *App) updateProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "?" {
		a.helpOn = true
		a.helpScroll = 0
		return a, nil
	}

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
	if a.tab == TabGit && (a.gitSubview == gitSubviewBranch || a.gitSubview == gitSubviewCommit) {
		return a.handleGitDedicatedKeys(msg, p)
	}
	if a.tab == TabAPI && a.apiOpen {
		return a.handleApiKeys(msg, p)
	}
	if a.tab == TabDB && a.dbOpen {
		return a.handleDbKeys(msg, p)
	}

	switch msg.String() {
	case "esc":
		if a.tab == TabContainers && a.containerSubview == containerSubviewDetail {
			a.containerSubview = containerSubviewList
			a.containerDetailCache = nil
			return a, nil
		}
		a.view = ViewDashboard
		a.selectedProject = nil
		a.gitRenderCache = nil
		a.projectGitLoading = false
		a.projectDockerLoading = false
		a.apiOpen = false
		a.dbOpen = false
		return a, nil
	case "tab":
		a.tab = Tab((int(a.tab) + 1) % len(AllTabs))
		a.tabCursor = 0
		a.apiOpen = false
		a.dbOpen = false
		if a.tab == TabGit {
			a.initGitTab(p)
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
		a.apiOpen = false
		a.dbOpen = false
		if a.tab == TabGit {
			a.initGitTab(p)
		}
		if a.tab == TabContainers {
			a.initContainersTab()
		}
	case "1":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabOverview
		a.tabCursor = 0
	case "2":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabGit
		a.tabCursor = 0
		if p := a.currentProject(); p != nil {
			a.initGitTab(p)
		}
	case "3":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabContainers
		a.tabCursor = 0
		a.initContainersTab()
	case "4":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabHealth
		a.tabCursor = 0
	case "5":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabLogs
		a.tabCursor = 0
		if cmd := a.initLogsTab(p); cmd != nil {
			return a, cmd
		}
	case "6":
		a.apiOpen = false
		a.dbOpen = false
		a.tab = TabMetrics
		a.tabCursor = 0
	case "7":
		a.dbOpen = false
		a.enterApiTab(p)
	case "8":
		a.apiOpen = false
		a.enterDbTab(p)
	case "pgup":
		a.projectContentScroll -= maxInt(1, a.projectPanelHeight()-4)
		if a.projectContentScroll < 0 {
			a.projectContentScroll = 0
		}
		return a, nil
	case "pgdown":
		a.projectContentScroll += maxInt(1, a.projectPanelHeight()-4)
		return a, nil
	case "L":
		return a, a.openLazyGit(p.Path)
	case "O", "shift+o":
		return a, a.openOpencode(p.Path)
	case "o":
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
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			shift := strings.HasPrefix(msg.String(), "shift+")
			return a, a.updateGitCursor(-1, p, shift)
		}
		if cmd := a.tabNav(-1, p); cmd != nil {
			return a, cmd
		}
		if a.tab == TabOverview || a.tab == TabHealth || a.tab == TabLogs {
			if a.projectContentScroll > 0 {
				a.projectContentScroll--
			}
			return a, nil
		}
	case "down", "j", "shift+down", "shift+j":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			shift := strings.HasPrefix(msg.String(), "shift+")
			return a, a.updateGitCursor(1, p, shift)
		}
		if cmd := a.tabNav(1, p); cmd != nil {
			return a, cmd
		}
		if a.tab == TabOverview || a.tab == TabHealth || a.tab == TabLogs {
			a.projectContentScroll++
			return a, nil
		}
	case "left":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			a.gitFocusPrev()
		}
	case "h", "H":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			a.gitFocusPrev()
		} else if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.apiOpen = false
			a.dbOpen = false
			a.tab = TabHealth
			a.tabCursor = 0
		}
	case "right":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			a.gitFocusNext()
		}
	case "l":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			a.gitFocusNext()
		} else if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.apiOpen = false
			a.dbOpen = false
			a.tab = TabLogs
			a.tabCursor = 0
			if cmd := a.initLogsTab(p); cmd != nil {
				return a, cmd
			}
		}
	case "enter":
		if a.tab == TabAPI && !a.apiOpen {
			a.openApiClient(p)
			return a, nil
		}
		if a.tab == TabDB && !a.dbOpen {
			return a, a.openDbClient(p)
		}
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.openContainerDetail(c, p.Path)
			}
		}
		if a.tab == TabGit && p.Git != nil && p.Git.IsRepo && a.gitSubview == gitSubviewMain {
			if a.gitFocus == gitFocusBranches {
				branches := a.filteredGitBranches(a.gitBranchesForUI())
				if a.gitBranchCursor < len(branches) {
					return a, a.openGitBranchHistory(p, branches[a.gitBranchCursor].Name)
				}
				return a, nil
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
		if pathsMatch(p.Path, a.selectedProject.Path) {
			cp := p
			a.selectedProject = &cp
			return &cp
		}
	}
	return a.selectedProject
}

func (a *App) currentProjectPath() string {
	if p := a.currentProject(); p != nil {
		return p.Path
	}
	return ""
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
		return a.renderHelpPopup(a.renderCurrentView())
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

	if a.gitDiffSearchOn {
		return a.renderGitDiffSearchPrompt()
	}

	if a.containerDetailSearchOn {
		return a.renderContainerDetailSearchPrompt()
	}

	if a.apiSearchOn {
		return a.renderApiSearchPrompt()
	}

	if a.dbSearchOn {
		return a.renderDbSearchPrompt()
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

	return a.renderCurrentView()
}

func (a *App) renderCurrentView() string {
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
	metrics := renderMetricPills(m)
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
	if a.projectContentTab != a.tab {
		a.projectContentTab = a.tab
		a.projectContentScroll = 0
	}
	if a.tab == TabContainers && a.containerSubview == containerSubviewDetail {
		return a.renderContainerTextScreen()
	}
	if a.tab == TabGit && (a.gitSubview == gitSubviewBranch || a.gitSubview == gitSubviewCommit) {
		return a.renderGitTab(p)
	}
	if a.tab == TabAPI && a.apiOpen {
		return a.renderApiTab(p)
	}
	if a.tab == TabDB && a.dbOpen {
		return a.renderDbTab(p)
	}

	sidebar := a.renderProjectSidebar()
	contentWidth := maxInt(50, a.width-lipgloss.Width(sidebar)-3)
	panelH := a.projectPanelHeight()
	accent := tabAccentColor(a.tab)

	originalWidth := a.width
	originalPanel := StylePanel
	a.width = contentWidth
	StylePanel = StylePanel.
		Width(maxInt(40, contentWidth-6)).
		BorderForeground(accent)
	content := a.renderTabContent(p)
	a.width = originalWidth
	StylePanel = originalPanel
	if a.tab == TabContainers && a.containerSubview == containerSubviewDetail {
		content = fitProjectPanel(content, contentWidth, panelH)
	} else {
		content = a.renderProjectPanel(content, contentWidth, panelH)
	}

	hints := "tab switch  pgup/pgdown scroll  esc back  q quit"
	if a.tab == TabGit {
		hints = "enter branch/commit  space checkout  shift+↑↓ range  x toggle  shift+c/v cherry  b filter  " + hints
	}
	if a.tab == TabContainers {
		if a.containerSubview == containerSubviewDetail {
			hints = "←→ tabs  ↑↓ scroll  esc back  " + hints
		} else {
			hints = "↑↓ navigate  enter detalhe  shift+e shell  shift+o opencode  s stop  r start/restart  p pause  d remove  shift+u up  shift+d down  " + hints
		}
	}
	if a.tab == TabOverview || a.tab == TabHealth || a.tab == TabLogs {
		hints = "↑↓ scroll  " + hints
	}
	if a.tab == TabAPI && !a.apiOpen {
		hints = "enter abrir API  " + hints
	}
	if a.tab == TabDB && !a.dbOpen {
		hints = "enter abrir Database  " + hints
	}
	compact := a.projectCompact()
	if compact {
		hints = "tab switch  ↑↓/pg scroll  esc back  ? help"
		if a.tab == TabAPI && !a.apiOpen {
			hints = "enter abrir API  " + hints
		}
		if a.tab == TabDB && !a.dbOpen {
			hints = "enter abrir Database  " + hints
		}
	}

	// Dual-pane shell: brand lives in the rail — keep top chrome light.
	chrome := a.renderHeader()
	layout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", content)
	if compact {
		return lipgloss.JoinVertical(lipgloss.Left,
			layout,
			a.renderStatusBar(hints),
		)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		chrome,
		"",
		layout,
		"",
		a.renderStatusBar(hints),
	)
}

func (a *App) projectPanelHeight() int {
	if a.height <= 0 {
		return 20
	}
	if a.projectCompact() {
		return maxInt(12, a.height-2)
	}
	// Header + status bar only (project brand moved into the rail).
	return maxInt(14, a.height-6)
}

func (a *App) projectCompact() bool {
	return (a.height > 0 && a.height < 34) || (a.width > 0 && a.width < 110)
}

func (a *App) renderProjectPanel(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, content)
	}

	top, bottom := lines[0], lines[len(lines)-1]
	panelW := lipgloss.Width(top)
	body := lines[1 : len(lines)-1]
	for len(body) > 0 && strings.TrimSpace(ansi.Strip(body[0])) == "" {
		body = body[1:]
	}
	for len(body) > 0 && strings.TrimSpace(ansi.Strip(body[len(body)-1])) == "" {
		body = body[:len(body)-1]
	}

	bodyHeight := maxInt(1, height-4)
	maxScroll := maxInt(0, len(body)-bodyHeight)
	if a.projectContentScroll > maxScroll {
		a.projectContentScroll = maxScroll
	}
	start := a.projectContentScroll
	end := minInt(start+bodyHeight, len(body))

	rendered := []string{
		top,
		projectPanelIndicator(panelW, start > 0, fmt.Sprintf("↑ %d linhas", start)),
	}
	rendered = append(rendered, body[start:end]...)
	for len(rendered) < height-2 {
		rendered = append(rendered, projectPanelIndicator(panelW, false, ""))
	}
	rendered = append(rendered,
		projectPanelIndicator(panelW, end < len(body), fmt.Sprintf("↓ %d linhas", len(body)-end)),
		bottom,
	)
	return strings.Join(rendered, "\n")
}

func projectPanelIndicator(width int, visible bool, text string) string {
	if width < 4 {
		return ""
	}
	content := ""
	if visible {
		content = StyleMuted.Render(" " + text)
	}
	inside := lipgloss.NewStyle().
		Width(width - 2).
		Background(ColorBgPanel).
		Render(content)
	border := lipgloss.NewStyle().Foreground(ColorBorder).Render("│")
	return border + inside + border
}

func fitProjectPanel(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, content)
	}

	top, bottom := lines[0], lines[len(lines)-1]
	body := lines[1 : len(lines)-1]
	if len(body) > height-2 {
		body = body[:height-2]
	}
	for len(body) < height-2 {
		body = append(body, projectPanelIndicator(width, false, ""))
	}
	return strings.Join(append(append([]string{top}, body...), bottom), "\n")
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
	case TabAPI:
		return a.renderApiLanding(p)
	case TabDB:
		return a.renderDbLanding(p)
	default:
		return a.renderOverviewTab(p)
	}
}

func (a *App) renderOverviewTab(p *core.Project) string {
	kv := func(k, v string) string {
		return StyleMuted.Render(fmt.Sprintf("%-12s", k)) + " " + v
	}

	lines := []string{
		StyleSection.Render("PROJETO"),
		kv("Path", StyleNormal.Render(p.Path)),
		kv("Status", projectStatusStyle(p.Status).Render(string(p.Status))),
		kv("Health", healthLabel(p.Health)),
		"",
		StyleSection.Render("STACK"),
	}

	frameworks := p.Frameworks
	if len(frameworks) == 0 && p.Framework.Name != "" && p.Framework.Name != "Unknown" {
		frameworks = []core.FrameworkInfo{p.Framework}
	}
	if len(frameworks) == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum detectado ainda — aguarde o scan)"))
	} else {
		for _, fw := range frameworks {
			ver := ""
			if fw.Version != "" {
				ver = StyleMuted.Render("  v" + fw.Version)
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s%s",
				frameworkIcon(fw.Name),
				StyleNormal.Render(fw.Name),
				StyleMuted.Render("("+fw.Language+")"),
				ver,
			))
		}
	}

	lines = append(lines, "", StyleSection.Render("RUNTIME"))
	if p.HasDockerCompose {
		lines = append(lines, kv("Docker", StyleIconDocker.Render("compose")+" "+StyleMuted.Render("detectado")))
	}
	if p.HasDockerfile {
		lines = append(lines, kv("Docker", StyleIconDocker.Render("Dockerfile")+" "+StyleMuted.Render("detectado")))
	}
	if p.ContainerCount > 0 {
		lines = append(lines, kv("Containers", StyleNormal.Render(fmt.Sprintf("%d vinculados", p.ContainerCount))))
	}
	if p.WorkerCount > 0 {
		lines = append(lines, kv("PM2", StyleNormal.Render(fmt.Sprintf("%d workers", p.WorkerCount))))
		for _, w := range p.Workers {
			st := StyleMuted
			if strings.EqualFold(w.Status, "online") {
				st = StyleRunning
			}
			lines = append(lines, fmt.Sprintf("  %s %s %s",
				st.Render("•"),
				StyleNormal.Render(w.Name),
				StyleMuted.Render(fmt.Sprintf("[%s] CPU %.1f%%", w.Status, w.CPU)),
			))
		}
	}
	if p.Metrics.CPUPercent > 0 || p.Metrics.MemoryMB > 0 {
		lines = append(lines, kv("Metrics",
			StyleMetricCPU.Render(fmt.Sprintf("CPU %.1f%%", p.Metrics.CPUPercent))+"  "+
				StyleMetricRAM.Render(fmt.Sprintf("RAM %d MB", p.Metrics.MemoryMB))))
	}
	if len(p.Ports) > 0 {
		lines = append(lines, kv("Ports", StyleAccent.Render(collectors.FormatPortsShort(p.Ports, 5))))
	}
	if p.DeployScript != "" {
		lines = append(lines, kv("Deploy", StyleKey.Render(p.DeployScript)+" "+StyleMuted.Render("(D)")))
	}

	if len(p.Modules) > 0 {
		lines = append(lines, "", StyleSection.Render("MODULES"))
		for _, m := range p.Modules {
			lines = append(lines, fmt.Sprintf("  %s %s %s",
				StyleTabActive.Render("•"),
				StyleNormal.Render(m.Name),
				StyleMuted.Render(fmt.Sprintf("[%s] — %s", m.Role, m.Path)),
			))
		}
	}

	if p.Git != nil && p.Git.IsRepo {
		lines = append(lines, "", StyleSection.Render("GIT"),
			kv("Branch", StyleWarning.Render(p.Git.Branch)),
			kv("Commit", StyleMuted.Render(p.Git.LastCommit)+" "+StyleNormal.Render(p.Git.LastCommitMsg)),
		)
	}

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderMetricsTab(p *core.Project) string {
	cpu, memoryMB := projectRuntimeMetrics(p)
	lines := []string{
		StyleSection.Render("PROJECT METRICS"),
		fmt.Sprintf("  CPU:        %.1f%%", cpu),
		fmt.Sprintf("  Memory:     %d MB", memoryMB),
		fmt.Sprintf("  Containers: %d", p.ContainerCount),
		fmt.Sprintf("  Workers:    %d", p.WorkerCount),
	}

	if len(p.Containers) > 0 {
		lines = append(lines, "", StyleSection.Render("CONTAINERS"))
		for _, c := range p.Containers {
			lines = append(lines, fmt.Sprintf(
				"  %-28s %-9s CPU %6.1f%%  RAM %6d MB",
				c.Name, c.Status, c.CPU, c.Memory/(1024*1024),
			))
		}
	}

	if len(p.Workers) > 0 {
		lines = append(lines, "", StyleSection.Render("WORKERS"))
		for _, w := range p.Workers {
			lines = append(lines, fmt.Sprintf(
				"  %-28s %-9s CPU %6.1f%%  RAM %6d MB",
				w.Name, w.Status, w.CPU, w.Memory/(1024*1024),
			))
		}
	}

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func projectRuntimeMetrics(p *core.Project) (float64, int64) {
	var cpu float64
	var memory int64
	for _, c := range p.Containers {
		cpu += c.CPU
		memory += c.Memory
	}
	for _, w := range p.Workers {
		if strings.EqualFold(w.Status, "online") {
			cpu += w.CPU
			memory += w.Memory
		}
	}
	return cpu, memory / (1024 * 1024)
}

func (a *App) helpViewport() int {
	if a.height <= 0 {
		return 12
	}
	return maxInt(8, a.height-10)
}

func (a *App) renderHelpPopup(background string) string {
	helpLines := strings.Split(strings.TrimSpace(getHelpText()), "\n")
	viewport := minInt(a.helpViewport(), len(helpLines))

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
	boxWidth := minInt(76, maxInt(44, a.width-8))
	helpBox := StylePanel.
		Width(boxWidth).
		Background(ColorBgPanel).
		Render(title + "\n\n" + strings.Join(visibleLines, "\n") + "\n\n" + footer)
	return overlayCentered(background, helpBox, a.width, a.height)
}

func overlayCentered(background, popup string, width, height int) string {
	if width <= 0 || height <= 0 {
		return popup
	}
	popupWidth := lipgloss.Width(popup)
	popupLines := strings.Split(popup, "\n")
	x := maxInt(0, (width-popupWidth)/2)
	y := maxInt(0, (height-len(popupLines))/2)

	backgroundLines := strings.Split(ansi.Strip(background), "\n")
	for len(backgroundLines) < height {
		backgroundLines = append(backgroundLines, "")
	}
	for i, popupLine := range popupLines {
		row := y + i
		if row >= len(backgroundLines) {
			break
		}
		line := backgroundLines[row]
		left := padRight(cellSlice(line, 0, x), x)
		rightWidth := maxInt(0, width-x-popupWidth)
		right := padRight(cellSlice(line, x+popupWidth, width), rightWidth)
		backgroundLines[row] = left + popupLine + right
	}
	return strings.Join(backgroundLines, "\n")
}

func cellSlice(s string, start, end int) string {
	var out strings.Builder
	column := 0
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if column >= end {
			break
		}
		if column >= start && column+w <= end {
			out.WriteRune(r)
		}
		column += w
	}
	return out.String()
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
  shift+o      Abrir Opencode no diretório do projeto
  g            Abrir direto na aba Git
  c            Abrir direto na aba Containers
  r            Forçar atualização rápida

Abas de Projeto:
  1-8          Overview, Git, Containers, Health, Logs, Metrics, API, Database
  h            Ir para aba Health
  l            Ir para aba Logs
  L            Abrir LazyGit no projeto
  D            Executar Deploy script (confirmação y/n)
  shift+u      Docker compose up -d
  shift+d      Docker compose down
  R            Docker compose restart
  o            Abrir URL do projeto no navegador

Aba API:
  tab          Request → URL → Headers → Auth
  []           Body │ Response
  ↑↓           Método (no Request) / scroll
  digitar      Edita URL / Headers / Auth / Body
  enter        Enviar request
  /            Buscar (só em Body/Response)
  u            Porta do projeto (no Request/URL)
  a            Tipo de Auth (no Auth)

Aba Database:
  tab          Conn → Tables → Query → Result
  digitar      Edita SQL (no Query) ou campo (no Conn)
  ctrl+enter   Executar query
  enter        Edita campo / SELECT * na tabela
  ↑↓           Campos / tabelas / scroll do Result
  ←→           Target (Conn) ou scroll horizontal
  s            Carregar schema
  /            Buscar no Result
  esc          Sair do client

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
