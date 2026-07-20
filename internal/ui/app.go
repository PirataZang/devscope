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
	"github.com/devscope/devscope/internal/ngrokutil"
	"github.com/devscope/devscope/internal/routeutil"
	"github.com/devscope/devscope/internal/wsutil"
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
	gitWTDiffScroll             int
	gitWTDiffHScroll            int
	gitListViewportOverride     int
	gitViewBranch               string
	gitWTDiff                   string
	gitWTDiffFile               string
	gitActivity                 []string
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
	containerFilterOn           bool
	containerFilterInput        string
	containerFilter             string
	containerPreviewID          string
	containerPreviewLogs        string
	containerPreviewStats       string
	containerPreviewVolumes     []string
	containerCPUHistory         []float64
	containerPreviewGen         int
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
	apiOpen                     bool // true = fullscreen API client; false = tab 8 landing
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
	dbEditing                   bool
	dbLoading                   bool
	dbPane                      dbPane
	dbTargets                   []collectors.DBTarget
	dbTargetIdx                 int
	dbTables                    []string
	dbTableCursor               int
	dbTablesScroll              int
	dbSQL                       string
	dbEditorCursor              int
	dbResult                    string
	dbResultScroll              int
	dbResultHScroll             int
	dbErr                       string
	k8sOpen                     bool
	k8sEditing                  bool
	k8sLoading                  bool
	k8sConfirmDelete            bool
	k8sFilterOn                 bool
	k8sKind                     k8sKind
	k8sSubTab                   k8sSubTab
	k8sFocus                    k8sFocus
	k8sPane                     k8sPane
	k8sNamespace                string
	k8sContext                  string
	k8sVersion                  string
	k8sFilter                   string
	k8sCursor                   int
	k8sScroll                   int
	k8sDetailScroll             int
	k8sLogsScroll               int
	k8sYAMLScroll               int
	k8sEditorCursor             int
	k8sNodeCount                int
	k8sResources                []collectors.K8sResource
	k8sManifests                []string
	k8sDetail                   string
	k8sLogs                     string
	k8sYAML                     string
	k8sEvents                   string
	k8sStatus                   string
	k8sErr                      string
	k8sInspectName              string
	jsonOpen                    bool
	jsonEditing                 bool
	jsonSearchOn                bool
	jsonPane                    jsonPane
	jsonInput                   string
	jsonOutput                  string
	jsonErr                     string
	jsonStatus                  string
	jsonEditorCursor            int
	jsonEditorAnchor            int // selection anchor; -1 = none
	jsonScrollIn                int
	jsonScrollOut               int
	jsonSearchInput             string
	jwtOpen                     bool
	jwtEditing                  bool
	jwtPane                     jwtPane
	jwtAlg                      string
	jwtSecret                   string
	jwtInput                    string
	jwtLastToken                string
	jwtOutput                   string
	jwtErr                      string
	jwtStatus                   string
	jwtEdit                     editorState
	jwtScrollIn                 int
	jwtScrollOut                int
	jwtHScrollIn                int
	jwtHScrollOut               int
	jwtHScrollSecret            int
	routesOpen                  bool
	routesLoading               bool
	routes                      []routeutil.Route
	routesCursor                int
	routesScroll                int
	routesErr                   string
	routesStatus                string
	routesFilterOn              bool
	routesFilterInput           string
	routesFilter                string
	wsOpen                      bool
	wsEditing                   bool
	wsConnected                 bool
	wsSubTab                    wsSubTab
	wsFocus                     wsFocus
	wsURL                       string
	wsHeaders                   string
	wsSend                      string
	wsStatus                    string
	wsErr                       string
	wsFrames                    []wsFrame
	wsFrameSeq                  int
	wsFrameCursor               int
	wsMsgScroll                 int
	wsFilter                    wsFilterKind
	wsSearchOn                  bool
	wsSearchInput               string
	wsSearch                    string
	wsPayloadMode               wsPayloadMode
	wsSendMode                  wsSendMode
	wsHistory                   []string
	wsRecent                    []string
	wsRecentCursor              int
	wsStats                     wsStats
	wsInfo                      wsutil.Info
	wsConnectedAt               time.Time
	wsLastSendAt                time.Time
	wsLatency                   time.Duration
	wsAutoReconnect             bool
	wsPortIndex                 int
	wsEdit                      editorState
	wsSess                      *wsutil.Session
	ngrokOpen                   bool
	ngrokLoading                bool
	ngrokWizard                 bool
	ngrokConfirmDelete          bool
	ngrokSubTab                 ngrokSubTab
	ngrokFocus                  ngrokFocus
	ngrokCursor                 int
	ngrokScroll                 int
	ngrokReqCursor              int
	ngrokReqScroll              int
	ngrokLogScroll              int
	ngrokNewPort                int
	ngrokNewPortStr             string
	ngrokNewName                string
	ngrokNewProto               string
	ngrokWizardField            int // 0 name, 1 port, 2 proto
	ngrokWizardCursor           int
	ngrokStatus                 string
	ngrokErr                    string
	ngrokTunnels                []ngrokutil.Tunnel
	ngrokRequests               []ngrokutil.Request
	ngrokCfg                    ngrokutil.ProjectConfig
	ngrokAgent                  ngrokutil.AgentInfo
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
		if a.routesFilterOn {
			return a.updateRoutesFilter(msg)
		}
		if a.wsSearchOn {
			return a.updateWsSearch(msg)
		}
		if a.gitDiffSearchOn {
			return a.updateGitDiffSearch(msg)
		}
		if a.containerFilterOn {
			return a.updateContainerFilter(msg)
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

	case gitWTDiffMsg:
		if msg.file == a.gitWTDiffFile || a.gitWTDiffFile == "" {
			a.gitWTDiff = msg.diff
			a.gitWTDiffFile = msg.file
			a.gitWTDiffScroll = 0
		}
		return a, nil

	case gitActionDoneMsg:
		a.handleGitActionDone(msg)
		a.pushGitActivity(msg)
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

	case dbTablesMsg:
		return a.handleDbMsg(msg)
	case dbQueryMsg:
		return a.handleDbMsg(msg)

	case k8sLoadedMsg, k8sActionMsg, k8sDetailMsg, k8sNsMsg, k8sEditReadyMsg, k8sInspectMsg, k8sMetaMsg:
		return a.handleK8sMsg(msg)

	case routesLoadedMsg:
		a.handleRoutesLoaded(msg)
		return a, nil

	case wsConnectedMsg, wsEventMsg:
		return a.handleWsMsg(msg)
	case ngrokLoadedMsg, ngrokActionMsg:
		return a.handleNgrokMsg(msg)

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
		return a, a.requestContainerPreview()

	case containerShellDoneMsg:
		cmd := a.handleContainerShellDone(msg)
		return a, cmd

	case containerPreviewMsg:
		a.handleContainerPreview(msg)
		return a, nil

	case dockerRefreshedMsg:
		a.snapshot = a.store.Get()
		containers := a.filteredContainers(a.currentProject())
		if len(containers) > 0 {
			a.tabCursor = clampCursor(a.tabCursor, len(containers))
			a.syncContainerScroll(len(containers))
		}
		return a, a.requestContainerPreview()

	case projectGitLoadedMsg:
		cmd := a.handleProjectGitLoaded(msg)
		return a, cmd

	case projectDockerLoadedMsg:
		a.handleProjectDockerLoaded(msg)
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			return a, a.requestContainerPreview()
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
	// API/DB field editing must receive every key (incl. q/k/?//) — like a normal text editor.
	if a.view == ViewProject && a.tab == TabAPI && a.apiOpen && a.apiEditing {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabDatabase && a.dbOpen && a.dbEditing {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabKubernetes && a.k8sOpen && a.k8sEditing {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabJSON && a.jsonOpen && (a.jsonEditing || a.jsonSearchOn) {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabJWT && a.jwtOpen && a.jwtEditing {
		return a.updateProject(msg)
	}
	if a.view == ViewProject && a.tab == TabWebSocket && a.wsOpen && a.wsEditing {
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
			return a, a.projectExecOpenCode(projects[a.cursor].Path)
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

func (a *App) closeToolClients() {
	a.apiOpen = false
	a.dbOpen = false
	a.k8sOpen = false
	a.jsonOpen = false
	a.jwtOpen = false
	a.routesOpen = false
	a.routesLoading = false
	a.routesFilterOn = false
	if a.wsOpen || a.wsConnected {
		a.wsCloseSession()
	}
	a.wsOpen = false
	a.wsEditing = false
	a.ngrokOpen = false
	a.ngrokWizard = false
	a.ngrokConfirmDelete = false
}

func tabIndex(t Tab) int {
	for i, x := range AllTabs {
		if x == t {
			return i
		}
	}
	return 0
}

func (a *App) cycleProjectTab(delta int, p *core.Project) tea.Cmd {
	a.closeToolClients()
	n := len(AllTabs)
	i := (tabIndex(a.tab) + delta%n + n) % n
	return a.switchProjectTab(AllTabs[i], p)
}

func (a *App) switchProjectTab(t Tab, p *core.Project) tea.Cmd {
	a.tab = t
	a.tabCursor = 0
	a.projectContentScroll = 0
	switch t {
	case TabGit:
		if p != nil {
			a.initGitTab(p)
		}
	case TabContainers:
		a.initContainersTab()
		return a.requestContainerPreview()
	case TabKubernetes:
		a.enterK8sTab(p)
	case TabLogs:
		if p != nil {
			return a.initLogsTab(p)
		}
	case TabAPI:
		a.enterApiTab(p)
	case TabDatabase:
		a.enterDbTab(p)
	case TabJSON:
		a.enterJsonTab(p)
	case TabJWT:
		a.enterJwtTab(p)
	case TabRoutes:
		a.enterRoutesTab(p)
	case TabWebSocket:
		a.enterWsTab(p)
	case TabNgrok:
		a.enterNgrokTab(p)
	}
	return nil
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
	if tab == TabDatabase {
		a.dbOpen = false
	}
	if tab == TabKubernetes {
		a.k8sOpen = false
	}
	if tab == TabJSON {
		a.jsonOpen = false
	}
	if tab == TabJWT {
		a.jwtOpen = false
	}
	if tab == TabRoutes {
		a.routesOpen = false
	}
	if tab == TabWebSocket {
		a.wsOpen = false
	}
	if tab == TabNgrok {
		a.ngrokOpen = false
	}
	var cmds []tea.Cmd
	cmds = append(cmds, a.startProjectLoad(cp.Path))
	if tab == TabLogs {
		cmds = append(cmds, a.initLogsTab(&cp))
	}
	if tab == TabContainers {
		cmds = append(cmds, a.requestContainerPreview())
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
	if a.tab == TabGit && (a.gitSubview == gitSubviewBranch || a.gitSubview == gitSubviewCommit || a.gitSubview == gitSubviewFileDiff) {
		return a.handleGitDedicatedKeys(msg, p)
	}
	if a.tab == TabAPI && a.apiOpen {
		return a.handleApiKeys(msg, p)
	}
	if a.tab == TabDatabase && a.dbOpen {
		return a.handleDbKeys(msg, p)
	}
	if a.tab == TabKubernetes && a.k8sOpen {
		return a.handleK8sKeys(msg, p)
	}
	if a.tab == TabJSON && a.jsonOpen {
		return a.handleJsonKeys(msg, p)
	}
	if a.tab == TabJWT && a.jwtOpen {
		return a.handleJwtKeys(msg, p)
	}
	if a.tab == TabRoutes && a.routesOpen {
		return a.handleRoutesKeys(msg, p)
	}
	if a.tab == TabWebSocket && a.wsOpen {
		return a.handleWsKeys(msg, p)
	}
	if a.tab == TabNgrok && a.ngrokOpen {
		return a.handleNgrokKeys(msg, p)
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
		a.closeToolClients()
		return a, nil
	case "tab":
		return a, a.cycleProjectTab(1, p)
	case "shift+tab":
		return a, a.cycleProjectTab(-1, p)
	case "pgup":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && a.gitFocus == gitFocusFiles {
			a.gitWTDiffScroll = maxInt(0, a.gitWTDiffScroll-5)
			return a, nil
		}
		a.projectContentScroll -= maxInt(1, a.projectPanelHeight()-4)
		if a.projectContentScroll < 0 {
			a.projectContentScroll = 0
		}
		return a, nil
	case "pgdown":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && a.gitFocus == gitFocusFiles {
			a.gitWTDiffScroll += 5
			return a, nil
		}
		a.projectContentScroll += maxInt(1, a.projectPanelHeight()-4)
		return a, nil
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
	case "e":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerExecShell(c)
			}
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
	case "/":
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			a.containerFilterOn = true
			a.containerFilterInput = a.containerFilter
			return a, nil
		}
	case "b":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain && p.Git != nil && p.Git.IsRepo {
			a.gitBranchFilterOn = true
			a.gitBranchFilterInput = a.gitBranchFilter
			a.gitFocus = gitFocusBranches
			return a, nil
		}
	case " ", "space":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			if g := a.projectGitInfo(p); g != nil && g.IsRepo {
				return a, a.gitSpaceAction(p)
			}
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
			return a, a.gitFocusPrev()
		}
	case "h", "H":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			return a, a.gitFocusPrev()
		} else if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.closeToolClients()
			a.tab = TabHealth
			a.tabCursor = 0
		}
	case "right":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			return a, a.gitFocusNext()
		}
	case "l":
		if a.tab == TabGit && a.gitSubview == gitSubviewMain {
			return a, a.gitFocusNext()
		}
		if a.tab == TabContainers && a.containerSubview == containerSubviewList {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.openContainerDetail(c, p.Path)
			}
			return a, nil
		}
		if a.tab != TabContainers || a.containerSubview != containerSubviewDetail {
			a.closeToolClients()
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
		if a.tab == TabDatabase && !a.dbOpen {
			return a, a.openDbClient(p)
		}
		if a.tab == TabKubernetes && !a.k8sOpen {
			return a, a.openK8sClient(p)
		}
		if a.tab == TabJSON && !a.jsonOpen {
			return a, a.openJsonClient(p)
		}
		if a.tab == TabJWT && !a.jwtOpen {
			return a, a.openJwtClient(p)
		}
		if a.tab == TabRoutes && !a.routesOpen {
			return a, a.openRoutesClient(p)
		}
		if a.tab == TabWebSocket && !a.wsOpen {
			return a, a.openWsClient(p)
		}
		if a.tab == TabNgrok && !a.ngrokOpen {
			return a, a.openNgrokClient(p)
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
			if a.gitFocus == gitFocusFiles {
				return a, a.openGitFileDiff(p)
			}
		}
	}
	return a, nil
}

func (a *App) tabNav(delta int, p *core.Project) tea.Cmd {
	switch a.tab {
	case TabContainers:
		return a.updateContainerCursor(delta, p)
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
	if a.tab == TabGit && (a.gitSubview == gitSubviewBranch || a.gitSubview == gitSubviewCommit || a.gitSubview == gitSubviewFileDiff) {
		return a.renderGitTab(p)
	}
	if a.tab == TabAPI && a.apiOpen {
		return a.renderApiTab(p)
	}
	if a.tab == TabDatabase && a.dbOpen {
		return a.renderDbTab(p)
	}
	if a.tab == TabKubernetes && a.k8sOpen {
		return a.renderK8sTab(p)
	}
	if a.tab == TabJSON && a.jsonOpen {
		return a.renderJsonTab(p)
	}
	if a.tab == TabJWT && a.jwtOpen {
		return a.renderJwtTab(p)
	}
	if a.tab == TabRoutes && a.routesOpen {
		return a.renderRoutesTab(p)
	}
	if a.tab == TabWebSocket && a.wsOpen {
		return a.renderWsTab(p)
	}
	if a.tab == TabNgrok && a.ngrokOpen {
		return a.renderNgrokTab(p)
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
	moduleDash := a.tab == TabOverview || a.tab == TabHealth || a.tab == TabLogs || a.tab == TabMetrics ||
		(a.tab == TabGit && a.gitSubview == gitSubviewMain) ||
		(a.tab == TabContainers && a.containerSubview == containerSubviewList) ||
		(a.tab == TabAPI && !a.apiOpen) || (a.tab == TabDatabase && !a.dbOpen) ||
		(a.tab == TabJSON && !a.jsonOpen) || (a.tab == TabJWT && !a.jwtOpen) ||
		(a.tab == TabRoutes && !a.routesOpen) || (a.tab == TabNgrok && !a.ngrokOpen)
	switch {
	case moduleDash:
		content = lipgloss.Place(contentWidth, panelH, lipgloss.Left, lipgloss.Top, content)
	case a.tab == TabContainers && a.containerSubview == containerSubviewDetail:
		content = fitProjectPanel(content, contentWidth, panelH)
	default:
		content = a.renderProjectPanel(content, contentWidth, panelH)
	}

	hints := "tab módulo  shift+tab anterior  enter abrir  r refresh  esc back  q sair"
	if a.tab != TabOverview {
		hints = "tab/shift+tab módulo  pgup/pgdown scroll  esc back  q quit"
	}
	if a.tab == TabGit {
		hints = "←→ painéis  enter detail/diff  space checkout  shift+↑↓ range  x cherry  b filter  " + hints
	}
	if a.tab == TabContainers {
		if a.containerSubview == containerSubviewDetail {
			hints = "←→ tabs  ↑↓ scroll  esc back  " + hints
		} else {
			hints = "↑↓ lista  enter/l detalhe  / buscar  e shell  s/r/p/d  shift+u/d compose  " + hints
		}
	}
	if a.tab == TabHealth || a.tab == TabLogs {
		hints = "↑↓ scroll  " + hints
	}
	if a.tab == TabAPI && !a.apiOpen {
		hints = "enter abrir API  " + hints
	}
	if a.tab == TabDatabase && !a.dbOpen {
		hints = "enter abrir Database  " + hints
	}
	if a.tab == TabKubernetes && !a.k8sOpen {
		hints = "enter abrir Kubernetes  " + hints
	}
	if a.tab == TabJSON && !a.jsonOpen {
		hints = "enter abrir JSON  " + hints
	}
	if a.tab == TabJWT && !a.jwtOpen {
		hints = "enter abrir JWT  " + hints
	}
	if a.tab == TabRoutes && !a.routesOpen {
		hints = "enter abrir Rotas  " + hints
	}
	if a.tab == TabWebSocket && !a.wsOpen {
		hints = "enter abrir WebSocket  " + hints
	}
	if a.tab == TabNgrok && !a.ngrokOpen {
		hints = "enter abrir Ngrok  " + hints
	}
	compact := a.projectCompact()
	if compact {
		hints = "tab switch  ↑↓/pg scroll  esc back  ? help"
		if a.tab == TabAPI && !a.apiOpen {
			hints = "enter abrir API  " + hints
		}
		if a.tab == TabDatabase && !a.dbOpen {
			hints = "enter abrir Database  " + hints
		}
		if a.tab == TabKubernetes && !a.k8sOpen {
			hints = "enter abrir Kubernetes  " + hints
		}
		if a.tab == TabJSON && !a.jsonOpen {
			hints = "enter abrir JSON  " + hints
		}
		if a.tab == TabJWT && !a.jwtOpen {
			hints = "enter abrir JWT  " + hints
		}
		if a.tab == TabRoutes && !a.routesOpen {
			hints = "enter abrir Rotas  " + hints
		}
		if a.tab == TabWebSocket && !a.wsOpen {
			hints = "enter abrir WebSocket  " + hints
		}
		if a.tab == TabNgrok && !a.ngrokOpen {
			hints = "enter abrir Ngrok  " + hints
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
		projectPanelIndicator(width, start > 0, fmt.Sprintf("↑ %d linhas", start)),
	}
	rendered = append(rendered, body[start:end]...)
	for len(rendered) < height-2 {
		rendered = append(rendered, projectPanelIndicator(width, false, ""))
	}
	rendered = append(rendered,
		projectPanelIndicator(width, end < len(body), fmt.Sprintf("↓ %d linhas", len(body)-end)),
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
	case TabDatabase:
		return a.renderDbLanding(p)
	case TabKubernetes:
		return a.renderK8sLanding(p)
	case TabJSON:
		return a.renderJsonLanding(p)
	case TabJWT:
		return a.renderJwtLanding(p)
	case TabRoutes:
		return a.renderRoutesLanding(p)
	case TabWebSocket:
		return a.renderWsLanding(p)
	case TabNgrok:
		return a.renderNgrokLanding(p)
	default:
		return a.renderOverviewTab(p)
	}
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
  shift+o      Abrir OpenCode no diretório do projeto
  g            Abrir direto na aba Git
  c            Abrir direto na aba Containers
  r            Forçar atualização rápida

Abas de Projeto:
  tab          Próximo módulo (sidebar)
  shift+tab    Módulo anterior
  h            Ir para aba Health
  l            Ir para aba Logs
  L            Abrir LazyGit no projeto
  D            Executar Deploy script (confirmação y/n)
  shift+u      Docker compose up -d
  shift+d      Docker compose down
  R            Docker compose restart
  o            Abrir URL do projeto no navegador

Aba JSON (UTILS):
  enter        Abrir editor input/output
  p            Pretty
  m            Minify
  v            Validate (linha/coluna do erro)
  s            Sort Keys
  w            JSON ⇄ YAML
  t            JSON ⇄ TOML
  x            JSON ⇄ XML
  d            Diff input ↔ output
  n            Remover campos nulos
  /            Buscar por chave
  c            Copiar resultado
  e            Editar input
  shift+←→↑↓   Selecionar (na edição)
  ctrl+←→      Pular palavra
  ctrl+shift+←→ Selecionar palavra
  ctrl+a/c/x/v Selecionar tudo / copiar / recortar / colar
  esc          Voltar para a landing / sair edição

Aba JWT (UTILS):
  enter        Abrir cliente (secret + token/claims + result)
  d            Decode
  v            Verify (HMAC + secret)
  g            Generate claims template
  s            Sign claims → JWT
  c            Copy Claims
  x            Export JSON (header+payload)
  y            Copy token (clipboard; sign já copia)
  Y            Copy result (decode/export)
  ctrl+y       Copy token (mesmo em modo edição)
  []           Alg HS256 / HS384 / HS512
  ←→ / h l     Scroll horizontal no painel focado
  e / enter    Editar secret ou token/claims (VS Code keys)
  tab          secret │ token │ result
  esc          Voltar para a landing

Aba Rotas (UTILS):
  enter        Detectar stack + escanear rotas (OpenAPI/parsers)
  ↑↓ / j k     Navegar rotas
  b            Filtrar rotas por palavra no path (ex: users)
  enter        Abrir na aba API (method + URL)
  r            Reescanear
  esc          Voltar para a landing / limpar filtro

Aba WebSocket (TOOLS):
  enter        Abrir Overview (3 colunas)
  0-4          Overview / Messages / Send / History / Settings
  c            Connect
  d / x        Disconnect (x no Send limpa editor)
  r            Reconnect
  tab          Connections → Filters → Messages → Send → Inspector
  f            Ciclar filtro (All/Text/JSON/Binary/Errors/In/Out)
  /            Buscar no payload
  m            Ciclar modo Send ou Payload view
  []           Pretty / Raw / Hex no inspector
  e            Editar Send (ou URL/Headers em Settings)
  enter        Enviar / conectar / reenviar history
  ctrl+enter   Enviar na edição
  ctrl+l       Limpar frames
  a            Auto reconnect (Settings)
  u            Porta do projeto
  esc          Voltar (desconecta)

Aba Kubernetes:
  enter        Abrir cliente (pods/deploy/svc/manifests)
  esc          Voltar para a landing
  []           Alternar kind (pods / deploy / svc / yaml)
  n/N          Namespace seguinte / anterior
  enter        Describe / ver yaml
  a            Apply YAML do editor (create/edit) ou arquivo (kind yaml)
  c            Criar (template → modo edição)
  e            Editar recurso/manifest selecionado
  enter        Nova linha (na edição YAML)
  ctrl+s       Apply do YAML em edição (Ctrl+Enter costuma = Enter no terminal)
  d            Delete (confirmação y)
  l            Logs do pod
  +/-          Scale deployment
  r            Refresh

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
  enter        Abrir cliente (tabelas + SQL)
  esc          Voltar para a landing
  tab          Tables │ SQL │ Result
  enter        Preview SELECT * LIMIT 50 na tabela
  e            Editar SQL
  ctrl+enter   Executar SQL
  []           Trocar banco detectado
  ←→ / h l     Scroll lateral no result
  r            Recarregar tabelas

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
