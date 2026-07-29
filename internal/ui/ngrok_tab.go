package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/ngrokutil"
)

const (
	ngrokWizName = iota
	ngrokWizPort
	ngrokWizProto
)

type ngrokSubTab int

const (
	ngrokTabOverview ngrokSubTab = iota
	ngrokTabTunnels
	ngrokTabRequests
	ngrokTabHistory
	ngrokTabDomains
	ngrokTabSettings
)

type ngrokFocus int

const (
	ngrokFocusTable ngrokFocus = iota
	ngrokFocusLogs
	ngrokFocusRequests
)

type ngrokLoadedMsg struct {
	tunnels  []ngrokutil.Tunnel
	requests []ngrokutil.Request
	agent    ngrokutil.AgentInfo
	cfg      ngrokutil.ProjectConfig
	foreign  int
	err      string
}

type ngrokActionMsg struct {
	out string
	err string
}

func (a *App) enterNgrokTab(_ *core.Project) {
	a.tab = TabNgrok
	a.tabCursor = 0
	a.ngrokOpen = false
}

func (a *App) openNgrokClient(p *core.Project) tea.Cmd {
	a.ngrokOpen = true
	a.ngrokSubTab = ngrokTabTunnels
	a.ngrokFocus = ngrokFocusTable
	a.ngrokCursor = 0
	a.ngrokScroll = 0
	a.ngrokReqCursor = 0
	a.ngrokReqScroll = 0
	a.ngrokLogScroll = 0
	a.ngrokErr = ""
	a.ngrokStatus = ""
	a.ngrokWizard = false
	a.ngrokConfirmDelete = false
	if a.ngrokNewName == "" {
		a.ngrokNewName = "api"
	}
	if a.ngrokNewPort == 0 && p != nil {
		a.ngrokNewPort = ngrokutil.SuggestPort(p.Ports, p.Framework.Name)
	}
	if a.ngrokNewProto == "" {
		a.ngrokNewProto = "http"
	}
	return a.refreshNgrok(p)
}

func (a *App) leaveNgrokTab() tea.Cmd {
	a.ngrokOpen = false
	a.ngrokWizard = false
	a.ngrokConfirmDelete = false
	a.tab = TabNgrok
	a.tabCursor = 0
	return nil
}

func (a *App) refreshNgrok(p *core.Project) tea.Cmd {
	a.ngrokLoading = true
	path, name := "", "project"
	if p != nil {
		path, name = p.Path, p.Name
	}
	showAll := a.ngrokShowAll
	return func() tea.Msg {
		cfg := ngrokutil.LoadProject(path, name)
		agent := ngrokutil.PingAgent()
		live, err := ngrokutil.ListLiveTunnels()
		if err != nil && agent.Connected {
			return ngrokLoadedMsg{cfg: cfg, agent: agent, err: err.Error()}
		}
		foreign := ngrokutil.CountForeignLive(cfg, live)
		tunnels := ngrokutil.MergeTunnels(cfg, live)
		if showAll {
			tunnels = ngrokutil.MergeTunnelsAll(cfg, live)
		}
		reqs, _ := ngrokutil.ListHTTPRequests(50)
		return ngrokLoadedMsg{
			tunnels: tunnels, requests: reqs, agent: agent, cfg: cfg,
			foreign: foreign,
		}
	}
}

func (a *App) handleNgrokMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case ngrokLoadedMsg:
		a.ngrokLoading = false
		a.ngrokCfg = m.cfg
		a.ngrokAgent = m.agent
		a.ngrokTunnels = m.tunnels
		a.ngrokRequests = m.requests
		a.ngrokForeign = m.foreign
		if m.err != "" {
			a.ngrokErr = m.err
		} else {
			a.ngrokErr = ""
		}
		if a.ngrokCursor >= len(a.ngrokTunnels) {
			a.ngrokCursor = maxInt(0, len(a.ngrokTunnels)-1)
		}
	case ngrokActionMsg:
		a.ngrokLoading = false
		a.ngrokConfirmDelete = false
		if m.err != "" {
			a.ngrokErr = m.err
			a.ngrokStatus = ""
			return a, nil
		}
		a.ngrokErr = ""
		a.ngrokStatus = truncate(m.out, 60)
		return a, a.refreshNgrok(a.currentProject())
	}
	return a, nil
}

func (a *App) renderNgrokLanding(p *core.Project) string {
	w, h := a.moduleSize()
	available := ngrokutil.Available()
	agent := ngrokutil.PingAgent()
	status := "offline"
	if agent.Connected {
		status = "connected"
	}
	ctx := a.renderModuleContext(p, w, "NGROK", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(6, bodyH*35/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("central de exposição de ambientes locais"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	if !available {
		openLines = append(openLines, "", StyleUnhealthy.Render("ngrok não encontrado no PATH"))
	} else {
		ver := ngrokutil.Version()
		openLines = append(openLines, "", StyleMuted.Render("versão  ")+StyleNormal.Render(ver))
		if agent.Connected {
			openLines = append(openLines, StyleHealthy.Render("● agente local online (:4040)"))
		} else {
			openLines = append(openLines, StyleMuted.Render("○ agente local offline — start cria o processo"))
		}
	}
	featLines := []string{
		StyleMuted.Render("túneis por projeto · start/stop/restart"),
		StyleMuted.Render("requests live · logs · copy URL"),
		StyleMuted.Render("config em .devscope/ngrok.json"),
		StyleMuted.Render("detecta porta do stack (Node/Laravel/…)"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("NGROK", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	details := []string{
		StyleMuted.Render("CLI     ") + StyleNormal.Render(boolLabel(available)),
		StyleMuted.Render("Agent   ") + StyleNormal.Render(boolLabel(agent.Connected)),
		StyleMuted.Render("API     ") + StyleMuted.Render(":4040"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir console"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderNgrokTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderNgrokHeader(p, w)
	nav := a.renderNgrokNav(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(nav)
	bodyH := maxInt(4, h-headerH-2)

	var body string
	if a.ngrokWizard {
		body = a.renderNgrokWizard(p, w, bodyH)
	} else {
		switch a.ngrokSubTab {
		case ngrokTabOverview:
			body = a.renderNgrokOverview(p, w, bodyH)
		case ngrokTabRequests:
			body = a.renderNgrokRequestsFull(w, bodyH)
		case ngrokTabHistory:
			body = a.renderNgrokHistory(w, bodyH)
		case ngrokTabDomains:
			body = a.renderNgrokDomains(w, bodyH)
		case ngrokTabSettings:
			body = a.renderNgrokSettings(p, w, bodyH)
		default:
			body = a.renderNgrokTunnelsView(p, w, bodyH)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, nav, body, a.renderStatusBar(a.ngrokHints()))
}

func (a *App) ngrokHints() string {
	if a.ngrokConfirmDelete {
		return "confirmar delete?  y sim  n/esc cancelar"
	}
	if a.ngrokWizard {
		return "tab campo  ←→ cursor  space proto  backspace/del  enter salvar+start  esc"
	}
	scope := "A todos"
	if a.ngrokShowAll {
		scope = "A projeto"
	}
	base := "0-5 aba  tab painel  n new  s start  x stop  r restart  c copy  o open  d delete  " + scope + "  esc"
	if a.ngrokLoading {
		base = "carregando…  " + base
	}
	if a.ngrokStatus != "" {
		return truncate(a.ngrokStatus, 36) + "  ·  " + base
	}
	if a.ngrokErr != "" {
		return StyleUnhealthy.Render(truncate(a.ngrokErr, 40)) + "  ·  " + base
	}
	return base
}

func (a *App) renderNgrokHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabNgrok)).Bold(true)
	name := "project"
	if p != nil {
		name = p.Name
	}
	env := projectEnvLabel(p)
	left := accent.Render("devscope") + StyleMuted.Render(" › ngrok") +
		StyleMuted.Render("  Projeto: ") + StyleNormal.Render(name) +
		StyleMuted.Render("  Ambiente: ") + StyleWarning.Render(env)

	badge := StyleMuted.Render("○ Offline")
	if a.ngrokAgent.Connected {
		badge = StyleHealthy.Render("● Connected")
	}
	online := 0
	for _, t := range a.ngrokTunnels {
		if t.Status == "online" {
			online++
		}
	}
	ver := a.ngrokAgent.Version
	if ver == "" {
		ver = "—"
	}
	region := a.ngrokCfg.Region
	if region == "" {
		region = "us"
	}
	scope := StyleMuted.Render("projeto")
	if a.ngrokShowAll {
		scope = StyleAccent.Render("TODOS")
	}
	right := badge + StyleMuted.Render(fmt.Sprintf("  Region:%s  v%s  Tunnels:%d  ", region, ver, online)) + scope
	if !a.ngrokShowAll && a.ngrokForeign > 0 {
		right += StyleMuted.Render(fmt.Sprintf("  (+%d outros · A)", a.ngrokForeign))
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderNgrokNav(width int) string {
	names := []string{"Overview", "Tunnels", "Requests", "History", "Domains", "Settings"}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf(" %d:%s ", i, n)
		if ngrokSubTab(i) == a.ngrokSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	line := strings.Join(parts, StyleMuted.Render("│"))
	pad := width - lipgloss.Width(stripANSI(line))
	if pad < 0 {
		pad = 0
	}
	return line + strings.Repeat(" ", pad)
}

func (a *App) renderNgrokOverview(p *core.Project, width, height int) string {
	rightW := a.moduleRightWidth(width)
	centerW := maxInt(36, width-rightW-1)
	online, offline := 0, 0
	for _, t := range a.ngrokTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	sumH := maxInt(8, height*45/100)
	listH := maxInt(6, height-sumH)
	lines := []string{
		StyleMuted.Render("Status     ") + ngrokStatusLabel(a.ngrokAgent.Connected),
		StyleMuted.Render("Account    ") + StyleMuted.Render("(local agent)"),
		StyleMuted.Render("Plan       ") + StyleNormal.Render("Free"),
		StyleMuted.Render("Region     ") + StyleNormal.Render(a.ngrokCfg.Region),
		StyleMuted.Render("Authtoken  ") + StyleMuted.Render("via ngrok config"),
		StyleMuted.Render("Tunnels    ") + StyleHealthy.Render(fmt.Sprintf("%d online", online)) +
			StyleMuted.Render(" / ") + StyleUnhealthy.Render(fmt.Sprintf("%d offline", offline)),
		StyleMuted.Render("Requests   ") + StyleNormal.Render(fmt.Sprintf("%d capturados", len(a.ngrokRequests))),
		StyleMuted.Render("Version    ") + StyleMuted.Render(a.ngrokAgent.Version),
	}
	evLines := make([]string, 0, listH-2)
	if len(a.ngrokRequests) == 0 {
		evLines = append(evLines, StyleMuted.Render("(sem eventos recentes)"))
	} else {
		n := minInt(listH-2, len(a.ngrokRequests))
		for i := 0; i < n; i++ {
			r := a.ngrokRequests[i]
			evLines = append(evLines, StyleMuted.Render(r.Time.Format("15:04"))+" "+
				StyleNormal.Render(fmt.Sprintf("%s %s %d", r.Method, truncate(r.Path, 28), r.Status)))
		}
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("OVERVIEW", fitExactLines(lines, sumH-2), centerW, sumH, false),
		renderApiTitledBox("RECENT EVENTS", fitExactLines(evLines, listH-2), centerW, listH, false),
	)
	details := []string{
		StyleHealthy.Render(fmt.Sprintf("online   %d", online)),
		StyleUnhealthy.Render(fmt.Sprintf("offline  %d", offline)),
		StyleMuted.Render(fmt.Sprintf("req/min  ~%d", len(a.ngrokRequests))),
	}
	if p != nil && len(p.Ports) > 0 {
		details = append(details, StyleMuted.Render("ports  ")+StyleAccent.Render(fmt.Sprintf("%v", p.Ports)))
	}
	actions := moduleActionLines(
		[2]string{"1", "túneis"},
		[2]string{"n", "novo túnel"},
		[2]string{"r", "refresh"},
	)
	right := a.renderModuleRightRail(rightW, height, details, actions)
	return lipgloss.JoinHorizontal(lipgloss.Top, center, right)
}

func ngrokStatusLabel(ok bool) string {
	if ok {
		return StyleHealthy.Render("● Connected")
	}
	return StyleMuted.Render("○ Offline")
}

func (a *App) renderNgrokTunnelsView(p *core.Project, width, height int) string {
	_ = p
	if height < 6 {
		height = 6
	}
	bottomH := height * 32 / 100
	if bottomH < 4 {
		bottomH = 4
	}
	if bottomH > height-4 {
		bottomH = height - 4
	}
	tableH := height - bottomH
	cmdW := tunnelCmdWidth(width)
	rest := width - cmdW
	statsW := rest / 2
	logsW := rest - statsW
	bottom := lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderNgrokQuickStats(statsW, bottomH),
		a.renderNgrokLogsPane(logsW, bottomH),
	)
	if cmdW >= 12 {
		bottom = lipgloss.JoinHorizontal(lipgloss.Top, bottom, a.renderNgrokCommands(cmdW, bottomH))
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderNgrokTunnelTable(width, tableH),
		bottom,
	)
}

func (a *App) renderNgrokQuickStats(width, height int) string {
	online, offline := 0, 0
	for _, t := range a.ngrokTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	scope := "projeto"
	if a.ngrokShowAll {
		scope = "TODOS"
	}
	region := a.ngrokCfg.Region
	if region == "" {
		region = "us"
	}
	lines := []string{
		StyleHealthy.Render(fmt.Sprintf("Online     %d", online)),
		StyleUnhealthy.Render(fmt.Sprintf("Offline    %d", offline)),
		StyleMuted.Render(fmt.Sprintf("Requests   %d", len(a.ngrokRequests))),
		StyleMuted.Render(fmt.Sprintf("Foreign    %d  (A)", a.ngrokForeign)),
		StyleMuted.Render("Scope      ") + StyleNormal.Render(scope),
		StyleMuted.Render("Region     ") + StyleNormal.Render(region),
	}
	return renderApiTitledBox("QUICK STATS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderNgrokCommands(width, height int) string {
	return renderActionsBox(width, height,
		[2]string{"s", "start"},
		[2]string{"x", "stop"},
		[2]string{"r", "restart"},
		[2]string{"n", "new"},
		[2]string{"e", "edit"},
		[2]string{"c", "copy"},
		[2]string{"o", "open"},
		[2]string{"A", "todos"},
		[2]string{"y", "dup"},
		[2]string{"d", "delete"},
	)
}

func (a *App) renderNgrokTunnelTable(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusTable
	n := len(a.ngrokTunnels)
	a.ngrokScroll = ensureVisible(a.ngrokCursor, a.ngrokScroll, height-3, n)
	cols := tunnelTableCols(width)
	header := fmt.Sprintf("%-3s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		"ST",
		cols.name, "NAME",
		cols.project, "PROJECT",
		cols.port, "PORT",
		cols.mode, "PROTO",
		cols.host, "DOMAIN",
		cols.uptime, "UPTIME",
		cols.pid, "REQ",
	)
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum túnel — n para criar)"))
	} else {
		start := a.ngrokScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			t := a.ngrokTunnels[i]
			dot := StyleMuted.Render("○")
			switch t.Status {
			case "online":
				dot = StyleHealthy.Render("●")
			case "starting":
				dot = StyleWarning.Render("●")
			default:
				dot = StyleUnhealthy.Render("●")
			}
			row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
				" ",
				cols.name, truncate(t.Name, cols.name),
				cols.project, truncate(t.Project, cols.project),
				cols.port, truncate(fmt.Sprintf("%d", t.Port), cols.port),
				cols.mode, truncate(t.Proto, cols.mode),
				cols.host, truncate(t.Domain, cols.host),
				cols.uptime, truncate(t.Uptime, cols.uptime),
				cols.pid, truncate(fmt.Sprintf("%d", t.Requests), cols.pid),
			)
			prefix := "  "
			style := StyleMuted
			if i == a.ngrokCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			lines = append(lines, style.Render(truncate(prefix+dot+" "+strings.TrimSpace(row), width-2)))
		}
	}
	title := fmt.Sprintf("TUNNELS (%d)", n)
	if focus {
		title = "> " + title
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderNgrokRequestsPane(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusRequests
	lines := make([]string, 0, height-2)
	lines = append(lines, StyleMuted.Render(truncate("TIME  METHOD ST  PATH", width-2)))
	if len(a.ngrokRequests) == 0 {
		lines = append(lines, StyleMuted.Render("(sem requests — agente/inspect)"))
	} else {
		a.ngrokReqScroll = ensureVisible(a.ngrokReqCursor, a.ngrokReqScroll, height-3, len(a.ngrokRequests))
		start := a.ngrokReqScroll
		end := minInt(start+height-3, len(a.ngrokRequests))
		for i := start; i < end; i++ {
			r := a.ngrokRequests[i]
			stStyle := StyleHealthy
			if r.Status >= 400 {
				stStyle = StyleUnhealthy
			} else if r.Status >= 300 {
				stStyle = StyleWarning
			}
			mark := "  "
			style := StyleMuted
			if i == a.ngrokReqCursor && focus {
				mark = "▸ "
				style = StyleSelected
			}
			line := fmt.Sprintf("%s %s %s %s",
				r.Time.Format("15:04:05"),
				fmt.Sprintf("%-4s", r.Method),
				stStyle.Render(fmt.Sprintf("%3d", r.Status)),
				truncate(r.Path, maxInt(8, width-22)),
			)
			lines = append(lines, style.Render(truncate(mark+stripANSI(line), width-2)))
		}
	}
	title := "LIVE REQUESTS"
	if focus {
		title = "> LIVE REQUESTS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderNgrokLogsPane(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusLogs
	var raw []string
	if t, ok := a.ngrokSelected(); ok {
		raw = append(raw, fmt.Sprintf("INF tunnel %s status=%s", t.Name, t.Status))
		if t.PublicURL != "" {
			raw = append(raw, "INF public "+t.PublicURL)
		}
		if t.LocalURL != "" {
			raw = append(raw, "INF local  "+t.LocalURL)
		}
	}
	for i, r := range a.ngrokRequests {
		if i >= 12 {
			break
		}
		level := "INF"
		if r.Status >= 500 {
			level = "ERR"
		} else if r.Status >= 400 {
			level = "WRN"
		}
		raw = append(raw, fmt.Sprintf("%s %s %s %d %dms", level, r.Time.Format("15:04:05"), r.Method, r.Status, r.LatencyMS))
	}
	if len(raw) == 0 {
		raw = []string{"INF aguardando atividade do agente"}
	}
	a.ngrokLogScroll = clampScroll(a.ngrokLogScroll, height-2, len(raw))
	start := a.ngrokLogScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		style := StyleMuted
		switch {
		case strings.HasPrefix(line, "ERR"):
			style = StyleUnhealthy
		case strings.HasPrefix(line, "WRN"):
			style = StyleWarning
		case strings.HasPrefix(line, "INF"):
			style = StyleHealthy
		case focus:
			style = StyleNormal
		}
		lines = append(lines, style.Render(truncate(line, width-2)))
	}
	title := "LOGS"
	if focus {
		title = "> LOGS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderNgrokRequestsFull(width, height int) string {
	a.ngrokFocus = ngrokFocusRequests
	return a.renderNgrokRequestsPane(width, height)
}

func (a *App) renderNgrokHistory(width, height int) string {
	lines := make([]string, 0, height-2)
	if len(a.ngrokCfg.History) == 0 {
		lines = append(lines, StyleMuted.Render("(histórico vazio — aparece após start/stop)"))
	} else {
		for _, h := range a.ngrokCfg.History {
			dur := "—"
			if !h.Stopped.IsZero() && !h.Started.IsZero() {
				dur = formatUptime(h.Stopped.Sub(h.Started))
			}
			lines = append(lines, StyleNormal.Render(fmt.Sprintf("%-12s :%d  %s  %s  req=%d",
				truncate(h.Name, 12), h.Port, h.Started.Format("01-02 15:04"), dur, h.Requests)))
		}
	}
	return renderApiTitledBox("HISTORY", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderNgrokDomains(width, height int) string {
	seen := map[string]bool{}
	lines := make([]string, 0, height-2)
	for _, t := range a.ngrokTunnels {
		if t.Domain == "" || seen[t.Domain] {
			continue
		}
		seen[t.Domain] = true
		st := StyleUnhealthy.Render("offline")
		if t.Status == "online" {
			st = StyleHealthy.Render("online")
		}
		lines = append(lines, StyleNormal.Render(truncate(t.Domain, width/2))+"  "+st+"  TLS")
	}
	if len(lines) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum domínio — planos pagos / reserved domain)"))
	}
	return renderApiTitledBox("DOMAINS", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderNgrokSettings(p *core.Project, width, height int) string {
	token := "via `ngrok config add-authtoken`"
	lines := []string{
		StyleMuted.Render("Authtoken      ") + StyleMuted.Render(token),
		StyleMuted.Render("Default Region ") + StyleNormal.Render(a.ngrokCfg.Region),
		StyleMuted.Render("Agent API      ") + StyleMuted.Render(ngrokutil.AgentBase()),
		StyleMuted.Render("Config file    ") + StyleMuted.Render(".devscope/ngrok.json"),
		StyleMuted.Render("Auto Start     ") + StyleMuted.Render("por túnel (flag no wizard)"),
		StyleMuted.Render("Inspect        ") + StyleHealthy.Render("on (agent :4040)"),
		"",
		StyleMuted.Render("CLI version    ") + StyleNormal.Render(ngrokutil.Version()),
	}
	if p != nil {
		lines = append(lines, StyleMuted.Render("Project path   ")+StyleMuted.Render(truncate(p.Path, width-18)))
	}
	return renderApiTitledBox("SETTINGS", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderNgrokWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	lines := []string{
		a.renderNgrokWizardField("Nome", a.ngrokNewName, ngrokWizName, true),
		a.renderNgrokWizardField("Porta", a.ngrokNewPortStr, ngrokWizPort, true),
		a.renderNgrokWizardField("Proto", a.ngrokNewProto, ngrokWizProto, false),
		StyleMuted.Render("Projeto   ") + StyleMuted.Render(proj+"  (fixo)"),
		"",
		StyleMuted.Render("tab/↑↓ campo · ←→ cursor · space proto · enter salvar · esc"),
	}
	return renderApiTitledBox("NOVO TÚNEL", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderNgrokWizardField(label, value string, field int, editable bool) string {
	prefix := StyleMuted.Render(fmt.Sprintf("%-9s ", label))
	focused := a.ngrokWizardField == field
	if !focused {
		return prefix + StyleNormal.Render(value)
	}
	if !editable {
		return prefix + StyleSelected.Render(value+"  ⟨space⟩")
	}
	runes := []rune(value)
	cur := a.ngrokWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return prefix + StyleSelected.Render(shown)
}

func (a *App) beginNgrokWizard(p *core.Project) {
	if a.ngrokNewName == "" {
		a.ngrokNewName = "api"
	}
	if a.ngrokNewProto == "" {
		a.ngrokNewProto = "http"
	}
	if a.ngrokNewPortStr == "" {
		port := a.ngrokNewPort
		if port == 0 && p != nil {
			port = ngrokutil.SuggestPort(p.Ports, p.Framework.Name)
		}
		if port == 0 {
			port = 3000
		}
		a.ngrokNewPort = port
		a.ngrokNewPortStr = strconv.Itoa(port)
	}
	a.ngrokWizard = true
	a.ngrokWizardField = ngrokWizName
	a.ngrokWizardCursor = len([]rune(a.ngrokNewName))
}

func (a *App) ngrokWizardText() string {
	switch a.ngrokWizardField {
	case ngrokWizName:
		return a.ngrokNewName
	case ngrokWizPort:
		return a.ngrokNewPortStr
	default:
		return ""
	}
}

func (a *App) setNgrokWizardText(s string) {
	switch a.ngrokWizardField {
	case ngrokWizName:
		a.ngrokNewName = s
	case ngrokWizPort:
		a.ngrokNewPortStr = s
	}
}

func (a *App) ngrokWizardFocusField(field int) {
	if field < ngrokWizName {
		field = ngrokWizProto
	}
	if field > ngrokWizProto {
		field = ngrokWizName
	}
	a.ngrokWizardField = field
	if field == ngrokWizProto {
		a.ngrokWizardCursor = 0
		return
	}
	a.ngrokWizardCursor = len([]rune(a.ngrokWizardText()))
}

func (a *App) cycleNgrokProto() {
	switch a.ngrokNewProto {
	case "http":
		a.ngrokNewProto = "tcp"
	case "tcp":
		a.ngrokNewProto = "https"
	default:
		a.ngrokNewProto = "http"
	}
}

func (a *App) ngrokSelected() (ngrokutil.Tunnel, bool) {
	if a.ngrokCursor < 0 || a.ngrokCursor >= len(a.ngrokTunnels) {
		return ngrokutil.Tunnel{}, false
	}
	return a.ngrokTunnels[a.ngrokCursor], true
}

func ngrokTunnelInConfig(cfg ngrokutil.ProjectConfig, t ngrokutil.Tunnel) bool {
	for _, c := range cfg.Tunnels {
		if c.Name == t.Name || (c.Port > 0 && c.Port == t.Port) {
			return true
		}
	}
	return false
}

func (a *App) handleNgrokKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.ngrokConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			return a, a.ngrokDeleteSelected(p)
		case "n", "N", "esc":
			a.ngrokConfirmDelete = false
			return a, nil
		}
		return a, nil
	}
	if a.ngrokWizard {
		return a.updateNgrokWizard(msg, p)
	}

	switch msg.String() {
	case "esc":
		return a, a.leaveNgrokTab()
	case "tab":
		if a.ngrokSubTab == ngrokTabRequests {
			a.ngrokFocus = ngrokFocusRequests
		} else {
			a.ngrokFocus = (a.ngrokFocus + 1) % 2
		}
	case "0":
		a.ngrokSubTab = ngrokTabOverview
	case "1":
		a.ngrokSubTab = ngrokTabTunnels
		a.ngrokFocus = ngrokFocusTable
	case "2":
		a.ngrokSubTab = ngrokTabRequests
		a.ngrokFocus = ngrokFocusRequests
	case "3":
		a.ngrokSubTab = ngrokTabHistory
	case "4":
		a.ngrokSubTab = ngrokTabDomains
	case "5":
		a.ngrokSubTab = ngrokTabSettings
	case "up", "k":
		return a, a.ngrokMove(-1)
	case "down", "j":
		return a, a.ngrokMove(1)
	case "n":
		a.ngrokNewName = "api"
		a.ngrokNewProto = "http"
		a.ngrokNewPort = 0
		a.ngrokNewPortStr = ""
		a.beginNgrokWizard(p)
	case "e":
		if t, ok := a.ngrokSelected(); ok {
			a.ngrokNewName = t.Name
			a.ngrokNewPort = t.Port
			a.ngrokNewPortStr = strconv.Itoa(t.Port)
			a.ngrokNewProto = t.Proto
			a.beginNgrokWizard(p)
		}
	case "s":
		return a, a.ngrokStartSelected(p)
	case "x":
		return a, a.ngrokStopSelected()
	case "r":
		if a.ngrokFocus == ngrokFocusTable {
			return a, a.ngrokRestartSelected(p)
		}
		return a, a.refreshNgrok(p)
	case "c":
		return a, a.ngrokCopyURL(false)
	case "C":
		return a, a.ngrokCopyURL(true)
	case "o", "O":
		return a, a.ngrokOpenBrowser()
	case "A":
		a.ngrokShowAll = !a.ngrokShowAll
		if a.ngrokShowAll {
			a.ngrokStatus = "mostrando todos os túneis do agent"
		} else {
			a.ngrokStatus = "filtrando túneis do projeto"
		}
		return a, a.refreshNgrok(p)
	case "d":
		if t, ok := a.ngrokSelected(); ok {
			if a.ngrokShowAll && !ngrokTunnelInConfig(a.ngrokCfg, t) {
				a.ngrokStatus = "túnel de outro projeto — só stop (x)"
				return a, nil
			}
			a.ngrokConfirmDelete = true
			a.ngrokStatus = "delete túnel da config?"
		}
	case "y":
		return a, a.ngrokDuplicateSelected(p)
	case "ctrl+r":
		return a, a.refreshNgrok(p)
	}
	return a, nil
}

func (a *App) ngrokMove(delta int) tea.Cmd {
	switch a.ngrokFocus {
	case ngrokFocusRequests:
		a.ngrokReqCursor += delta
		if a.ngrokReqCursor < 0 {
			a.ngrokReqCursor = 0
		}
		if a.ngrokReqCursor > len(a.ngrokRequests)-1 {
			a.ngrokReqCursor = maxInt(0, len(a.ngrokRequests)-1)
		}
	case ngrokFocusLogs:
		a.ngrokLogScroll += delta
		if a.ngrokLogScroll < 0 {
			a.ngrokLogScroll = 0
		}
	default:
		a.ngrokCursor += delta
		if a.ngrokCursor < 0 {
			a.ngrokCursor = 0
		}
		if a.ngrokCursor > len(a.ngrokTunnels)-1 {
			a.ngrokCursor = maxInt(0, len(a.ngrokTunnels)-1)
		}
	}
	return nil
}

func (a *App) updateNgrokWizard(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.ngrokWizard = false
		return a, nil
	case "enter":
		name := strings.TrimSpace(a.ngrokNewName)
		if name == "" {
			a.ngrokStatus = "nome vazio"
			return a, nil
		}
		port, err := strconv.Atoi(strings.TrimSpace(a.ngrokNewPortStr))
		if err != nil || port < 1 || port > 65535 {
			a.ngrokStatus = "porta inválida"
			a.ngrokWizardField = ngrokWizPort
			a.ngrokWizardCursor = len([]rune(a.ngrokNewPortStr))
			return a, nil
		}
		a.ngrokNewName = name
		a.ngrokNewPort = port
		a.ngrokWizard = false
		return a, a.ngrokCreateAndStart(p)
	case "tab", "down":
		a.ngrokWizardFocusField(a.ngrokWizardField + 1)
		return a, nil
	case "shift+tab", "up":
		a.ngrokWizardFocusField(a.ngrokWizardField - 1)
		return a, nil
	case "[", "]":
		a.cycleNgrokProto()
		a.ngrokWizardField = ngrokWizProto
		return a, nil
	case " ":
		if a.ngrokWizardField == ngrokWizProto {
			a.cycleNgrokProto()
		}
		return a, nil
	}

	if a.ngrokWizardField == ngrokWizProto {
		switch msg.String() {
		case "left", "right":
			a.cycleNgrokProto()
		}
		return a, nil
	}

	text := a.ngrokWizardText()
	runes := []rune(text)
	cur := a.ngrokWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}

	switch msg.String() {
	case "left":
		if cur > 0 {
			cur--
		}
	case "right":
		if cur < len(runes) {
			cur++
		}
	case "home":
		cur = 0
	case "end":
		cur = len(runes)
	case "backspace":
		if cur > 0 {
			runes = append(runes[:cur-1], runes[cur:]...)
			cur--
			a.setNgrokWizardText(string(runes))
		}
	case "delete":
		if cur < len(runes) {
			runes = append(runes[:cur], runes[cur+1:]...)
			a.setNgrokWizardText(string(runes))
		}
	default:
		if len(msg.Runes) > 0 {
			inserted := append([]rune(nil), msg.Runes...)
			if a.ngrokWizardField == ngrokWizPort {
				for _, r := range inserted {
					if r < '0' || r > '9' {
						return a, nil
					}
				}
			}
			runes = append(runes[:cur], append(inserted, runes[cur:]...)...)
			cur += len(inserted)
			a.setNgrokWizardText(string(runes))
		}
	}
	a.ngrokWizardCursor = cur
	return a, nil
}

func (a *App) ngrokCreateAndStart(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	name := strings.TrimSpace(a.ngrokNewName)
	port := a.ngrokNewPort
	proto := a.ngrokNewProto
	if port == 0 {
		if v, err := strconv.Atoi(strings.TrimSpace(a.ngrokNewPortStr)); err == nil {
			port = v
		}
	}
	if port == 0 {
		port = ngrokutil.SuggestPort(p.Ports, p.Framework.Name)
	}
	cfg := a.ngrokCfg
	cfg.Project = p.Name
	cfg.UpsertTunnel(ngrokutil.TunnelConfig{Name: name, Port: port, Proto: proto, Region: cfg.Region})
	_ = ngrokutil.SaveProject(p.Path, cfg)
	a.ngrokCfg = cfg
	a.ngrokLoading = true
	a.ngrokStatus = "starting " + name + "…"
	return func() tea.Msg {
		err := ngrokutil.StartTunnel(name, port, proto)
		if err != nil {
			return ngrokActionMsg{err: err.Error()}
		}
		return ngrokActionMsg{out: "started " + name}
	}
}

func (a *App) ngrokStartSelected(p *core.Project) tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok {
		return nil
	}
	if t.Status == "online" {
		a.ngrokStatus = t.Name + " já online"
		return nil
	}
	a.ngrokLoading = true
	return func() tea.Msg {
		err := ngrokutil.StartTunnel(t.Name, t.Port, t.Proto)
		if err != nil {
			return ngrokActionMsg{err: err.Error()}
		}
		if p != nil {
			cfg := ngrokutil.LoadProject(p.Path, p.Name)
			cfg.UpsertTunnel(ngrokutil.TunnelConfig{Name: t.Name, Port: t.Port, Proto: t.Proto})
			cfg.History = append([]ngrokutil.HistoryEntry{{
				Name: t.Name, Port: t.Port, Proto: t.Proto, Started: time.Now(),
			}}, cfg.History...)
			if len(cfg.History) > 40 {
				cfg.History = cfg.History[:40]
			}
			_ = ngrokutil.SaveProject(p.Path, cfg)
		}
		return ngrokActionMsg{out: "started " + t.Name}
	}
}

func (a *App) ngrokStopSelected() tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok {
		return nil
	}
	a.ngrokLoading = true
	return func() tea.Msg {
		err := ngrokutil.StopTunnel(t.Name)
		if err != nil {
			return ngrokActionMsg{err: err.Error()}
		}
		return ngrokActionMsg{out: "stopped " + t.Name}
	}
}

func (a *App) ngrokRestartSelected(p *core.Project) tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok {
		return nil
	}
	a.ngrokLoading = true
	return func() tea.Msg {
		_ = ngrokutil.StopTunnel(t.Name)
		time.Sleep(400 * time.Millisecond)
		err := ngrokutil.StartTunnel(t.Name, t.Port, t.Proto)
		if err != nil {
			return ngrokActionMsg{err: err.Error()}
		}
		return ngrokActionMsg{out: "restarted " + t.Name}
	}
}

func (a *App) ngrokDeleteSelected(p *core.Project) tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok || p == nil {
		a.ngrokConfirmDelete = false
		return nil
	}
	_ = ngrokutil.StopTunnel(t.Name)
	cfg := a.ngrokCfg
	cfg.RemoveTunnel(t.Name)
	_ = ngrokutil.SaveProject(p.Path, cfg)
	a.ngrokCfg = cfg
	a.ngrokConfirmDelete = false
	return a.refreshNgrok(p)
}

func (a *App) ngrokDuplicateSelected(p *core.Project) tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok {
		return nil
	}
	a.ngrokNewName = t.Name + "-copy"
	a.ngrokNewPort = t.Port
	a.ngrokNewPortStr = strconv.Itoa(t.Port)
	a.ngrokNewProto = t.Proto
	a.beginNgrokWizard(p)
	return nil
}

func (a *App) ngrokCopyURL(local bool) tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok {
		return nil
	}
	url := t.PublicURL
	if local {
		url = t.LocalURL
	}
	if url == "" {
		a.ngrokErr = "URL vazia"
		return nil
	}
	if err := copyToClipboard(url); err != nil {
		a.ngrokErr = "clipboard: " + err.Error()
		return nil
	}
	a.ngrokStatus = "copied " + truncate(url, 40)
	return nil
}

func (a *App) ngrokOpenBrowser() tea.Cmd {
	t, ok := a.ngrokSelected()
	if !ok || t.PublicURL == "" {
		a.ngrokErr = "sem URL pública"
		return nil
	}
	url := t.PublicURL
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		_ = cmd.Start()
		return ngrokActionMsg{out: "opened browser"}
	}
}
