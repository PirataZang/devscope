package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type ghaKind int

const (
	ghaKindProcesses ghaKind = iota
	ghaKindRuns
	ghaKindWorkflows
	ghaKindCount
)

// ghaRunScope filters the Runs list by execution state.
type ghaRunScope int

const (
	ghaRunScopeAll ghaRunScope = iota
	ghaRunScopeRunning
)

func (s ghaRunScope) String() string {
	switch s {
	case ghaRunScopeRunning:
		return "running"
	default:
		return "all"
	}
}

func (k ghaKind) String() string {
	switch k {
	case ghaKindProcesses:
		return "Processes"
	case ghaKindRuns:
		return "Runs"
	case ghaKindWorkflows:
		return "Workflows"
	default:
		return "Processes"
	}
}

type ghaScreen int

const (
	ghaScrCluster ghaScreen = iota
	ghaScrDetail
	ghaScrLogs
	ghaScrForm
	ghaScrProcess // tela dedicada do processo (estilo Docker detail)
)

// Painéis ciclados por Tab (não confundir com kinds Processes/Runs/Workflows).
const (
	ghaFocusTable   = 0
	ghaFocusResumo  = 1
	ghaFocusActions = 2
	ghaFocusCount   = 3
)

type ghaProcTab int

const (
	ghaProcTabOverview ghaProcTab = iota
	ghaProcTabRuns
	ghaProcTabJobs
	ghaProcTabLogs
	ghaProcTabYAML
	ghaProcTabCount
)

func (t ghaProcTab) String() string {
	switch t {
	case ghaProcTabOverview:
		return "Overview"
	case ghaProcTabRuns:
		return "Runs"
	case ghaProcTabJobs:
		return "Timeline"
	case ghaProcTabLogs:
		return "Logs"
	case ghaProcTabYAML:
		return "YAML"
	default:
		return "Overview"
	}
}

// ghaProcLive is the live indicator derived from recent runs / trigger local.
type ghaProcLive struct {
	Label      string // idle | triggered | queued | running | success | failure | cancelled
	Event      string
	RunID      string
	Conclusion string
	Status     string
	Title      string
	CreatedAt  string
}

type ghaFormKind int

const (
	ghaFormNone ghaFormKind = iota
	ghaFormCreate
	ghaFormViewYAML
	ghaFormSetup
	ghaFormTrigger
)

type ghaAuthDoneMsg struct {
	gen int
	err error
}

type ghaLoadedMsg struct {
	gen       int
	info      collectors.GHAInfo
	billing   collectors.GHAActionsBilling
	hasBill   bool
	processes []collectors.GHAProcess
	workflows []collectors.GHAWorkflow
	runs      []collectors.GHARun
	err       string
}

type ghaActionMsg struct {
	gen int
	out string
	err string
}

type ghaDetailMsg struct {
	gen  int
	body string
	err  string
}

type ghaTickMsg struct {
	gen int
}

func (a *App) enterGHATab(_ *core.Project) {
	a.tab = TabActions
	a.tabCursor = 0
	a.ghaOpen = false
	a.resetGHATransient()
}

func (a *App) resetGHATransient() {
	a.ghaConfirm = false
	a.ghaConfirmAction = ""
	a.ghaScreen = ghaScrCluster
	a.ghaForm = ghaFormNone
	a.ghaFormField = 0
	a.ghaFormName = ""
	a.ghaFormDesc = ""
	a.ghaFormTemplate = "ci"
	a.ghaFormInput = ""
	a.ghaDetail = ""
	a.ghaStatus = ""
	a.ghaErr = ""
	a.ghaFocus = 0
	a.ghaActionIdx = 0
	a.ghaProcTab = ghaProcTabOverview
	a.ghaProcName = ""
	a.ghaProcFile = ""
	a.ghaProcRunID = ""
	a.ghaProcJobs = ""
	a.ghaProcLogs = ""
	a.ghaProcYAML = ""
	a.ghaProcScroll = 0
	a.ghaProcRunCursor = 0
	a.ghaTriggerBranches = nil
	a.ghaTriggerCursor = 0
	a.ghaTriggerScroll = 0
	a.ghaTriggerWF = ""
	a.ghaTriggerProc = ""
	a.ghaTriggerReturn = ghaScrCluster
	a.ghaRunScope = ghaRunScopeAll
	a.ghaRunProcFilter = ""
	a.ghaRunMarked = map[string]bool{}
	a.ghaNotes = map[string]string{}
	a.ghaTriggerInputs = nil
	a.ghaTriggerInputVals = nil
	a.ghaTriggerInputIdx = -1
	a.ghaTriggerAhead = 0
	a.ghaTriggerForce = false
	a.ghaNoteEditing = false
	a.ghaNoteInput = ""
	if a.ghaTriggered == nil {
		a.ghaTriggered = map[string]time.Time{}
	}
}

func (a *App) openGHAClient(p *core.Project) tea.Cmd {
	a.ghaOpen = true
	a.resetGHATransient()
	a.ghaKind = ghaKindProcesses
	a.ghaCursor = 0
	a.ghaScroll = 0
	a.ghaDetailScroll = 0
	a.ghaPath = ""
	a.ghaRemote = ""
	if p != nil {
		a.ghaPath = p.Path
		if p.Git != nil {
			a.ghaRemote = p.Git.Remote
		}
		if a.ghaRemote == "" && p.Path != "" {
			a.ghaRemote = collectors.GitRemoteOrigin(p.Path)
		}
	}
	a.ghaGen++
	a.ghaSetupShown = false
	return a.refreshGHA()
}

func (a *App) ghaNeedsSetup() bool {
	return !a.ghaInfo.Available || !a.ghaInfo.Authed
}

func (a *App) ghaShowSetup() {
	a.ghaScreen = ghaScrForm
	a.ghaForm = ghaFormSetup
	a.ghaSetupShown = true
}

func (a *App) leaveGHATab() tea.Cmd {
	a.ghaOpen = false
	a.ghaGen++
	a.resetGHATransient()
	a.tab = TabActions
	a.tabCursor = 0
	return nil
}

func (a *App) ghaHasActiveWork() bool {
	if a.ghaTriggered != nil {
		for _, t := range a.ghaTriggered {
			if time.Since(t) < 2*time.Minute {
				return true
			}
		}
	}
	for _, r := range a.ghaRuns {
		if ghaRunIsActive(r) {
			return true
		}
	}
	return false
}

func (a *App) ghaTickInterval() time.Duration {
	// Watching a run / jobs / logs → poll like Docker detail (fast).
	if a.ghaScreen == ghaScrProcess || a.ghaHasActiveWork() {
		return 3 * time.Second
	}
	return 6 * time.Second
}

func (a *App) scheduleGHATick() tea.Cmd {
	gen := a.ghaGen
	d := a.ghaTickInterval()
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return ghaTickMsg{gen: gen}
	})
}

func (a *App) refreshGHA() tea.Cmd {
	return a.refreshGHAOpts(true)
}

func (a *App) refreshGHASilent() tea.Cmd {
	return a.refreshGHAOpts(false)
}

func (a *App) refreshGHAOpts(showLoading bool) tea.Cmd {
	if showLoading {
		a.ghaLoading = true
	}
	gen := a.ghaGen
	path := a.ghaPath
	remote := a.ghaRemote
	fetchBilling := showLoading || !a.ghaBilling.OK
	return func() tea.Msg {
		info := collectors.GHARepoInfo(path, remote)
		msg := ghaLoadedMsg{gen: gen, info: info}
		procs, err := collectors.GHAListLocalWorkflowFiles(path)
		if err != nil {
			msg.err = err.Error()
		} else {
			msg.processes = procs
		}
		if info.Authed {
			if wfs, err := collectors.GHAListWorkflows(path); err != nil {
				if msg.err == "" {
					msg.err = err.Error()
				}
			} else {
				msg.workflows = wfs
			}
			if runs, err := collectors.GHAListRuns(path, 25); err != nil {
				if msg.err == "" {
					msg.err = err.Error()
				}
			} else {
				msg.runs = runs
			}
			if fetchBilling {
				msg.billing = collectors.GHAFetchActionsBilling(path, info.Owner)
				msg.hasBill = true
			}
		}
		return msg
	}
}

func (a *App) handleGHAMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case ghaLoadedMsg:
		if m.gen != a.ghaGen {
			return a, nil
		}
		a.ghaLoading = false
		a.ghaInfo = m.info
		a.ghaProcesses = m.processes
		a.ghaWorkflows = m.workflows
		a.ghaRuns = m.runs
		if m.hasBill {
			a.ghaBilling = m.billing
		}
		if m.err != "" {
			a.ghaErr = m.err
		} else {
			a.ghaErr = ""
		}
		a.ghaReloadNotes()
		a.clampGHACursor()
		if a.ghaOpen && a.ghaNeedsSetup() && !a.ghaSetupShown && a.ghaScreen == ghaScrCluster {
			a.ghaShowSetup()
		}
		var cmds []tea.Cmd
		if a.ghaScreen == ghaScrCluster {
			cmds = append(cmds, a.ghaInspectSelected())
		}
		if a.ghaOpen {
			cmds = append(cmds, a.scheduleGHATick())
		}
		return a, tea.Batch(cmds...)
	case ghaAuthDoneMsg:
		if m.gen != a.ghaGen {
			return a, nil
		}
		if m.err != nil {
			a.ghaStatus = "✗ login: " + m.err.Error()
			a.ghaShowSetup()
		} else {
			a.ghaStatus = "✓ login concluído"
			a.ghaScreen = ghaScrCluster
			a.ghaForm = ghaFormNone
			a.ghaSetupShown = false
		}
		return a, a.refreshGHA()
	case ghaActionMsg:
		if m.gen != a.ghaGen {
			return a, nil
		}
		a.ghaLoading = false
		wasProcess := a.ghaScreen == ghaScrProcess
		wasTrigger := a.ghaForm == ghaFormTrigger
		if m.err != "" {
			a.ghaStatus = "✗ " + m.err
			a.ghaErr = m.err
			if wasTrigger && strings.HasPrefix(m.err, "push:") {
				return a, nil // keep trigger modal open
			}
		} else {
			a.ghaStatus = "✓ " + firstNonEmpty(m.out, "ok")
			a.ghaErr = ""
			if strings.HasPrefix(m.out, "triggered ") {
				name := strings.TrimSpace(strings.TrimPrefix(m.out, "triggered "))
				if i := strings.Index(name, " · "); i >= 0 {
					name = strings.TrimSpace(name[:i])
				}
				if name != "" {
					if a.ghaTriggered == nil {
						a.ghaTriggered = map[string]time.Time{}
					}
					a.ghaTriggered[name] = time.Now()
					a.ghaProcName = name
				}
			}
			if strings.HasPrefix(m.out, "noted ") {
				a.ghaReloadNotes()
				return a, nil
			}
			if strings.HasPrefix(m.out, "pushed ") && wasTrigger {
				a.ghaRefreshTriggerAhead()
				a.ghaTriggerForce = false
				a.ghaStatus = "✓ push ok — enter para disparar"
				return a, nil
			}
			if strings.HasPrefix(m.out, "bulk-stopped ") {
				a.ghaRunMarked = map[string]bool{}
			}
		}
		a.ghaForm = ghaFormNone
		if strings.HasPrefix(m.out, "stopped ") && a.ghaTriggered != nil {
			// out: "stopped <id> · <proc> · ..." ou "stopped N/M jobs de <name>"
			if i := strings.Index(m.out, " · "); i >= 0 {
				rest := m.out[i+3:]
				if j := strings.Index(rest, " · "); j >= 0 {
					delete(a.ghaTriggered, strings.TrimSpace(rest[:j]))
				}
			}
			if k := strings.Index(m.out, " jobs de "); k >= 0 {
				delete(a.ghaTriggered, strings.TrimSpace(m.out[k+len(" jobs de "):]))
			}
		}
		if wasProcess || strings.HasPrefix(m.out, "triggered ") {
			a.ghaScreen = ghaScrProcess
			if a.ghaProcFile == "" && a.ghaProcName != "" {
				for _, p := range a.ghaProcesses {
					if p.Name == a.ghaProcName {
						a.ghaProcFile = p.File
						break
					}
				}
			}
			a.ghaProcTab = ghaProcTabOverview
			return a, tea.Batch(a.refreshGHA(), a.ghaLoadProcessDetail())
		}
		a.ghaScreen = ghaScrCluster
		return a, a.refreshGHA()
	case ghaDetailMsg:
		if m.gen != a.ghaGen {
			return a, nil
		}
		body := m.body
		if m.err != "" {
			body = m.err
		}
		a.ghaDetail = body
		if a.ghaScreen == ghaScrProcess {
			switch a.ghaProcTab {
			case ghaProcTabJobs:
				first := strings.TrimSpace(a.ghaProcJobs) == ""
				a.ghaProcJobs = body
				if first {
					a.ghaProcScroll = 0
				}
			case ghaProcTabLogs:
				first := strings.TrimSpace(a.ghaProcLogs) == ""
				wasBottom := a.ghaProcAtBottom()
				a.ghaProcLogs = body
				if first {
					a.ghaProcScroll = 0
				} else if wasBottom {
					a.ghaProcStickBottom()
				}
			case ghaProcTabYAML:
				first := strings.TrimSpace(a.ghaProcYAML) == ""
				a.ghaProcYAML = body
				if first {
					a.ghaProcScroll = 0
				}
			}
		} else {
			a.ghaDetailScroll = 0
		}
		if a.ghaScreen == ghaScrLogs || a.ghaForm == ghaFormViewYAML {
			a.ghaStatus = ""
		}
		return a, nil
	case ghaTickMsg:
		if m.gen != a.ghaGen || !a.ghaOpen {
			return a, nil
		}
		// Keep the poll chain alive even during confirm/setup.
		if a.ghaConfirm || a.ghaScreen == ghaScrForm {
			return a, a.scheduleGHATick()
		}
		if a.ghaScreen == ghaScrProcess {
			cmds := []tea.Cmd{a.refreshGHASilent()}
			switch a.ghaProcTab {
			case ghaProcTabJobs, ghaProcTabLogs:
				cmds = append(cmds, a.ghaLoadProcessDetail())
			}
			// next tick is scheduled when ghaLoadedMsg lands
			return a, tea.Batch(cmds...)
		}
		return a, a.refreshGHASilent()
	}
	return a, nil
}

func (a *App) ghaProcAtBottom() bool {
	body := a.ghaProcLogs
	if a.ghaProcTab == ghaProcTabJobs {
		body = a.ghaProcJobs
	}
	lines := strings.Split(body, "\n")
	inner := 12 // approximate; stick-bottom corrected on next render via clamp
	return a.ghaProcScroll >= maxInt(0, len(lines)-inner)
}

func (a *App) ghaProcStickBottom() {
	body := a.ghaProcLogs
	if a.ghaProcTab == ghaProcTabJobs {
		body = a.ghaProcJobs
	}
	lines := len(strings.Split(body, "\n"))
	a.ghaProcScroll = maxInt(0, lines-1)
}

func (a *App) clampGHACursor() {
	n := a.ghaRowCount()
	if n == 0 {
		a.ghaCursor = 0
		return
	}
	a.ghaCursor = clampCursor(a.ghaCursor, n)
}

func (a *App) ghaRowCount() int {
	switch a.ghaKind {
	case ghaKindProcesses:
		return len(a.ghaProcesses)
	case ghaKindRuns:
		return len(a.ghaFilteredRuns())
	case ghaKindWorkflows:
		return len(a.ghaWorkflows)
	}
	return 0
}

func (a *App) ghaFilteredRuns() []collectors.GHARun {
	if a.ghaRunScope == ghaRunScopeAll && a.ghaRunProcFilter == "" {
		return a.ghaRuns
	}
	out := make([]collectors.GHARun, 0, len(a.ghaRuns))
	filt := strings.ToLower(strings.TrimSpace(a.ghaRunProcFilter))
	for _, r := range a.ghaRuns {
		if a.ghaRunScope == ghaRunScopeRunning && !ghaRunIsActive(r) {
			continue
		}
		if filt != "" {
			wf := strings.ToLower(r.Workflow)
			sn := sanitizeGHAName(r.Workflow)
			if wf != filt && sn != filt && !strings.Contains(wf, filt) && !strings.Contains(sn, filt) {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

func (a *App) ghaSelectedRun() (collectors.GHARun, bool) {
	runs := a.ghaFilteredRuns()
	if a.ghaCursor < 0 || a.ghaCursor >= len(runs) {
		return collectors.GHARun{}, false
	}
	return runs[a.ghaCursor], true
}

func (a *App) ghaRunProcessFilterOptions() []string {
	seen := map[string]bool{}
	opts := []string{""} // all
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if seen[key] {
			return
		}
		seen[key] = true
		opts = append(opts, name)
	}
	for _, p := range a.ghaProcesses {
		add(p.Name)
	}
	for _, r := range a.ghaRuns {
		add(r.Workflow)
	}
	for _, w := range a.ghaWorkflows {
		add(w.Name)
	}
	return opts
}

func (a *App) ghaCycleRunScope() {
	if a.ghaRunScope == ghaRunScopeAll {
		a.ghaRunScope = ghaRunScopeRunning
	} else {
		a.ghaRunScope = ghaRunScopeAll
	}
	a.ghaCursor = 0
	a.ghaScroll = 0
	a.ghaStatus = "filtro status · " + a.ghaRunScope.String()
}

func (a *App) ghaCycleRunProcFilter(dir int) {
	opts := a.ghaRunProcessFilterOptions()
	if len(opts) == 0 {
		return
	}
	idx := 0
	cur := strings.ToLower(a.ghaRunProcFilter)
	for i, o := range opts {
		if strings.ToLower(o) == cur {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(opts)*8) % len(opts)
	a.ghaRunProcFilter = opts[idx]
	a.ghaCursor = 0
	a.ghaScroll = 0
	if a.ghaRunProcFilter == "" {
		a.ghaStatus = "filtro processo · todos"
	} else {
		a.ghaStatus = "filtro processo · " + a.ghaRunProcFilter
	}
}

func (a *App) ghaRunsFilterLabel() string {
	parts := []string{a.ghaRunScope.String()}
	if a.ghaRunProcFilter != "" {
		parts = append(parts, a.ghaRunProcFilter)
	}
	return strings.Join(parts, " · ")
}

// ghaSeedRunFilterFromProcess pre-filters Runs when leaving the Processes list.
func (a *App) ghaSeedRunFilterFromProcess() {
	if a.ghaKind != ghaKindProcesses || a.ghaCursor >= len(a.ghaProcesses) {
		return
	}
	a.ghaRunProcFilter = a.ghaProcesses[a.ghaCursor].Name
}

func (a *App) handleGHAKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.ghaNoteEditing {
		switch msg.String() {
		case "esc":
			a.ghaNoteEditing = false
			a.ghaNoteInput = ""
			a.ghaConfirmAction = ""
			a.ghaStatus = "nota cancelada"
			return a, nil
		case "enter":
			return a, a.ghaSaveNote()
		case "backspace":
			if len(a.ghaNoteInput) > 0 {
				a.ghaNoteInput = a.ghaNoteInput[:len(a.ghaNoteInput)-1]
			}
		default:
			if len(msg.Runes) == 1 {
				a.ghaNoteInput += string(msg.Runes)
			}
		}
		return a, nil
	}
	if a.ghaConfirm {
		switch msg.String() {
		case "y", "Y":
			action := a.ghaConfirmAction
			a.ghaConfirm = false
			a.ghaConfirmAction = ""
			return a, a.ghaRunConfirm(action)
		case "n", "N", "esc":
			a.ghaConfirm = false
			a.ghaConfirmAction = ""
			a.ghaStatus = "cancelado"
			return a, nil
		}
		return a, nil
	}

	if a.ghaScreen == ghaScrForm {
		return a.handleGHAFormKeys(msg)
	}

	if a.ghaScreen == ghaScrProcess {
		return a.handleGHAProcessKeys(msg, p)
	}

	if a.ghaScreen == ghaScrLogs || a.ghaScreen == ghaScrDetail {
		switch msg.String() {
		case "esc":
			a.ghaScreen = ghaScrCluster
			a.ghaForm = ghaFormNone
			return a, nil
		case "up", "k":
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-1)
		case "down", "j":
			a.ghaDetailScroll++
		case "pgup":
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-10)
		case "pgdown":
			a.ghaDetailScroll += 10
		case "r", "f":
			if a.ghaScreen == ghaScrLogs {
				return a, a.ghaShowLogs()
			}
			return a, a.ghaInspectSelected()
		case "l":
			return a, a.ghaShowLogs()
		}
		return a, nil
	}

	switch msg.String() {
	case "esc":
		if a.ghaFocus != ghaFocusTable {
			a.ghaFocus = ghaFocusTable
			return a, nil
		}
		return a, a.leaveGHATab()
	case "tab":
		a.ghaFocus = (a.ghaFocus + 1) % ghaFocusCount
		return a, nil
	case "shift+tab":
		a.ghaFocus = (a.ghaFocus + ghaFocusCount - 1) % ghaFocusCount
		return a, nil
	case "[", "left":
		// Troca Processes/Runs/Workflows só no painel da lista
		if a.ghaFocus != ghaFocusTable {
			return a, nil
		}
		a.ghaSeedRunFilterFromProcess()
		a.ghaKind = ghaKind((int(a.ghaKind) + int(ghaKindCount) - 1) % int(ghaKindCount))
		a.ghaCursor, a.ghaScroll, a.ghaActionIdx = 0, 0, 0
		a.ghaDetailScroll = 0
		return a, a.ghaInspectSelected()
	case "]", "right":
		if a.ghaFocus != ghaFocusTable {
			return a, nil
		}
		a.ghaSeedRunFilterFromProcess()
		a.ghaKind = ghaKind((int(a.ghaKind) + 1) % int(ghaKindCount))
		a.ghaCursor, a.ghaScroll, a.ghaActionIdx = 0, 0, 0
		a.ghaDetailScroll = 0
		return a, a.ghaInspectSelected()
	case "1":
		if a.ghaFocus != ghaFocusTable {
			return a, nil
		}
		a.ghaKind = ghaKindProcesses
		a.ghaCursor, a.ghaScroll, a.ghaActionIdx = 0, 0, 0
		a.ghaDetailScroll = 0
		return a, a.ghaInspectSelected()
	case "2":
		if a.ghaFocus != ghaFocusTable {
			return a, nil
		}
		a.ghaSeedRunFilterFromProcess()
		a.ghaKind = ghaKindRuns
		a.ghaCursor, a.ghaScroll, a.ghaActionIdx = 0, 0, 0
		a.ghaDetailScroll = 0
		return a, a.ghaInspectSelected()
	case "3":
		if a.ghaFocus != ghaFocusTable {
			return a, nil
		}
		a.ghaKind = ghaKindWorkflows
		a.ghaCursor, a.ghaScroll, a.ghaActionIdx = 0, 0, 0
		a.ghaDetailScroll = 0
		return a, a.ghaInspectSelected()
	case "up", "k":
		switch a.ghaFocus {
		case ghaFocusResumo:
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-1)
			return a, nil
		case ghaFocusActions:
			if a.ghaActionIdx > 0 {
				a.ghaActionIdx--
			}
			return a, nil
		default:
			if a.ghaCursor > 0 {
				a.ghaCursor--
				a.ghaDetailScroll = 0
				return a, a.ghaInspectSelected()
			}
		}
	case "down", "j":
		switch a.ghaFocus {
		case ghaFocusResumo:
			a.ghaDetailScroll++
			return a, nil
		case ghaFocusActions:
			if a.ghaActionIdx < len(a.ghaQuickActionItems())-1 {
				a.ghaActionIdx++
			}
			return a, nil
		default:
			if a.ghaCursor < a.ghaRowCount()-1 {
				a.ghaCursor++
				a.ghaDetailScroll = 0
				return a, a.ghaInspectSelected()
			}
		}
	case "pgup":
		if a.ghaFocus == ghaFocusResumo || a.ghaFocus == ghaFocusTable {
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-8)
		}
		return a, nil
	case "pgdown":
		if a.ghaFocus == ghaFocusResumo || a.ghaFocus == ghaFocusTable {
			a.ghaDetailScroll += 8
		}
		return a, nil
	case "enter":
		if a.ghaFocus == ghaFocusActions {
			return a, a.ghaRunQuickAction(p)
		}
		return a, a.ghaOpenProcessDetail()
	case "r":
		return a, a.refreshGHA()
	case "c":
		return a, a.ghaBeginCreate()
	case "d", "D", "delete":
		return a, a.ghaAskDelete()
	case "t":
		if a.ghaNeedsSetup() {
			a.ghaShowSetup()
			a.ghaStatus = "trigger precisa do gh autenticado"
			return a, nil
		}
		return a, a.ghaTrigger()
	case "l":
		return a, a.ghaOpenProcessDetailAt(ghaProcTabLogs)
	case "F":
		return a, a.ghaShowFailedLogs()
	case "y":
		return a, a.ghaOpenProcessDetailAt(ghaProcTabYAML)
	case "o", "O":
		return a, a.ghaOpenBrowser()
	case "L":
		return a, a.ghaBeginLogin()
	case "!":
		a.ghaShowSetup()
		return a, nil
	case "R":
		return a, a.ghaRerun()
	case "s", "x":
		return a, a.ghaAskStopJob()
	case "S":
		if a.ghaKind == ghaKindRuns {
			return a, a.ghaAskBulkStopMarked()
		}
		return a, a.ghaAskStopAllJobs()
	case " ":
		if a.ghaKind == ghaKindRuns && a.ghaFocus == ghaFocusTable {
			a.ghaToggleMarkSelectedRun()
			return a, nil
		}
	case "i":
		if a.ghaKind == ghaKindRuns {
			return a, a.ghaBeginNoteEdit()
		}
	case "f":
		if a.ghaKind == ghaKindRuns {
			a.ghaCycleRunScope()
			return a, a.ghaInspectSelected()
		}
	case "p", "P":
		if a.ghaKind == ghaKindRuns {
			dir := 1
			if msg.String() == "P" {
				dir = -1
			}
			a.ghaCycleRunProcFilter(dir)
			return a, a.ghaInspectSelected()
		}
	case "0":
		if a.ghaKind == ghaKindRuns {
			a.ghaRunScope = ghaRunScopeAll
			a.ghaRunProcFilter = ""
			a.ghaCursor = 0
			a.ghaScroll = 0
			a.ghaStatus = "filtros limpos"
			return a, a.ghaInspectSelected()
		}
	}
	return a, nil
}

func (a *App) ghaQuickActionItems() [][2]string {
	setup := [][2]string{}
	if !a.ghaInfo.Available {
		setup = append(setup, [2]string{"!", "Setup / instalar gh"})
	} else if !a.ghaInfo.Authed {
		setup = append(setup, [2]string{"L", "Login GitHub (gh)"}, [2]string{"!", "Aviso setup"})
	}
	var items [][2]string
	switch a.ghaKind {
	case ghaKindRuns:
		items = [][2]string{
			{"f", "Filtro: all/running"},
			{"p", "Filtro: processo"},
			{"0", "Limpar filtros"},
			{"space", "Marcar p/ bulk stop"},
			{"S", "Bulk stop marcados"},
			{"i", "Anotar incidente"},
			{"l", "Ver logs"},
			{"F", "Logs failed only"},
			{"t", "Trigger"},
			{"s", "Parar job"},
			{"R", "Re-run"},
			{"o", "Abrir no GitHub"},
			{"r", "Atualizar"},
		}
	case ghaKindWorkflows:
		items = [][2]string{
			{"t", "Trigger workflow"},
			{"s", "Parar job"},
			{"y", "Ver YAML local"},
			{"o", "Abrir no GitHub"},
			{"r", "Atualizar"},
		}
	default:
		items = [][2]string{
			{"c", "Criar processo"},
			{"d", "Deletar processo"},
			{"t", "Trigger"},
			{"s", "Parar job"},
			{"S", "Parar todos ativos"},
			{"y", "Ver YAML"},
			{"l", "Logs do run"},
			{"o", "Abrir no GitHub"},
			{"r", "Atualizar"},
		}
	}
	return append(setup, items...)
}

func (a *App) ghaRunQuickAction(p *core.Project) tea.Cmd {
	items := a.ghaQuickActionItems()
	if a.ghaActionIdx < 0 || a.ghaActionIdx >= len(items) {
		return nil
	}
	switch items[a.ghaActionIdx][0] {
	case "c":
		return a.ghaBeginCreate()
	case "d":
		return a.ghaAskDelete()
	case "t":
		if a.ghaNeedsSetup() {
			a.ghaShowSetup()
			return nil
		}
		return a.ghaTrigger()
	case "y":
		return a.ghaOpenProcessDetailAt(ghaProcTabYAML)
	case "l":
		return a.ghaOpenProcessDetailAt(ghaProcTabLogs)
	case "o":
		return a.ghaOpenBrowser()
	case "L":
		return a.ghaBeginLogin()
	case "!":
		a.ghaShowSetup()
		return nil
	case "R":
		return a.ghaRerun()
	case "s", "x":
		return a.ghaAskStopJob()
	case "S":
		if a.ghaKind == ghaKindRuns {
			return a.ghaAskBulkStopMarked()
		}
		return a.ghaAskStopAllJobs()
	case "f":
		a.ghaCycleRunScope()
		return a.ghaInspectSelected()
	case "p":
		a.ghaCycleRunProcFilter(1)
		return a.ghaInspectSelected()
	case "0":
		a.ghaRunScope = ghaRunScopeAll
		a.ghaRunProcFilter = ""
		a.ghaCursor = 0
		a.ghaScroll = 0
		a.ghaStatus = "filtros limpos"
		return a.ghaInspectSelected()
	case "space":
		a.ghaToggleMarkSelectedRun()
		return nil
	case "i":
		return a.ghaBeginNoteEdit()
	case "F":
		return a.ghaShowFailedLogs()
	case "r":
		return a.refreshGHA()
	}
	return nil
}

func (a *App) ghaBeginLogin() tea.Cmd {
	if !collectors.GHAAvailable() {
		a.ghaShowSetup()
		a.ghaStatus = "instale o GitHub CLI antes do login"
		return nil
	}
	a.ghaStatus = "abrindo gh auth login…"
	gen := a.ghaGen
	cmd := collectors.GHAAuthLoginCmd()
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return ghaAuthDoneMsg{gen: gen, err: err}
	})
}

func (a *App) ghaOpenDetail() tea.Cmd {
	a.ghaScreen = ghaScrDetail
	a.ghaDetailScroll = 0
	return a.ghaInspectSelected()
}

func (a *App) ghaInspectSelected() tea.Cmd {
	gen := a.ghaGen
	switch a.ghaKind {
	case ghaKindProcesses:
		if a.ghaCursor >= len(a.ghaProcesses) {
			a.ghaDetail = "nenhum processo — c cria · catalog .devscope/actions.yaml"
			return nil
		}
		p := a.ghaProcesses[a.ghaCursor]
		path := a.ghaPath
		return func() tea.Msg {
			body, err := collectors.GHAReadProcessFile(path, p.File)
			if err != nil {
				return ghaDetailMsg{gen: gen, body: fmt.Sprintf("PROCESS %s\nFile  %s\nDesc  %s\n\n%s",
					p.Name, p.File, p.Description, err.Error())}
			}
			preview := body
			if len(preview) > 4000 {
				preview = preview[:4000] + "\n…(truncated)"
			}
			hdr := fmt.Sprintf("PROCESS %s\nFile         %s\nDescription  %s\nTemplate     %s\nCatalog      .devscope/actions.yaml\n\n── WORKFLOW YAML ──\n",
				p.Name, p.File, firstNonEmpty(p.Description, "-"), firstNonEmpty(p.Template, "-"))
			return ghaDetailMsg{gen: gen, body: hdr + preview}
		}
	case ghaKindRuns:
		r, ok := a.ghaSelectedRun()
		if !ok {
			a.ghaDetail = "nenhum run com os filtros atuais\n\nf all/running  ·  p processo  ·  0 limpar"
			return nil
		}
		a.ghaDetail = fmt.Sprintf("RUN #%s\nTitle       %s\nWorkflow    %s\nStatus      %s\nConclusion  %s\nBranch      %s\nEvent       %s\nCreated     %s\nURL         %s",
			r.ID, firstNonEmpty(r.DisplayTitle, r.Name), r.Workflow, r.Status, firstNonEmpty(r.Conclusion, "-"),
			r.Branch, r.Event, r.CreatedAt, r.URL)
		return nil
	case ghaKindWorkflows:
		if a.ghaCursor >= len(a.ghaWorkflows) {
			a.ghaDetail = "nenhum workflow remoto (gh auth / push)"
			return nil
		}
		w := a.ghaWorkflows[a.ghaCursor]
		a.ghaDetail = fmt.Sprintf("WORKFLOW\nName   %s\nID     %s\nState  %s\nPath   %s",
			w.Name, w.ID, w.State, w.Path)
		return nil
	}
	return nil
}

func (a *App) ghaBeginCreate() tea.Cmd {
	a.ghaScreen = ghaScrForm
	a.ghaForm = ghaFormCreate
	a.ghaFormField = 0
	a.ghaFormName = ""
	a.ghaFormDesc = ""
	a.ghaFormTemplate = "ci"
	a.ghaFormInput = ""
	return nil
}

func (a *App) ghaAskDelete() tea.Cmd {
	if a.ghaKind != ghaKindProcesses || a.ghaCursor >= len(a.ghaProcesses) {
		a.ghaStatus = "delete: selecione um processo"
		return nil
	}
	name := a.ghaProcesses[a.ghaCursor].Name
	a.ghaConfirm = true
	a.ghaConfirmAction = "rm-process:" + name
	return nil
}

func (a *App) ghaTrigger() tea.Cmd {
	var wf, procName, file string
	switch a.ghaKind {
	case ghaKindProcesses:
		if a.ghaCursor < len(a.ghaProcesses) {
			procName = a.ghaProcesses[a.ghaCursor].Name
			file = a.ghaProcesses[a.ghaCursor].File
			wf = filepath.Base(file)
		}
	case ghaKindWorkflows:
		if a.ghaCursor < len(a.ghaWorkflows) {
			procName = sanitizeGHAName(a.ghaWorkflows[a.ghaCursor].Name)
			file = a.ghaWorkflows[a.ghaCursor].Path
			wf = firstNonEmpty(filepath.Base(file), a.ghaWorkflows[a.ghaCursor].Name)
		}
	case ghaKindRuns:
		if r, ok := a.ghaSelectedRun(); ok {
			procName = sanitizeGHAName(r.Workflow)
			wf = r.Workflow
		}
	}
	if wf == "" {
		a.ghaStatus = "trigger: selecione processo/workflow"
		return nil
	}
	if procName == "" {
		procName = strings.TrimSuffix(wf, filepath.Ext(wf))
	}
	return a.ghaBeginTrigger(procName, file, wf, ghaScrCluster)
}

func (a *App) ghaBeginTrigger(procName, file, wf string, ret ghaScreen) tea.Cmd {
	a.ghaTriggerReturn = ret
	a.ghaTriggerProc = procName
	a.ghaTriggerWF = wf
	if file != "" {
		a.ghaProcFile = file
	}
	a.ghaProcName = procName
	a.ghaTriggerBranches = collectors.GitRemoteBranchNames(a.ghaPath)
	a.ghaTriggerCursor = 0
	a.ghaTriggerScroll = 0
	a.ghaTriggerInputIdx = -1
	a.ghaTriggerForce = false
	current := collectors.GitCurrentBranchName(a.ghaPath)
	for i, b := range a.ghaTriggerBranches {
		if b == current {
			a.ghaTriggerCursor = i
			break
		}
	}
	a.ghaTriggerInputs = nil
	a.ghaTriggerInputVals = nil
	if inputs, err := collectors.GHAParseWorkflowInputs(a.ghaPath, firstNonEmpty(file, wf)); err == nil {
		a.ghaTriggerInputs = inputs
		a.ghaTriggerInputVals = make([]string, len(inputs))
		for i, in := range inputs {
			a.ghaTriggerInputVals[i] = in.Default
		}
	}
	a.ghaRefreshTriggerAhead()
	a.ghaScreen = ghaScrForm
	a.ghaForm = ghaFormTrigger
	if len(a.ghaTriggerBranches) == 0 {
		a.ghaStatus = "nenhuma branch no origin — faça push ou git fetch"
	} else {
		a.ghaStatus = fmt.Sprintf("trigger %s · branch + inputs", procName)
	}
	return nil
}

func (a *App) ghaRefreshTriggerAhead() {
	a.ghaTriggerAhead = 0
	if a.ghaTriggerCursor >= 0 && a.ghaTriggerCursor < len(a.ghaTriggerBranches) {
		a.ghaTriggerAhead = collectors.GitBranchAheadCount(a.ghaPath, a.ghaTriggerBranches[a.ghaTriggerCursor])
	}
}

func (a *App) ghaCloseTriggerForm() {
	ret := a.ghaTriggerReturn
	a.ghaForm = ghaFormNone
	a.ghaScreen = ret
	a.ghaTriggerWF = ""
	a.ghaTriggerProc = ""
	a.ghaTriggerBranches = nil
	a.ghaTriggerInputs = nil
	a.ghaTriggerInputVals = nil
	a.ghaTriggerInputIdx = -1
	a.ghaTriggerAhead = 0
	a.ghaTriggerForce = false
	a.ghaTriggerReturn = ghaScrCluster
}

func (a *App) ghaTriggerInputMap() map[string]string {
	if len(a.ghaTriggerInputs) == 0 {
		return nil
	}
	m := map[string]string{}
	for i, in := range a.ghaTriggerInputs {
		val := ""
		if i < len(a.ghaTriggerInputVals) {
			val = a.ghaTriggerInputVals[i]
		}
		if val == "" {
			val = in.Default
		}
		if val != "" || in.Required {
			m[in.Name] = val
		}
	}
	return m
}

func (a *App) ghaConfirmTrigger() tea.Cmd {
	if a.ghaTriggerCursor < 0 || a.ghaTriggerCursor >= len(a.ghaTriggerBranches) {
		a.ghaStatus = "selecione uma branch pushed (origin)"
		return nil
	}
	branch := a.ghaTriggerBranches[a.ghaTriggerCursor]
	a.ghaRefreshTriggerAhead()
	if a.ghaTriggerAhead > 0 && !a.ghaTriggerForce {
		a.ghaStatus = fmt.Sprintf("PUSH NEEDED · %s tem %d commit(s) local — P push  ·  y força  ·  esc", branch, a.ghaTriggerAhead)
		return nil
	}
	for i, in := range a.ghaTriggerInputs {
		if !in.Required {
			continue
		}
		val := ""
		if i < len(a.ghaTriggerInputVals) {
			val = strings.TrimSpace(a.ghaTriggerInputVals[i])
		}
		if val == "" {
			a.ghaStatus = "input obrigatório: " + in.Name
			a.ghaTriggerInputIdx = i
			return nil
		}
	}
	wf := a.ghaTriggerWF
	procName := a.ghaTriggerProc
	if wf == "" || procName == "" {
		a.ghaStatus = "trigger incompleto"
		return nil
	}
	inputs := a.ghaTriggerInputMap()
	if a.ghaTriggered == nil {
		a.ghaTriggered = map[string]time.Time{}
	}
	a.ghaTriggered[procName] = time.Now()
	a.ghaProcName = procName
	gen := a.ghaGen
	path := a.ghaPath
	a.ghaCloseTriggerForm()
	a.ghaStatus = "Triggering " + procName + " @ " + branch + "..."
	return func() tea.Msg {
		out, err := collectors.GHATriggerWorkflowRefInputs(path, wf, branch, inputs)
		if err != nil {
			name := strings.TrimSuffix(wf, filepath.Ext(wf))
			if name != wf {
				out, err = collectors.GHATriggerWorkflowRefInputs(path, name, branch, inputs)
			}
			if err != nil {
				return ghaActionMsg{gen: gen, err: err.Error()}
			}
		}
		return ghaActionMsg{gen: gen, out: "triggered " + procName + " · " + branch + " · " + firstNonEmpty(firstLine(out), "ok")}
	}
}

func (a *App) ghaPushThenTrigger() tea.Cmd {
	if a.ghaTriggerCursor < 0 || a.ghaTriggerCursor >= len(a.ghaTriggerBranches) {
		return nil
	}
	branch := a.ghaTriggerBranches[a.ghaTriggerCursor]
	gen := a.ghaGen
	path := a.ghaPath
	a.ghaStatus = "push origin/" + branch + "..."
	return func() tea.Msg {
		out, err := collectors.GitPushBranch(path, branch)
		if err != nil {
			return ghaActionMsg{gen: gen, err: "push: " + err.Error()}
		}
		return ghaActionMsg{gen: gen, out: "pushed " + branch + " · " + firstNonEmpty(firstLine(out), "ok")}
	}
}

func (a *App) ghaReloadNotes() {
	a.ghaNotes = map[string]string{}
	f, err := collectors.LoadGHANotes(a.ghaPath)
	if err != nil {
		return
	}
	for _, n := range f.Notes {
		a.ghaNotes[n.RunID] = n.Note
	}
}

func (a *App) ghaToggleMarkSelectedRun() {
	r, ok := a.ghaSelectedRun()
	if !ok {
		return
	}
	if a.ghaRunMarked == nil {
		a.ghaRunMarked = map[string]bool{}
	}
	if a.ghaRunMarked[r.ID] {
		delete(a.ghaRunMarked, r.ID)
		a.ghaStatus = "desmarcado #" + r.ID
	} else {
		a.ghaRunMarked[r.ID] = true
		a.ghaStatus = "marcado #" + r.ID + " · space marca  S para bulk stop"
	}
}

func (a *App) ghaAskBulkStopMarked() tea.Cmd {
	if a.ghaNeedsSetup() {
		a.ghaShowSetup()
		return nil
	}
	ids := make([]string, 0, len(a.ghaRunMarked))
	for id, on := range a.ghaRunMarked {
		if !on {
			continue
		}
		for _, r := range a.ghaRuns {
			if r.ID == id && ghaRunIsActive(r) {
				ids = append(ids, id)
				break
			}
		}
	}
	if len(ids) == 0 {
		// fallback: stop all active in current filter
		return a.ghaAskStopAllFiltered()
	}
	a.ghaConfirm = true
	a.ghaConfirmAction = "stop-marked:" + strings.Join(ids, ",")
	a.ghaStatus = fmt.Sprintf("parar %d run(s) marcados?", len(ids))
	return nil
}

func (a *App) ghaAskStopAllFiltered() tea.Cmd {
	ids := []string{}
	for _, r := range a.ghaFilteredRuns() {
		if ghaRunIsActive(r) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		a.ghaStatus = "nenhum run ativo (marque com space ou filtre running)"
		return nil
	}
	a.ghaConfirm = true
	a.ghaConfirmAction = "stop-marked:" + strings.Join(ids, ",")
	a.ghaStatus = fmt.Sprintf("parar %d run(s) ativos filtrados?", len(ids))
	return nil
}

func (a *App) ghaBeginNoteEdit() tea.Cmd {
	r, ok := a.ghaSelectedRun()
	if !ok && a.ghaProcRunID == "" {
		a.ghaStatus = "selecione um run para anotar"
		return nil
	}
	id := a.ghaProcRunID
	if ok {
		id = r.ID
	}
	a.ghaNoteEditing = true
	a.ghaNoteInput = firstNonEmpty(a.ghaNotes[id], "incidente")
	a.ghaStatus = "nota run #" + id + " · enter salva  esc cancela"
	a.ghaConfirmAction = "note:" + id // stash id
	return nil
}

func (a *App) ghaSaveNote() tea.Cmd {
	id := strings.TrimPrefix(a.ghaConfirmAction, "note:")
	note := strings.TrimSpace(a.ghaNoteInput)
	a.ghaNoteEditing = false
	a.ghaNoteInput = ""
	a.ghaConfirmAction = ""
	if id == "" || note == "" {
		a.ghaStatus = "nota cancelada"
		return nil
	}
	proc := a.ghaProcName
	if r, ok := a.ghaSelectedRun(); ok {
		proc = sanitizeGHAName(r.Workflow)
	}
	path := a.ghaPath
	gen := a.ghaGen
	return func() tea.Msg {
		if err := collectors.SaveGHANote(path, id, note, proc); err != nil {
			return ghaActionMsg{gen: gen, err: err.Error()}
		}
		return ghaActionMsg{gen: gen, out: "noted #" + id + " · " + note}
	}
}

func (a *App) ghaShowLogs() tea.Cmd {
	return a.ghaFetchLogs(false)
}

func (a *App) ghaShowFailedLogs() tea.Cmd {
	return a.ghaFetchLogs(true)
}

func (a *App) ghaResolveLogRunID() string {
	if a.ghaScreen == ghaScrProcess && a.ghaProcRunID != "" {
		return a.ghaProcRunID
	}
	if r, ok := a.ghaSelectedRun(); ok {
		return r.ID
	}
	if a.ghaKind == ghaKindProcesses && a.ghaCursor < len(a.ghaProcesses) {
		name := a.ghaProcesses[a.ghaCursor].Name
		for _, r := range a.ghaRuns {
			if strings.EqualFold(r.Workflow, name) || strings.Contains(strings.ToLower(r.Workflow), name) {
				return r.ID
			}
		}
	}
	return ""
}

func (a *App) ghaFetchLogs(failedOnly bool) tea.Cmd {
	id := a.ghaResolveLogRunID()
	if id == "" {
		a.ghaStatus = "logs: selecione um run"
		return nil
	}
	gen := a.ghaGen
	path := a.ghaPath
	if a.ghaScreen == ghaScrProcess {
		a.ghaProcTab = ghaProcTabLogs
		a.ghaProcScroll = 0
		a.ghaStatus = "Fetching failed logs..."
		if !failedOnly {
			a.ghaStatus = "Fetching logs..."
		}
		return func() tea.Msg {
			var body string
			var err error
			if failedOnly {
				body, err = collectors.GHARunFailedLogs(path, id)
			} else {
				body, err = collectors.GHARunLogs(path, id)
			}
			e := ""
			if err != nil {
				e = err.Error()
			}
			return ghaDetailMsg{gen: gen, body: body, err: e}
		}
	}
	a.ghaScreen = ghaScrLogs
	a.ghaDetailScroll = 0
	a.ghaStatus = "Fetching logs..."
	if failedOnly {
		a.ghaStatus = "Fetching failed logs..."
	}
	return func() tea.Msg {
		var body string
		var err error
		if failedOnly {
			body, err = collectors.GHARunFailedLogs(path, id)
		} else {
			body, err = collectors.GHARunLogs(path, id)
		}
		e := ""
		if err != nil {
			e = err.Error()
		}
		return ghaDetailMsg{gen: gen, body: body, err: e}
	}
}

func (a *App) ghaViewYAML() tea.Cmd {
	gen := a.ghaGen
	path := a.ghaPath
	var file string
	switch a.ghaKind {
	case ghaKindProcesses:
		if a.ghaCursor < len(a.ghaProcesses) {
			file = a.ghaProcesses[a.ghaCursor].File
		}
	case ghaKindWorkflows:
		if a.ghaCursor < len(a.ghaWorkflows) {
			file = a.ghaWorkflows[a.ghaCursor].Path
		}
	}
	if file == "" {
		a.ghaStatus = "yaml: selecione um processo"
		return nil
	}
	a.ghaFormName = file // title in modal
	a.ghaScreen = ghaScrForm
	a.ghaForm = ghaFormViewYAML
	a.ghaDetailScroll = 0
	a.ghaStatus = "abrindo " + filepath.Base(file)
	return func() tea.Msg {
		body, err := collectors.GHAReadProcessFile(path, file)
		e := ""
		if err != nil {
			e = err.Error()
		}
		return ghaDetailMsg{gen: gen, body: body, err: e}
	}
}

func (a *App) ghaResolveOwnerRepo() (owner, repo string) {
	if a.ghaInfo.Owner != "" && a.ghaInfo.Repo != "" {
		return a.ghaInfo.Owner, a.ghaInfo.Repo
	}
	if owner, repo, ok := collectors.ParseGitHubRepo(a.ghaRemote); ok {
		return owner, repo
	}
	return "", ""
}

func (a *App) ghaOpenBrowser() tea.Cmd {
	owner, repo := a.ghaResolveOwnerRepo()
	url := ""
	switch a.ghaKind {
	case ghaKindRuns:
		if r, ok := a.ghaSelectedRun(); ok {
			url = r.URL
			if url == "" {
				url = collectors.GHARunURL(owner, repo, r.ID)
			}
		}
	case ghaKindProcesses:
		file := ""
		if a.ghaCursor < len(a.ghaProcesses) {
			file = a.ghaProcesses[a.ghaCursor].File
		}
		url = collectors.GHAActionsURL(owner, repo, file)
	case ghaKindWorkflows:
		file := ""
		if a.ghaCursor < len(a.ghaWorkflows) {
			file = a.ghaWorkflows[a.ghaCursor].Path
		}
		url = collectors.GHAActionsURL(owner, repo, file)
	}
	if url == "" {
		url = collectors.GHAActionsURL(owner, repo, "")
	}
	if url == "" {
		a.ghaStatus = "sem URL — configure remote git@github.com:owner/repo.git"
		return nil
	}
	if err := collectors.GHAOpenRunURL(url); err != nil {
		a.ghaStatus = err.Error() + " · " + url
		return nil
	}
	a.ghaStatus = "browser · " + url
	return nil
}

func (a *App) ghaRerun() tea.Cmd {
	r, ok := a.ghaSelectedRun()
	if a.ghaKind != ghaKindRuns || !ok {
		a.ghaStatus = "rerun: selecione um run"
		return nil
	}
	id := r.ID
	gen := a.ghaGen
	path := a.ghaPath
	return func() tea.Msg {
		out, err := collectors.GHARerun(path, id)
		if err != nil {
			return ghaActionMsg{gen: gen, err: err.Error()}
		}
		return ghaActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "rerun "+id)}
	}
}

func ghaRunIsActive(r collectors.GHARun) bool {
	switch strings.ToLower(r.Status) {
	case "queued", "in_progress", "waiting", "requested", "pending", "waiting_for_progress":
		return true
	default:
		return false
	}
}

// ghaResolveStopTarget finds the active run to cancel for the current selection.
func (a *App) ghaResolveStopTarget() (runID, processName, label string) {
	switch {
	case a.ghaScreen == ghaScrProcess && a.ghaProcName != "":
		processName = a.ghaProcName
		runs := a.ghaRunsForProcess(a.ghaProcName, a.ghaProcFile)
		for _, r := range runs {
			if ghaRunIsActive(r) {
				return r.ID, processName, r.Workflow + " #" + r.ID
			}
		}
		live := a.ghaStatusForProcess(a.ghaProcName, a.ghaProcFile)
		if live.RunID != "" && (live.Label == "running" || live.Label == "queued" || live.Label == "triggered") {
			return live.RunID, processName, a.ghaProcName + " #" + live.RunID
		}
	case a.ghaKind == ghaKindRuns:
		r, ok := a.ghaSelectedRun()
		if !ok || !ghaRunIsActive(r) {
			return "", "", ""
		}
		return r.ID, sanitizeGHAName(r.Workflow), r.Workflow + " #" + r.ID
	case a.ghaKind == ghaKindProcesses && a.ghaCursor < len(a.ghaProcesses):
		p := a.ghaProcesses[a.ghaCursor]
		processName = p.Name
		for _, r := range a.ghaRunsForProcess(p.Name, p.File) {
			if ghaRunIsActive(r) {
				return r.ID, processName, r.Workflow + " #" + r.ID
			}
		}
		live := a.ghaStatusForProcess(p.Name, p.File)
		if live.RunID != "" && (live.Label == "running" || live.Label == "queued" || live.Label == "triggered") {
			return live.RunID, processName, p.Name + " #" + live.RunID
		}
	case a.ghaKind == ghaKindWorkflows && a.ghaCursor < len(a.ghaWorkflows):
		w := a.ghaWorkflows[a.ghaCursor]
		processName = sanitizeGHAName(w.Name)
		for _, r := range a.ghaRunsForProcess(processName, w.Path) {
			if ghaRunIsActive(r) {
				return r.ID, processName, r.Workflow + " #" + r.ID
			}
		}
	}
	return "", "", ""
}

func (a *App) ghaAskStopJob() tea.Cmd {
	if a.ghaNeedsSetup() {
		a.ghaShowSetup()
		a.ghaStatus = "parar job precisa do gh autenticado"
		return nil
	}
	id, proc, label := a.ghaResolveStopTarget()
	if id == "" {
		a.ghaStatus = "nada para parar — sem job ativo (queued/running)"
		return nil
	}
	a.ghaConfirm = true
	a.ghaConfirmAction = "stop-run:" + id
	if proc != "" {
		a.ghaConfirmAction += ":" + proc
	}
	a.ghaStatus = "parar " + label + "?"
	return nil
}

func (a *App) ghaAskStopAllJobs() tea.Cmd {
	if a.ghaNeedsSetup() {
		a.ghaShowSetup()
		return nil
	}
	name, file := "", ""
	if a.ghaScreen == ghaScrProcess {
		name, file = a.ghaProcName, a.ghaProcFile
	} else if a.ghaKind == ghaKindProcesses && a.ghaCursor < len(a.ghaProcesses) {
		name, file = a.ghaProcesses[a.ghaCursor].Name, a.ghaProcesses[a.ghaCursor].File
	} else {
		name, file = a.ghaSelectedProcessRef()
	}
	if name == "" {
		a.ghaStatus = "selecione um processo para parar todos"
		return nil
	}
	n := 0
	for _, r := range a.ghaRunsForProcess(name, file) {
		if ghaRunIsActive(r) {
			n++
		}
	}
	if n == 0 {
		a.ghaStatus = "nenhum job ativo em " + name
		return nil
	}
	a.ghaConfirm = true
	a.ghaConfirmAction = fmt.Sprintf("stop-all:%s:%d", name, n)
	a.ghaStatus = fmt.Sprintf("parar %d job(s) ativos de %s?", n, name)
	return nil
}

func (a *App) ghaCancelRun() tea.Cmd {
	return a.ghaAskStopJob()
}

func (a *App) handleGHAFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.ghaForm == ghaFormSetup {
		switch msg.String() {
		case "esc", "enter", "n", "N":
			a.ghaScreen = ghaScrCluster
			a.ghaForm = ghaFormNone
			a.ghaSetupShown = true
			return a, nil
		case "L":
			return a, a.ghaBeginLogin()
		case "o", "O":
			_ = collectors.GHAOpenRunURL("https://cli.github.com/")
			a.ghaStatus = "docs · https://cli.github.com/"
			return a, nil
		case "r":
			return a, a.refreshGHA()
		}
		return a, nil
	}
	if a.ghaForm == ghaFormTrigger {
		return a.handleGHATriggerFormKeys(msg)
	}
	if a.ghaForm == ghaFormViewYAML {
		switch msg.String() {
		case "esc":
			a.ghaScreen = ghaScrCluster
			a.ghaForm = ghaFormNone
			a.ghaFormName = ""
		case "o", "O":
			return a, a.ghaOpenBrowser()
		case "up", "k":
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-1)
		case "down", "j":
			a.ghaDetailScroll++
		case "pgup":
			a.ghaDetailScroll = maxInt(0, a.ghaDetailScroll-10)
		case "pgdown":
			a.ghaDetailScroll += 10
		}
		return a, nil
	}

	// create form: fields name, desc, template
	fields := []*string{&a.ghaFormName, &a.ghaFormDesc, &a.ghaFormTemplate}
	switch msg.String() {
	case "esc":
		a.ghaScreen = ghaScrCluster
		a.ghaForm = ghaFormNone
		return a, nil
	case "tab", "down", "j":
		a.syncGHAFormField(fields)
		a.ghaFormField = (a.ghaFormField + 1) % len(fields)
		a.ghaFormInput = *fields[a.ghaFormField]
	case "shift+tab", "up", "k":
		a.syncGHAFormField(fields)
		a.ghaFormField = (a.ghaFormField + len(fields) - 1) % len(fields)
		a.ghaFormInput = *fields[a.ghaFormField]
	case "backspace":
		if len(a.ghaFormInput) > 0 {
			a.ghaFormInput = a.ghaFormInput[:len(a.ghaFormInput)-1]
		}
	case "[", "]":
		if a.ghaFormField == 2 {
			a.ghaFormTemplate = ghaCycleTemplate(a.ghaFormTemplate, msg.String() == "]")
			a.ghaFormInput = a.ghaFormTemplate
		}
	case "enter", "y":
		a.syncGHAFormField(fields)
		name := strings.TrimSpace(a.ghaFormName)
		if name == "" {
			a.ghaStatus = "nome obrigatório"
			return a, nil
		}
		desc := strings.TrimSpace(a.ghaFormDesc)
		tpl := strings.TrimSpace(a.ghaFormTemplate)
		path := a.ghaPath
		gen := a.ghaGen
		a.ghaScreen = ghaScrCluster
		a.ghaForm = ghaFormNone
		a.ghaStatus = "Creating process..."
		return a, func() tea.Msg {
			proc, err := collectors.GHACreateProcess(path, name, desc, tpl)
			if err != nil {
				return ghaActionMsg{gen: gen, err: err.Error()}
			}
			return ghaActionMsg{gen: gen, out: "created " + proc.File}
		}
	default:
		if len(msg.Runes) == 1 {
			a.ghaFormInput += string(msg.Runes)
		}
	}
	return a, nil
}

func (a *App) handleGHATriggerFormKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(a.ghaTriggerBranches)
	inInputs := a.ghaTriggerInputIdx >= 0 && len(a.ghaTriggerInputs) > 0
	switch msg.String() {
	case "esc":
		if a.ghaTriggerForce || (a.ghaTriggerAhead > 0 && strings.Contains(a.ghaStatus, "PUSH NEEDED")) {
			a.ghaTriggerForce = false
			a.ghaStatus = "trigger · escolha branch"
			return a, nil
		}
		a.ghaCloseTriggerForm()
		a.ghaStatus = "trigger cancelado"
		return a, nil
	case "tab":
		if len(a.ghaTriggerInputs) == 0 {
			return a, nil
		}
		if a.ghaTriggerInputIdx < 0 {
			a.ghaTriggerInputIdx = 0
		} else {
			a.ghaTriggerInputIdx = (a.ghaTriggerInputIdx + 1) % len(a.ghaTriggerInputs)
		}
		return a, nil
	case "shift+tab":
		if len(a.ghaTriggerInputs) == 0 {
			return a, nil
		}
		if a.ghaTriggerInputIdx <= 0 {
			a.ghaTriggerInputIdx = -1
		} else {
			a.ghaTriggerInputIdx--
		}
		return a, nil
	case "up", "k":
		if inInputs {
			if a.ghaTriggerInputIdx > 0 {
				a.ghaTriggerInputIdx--
			} else {
				a.ghaTriggerInputIdx = -1
			}
			return a, nil
		}
		if a.ghaTriggerCursor > 0 {
			a.ghaTriggerCursor--
			a.ghaRefreshTriggerAhead()
			a.ghaTriggerForce = false
		}
	case "down", "j":
		if inInputs {
			if a.ghaTriggerInputIdx < len(a.ghaTriggerInputs)-1 {
				a.ghaTriggerInputIdx++
			}
			return a, nil
		}
		if a.ghaTriggerCursor < n-1 {
			a.ghaTriggerCursor++
			a.ghaRefreshTriggerAhead()
			a.ghaTriggerForce = false
		}
	case "pgup":
		a.ghaTriggerCursor = maxInt(0, a.ghaTriggerCursor-8)
		a.ghaRefreshTriggerAhead()
	case "pgdown":
		if n > 0 {
			a.ghaTriggerCursor = minInt(n-1, a.ghaTriggerCursor+8)
			a.ghaRefreshTriggerAhead()
		}
	case "[":
		if inInputs {
			in := a.ghaTriggerInputs[a.ghaTriggerInputIdx]
			if len(in.Options) > 0 {
				a.ghaCycleTriggerChoice(-1)
			} else if in.Type == "boolean" {
				a.ghaTriggerInputVals[a.ghaTriggerInputIdx] = "false"
			}
		}
	case "]":
		if inInputs {
			in := a.ghaTriggerInputs[a.ghaTriggerInputIdx]
			if len(in.Options) > 0 {
				a.ghaCycleTriggerChoice(1)
			} else if in.Type == "boolean" {
				a.ghaTriggerInputVals[a.ghaTriggerInputIdx] = "true"
			}
		}
	case "backspace":
		if inInputs {
			v := a.ghaTriggerInputVals[a.ghaTriggerInputIdx]
			if len(v) > 0 {
				a.ghaTriggerInputVals[a.ghaTriggerInputIdx] = v[:len(v)-1]
			}
		}
	case "P":
		return a, a.ghaPushThenTrigger()
	case "r":
		a.ghaTriggerBranches = collectors.GitRemoteBranchNames(a.ghaPath)
		if a.ghaTriggerCursor >= len(a.ghaTriggerBranches) {
			a.ghaTriggerCursor = maxInt(0, len(a.ghaTriggerBranches)-1)
		}
		a.ghaRefreshTriggerAhead()
		a.ghaStatus = fmt.Sprintf("%d branches no origin", len(a.ghaTriggerBranches))
	case "y", "Y":
		if a.ghaTriggerAhead > 0 {
			a.ghaTriggerForce = true
		}
		return a, a.ghaConfirmTrigger()
	case "enter", "t":
		return a, a.ghaConfirmTrigger()
	default:
		if inInputs && len(msg.Runes) == 1 {
			a.ghaTriggerInputVals[a.ghaTriggerInputIdx] += string(msg.Runes)
		}
	}
	return a, nil
}

func (a *App) ghaCycleTriggerChoice(dir int) {
	i := a.ghaTriggerInputIdx
	if i < 0 || i >= len(a.ghaTriggerInputs) {
		return
	}
	opts := a.ghaTriggerInputs[i].Options
	if len(opts) == 0 {
		return
	}
	cur := a.ghaTriggerInputVals[i]
	idx := 0
	for j, o := range opts {
		if o == cur {
			idx = j
			break
		}
	}
	idx = (idx + dir + len(opts)*8) % len(opts)
	a.ghaTriggerInputVals[i] = opts[idx]
}

func (a *App) syncGHAFormField(fields []*string) {
	if a.ghaFormField >= 0 && a.ghaFormField < len(fields) {
		*fields[a.ghaFormField] = a.ghaFormInput
	}
}

func ghaCycleTemplate(cur string, next bool) string {
	opts := []string{"ci", "deploy", "manual"}
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	if next {
		idx = (idx + 1) % len(opts)
	} else {
		idx = (idx + len(opts) - 1) % len(opts)
	}
	return opts[idx]
}

func (a *App) ghaRunConfirm(action string) tea.Cmd {
	gen := a.ghaGen
	path := a.ghaPath
	switch {
	case strings.HasPrefix(action, "rm-process:"):
		name := strings.TrimPrefix(action, "rm-process:")
		return func() tea.Msg {
			err := collectors.GHADeleteProcess(path, name)
			if err != nil {
				return ghaActionMsg{gen: gen, err: err.Error()}
			}
			return ghaActionMsg{gen: gen, out: "deleted process " + name}
		}
	case strings.HasPrefix(action, "cancel-run:"), strings.HasPrefix(action, "stop-run:"):
		rest := strings.TrimPrefix(action, "cancel-run:")
		rest = strings.TrimPrefix(rest, "stop-run:")
		id := rest
		proc := ""
		if i := strings.Index(rest, ":"); i >= 0 {
			id = rest[:i]
			proc = rest[i+1:]
		}
		return func() tea.Msg {
			out, err := collectors.GHACancelRun(path, id)
			if err != nil {
				return ghaActionMsg{gen: gen, err: err.Error()}
			}
			return ghaActionMsg{gen: gen, out: "stopped " + id + " · " + proc + " · " + firstNonEmpty(firstLine(out), "ok")}
		}
	case strings.HasPrefix(action, "stop-all:"):
		// stop-all:name:count
		parts := strings.SplitN(strings.TrimPrefix(action, "stop-all:"), ":", 2)
		name := parts[0]
		file := ""
		for _, p := range a.ghaProcesses {
			if p.Name == name {
				file = p.File
				break
			}
		}
		var ids []string
		for _, r := range a.ghaRunsForProcess(name, file) {
			if ghaRunIsActive(r) {
				ids = append(ids, r.ID)
			}
		}
		return func() tea.Msg {
			if len(ids) == 0 {
				return ghaActionMsg{gen: gen, err: "nenhum job ativo"}
			}
			ok, fail := 0, 0
			var last string
			for _, id := range ids {
				out, err := collectors.GHACancelRun(path, id)
				if err != nil {
					fail++
					last = err.Error()
					continue
				}
				ok++
				last = firstLine(out)
			}
			if ok == 0 {
				return ghaActionMsg{gen: gen, err: last}
			}
			msg := fmt.Sprintf("stopped %d/%d jobs de %s", ok, len(ids), name)
			if fail > 0 {
				msg += fmt.Sprintf(" (%d falharam)", fail)
			}
			return ghaActionMsg{gen: gen, out: msg}
		}
	case strings.HasPrefix(action, "stop-marked:"):
		ids := strings.Split(strings.TrimPrefix(action, "stop-marked:"), ",")
		return func() tea.Msg {
			ok, fail := 0, 0
			var last string
			for _, id := range ids {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				out, err := collectors.GHACancelRun(path, id)
				if err != nil {
					fail++
					last = err.Error()
					continue
				}
				ok++
				last = firstLine(out)
			}
			if ok == 0 {
				return ghaActionMsg{gen: gen, err: firstNonEmpty(last, "nenhum cancelado")}
			}
			msg := fmt.Sprintf("bulk-stopped %d/%d", ok, ok+fail)
			if fail > 0 {
				msg += fmt.Sprintf(" (%d falharam)", fail)
			}
			return ghaActionMsg{gen: gen, out: msg}
		}
	}
	return nil
}
