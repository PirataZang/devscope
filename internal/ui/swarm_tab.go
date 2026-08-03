package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type swarmKind int

const (
	swarmKindServices swarmKind = iota
	swarmKindNodes
	swarmKindTasks
	swarmKindStacks
	swarmKindNetworks
	swarmKindSecrets
	swarmKindConfigs
	swarmKindEvents
	swarmKindCount
)

func (k swarmKind) String() string {
	switch k {
	case swarmKindServices:
		return "Services"
	case swarmKindNodes:
		return "Nodes"
	case swarmKindTasks:
		return "Tasks"
	case swarmKindStacks:
		return "Stacks"
	case swarmKindNetworks:
		return "Networks"
	case swarmKindSecrets:
		return "Secrets"
	case swarmKindConfigs:
		return "Configs"
	case swarmKindEvents:
		return "Events"
	default:
		return "Services"
	}
}

type swarmScreen int

const (
	swarmScrCluster swarmScreen = iota
	swarmScrDetail
	swarmScrLogs
	swarmScrForm
)

type swarmFormKind int

const (
	swarmFormNone swarmFormKind = iota
	swarmFormScale
	swarmFormCreate
	swarmFormDeploy
	swarmFormInit
	swarmFormToken
	swarmFormUpdate
	swarmFormAvail
)

type swarmNavFrame struct {
	kind   swarmKind
	cursor int
	scroll int
	name   string
}

type swarmLoadedMsg struct {
	gen      int
	info     collectors.SwarmInfo
	nodes    []collectors.SwarmNode
	services []collectors.SwarmService
	stacks   []collectors.SwarmStack
	tasks    []collectors.SwarmTask
	networks []collectors.SwarmNetwork
	secrets  []collectors.SwarmSecret
	configs  []collectors.SwarmConfig
	events   []collectors.SwarmEvent
	err      string
}

type swarmActionMsg struct {
	gen int
	out string
	err string
}

type swarmDetailMsg struct {
	gen  int
	name string
	body string
	err  string
}

type swarmTickMsg struct {
	gen int
}

func (a *App) enterSwarmTab(_ *core.Project) {
	a.tab = TabSwarm
	a.tabCursor = 0
	a.swarmOpen = false
	a.resetSwarmTransient()
}

func (a *App) resetSwarmTransient() {
	a.swarmConfirm = false
	a.swarmConfirmAction = ""
	a.swarmScreen = swarmScrCluster
	a.swarmForm = swarmFormNone
	a.swarmFormField = 0
	a.swarmFormInput = ""
	a.swarmFormName = ""
	a.swarmFormImage = ""
	a.swarmFormReplicas = "1"
	a.swarmFormPort = ""
	a.swarmFormNetwork = ""
	a.swarmFormAvail = "active"
	a.swarmNav = nil
	a.swarmDetail = ""
	a.swarmLogs = ""
	a.swarmStatus = ""
	a.swarmErr = ""
	a.swarmActionIdx = 0
	a.swarmFocus = 0
}

func (a *App) openSwarmClient(p *core.Project) tea.Cmd {
	a.swarmOpen = true
	a.resetSwarmTransient()
	a.swarmKind = swarmKindServices
	a.swarmCursor = 0
	a.swarmScroll = 0
	a.swarmDetailScroll = 0
	a.swarmCompose = ""
	a.swarmProject = ""
	if p != nil {
		a.swarmCompose = collectors.DiscoverSwarmCompose(p.Path)
		a.swarmProject = p.Name
	}
	a.swarmGen++
	return a.refreshSwarm()
}

func (a *App) leaveSwarmTab() tea.Cmd {
	a.swarmOpen = false
	a.swarmGen++
	a.resetSwarmTransient()
	a.tab = TabSwarm
	a.tabCursor = 0
	return nil
}

func (a *App) scheduleSwarmTick() tea.Cmd {
	gen := a.swarmGen
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return swarmTickMsg{gen: gen}
	})
}

func (a *App) refreshSwarm() tea.Cmd {
	a.swarmLoading = true
	gen := a.swarmGen
	return func() tea.Msg {
		info := collectors.SwarmClusterInfo()
		msg := swarmLoadedMsg{gen: gen, info: info}
		if info.Error != "" && !info.Active {
			msg.err = info.Error
			return msg
		}
		if !info.Active {
			return msg
		}
		var err error
		if msg.nodes, err = collectors.SwarmListNodes(); err != nil {
			msg.err = err.Error()
			return msg
		}
		if msg.services, err = collectors.SwarmListServices(); err != nil {
			msg.err = err.Error()
			return msg
		}
		msg.stacks, _ = collectors.SwarmListStacks()
		msg.tasks, _ = collectors.SwarmListTasks()
		msg.networks, _ = collectors.SwarmListNetworks()
		msg.secrets, _ = collectors.SwarmListSecrets()
		msg.configs, _ = collectors.SwarmListConfigs()
		msg.events, _ = collectors.SwarmRecentEvents("15m")
		info.Services = len(msg.services)
		info.Tasks = len(msg.tasks)
		info.Networks = len(msg.networks)
		managers := 0
		for _, n := range msg.nodes {
			if n.Role == "manager" {
				managers++
			}
		}
		if managers > 0 {
			info.Managers = managers
			info.Workers = len(msg.nodes) - managers
			info.Nodes = len(msg.nodes)
		}
		// degraded if any node not Ready
		if info.Active {
			down := 0
			for _, n := range msg.nodes {
				if !strings.EqualFold(n.Status, "Ready") {
					down++
				}
			}
			if down > 0 {
				info.State = "degraded"
			}
		}
		msg.info = info
		return msg
	}
}

func (a *App) handleSwarmMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case swarmLoadedMsg:
		if m.gen != a.swarmGen {
			return a, nil
		}
		a.swarmLoading = false
		a.swarmInfo = m.info
		a.swarmNodes = m.nodes
		a.swarmServices = m.services
		a.swarmStacks = m.stacks
		a.swarmTasks = m.tasks
		a.swarmNetworks = m.networks
		a.swarmSecrets = m.secrets
		a.swarmConfigs = m.configs
		a.swarmEvents = m.events
		if m.err != "" {
			a.swarmErr = m.err
		} else {
			a.swarmErr = ""
		}
		a.clampSwarmCursor()
		var cmds []tea.Cmd
		if a.swarmScreen == swarmScrCluster && a.swarmDetail == "" {
			cmds = append(cmds, a.swarmInspectSelected())
		}
		if a.swarmOpen {
			cmds = append(cmds, a.scheduleSwarmTick())
		}
		return a, tea.Batch(cmds...)
	case swarmActionMsg:
		if m.gen != a.swarmGen {
			return a, nil
		}
		a.swarmLoading = false
		if m.err != "" {
			a.swarmStatus = "✗ " + m.err
			a.swarmErr = m.err
		} else {
			a.swarmStatus = "✓ " + firstNonEmpty(m.out, "ok")
			a.swarmErr = ""
		}
		a.swarmScreen = swarmScrCluster
		a.swarmForm = swarmFormNone
		return a, a.refreshSwarm()
	case swarmDetailMsg:
		if m.gen != a.swarmGen {
			return a, nil
		}
		if m.err != "" {
			a.swarmDetail = m.err
		} else {
			a.swarmDetail = m.body
		}
		a.swarmDetailScroll = 0
	case swarmTickMsg:
		if m.gen != a.swarmGen || !a.swarmOpen || a.swarmConfirm {
			return a, nil
		}
		if a.swarmScreen == swarmScrForm || a.swarmScreen == swarmScrLogs {
			return a, a.scheduleSwarmTick()
		}
		return a, a.refreshSwarm()
	}
	return a, nil
}

func (a *App) clampSwarmCursor() {
	n := a.swarmRowCount()
	if n == 0 {
		a.swarmCursor = 0
		return
	}
	a.swarmCursor = clampCursor(a.swarmCursor, n)
}

func (a *App) swarmRowCount() int {
	switch a.swarmKind {
	case swarmKindNodes:
		return len(a.swarmNodes)
	case swarmKindServices:
		return len(a.swarmServices)
	case swarmKindTasks:
		return len(a.swarmTasks)
	case swarmKindStacks:
		return len(a.swarmStacks)
	case swarmKindNetworks:
		return len(a.swarmNetworks)
	case swarmKindSecrets:
		return len(a.swarmSecrets)
	case swarmKindConfigs:
		return len(a.swarmConfigs)
	case swarmKindEvents:
		return len(a.swarmEvents)
	}
	return 0
}

func (a *App) swarmSelectedName() string {
	switch a.swarmKind {
	case swarmKindNodes:
		if a.swarmCursor < len(a.swarmNodes) {
			return a.swarmNodes[a.swarmCursor].Hostname
		}
	case swarmKindServices:
		if a.swarmCursor < len(a.swarmServices) {
			return a.swarmServices[a.swarmCursor].Name
		}
	case swarmKindTasks:
		if a.swarmCursor < len(a.swarmTasks) {
			return a.swarmTasks[a.swarmCursor].Name
		}
	case swarmKindStacks:
		if a.swarmCursor < len(a.swarmStacks) {
			return a.swarmStacks[a.swarmCursor].Name
		}
	case swarmKindNetworks:
		if a.swarmCursor < len(a.swarmNetworks) {
			return a.swarmNetworks[a.swarmCursor].Name
		}
	case swarmKindSecrets:
		if a.swarmCursor < len(a.swarmSecrets) {
			return a.swarmSecrets[a.swarmCursor].Name
		}
	case swarmKindConfigs:
		if a.swarmCursor < len(a.swarmConfigs) {
			return a.swarmConfigs[a.swarmCursor].Name
		}
	}
	return ""
}

func (a *App) swarmPushNav() {
	a.swarmNav = append(a.swarmNav, swarmNavFrame{
		kind:   a.swarmKind,
		cursor: a.swarmCursor,
		scroll: a.swarmScroll,
		name:   a.swarmSelectedName(),
	})
}

func (a *App) swarmPopNav() bool {
	if len(a.swarmNav) == 0 {
		return false
	}
	f := a.swarmNav[len(a.swarmNav)-1]
	a.swarmNav = a.swarmNav[:len(a.swarmNav)-1]
	a.swarmKind = f.kind
	a.swarmCursor = f.cursor
	a.swarmScroll = f.scroll
	return true
}

func (a *App) handleSwarmKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.swarmConfirm {
		switch msg.String() {
		case "y", "Y":
			action := a.swarmConfirmAction
			a.swarmConfirm = false
			a.swarmConfirmAction = ""
			return a, a.swarmRunConfirm(action, p)
		case "n", "N", "esc":
			a.swarmConfirm = false
			a.swarmConfirmAction = ""
			a.swarmStatus = "cancelado"
			return a, nil
		}
		return a, nil
	}

	if a.swarmScreen == swarmScrForm {
		return a.handleSwarmFormKeys(msg, p)
	}

	if a.swarmScreen == swarmScrLogs {
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmLogs = ""
			return a, nil
		case "r", "f":
			return a, a.swarmShowLogs()
		case "c":
			a.swarmLogs = ""
			return a, nil
		case "up", "k":
			a.swarmDetailScroll = maxInt(0, a.swarmDetailScroll-1)
		case "down", "j":
			a.swarmDetailScroll++
		case "pgup":
			a.swarmDetailScroll = maxInt(0, a.swarmDetailScroll-10)
		case "pgdown":
			a.swarmDetailScroll += 10
		}
		return a, nil
	}

	if a.swarmScreen == swarmScrDetail {
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmDetail = ""
			if a.swarmPopNav() {
				return a, a.swarmInspectSelected()
			}
			return a, nil
		case "l":
			return a, a.swarmShowLogs()
		case "s":
			if a.swarmKind == swarmKindServices {
				return a, a.swarmBeginScale()
			}
		case "u":
			if a.swarmKind == swarmKindServices {
				return a, a.swarmBeginUpdate()
			}
		case "R":
			if a.swarmKind == swarmKindServices {
				return a, a.swarmForceUpdate()
			}
		case "b":
			if a.swarmKind == swarmKindServices {
				return a, a.swarmRollback()
			}
		case "up", "k":
			a.swarmDetailScroll = maxInt(0, a.swarmDetailScroll-1)
		case "down", "j":
			a.swarmDetailScroll++
		case "pgup":
			a.swarmDetailScroll = maxInt(0, a.swarmDetailScroll-10)
		case "pgdown":
			a.swarmDetailScroll += 10
		case "r":
			return a, a.refreshSwarm()
		}
		return a, nil
	}

	// Cluster dashboard
	switch msg.String() {
	case "esc":
		return a, a.leaveSwarmTab()
	case "tab":
		a.swarmFocus = (a.swarmFocus + 1) % 3
		return a, nil
	case "shift+tab":
		a.swarmFocus = (a.swarmFocus + 2) % 3
		return a, nil
	case "[", "left":
		a.swarmKind = swarmKind((int(a.swarmKind) + int(swarmKindCount) - 1) % int(swarmKindCount))
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		a.swarmDetail = ""
		return a, a.swarmInspectSelected()
	case "]", "right":
		a.swarmKind = swarmKind((int(a.swarmKind) + 1) % int(swarmKindCount))
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		a.swarmDetail = ""
		return a, a.swarmInspectSelected()
	case "1":
		a.swarmKind = swarmKindServices
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "2":
		a.swarmKind = swarmKindNodes
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "3":
		a.swarmKind = swarmKindTasks
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "4":
		a.swarmKind = swarmKindStacks
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "5":
		a.swarmKind = swarmKindNetworks
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "6":
		a.swarmKind = swarmKindSecrets
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "7":
		a.swarmKind = swarmKindConfigs
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, a.swarmInspectSelected()
	case "8", "e":
		a.swarmKind = swarmKindEvents
		a.swarmCursor, a.swarmScroll, a.swarmActionIdx = 0, 0, 0
		return a, nil
	case "up", "k":
		if a.swarmFocus == 2 {
			if a.swarmActionIdx > 0 {
				a.swarmActionIdx--
			}
			return a, nil
		}
		if a.swarmCursor > 0 {
			a.swarmCursor--
			return a, a.swarmInspectSelected()
		}
	case "down", "j":
		if a.swarmFocus == 2 {
			if a.swarmActionIdx < len(a.swarmQuickActionItems())-1 {
				a.swarmActionIdx++
			}
			return a, nil
		}
		if a.swarmCursor < a.swarmRowCount()-1 {
			a.swarmCursor++
			return a, a.swarmInspectSelected()
		}
	case "pgup":
		a.swarmDetailScroll = maxInt(0, a.swarmDetailScroll-8)
	case "pgdown":
		a.swarmDetailScroll += 8
	case "enter":
		if a.swarmFocus == 2 {
			return a, a.swarmRunQuickAction(p)
		}
		return a, a.swarmOpenDetail()
	case "l":
		return a, a.swarmShowLogs()
	case "r":
		return a, a.refreshSwarm()
	case "+":
		return a, a.swarmScale(1)
	case "-":
		return a, a.swarmScale(-1)
	case "s":
		return a, a.swarmBeginScale()
	case "u":
		return a, a.swarmBeginUpdate()
	case "c":
		return a, a.swarmBeginCreate()
	case "d":
		return a, a.swarmBeginDeploy(p)
	case "D", "delete":
		return a, a.swarmAskRemove()
	case "x", "X":
		a.swarmConfirm = true
		a.swarmConfirmAction = "leave"
		return a, nil
	case "i":
		return a, a.swarmBeginInit()
	case "t":
		return a, a.swarmBeginToken(false)
	case "T", "J":
		return a, a.swarmBeginToken(true)
	case "p":
		return a, a.swarmPromote()
	case "m":
		return a, a.swarmDemote()
	case "a":
		return a, a.swarmBeginAvail()
	case "R":
		return a, a.swarmForceUpdate()
	case "b":
		return a, a.swarmRollback()
	case "P":
		a.swarmConfirm = true
		a.swarmConfirmAction = "prune"
		return a, nil
	}
	return a, nil
}

func (a *App) swarmQuickActionItems() [][2]string {
	switch a.swarmKind {
	case swarmKindNodes:
		return [][2]string{
			{"a", "Availability"},
			{"p", "Promote"},
			{"m", "Demote"},
			{"D", "Remover node"},
			{"X", "Leave swarm"},
			{"t", "Join Token"},
			{"r", "Atualizar"},
		}
	default:
		return [][2]string{
			{"c", "Criar Service"},
			{"d", "Deploy Stack"},
			{"s", "Scale Service"},
			{"u", "Update Service"},
			{"l", "Logs"},
			{"D", "Remover"},
			{"t", "Join Token"},
			{"X", "Leave swarm"},
			{"P", "Prune networks"},
			{"r", "Atualizar"},
		}
	}
}

func (a *App) swarmRunQuickAction(p *core.Project) tea.Cmd {
	items := a.swarmQuickActionItems()
	if a.swarmActionIdx < 0 || a.swarmActionIdx >= len(items) {
		return nil
	}
	key := items[a.swarmActionIdx][0]
	switch key {
	case "c":
		return a.swarmBeginCreate()
	case "d":
		return a.swarmBeginDeploy(p)
	case "s":
		return a.swarmBeginScale()
	case "u":
		return a.swarmBeginUpdate()
	case "l":
		return a.swarmShowLogs()
	case "t":
		return a.swarmBeginToken(false)
	case "i":
		return a.swarmBeginInit()
	case "a":
		return a.swarmBeginAvail()
	case "p":
		return a.swarmPromote()
	case "m":
		return a.swarmDemote()
	case "D":
		return a.swarmAskRemove()
	case "X":
		a.swarmConfirm = true
		a.swarmConfirmAction = "leave"
		return nil
	case "P":
		a.swarmConfirm = true
		a.swarmConfirmAction = "prune"
		return nil
	case "r":
		return a.refreshSwarm()
	}
	return nil
}

func (a *App) swarmOpenDetail() tea.Cmd {
	if a.swarmKind == swarmKindEvents {
		return nil
	}
	name := a.swarmSelectedName()
	if name == "" {
		return nil
	}
	a.swarmPushNav()
	a.swarmScreen = swarmScrDetail
	a.swarmDetailScroll = 0
	return a.swarmInspectSelected()
}

func (a *App) swarmInspectSelected() tea.Cmd {
	gen := a.swarmGen
	switch a.swarmKind {
	case swarmKindNodes:
		if a.swarmCursor >= len(a.swarmNodes) {
			a.swarmDetail = ""
			return nil
		}
		n := a.swarmNodes[a.swarmCursor]
		return func() tea.Msg {
			body, err := collectors.SwarmInspectNode(n.ID)
			if err != nil {
				body = fmt.Sprintf("Node %s\nHostname  %s\nRole      %s\nStatus    %s\nAvail     %s\nManager   %s\nEngine    %s\nID        %s",
					n.Hostname, n.Hostname, n.Role, n.Status, n.Availability, firstNonEmpty(n.Manager, "-"), n.Engine, n.ID)
				return swarmDetailMsg{gen: gen, name: n.Hostname, body: body}
			}
			return swarmDetailMsg{gen: gen, name: n.Hostname, body: body}
		}
	case swarmKindServices:
		if a.swarmCursor >= len(a.swarmServices) {
			a.swarmDetail = ""
			return nil
		}
		name := a.swarmServices[a.swarmCursor].Name
		return func() tea.Msg {
			pretty, err1 := collectors.SwarmInspectService(name)
			tasks, _ := collectors.SwarmServiceTasks(name)
			body := pretty
			if tasks != "" {
				body += "\n\n── TASKS ──\n" + tasks
			}
			err := ""
			if err1 != nil {
				err = err1.Error()
			}
			return swarmDetailMsg{gen: gen, name: name, body: body, err: err}
		}
	case swarmKindTasks:
		if a.swarmCursor >= len(a.swarmTasks) {
			a.swarmDetail = ""
			return nil
		}
		t := a.swarmTasks[a.swarmCursor]
		a.swarmDetail = fmt.Sprintf("TASK %s\nID       %s\nService  %s\nNode     %s\nDesired  %s\nCurrent  %s\nImage    %s\nError    %s",
			t.Name, t.ID, t.Service, t.Node, t.DesiredState, t.CurrentState, t.Image, firstNonEmpty(t.Error, "-"))
		return nil
	case swarmKindStacks:
		if a.swarmCursor >= len(a.swarmStacks) {
			a.swarmDetail = ""
			return nil
		}
		name := a.swarmStacks[a.swarmCursor].Name
		return func() tea.Msg {
			body, err := collectors.SwarmStackServices(name)
			e := ""
			if err != nil {
				e = err.Error()
			}
			return swarmDetailMsg{gen: gen, name: name, body: body, err: e}
		}
	case swarmKindNetworks:
		if a.swarmCursor >= len(a.swarmNetworks) {
			a.swarmDetail = ""
			return nil
		}
		name := a.swarmNetworks[a.swarmCursor].Name
		return func() tea.Msg {
			body, err := collectors.SwarmInspectNetwork(name)
			e := ""
			if err != nil {
				e = err.Error()
			}
			return swarmDetailMsg{gen: gen, name: name, body: body, err: e}
		}
	case swarmKindSecrets:
		if a.swarmCursor >= len(a.swarmSecrets) {
			a.swarmDetail = ""
			return nil
		}
		name := a.swarmSecrets[a.swarmCursor].Name
		return func() tea.Msg {
			body, err := collectors.SwarmInspectSecret(name)
			e := ""
			if err != nil {
				e = err.Error()
			}
			return swarmDetailMsg{gen: gen, name: name, body: body, err: e}
		}
	case swarmKindConfigs:
		if a.swarmCursor >= len(a.swarmConfigs) {
			a.swarmDetail = ""
			return nil
		}
		name := a.swarmConfigs[a.swarmCursor].Name
		return func() tea.Msg {
			body, err := collectors.SwarmInspectConfig(name)
			e := ""
			if err != nil {
				e = err.Error()
			}
			return swarmDetailMsg{gen: gen, name: name, body: body, err: e}
		}
	}
	return nil
}

func (a *App) swarmShowLogs() tea.Cmd {
	name := ""
	if a.swarmKind == swarmKindServices && a.swarmCursor < len(a.swarmServices) {
		name = a.swarmServices[a.swarmCursor].Name
	} else if a.swarmKind == swarmKindTasks && a.swarmCursor < len(a.swarmTasks) {
		name = a.swarmTasks[a.swarmCursor].Service
	}
	if name == "" {
		a.swarmStatus = "logs: selecione um service"
		return nil
	}
	gen := a.swarmGen
	a.swarmScreen = swarmScrLogs
	a.swarmDetailScroll = 0
	a.swarmStatus = "Fetching logs..."
	return func() tea.Msg {
		body, err := collectors.SwarmServiceLogs(name, 120)
		e := ""
		if err != nil {
			e = err.Error()
		}
		return swarmDetailMsg{gen: gen, name: name, body: body, err: e}
	}
}

func (a *App) swarmScale(delta int) tea.Cmd {
	if a.swarmKind != swarmKindServices || a.swarmCursor >= len(a.swarmServices) {
		a.swarmStatus = "scale: selecione um service"
		return nil
	}
	svc := a.swarmServices[a.swarmCursor]
	_, desired := collectors.ParseServiceReplicas(svc.Replicas)
	next := desired + delta
	if next < 0 {
		next = 0
	}
	name := svc.Name
	gen := a.swarmGen
	a.swarmStatus = fmt.Sprintf("Scaling %s → %d...", name, next)
	return func() tea.Msg {
		err := collectors.SwarmServiceScale(name, next)
		if err != nil {
			return swarmActionMsg{gen: gen, err: err.Error()}
		}
		return swarmActionMsg{gen: gen, out: fmt.Sprintf("scaled %s → %d", name, next)}
	}
}

func (a *App) swarmBeginScale() tea.Cmd {
	if a.swarmKind != swarmKindServices || a.swarmCursor >= len(a.swarmServices) {
		a.swarmStatus = "scale: selecione um service"
		return nil
	}
	svc := a.swarmServices[a.swarmCursor]
	_, desired := collectors.ParseServiceReplicas(svc.Replicas)
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormScale
	a.swarmFormName = svc.Name
	a.swarmFormReplicas = strconv.Itoa(desired)
	a.swarmFormInput = a.swarmFormReplicas
	a.swarmFormField = 0
	return nil
}

func (a *App) swarmBeginUpdate() tea.Cmd {
	if a.swarmKind != swarmKindServices || a.swarmCursor >= len(a.swarmServices) {
		a.swarmStatus = "update: selecione um service"
		return nil
	}
	svc := a.swarmServices[a.swarmCursor]
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormUpdate
	a.swarmFormName = svc.Name
	a.swarmFormImage = svc.Image
	_, d := collectors.ParseServiceReplicas(svc.Replicas)
	a.swarmFormReplicas = strconv.Itoa(d)
	a.swarmFormField = 0
	a.swarmFormInput = a.swarmFormImage
	return nil
}

func (a *App) swarmBeginCreate() tea.Cmd {
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormCreate
	a.swarmFormName = ""
	a.swarmFormImage = "nginx:latest"
	a.swarmFormReplicas = "1"
	a.swarmFormPort = ""
	a.swarmFormNetwork = ""
	a.swarmFormField = 0
	a.swarmFormInput = ""
	return nil
}

func (a *App) swarmBeginDeploy(p *core.Project) tea.Cmd {
	compose := a.swarmCompose
	if compose == "" && p != nil {
		compose = collectors.DiscoverSwarmCompose(p.Path)
		a.swarmCompose = compose
	}
	if compose == "" {
		a.swarmStatus = "nenhum compose encontrado no projeto"
		return nil
	}
	name := sanitizeSwarmStackName(a.swarmProject)
	if p != nil && name == "stack" {
		name = sanitizeSwarmStackName(p.Name)
	}
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormDeploy
	a.swarmFormName = name
	a.swarmFormInput = compose
	a.swarmFormField = 0
	return nil
}

func (a *App) swarmBeginInit() tea.Cmd {
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormInit
	a.swarmFormInput = ""
	a.swarmFormField = 0
	return nil
}

func (a *App) swarmBeginToken(manager bool) tea.Cmd {
	gen := a.swarmGen
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormToken
	if manager {
		a.swarmFormName = "manager"
	} else {
		a.swarmFormName = "worker"
	}
	a.swarmDetail = "Fetching token..."
	return func() tea.Msg {
		out, err := collectors.SwarmJoinToken(manager)
		e := ""
		if err != nil {
			e = err.Error()
		}
		return swarmDetailMsg{gen: gen, body: out, err: e}
	}
}

func (a *App) swarmBeginAvail() tea.Cmd {
	if a.swarmKind != swarmKindNodes || a.swarmCursor >= len(a.swarmNodes) {
		a.swarmStatus = "availability: selecione um node"
		return nil
	}
	n := a.swarmNodes[a.swarmCursor]
	a.swarmScreen = swarmScrForm
	a.swarmForm = swarmFormAvail
	a.swarmFormName = n.Hostname
	a.swarmFormAvail = strings.ToLower(n.Availability)
	if a.swarmFormAvail == "" {
		a.swarmFormAvail = "active"
	}
	a.swarmFormField = 0
	return nil
}

func (a *App) swarmAskRemove() tea.Cmd {
	switch a.swarmKind {
	case swarmKindServices:
		if a.swarmCursor < len(a.swarmServices) {
			a.swarmConfirm = true
			a.swarmConfirmAction = "rm-service:" + a.swarmServices[a.swarmCursor].Name
		}
	case swarmKindStacks:
		if a.swarmCursor < len(a.swarmStacks) {
			a.swarmConfirm = true
			a.swarmConfirmAction = "rm-stack:" + a.swarmStacks[a.swarmCursor].Name
		}
	case swarmKindNodes:
		if a.swarmCursor < len(a.swarmNodes) {
			// Único manager/leader local: node rm falha — use leave swarm.
			if len(a.swarmNodes) == 1 || strings.EqualFold(a.swarmNodes[a.swarmCursor].Manager, "Leader") {
				a.swarmConfirm = true
				a.swarmConfirmAction = "leave"
				a.swarmStatus = "node local/leader → leave swarm (não node rm)"
				return nil
			}
			a.swarmConfirm = true
			a.swarmConfirmAction = "rm-node:" + a.swarmNodes[a.swarmCursor].ID
		}
	case swarmKindSecrets:
		if a.swarmCursor < len(a.swarmSecrets) {
			a.swarmConfirm = true
			a.swarmConfirmAction = "rm-secret:" + a.swarmSecrets[a.swarmCursor].Name
		}
	case swarmKindConfigs:
		if a.swarmCursor < len(a.swarmConfigs) {
			a.swarmConfirm = true
			a.swarmConfirmAction = "rm-config:" + a.swarmConfigs[a.swarmCursor].Name
		}
	}
	return nil
}

func (a *App) swarmPromote() tea.Cmd {
	if a.swarmKind != swarmKindNodes || a.swarmCursor >= len(a.swarmNodes) {
		a.swarmStatus = "promote: selecione um node"
		return nil
	}
	id := a.swarmNodes[a.swarmCursor].ID
	gen := a.swarmGen
	return func() tea.Msg {
		out, err := collectors.SwarmNodePromote(id)
		if err != nil {
			return swarmActionMsg{gen: gen, err: err.Error()}
		}
		return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "promoted")}
	}
}

func (a *App) swarmDemote() tea.Cmd {
	if a.swarmKind != swarmKindNodes || a.swarmCursor >= len(a.swarmNodes) {
		a.swarmStatus = "demote: selecione um node"
		return nil
	}
	a.swarmConfirm = true
	a.swarmConfirmAction = "demote:" + a.swarmNodes[a.swarmCursor].ID
	return nil
}

func (a *App) swarmForceUpdate() tea.Cmd {
	if a.swarmKind != swarmKindServices || a.swarmCursor >= len(a.swarmServices) {
		a.swarmStatus = "force update: selecione um service"
		return nil
	}
	name := a.swarmServices[a.swarmCursor].Name
	gen := a.swarmGen
	a.swarmStatus = "Force updating " + name + "..."
	return func() tea.Msg {
		out, err := collectors.SwarmServiceForceUpdate(name)
		if err != nil {
			return swarmActionMsg{gen: gen, err: err.Error()}
		}
		return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "force updated "+name)}
	}
}

func (a *App) swarmRollback() tea.Cmd {
	if a.swarmKind != swarmKindServices || a.swarmCursor >= len(a.swarmServices) {
		a.swarmStatus = "rollback: selecione um service"
		return nil
	}
	name := a.swarmServices[a.swarmCursor].Name
	gen := a.swarmGen
	return func() tea.Msg {
		out, err := collectors.SwarmServiceRollback(name)
		if err != nil {
			return swarmActionMsg{gen: gen, err: err.Error()}
		}
		return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "rollback "+name)}
	}
}

func (a *App) handleSwarmFormKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch a.swarmForm {
	case swarmFormToken:
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "m", "M":
			return a, a.swarmBeginToken(true)
		case "w", "W":
			return a, a.swarmBeginToken(false)
		}
		return a, nil
	case swarmFormAvail:
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "up", "k", "left", "[":
			a.swarmFormAvail = swarmCycleAvail(a.swarmFormAvail, -1)
		case "down", "j", "right", "]":
			a.swarmFormAvail = swarmCycleAvail(a.swarmFormAvail, 1)
		case "enter", "y":
			name := a.swarmFormName
			avail := a.swarmFormAvail
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, func() tea.Msg {
				out, err := collectors.SwarmNodeAvailability(name, avail)
				if err != nil {
					return swarmActionMsg{gen: gen, err: err.Error()}
				}
				return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), name+" → "+avail)}
			}
		}
		return a, nil
	case swarmFormInit:
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "backspace":
			if len(a.swarmFormInput) > 0 {
				a.swarmFormInput = a.swarmFormInput[:len(a.swarmFormInput)-1]
			}
		case "enter":
			addr := strings.TrimSpace(a.swarmFormInput)
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			a.swarmStatus = "Initializing swarm..."
			return a, func() tea.Msg {
				out, err := collectors.SwarmInit(addr)
				if err != nil {
					return swarmActionMsg{gen: gen, err: err.Error()}
				}
				return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "swarm initialized")}
			}
		default:
			if len(msg.Runes) == 1 {
				a.swarmFormInput += string(msg.Runes)
			}
		}
		return a, nil
	case swarmFormDeploy:
		switch msg.String() {
		case "esc", "n", "N":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "enter", "y", "Y":
			compose := a.swarmCompose
			name := a.swarmFormName
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			a.swarmStatus = "Deploying stack..."
			return a, func() tea.Msg {
				out, err := collectors.SwarmStackDeploy(compose, name)
				if err != nil {
					return swarmActionMsg{gen: gen, err: err.Error()}
				}
				return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "stack "+name+" deployed")}
			}
		}
		return a, nil
	case swarmFormScale:
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "backspace":
			if len(a.swarmFormInput) > 0 {
				a.swarmFormInput = a.swarmFormInput[:len(a.swarmFormInput)-1]
			}
		case "enter":
			n, err := strconv.Atoi(strings.TrimSpace(a.swarmFormInput))
			if err != nil || n < 0 {
				a.swarmStatus = "replicas inválidas"
				return a, nil
			}
			name := a.swarmFormName
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			a.swarmStatus = fmt.Sprintf("Scaling %s → %d...", name, n)
			return a, func() tea.Msg {
				if err := collectors.SwarmServiceScale(name, n); err != nil {
					return swarmActionMsg{gen: gen, err: err.Error()}
				}
				return swarmActionMsg{gen: gen, out: fmt.Sprintf("scaled %s → %d", name, n)}
			}
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' {
				a.swarmFormInput += string(msg.Runes)
			}
		}
		return a, nil
	case swarmFormUpdate:
		fields := []*string{&a.swarmFormImage, &a.swarmFormReplicas}
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "tab", "down", "j":
			a.syncSwarmFormField(fields)
			a.swarmFormField = (a.swarmFormField + 1) % len(fields)
			a.swarmFormInput = *fields[a.swarmFormField]
		case "shift+tab", "up", "k":
			a.syncSwarmFormField(fields)
			a.swarmFormField = (a.swarmFormField + len(fields) - 1) % len(fields)
			a.swarmFormInput = *fields[a.swarmFormField]
		case "backspace":
			if len(a.swarmFormInput) > 0 {
				a.swarmFormInput = a.swarmFormInput[:len(a.swarmFormInput)-1]
			}
		case "enter":
			a.syncSwarmFormField(fields)
			name := a.swarmFormName
			image := strings.TrimSpace(a.swarmFormImage)
			reps, _ := strconv.Atoi(strings.TrimSpace(a.swarmFormReplicas))
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			a.swarmStatus = "Updating " + name + "..."
			return a, func() tea.Msg {
				if image != "" {
					if _, err := collectors.SwarmServiceUpdateImage(name, image); err != nil {
						return swarmActionMsg{gen: gen, err: err.Error()}
					}
				}
				if reps >= 0 {
					if err := collectors.SwarmServiceScale(name, reps); err != nil {
						return swarmActionMsg{gen: gen, err: err.Error()}
					}
				}
				return swarmActionMsg{gen: gen, out: "updated " + name}
			}
		default:
			if len(msg.Runes) == 1 {
				a.swarmFormInput += string(msg.Runes)
			}
		}
		return a, nil
	case swarmFormCreate:
		fields := []*string{&a.swarmFormName, &a.swarmFormImage, &a.swarmFormReplicas, &a.swarmFormPort, &a.swarmFormNetwork}
		switch msg.String() {
		case "esc":
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			return a, nil
		case "tab", "down", "j":
			a.syncSwarmFormField(fields)
			a.swarmFormField = (a.swarmFormField + 1) % len(fields)
			a.swarmFormInput = *fields[a.swarmFormField]
		case "shift+tab", "up", "k":
			a.syncSwarmFormField(fields)
			a.swarmFormField = (a.swarmFormField + len(fields) - 1) % len(fields)
			a.swarmFormInput = *fields[a.swarmFormField]
		case "backspace":
			if len(a.swarmFormInput) > 0 {
				a.swarmFormInput = a.swarmFormInput[:len(a.swarmFormInput)-1]
			}
		case "enter", "y":
			a.syncSwarmFormField(fields)
			name := strings.TrimSpace(a.swarmFormName)
			image := strings.TrimSpace(a.swarmFormImage)
			if name == "" || image == "" {
				a.swarmStatus = "name e image obrigatórios"
				return a, nil
			}
			reps, _ := strconv.Atoi(strings.TrimSpace(a.swarmFormReplicas))
			if reps <= 0 {
				reps = 1
			}
			port := strings.TrimSpace(a.swarmFormPort)
			net := strings.TrimSpace(a.swarmFormNetwork)
			gen := a.swarmGen
			a.swarmScreen = swarmScrCluster
			a.swarmForm = swarmFormNone
			a.swarmStatus = "Creating service..."
			return a, func() tea.Msg {
				out, err := collectors.SwarmServiceCreate(name, image, reps, port, net)
				if err != nil {
					return swarmActionMsg{gen: gen, err: err.Error()}
				}
				return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "created "+name)}
			}
		default:
			if len(msg.Runes) == 1 {
				a.swarmFormInput += string(msg.Runes)
			}
		}
		return a, nil
	}
	a.swarmScreen = swarmScrCluster
	a.swarmForm = swarmFormNone
	return a, nil
}

func (a *App) syncSwarmFormField(fields []*string) {
	if a.swarmFormField >= 0 && a.swarmFormField < len(fields) {
		*fields[a.swarmFormField] = a.swarmFormInput
	}
}

func swarmCycleAvail(cur string, delta int) string {
	opts := []string{"active", "pause", "drain"}
	idx := 0
	for i, o := range opts {
		if o == cur {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(opts)) % len(opts)
	return opts[idx]
}

func (a *App) swarmRunConfirm(action string, _ *core.Project) tea.Cmd {
	gen := a.swarmGen
	switch {
	case action == "leave":
		return func() tea.Msg {
			out, err := collectors.SwarmLeave(true)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "left swarm")}
		}
	case action == "prune":
		return func() tea.Msg {
			out, err := collectors.SwarmPruneNetworks()
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "networks pruned")}
		}
	case strings.HasPrefix(action, "demote:"):
		id := strings.TrimPrefix(action, "demote:")
		return func() tea.Msg {
			out, err := collectors.SwarmNodeDemote(id)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: firstNonEmpty(firstLine(out), "demoted")}
		}
	case strings.HasPrefix(action, "rm-service:"):
		name := strings.TrimPrefix(action, "rm-service:")
		return func() tea.Msg {
			err := collectors.SwarmServiceRemove(name)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: "removed service " + name}
		}
	case strings.HasPrefix(action, "rm-stack:"):
		name := strings.TrimPrefix(action, "rm-stack:")
		return func() tea.Msg {
			err := collectors.SwarmStackRemove(name)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: "removed stack " + name}
		}
	case strings.HasPrefix(action, "rm-node:"):
		id := strings.TrimPrefix(action, "rm-node:")
		return func() tea.Msg {
			err := collectors.SwarmNodeRemove(id, true)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: "removed node"}
		}
	case strings.HasPrefix(action, "rm-secret:"):
		name := strings.TrimPrefix(action, "rm-secret:")
		return func() tea.Msg {
			_, err := collectors.SwarmSecretRemove(name)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: "removed secret " + name}
		}
	case strings.HasPrefix(action, "rm-config:"):
		name := strings.TrimPrefix(action, "rm-config:")
		return func() tea.Msg {
			_, err := collectors.SwarmConfigRemove(name)
			if err != nil {
				return swarmActionMsg{gen: gen, err: err.Error()}
			}
			return swarmActionMsg{gen: gen, out: "removed config " + name}
		}
	}
	return nil
}

func sanitizeSwarmStackName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "stack"
	}
	if len(out) > 32 {
		out = out[:32]
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// used by landing compose path display
func swarmComposeBase(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}
