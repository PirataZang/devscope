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
	"github.com/devscope/devscope/internal/cfutil"
	"github.com/devscope/devscope/internal/core"
)

const (
	cfWizName = iota
	cfWizURL
	cfWizHostname
	cfWizMode
)

// cfDefaultURL é o destino local pré-preenchido no wizard. 127.0.0.1 em vez de
// localhost porque localhost pode resolver para ::1 e render 502 na origem.
const cfDefaultURL = "http://127.0.0.1:3000"

type cfSubTab int

const (
	cfTabOverview cfSubTab = iota
	cfTabTunnels
	cfTabAccount
	cfTabHistory
	cfTabSetup
	cfTabSettings
)

type cfFocus int

const (
	cfFocusTable cfFocus = iota
	cfFocusDetails
	cfFocusLogs
)

type cfLoadedMsg struct {
	tunnels []cfutil.Tunnel
	account []cfutil.AccountTunnel
	cfg     cfutil.ProjectConfig
	auth    cfutil.AuthInfo
	foreign int
	err     string
}

type cfActionMsg struct {
	out string
	err string
}

func (a *App) enterCFTab(_ *core.Project) {
	a.tab = TabCFTunnel
	a.tabCursor = 0
	a.cfOpen = false
}

func (a *App) openCFClient(p *core.Project) tea.Cmd {
	a.cfOpen = true
	a.cfSubTab = cfTabTunnels
	a.cfFocus = cfFocusTable
	a.cfCursor = 0
	a.cfScroll = 0
	a.cfAcctCursor = 0
	a.cfAcctScroll = 0
	a.cfLogScroll = 0
	a.cfDetailsScroll = 0
	a.cfErr = ""
	a.cfStatus = ""
	a.cfWizard = false
	a.cfConfirmDelete = false
	if a.cfNewName == "" {
		a.cfNewName = "api"
	}
	if a.cfNewURL == "" {
		a.cfNewURL = cfDefaultURL
	}
	if a.cfNewMode == "" {
		a.cfNewMode = "quick"
	}
	return a.refreshCF(p)
}

func (a *App) leaveCFTab() tea.Cmd {
	a.cfOpen = false
	a.cfWizard = false
	a.cfConfirmDelete = false
	a.tab = TabCFTunnel
	a.tabCursor = 0
	return nil
}

func (a *App) refreshCF(p *core.Project) tea.Cmd {
	a.cfLoading = true
	path, name := "", "project"
	if p != nil {
		path, name = p.Path, p.Name
	}
	showAll := a.cfShowAll
	return func() tea.Msg {
		cfg := cfutil.LoadProject(path, name)
		auth := cfutil.Auth()
		live := cfutil.ListLiveTunnels()
		foreign := cfutil.CountForeignLive(cfg, live)
		tunnels := cfutil.MergeTunnels(cfg, live)
		if showAll {
			tunnels = cfutil.MergeTunnelsAll(cfg, live)
		}
		var account []cfutil.AccountTunnel
		var errStr string
		if cfutil.LoggedIn() {
			list, err := cfutil.ListAccountTunnels()
			if err != nil {
				errStr = err.Error()
			} else {
				account = list
			}
		}
		return cfLoadedMsg{
			tunnels: tunnels, account: account, cfg: cfg, auth: auth,
			foreign: foreign, err: errStr,
		}
	}
}

func (a *App) handleCFMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case cfLoadedMsg:
		a.cfLoading = false
		a.cfCfg = m.cfg
		a.cfAuth = m.auth
		a.cfTunnels = m.tunnels
		a.cfAccount = m.account
		a.cfForeign = m.foreign
		if m.err != "" {
			a.cfErr = m.err
		} else {
			a.cfErr = ""
		}
		if a.cfCursor >= len(a.cfTunnels) {
			a.cfCursor = maxInt(0, len(a.cfTunnels)-1)
		}
		if a.cfAcctCursor >= len(a.cfAccount) {
			a.cfAcctCursor = maxInt(0, len(a.cfAccount)-1)
		}
	case cfActionMsg:
		a.cfLoading = false
		a.cfConfirmDelete = false
		if m.err != "" {
			a.cfErr = m.err
			a.cfStatus = ""
			return a, nil
		}
		a.cfErr = ""
		a.cfStatus = truncate(m.out, 80)
		return a, a.refreshCF(a.currentProject())
	}
	return a, nil
}

func (a *App) renderCFLanding(p *core.Project) string {
	w, h := a.moduleSize()
	auth := a.landingCF
	status := "…"
	if a.landingCFOK {
		status = "offline"
		if auth.CLI && auth.LoggedIn {
			status = "ready"
		} else if auth.CLI {
			status = "no-auth"
		}
	}
	ctx := a.renderModuleContext(p, w, "CLOUDFLARE TUNNEL", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(7, bodyH*40/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("exposição local via edge Cloudflare — quick, named & http2"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingCFOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando ambiente…"))
	case !auth.CLI:
		openLines = append(openLines, "", StyleUnhealthy.Render("cloudflared não encontrado no PATH"))
		openLines = append(openLines, StyleMuted.Render("abra o console e pressione I para instalar"))
	default:
		openLines = append(openLines, "", StyleMuted.Render("versão  ")+StyleNormal.Render(auth.Version))
		if auth.LoggedIn {
			openLines = append(openLines, a.livePulse("autenticado (cert.pem)"))
		} else {
			openLines = append(openLines, StyleWarning.Render("○ sem login — quick tunnels ainda funcionam"))
			openLines = append(openLines, StyleMuted.Render("named tunnels: pressione L no console"))
		}
	}
	featLines := []string{
		StyleMuted.Render("quick tunnel · trycloudflare.com (sem login)"),
		StyleMuted.Render("named tunnel · hostname no seu domínio"),
		StyleMuted.Render("install CLI · login · create · route dns"),
		StyleMuted.Render("config em .devscope/cloudflare.json"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("CLOUDFLARE", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	cliLabel, authLabel := "…", "…"
	if a.landingCFOK {
		cliLabel, authLabel = boolLabel(auth.CLI), boolLabel(auth.LoggedIn)
	}
	details := []string{
		StyleMuted.Render("CLI     ") + StyleNormal.Render(cliLabel),
		StyleMuted.Render("Auth    ") + StyleNormal.Render(authLabel),
		StyleMuted.Render("Edge    ") + StyleMuted.Render("global"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir console"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderCFTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderCFHeader(p, w)
	nav := a.renderCFNav(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(nav)
	bodyH := maxInt(4, h-headerH-2)

	var body string
	switch a.cfSubTab {
	case cfTabOverview:
		body = a.renderCFOverview(p, w, bodyH)
	case cfTabAccount:
		body = a.renderCFAccount(w, bodyH)
	case cfTabHistory:
		body = a.renderCFHistory(w, bodyH)
	case cfTabSetup:
		body = a.renderCFSetup(w, bodyH)
	case cfTabSettings:
		body = a.renderCFSettings(p, w, bodyH)
	default:
		body = a.renderCFTunnelsView(p, w, bodyH)
	}
	view := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, a.renderStatusBar(a.cfHints()))
	if a.cfWizard {
		view = overlayCentered(view, a.renderCFWizard(p, w, h), w, h)
	}
	if a.cfConfirmDelete {
		target, detail := a.cfDeleteConfirmLabels()
		box := renderTunnelDeleteConfirmBox("CLOUDFLARE", tabAccentColor(TabCFTunnel), target, detail, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) cfHints() string {
	if a.cfConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.cfWizard {
		return "modal novo túnel  tab campo  ←→ cursor  space mode  enter salvar+start  esc"
	}
	scope := "A todos"
	if a.cfShowAll {
		scope = "A projeto"
	}
	base := "0-5 aba  tab lista/detalhes/logs  n new  s start  x stop  I install  L login  C create  R route  c copy  o open  d delete  " + scope + "  esc"
	if a.cfLoading {
		base = a.spinner() + " carregando…  " + base
	}
	if a.cfStatus != "" {
		return truncate(a.cfStatus, 72) + "  ·  " + base
	}
	if a.cfErr != "" {
		return StyleUnhealthy.Render(truncate(a.cfErr, 40)) + "  ·  " + base
	}
	return base
}

func (a *App) renderCFHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabCFTunnel)).Bold(true)
	name := "project"
	if p != nil {
		name = p.Name
	}
	env := projectEnvLabel(p)
	left := accent.Render("devscope") + StyleMuted.Render(" › cloudflare") +
		StyleMuted.Render("  Projeto: ") + StyleNormal.Render(name) +
		StyleMuted.Render("  Ambiente: ") + StyleWarning.Render(env)

	badge := StyleMuted.Render("○ CLI off")
	if a.cfAuth.CLI && a.cfAuth.LoggedIn {
		badge = a.livePulse("Authenticated")
	} else if a.cfAuth.CLI {
		badge = StyleWarning.Render("● CLI · no auth")
	}
	online := 0
	for _, t := range a.cfTunnels {
		if t.Status == "online" {
			online++
		}
	}
	ver := a.cfAuth.Version
	if ver == "" {
		ver = "—"
	}
	scope := StyleMuted.Render("projeto")
	if a.cfShowAll {
		scope = StyleAccent.Render("TODOS")
	}
	right := badge + StyleMuted.Render(fmt.Sprintf("  v%s  Live:%d  Account:%d  ", ver, online, len(a.cfAccount))) + scope
	if !a.cfShowAll && a.cfForeign > 0 {
		right += StyleMuted.Render(fmt.Sprintf("  (+%d outros · A)", a.cfForeign))
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderCFNav(width int) string {
	names := []string{"Overview", "Tunnels", "Account", "History", "Setup", "Settings"}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf(" %d:%s ", i, n)
		if cfSubTab(i) == a.cfSubTab {
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

func (a *App) renderCFOverview(p *core.Project, width, height int) string {
	rightW := a.moduleRightWidth(width)
	centerW := maxInt(36, width-rightW-1)
	online, offline := 0, 0
	for _, t := range a.cfTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	sumH := maxInt(8, height*50/100)
	listH := maxInt(6, height-sumH)
	authLabel := StyleWarning.Render("○ not logged in")
	if a.cfAuth.LoggedIn {
		authLabel = a.livePulse("logged in")
	}
	cliLabel := StyleUnhealthy.Render("missing")
	if a.cfAuth.CLI {
		cliLabel = StyleHealthy.Render("installed")
	}
	lines := []string{
		StyleMuted.Render("CLI        ") + cliLabel,
		StyleMuted.Render("Auth       ") + authLabel,
		StyleMuted.Render("Cert       ") + StyleMuted.Render(truncate(a.cfAuth.CertPath, maxInt(12, centerW-14))),
		StyleMuted.Render("Tunnels    ") + StyleHealthy.Render(fmt.Sprintf("%d online", online)) +
			StyleMuted.Render(" / ") + StyleUnhealthy.Render(fmt.Sprintf("%d offline", offline)),
		StyleMuted.Render("Account    ") + StyleNormal.Render(fmt.Sprintf("%d named", len(a.cfAccount))),
		StyleMuted.Render("Version    ") + StyleMuted.Render(a.cfAuth.Version),
		StyleMuted.Render("Edge       ") + StyleNormal.Render("Cloudflare Global"),
	}
	evLines := make([]string, 0, listH-2)
	if len(a.cfCfg.History) == 0 {
		evLines = append(evLines, StyleMuted.Render("(sem eventos — start um túnel)"))
	} else {
		n := minInt(listH-2, len(a.cfCfg.History))
		for i := 0; i < n; i++ {
			h := a.cfCfg.History[i]
			evLines = append(evLines, StyleMuted.Render(h.Started.Format("15:04"))+" "+
				StyleNormal.Render(fmt.Sprintf("%s %s %s", truncate(h.Name, 12), truncate(h.Target, 24), h.Mode)))
		}
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("OVERVIEW", fitExactLines(lines, sumH-2), centerW, sumH, false),
		renderApiTitledBox("RECENT", fitExactLines(evLines, listH-2), centerW, listH, false),
	)
	details := []string{
		StyleHealthy.Render(fmt.Sprintf("online   %d", online)),
		StyleUnhealthy.Render(fmt.Sprintf("offline  %d", offline)),
		StyleMuted.Render(fmt.Sprintf("named    %d", len(a.cfAccount))),
	}
	if p != nil && len(p.Ports) > 0 {
		details = append(details, StyleMuted.Render("ports  ")+StyleAccent.Render(fmt.Sprintf("%v", p.Ports)))
	}
	actions := moduleActionLines(
		[2]string{"1", "túneis"},
		[2]string{"4", "setup"},
		[2]string{"n", "novo túnel"},
		[2]string{"L", "login"},
		[2]string{"I", "install"},
	)
	right := a.renderModuleRightRail(rightW, height, details, actions)
	return lipgloss.JoinHorizontal(lipgloss.Top, center, right)
}

func (a *App) renderCFTunnelsView(p *core.Project, width, height int) string {
	_ = p
	if height < 6 {
		height = 6
	}
	leftW := maxInt(32, width*40/100)
	rightW := maxInt(28, width-leftW-1)
	logsH := maxInt(4, height*34/100)
	if logsH > height-6 {
		logsH = height - 6
	}
	detailsH := height - logsH
	left := a.renderCFTunnelTable(leftW, height)
	right := lipgloss.JoinVertical(lipgloss.Left,
		a.renderCFDetailsPane(rightW, detailsH),
		a.renderCFLogsPane(rightW, logsH),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (a *App) renderCFQuickStats(width, height int) string {
	online, offline := 0, 0
	for _, t := range a.cfTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	scope := "projeto"
	if a.cfShowAll {
		scope = "TODOS"
	}
	lines := []string{
		StyleHealthy.Render(fmt.Sprintf("Online     %d", online)),
		StyleUnhealthy.Render(fmt.Sprintf("Offline    %d", offline)),
		StyleMuted.Render(fmt.Sprintf("Account    %d named", len(a.cfAccount))),
		StyleMuted.Render(fmt.Sprintf("Foreign    %d  (A)", a.cfForeign)),
		StyleMuted.Render("Scope      ") + StyleNormal.Render(scope),
		StyleMuted.Render("Edge       Cloudflare Global"),
	}
	return renderApiTitledBox("QUICK STATS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderCFCommands(width, height int) string {
	return renderActionsBox(width, height,
		[2]string{"s", "start"},
		[2]string{"x", "stop"},
		[2]string{"r", "restart"},
		[2]string{"n", "new"},
		[2]string{"c", "copy"},
		[2]string{"o", "open"},
		[2]string{"A", "todos"},
		[2]string{"I", "install"},
		[2]string{"L", "login"},
		[2]string{"C", "create"},
		[2]string{"R", "route"},
		[2]string{"d", "delete"},
	)
}

func (a *App) renderCFTunnelTable(width, height int) string {
	focus := a.cfFocus == cfFocusTable
	n := len(a.cfTunnels)
	a.cfScroll = ensureVisible(a.cfCursor, a.cfScroll, height-3, n)
	nameW := maxInt(8, width-20)
	header := fmt.Sprintf("%-3s %-*s %4s %-6s", "ST", nameW, "NAME", "PORT", "MODE")
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		hint := "  (nenhum túnel do projeto — n para criar"
		if a.cfForeign > 0 {
			hint += fmt.Sprintf(" · A vê +%d no host", a.cfForeign)
		}
		lines = append(lines, StyleMuted.Render(hint+")"))
	} else {
		start := a.cfScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			t := a.cfTunnels[i]
			dot := StyleUnhealthy.Render("●")
			switch t.Status {
			case "online":
				dot = StyleHealthy.Render(a.pulse())
			case "starting":
				dot = StyleWarning.Render(a.spinner())
			}
			port := "—"
			if t.Port > 0 {
				port = strconv.Itoa(t.Port)
			}
			row := fmt.Sprintf("%-*s %4s %-6s",
				nameW, truncate(t.Name, nameW), port, truncate(t.Mode, 6),
			)
			prefix := "  "
			style := StyleMuted
			if i == a.cfCursor {
				prefix = "▸ "
				if focus {
					style = StyleSelected
				} else {
					style = StyleNormal
				}
			}
			lines = append(lines, style.Render(truncate(prefix+dot+" "+row, width-2)))
		}
	}
	title := fmt.Sprintf("TUNNELS (%d)", n)
	if focus {
		title = "> " + title
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderCFDetailsPane(width, height int) string {
	focus := a.cfFocus == cfFocusDetails
	innerW := maxInt(20, width-2)
	var raw []string
	t, ok := a.cfSelected()
	if !ok {
		raw = []string{StyleMuted.Render("(selecione um túnel na lista)")}
	} else {
		host := t.Hostname
		if host == "" {
			host = publicHostFromURL(t.PublicURL)
		}
		pid := "—"
		if t.PID > 0 {
			pid = strconv.Itoa(t.PID)
		}
		raw = append(raw,
			StyleNormal.Bold(true).Render(truncate(t.Name, innerW))+"  "+tunnelStatusBadge(t.Status, a.animFrame),
			"",
		)
		portLabel := "—"
		if t.Port > 0 {
			portLabel = strconv.Itoa(t.Port)
		}
		metrics := tunnelMetricRow([][2]string{
			{"STATUS", t.Status},
			{"MODE", firstNonEmpty(t.Mode, "—")},
			{"PORT", portLabel},
			{"PID", pid},
		}, innerW)
		if metrics != "" {
			raw = append(raw, strings.Split(metrics, "\n")...)
			raw = append(raw, "")
		}
		raw = append(raw,
			tunnelDetailKV("Public", t.PublicURL),
			tunnelDetailKV("Local", t.LocalURL),
			tunnelDetailKV("Host", host),
			tunnelDetailKV("Uptime", t.Uptime),
			tunnelDetailKV("Project", t.Project),
			tunnelDetailKV("TunnelID", t.TunnelID),
		)
	}
	a.cfDetailsScroll = clampScroll(a.cfDetailsScroll, height-2, len(raw))
	start := a.cfDetailsScroll
	end := minInt(start+height-2, len(raw))
	lines := raw[start:end]
	title := "DETALHES"
	if focus {
		title = "> DETALHES"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func publicHostFromURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func (a *App) renderCFLogsPane(width, height int) string {
	focus := a.cfFocus == cfFocusLogs
	var raw []string
	if t, ok := a.cfSelected(); ok {
		raw = append(raw, fmt.Sprintf("INF tunnel %s status=%s mode=%s", t.Name, t.Status, t.Mode))
		if t.PublicURL != "" {
			raw = append(raw, "INF public "+t.PublicURL)
		}
		if t.LocalURL != "" {
			raw = append(raw, "INF local  "+t.LocalURL)
		}
		for _, line := range cfutil.RecentLogs(t.Name, 16) {
			raw = append(raw, "LOG "+truncate(line, maxInt(20, width-6)))
		}
	}
	if len(raw) == 0 {
		raw = []string{"INF selecione um túnel ou pressione n para criar"}
	}
	a.cfLogScroll = clampScroll(a.cfLogScroll, height-2, len(raw))
	start := a.cfLogScroll
	end := minInt(start+height-2, len(raw))
	lines := make([]string, 0, height-2)
	for _, line := range raw[start:end] {
		style := StyleMuted
		switch {
		case strings.HasPrefix(line, "ERR"), strings.Contains(line, "ERR"):
			style = StyleUnhealthy
		case strings.HasPrefix(line, "WRN"), strings.Contains(line, "WRN"):
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

func (a *App) renderCFAccount(width, height int) string {
	a.cfFocus = cfFocusTable
	n := len(a.cfAccount)
	a.cfAcctScroll = ensureVisible(a.cfAcctCursor, a.cfAcctScroll, height-3, n)
	lines := []string{StyleMuted.Render(truncate("NAME                 ID                                    CONNS  CREATED", width-2))}
	if !a.cfAuth.LoggedIn {
		lines = append(lines, StyleWarning.Render("  faça login (L) para listar túneis da conta"))
	} else if n == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum named tunnel — C para criar)"))
	} else {
		start := a.cfAcctScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			t := a.cfAccount[i]
			prefix := "  "
			style := StyleMuted
			if i == a.cfAcctCursor {
				prefix = "▸ "
				style = StyleSelected
			}
			created := "—"
			if !t.CreatedAt.IsZero() {
				created = t.CreatedAt.Format("2006-01-02")
			}
			row := fmt.Sprintf("%-20s %-37s %-5d  %s",
				truncate(t.Name, 20), truncate(t.ID, 37), t.Connections, created)
			lines = append(lines, style.Render(truncate(prefix+row, width-2)))
		}
	}
	return renderApiTitledBox(fmt.Sprintf("ACCOUNT TUNNELS (%d)", n), fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderCFHistory(width, height int) string {
	lines := make([]string, 0, height-2)
	if len(a.cfCfg.History) == 0 {
		lines = append(lines, StyleMuted.Render("(histórico vazio — aparece após start/stop)"))
	} else {
		for _, h := range a.cfCfg.History {
			dur := "—"
			if !h.Stopped.IsZero() && !h.Started.IsZero() {
				dur = formatUptime(h.Stopped.Sub(h.Started))
			}
			url := h.URL
			if url == "" {
				url = h.Hostname
			}
			lines = append(lines, StyleNormal.Render(fmt.Sprintf("%-12s %-24s  %-6s  %s  %s  %s",
				truncate(h.Name, 12), truncate(h.Target, 24), h.Mode, h.Started.Format("01-02 15:04"), dur, truncate(url, 28))))
		}
	}
	return renderApiTitledBox("HISTORY", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderCFSetup(width, height int) string {
	cli := StyleUnhealthy.Render("✗ não instalado")
	if a.cfAuth.CLI {
		cli = StyleHealthy.Render("✓ " + a.cfAuth.Version)
	}
	auth := StyleWarning.Render("✗ sem cert.pem")
	if a.cfAuth.LoggedIn {
		auth = StyleHealthy.Render("✓ autenticado")
	}
	lines := []string{
		StyleNormal.Render("①  INSTALL CLOUDFLARED"),
		StyleMuted.Render("    status  ") + cli,
		StyleMuted.Render("    atalho  ") + StyleKey.Render("I") + StyleMuted.Render("  baixa para ~/.local/bin"),
		"",
		StyleNormal.Render("②  LOGIN NA CONTA"),
		StyleMuted.Render("    status  ") + auth,
		StyleMuted.Render("    cert    ") + StyleMuted.Render(truncate(a.cfAuth.CertPath, maxInt(20, width-14))),
		StyleMuted.Render("    atalho  ") + StyleKey.Render("L") + StyleMuted.Render("  abre browser Cloudflare"),
		"",
		StyleNormal.Render("③  CADASTRAR TÚNEL NAMED"),
		StyleMuted.Render("    atalho  ") + StyleKey.Render("C") + StyleMuted.Render("  cloudflared tunnel create"),
		StyleMuted.Render("    route   ") + StyleKey.Render("R") + StyleMuted.Render("  DNS hostname → tunnel"),
		"",
		StyleNormal.Render("④  QUICK TUNNEL (sem login)"),
		StyleMuted.Render("    atalho  ") + StyleKey.Render("n") + StyleMuted.Render("  mode=quick · trycloudflare.com"),
		"",
		StyleMuted.Render("fluxo tipico: I → L → n (named + hostname) → R → s"),
	}
	return renderApiTitledBox("SETUP", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderCFSettings(p *core.Project, width, height int) string {
	lines := []string{
		StyleMuted.Render("CLI binary     ") + StyleNormal.Render(boolLabel(a.cfAuth.CLI)),
		StyleMuted.Render("CLI version    ") + StyleNormal.Render(a.cfAuth.Version),
		StyleMuted.Render("Origin cert    ") + StyleMuted.Render(truncate(a.cfAuth.CertPath, maxInt(20, width-18))),
		StyleMuted.Render("Logged in      ") + StyleNormal.Render(boolLabel(a.cfAuth.LoggedIn)),
		StyleMuted.Render("Config file    ") + StyleMuted.Render(".devscope/cloudflare.json"),
		StyleMuted.Render("Quick tunnels  ") + StyleHealthy.Render("trycloudflare.com"),
		StyleMuted.Render("Named tunnels  ") + StyleMuted.Render("exige login + create"),
		"",
		StyleMuted.Render("Docs           ") + StyleAccent.Render("developers.cloudflare.com/cloudflare-one"),
	}
	if p != nil {
		lines = append(lines, StyleMuted.Render("Project path   ")+StyleMuted.Render(truncate(p.Path, width-18)))
	}
	return renderApiTitledBox("SETTINGS", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderCFWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(54, width*62/100))
	boxH := minInt(height-2, maxInt(22, height*62/100))
	innerW := maxInt(28, boxW-6)
	accent := tabAccentColor(TabCFTunnel)

	lines := tunnelModalChrome("CLOUDFLARE", accent, "Novo túnel", "quick, named + hostname ou http2", proj, innerW)
	lines = append(lines, "")

	nameBox := renderApiTitledBox("nome",
		[]string{a.renderCFWizardFieldValue(a.cfNewName, cfWizName, true)},
		innerW, 3, a.cfWizardField == cfWizName,
	)
	urlBox := renderApiTitledBox("url / porta",
		[]string{a.renderCFWizardFieldValue(a.cfNewURL, cfWizURL, true)},
		innerW, 3, a.cfWizardField == cfWizURL,
	)
	hostBox := renderApiTitledBox("hostname",
		[]string{a.renderCFWizardFieldValue(a.cfNewHostname, cfWizHostname, true)},
		innerW, 3, a.cfWizardField == cfWizHostname,
	)
	modeShown := a.cfNewMode
	if a.cfWizardField == cfWizMode {
		modeShown = a.cfNewMode + "  ⟨space⟩"
	}
	modeBox := renderApiTitledBox("mode",
		[]string{a.renderCFWizardFieldValue(modeShown, cfWizMode, false)},
		innerW, 3, a.cfWizardField == cfWizMode,
	)

	preview := StyleMuted.Render("preview  ")
	name := strings.TrimSpace(a.cfNewName)
	if name == "" {
		preview += StyleMuted.Render("(preencha nome e destino)")
	} else {
		preview += StyleHealthy.Render(truncate(name, 14)) +
			StyleMuted.Render("  ·  ") +
			StyleWarning.Render(truncate(firstNonEmpty(a.cfNewURL, "?"), 22)) +
			StyleMuted.Render("  ·  ") +
			StyleNormal.Render(a.cfNewMode)
	}

	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(urlBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(hostBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(modeBox, "\n")...)
	lines = append(lines, "",
		StyleMuted.Render("projeto fixo — túnel fica ligado a "+firstNonEmpty(proj, "este projeto")),
		StyleMuted.Render("url: http://127.0.0.1:3000 ou só 4321 · localhost → 127.0.0.1"),
		StyleMuted.Render("quick = trycloudflare · named = hostname no domínio"),
		StyleMuted.Render("http2 = quick forçando --protocol http2, pra rede sem QUIC"),
		preview,
		"",
		StyleMuted.Render("tab campo  ·  ←→ cursor  ·  space mode  ·  enter salva e inicia  ·  esc"),
	)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) cfDeleteConfirmLabels() (target, detail string) {
	if a.cfSubTab == cfTabAccount {
		if a.cfAcctCursor >= 0 && a.cfAcctCursor < len(a.cfAccount) {
			t := a.cfAccount[a.cfAcctCursor]
			return t.Name, "named tunnel da conta · " + truncate(t.ID, 36)
		}
		return "—", ""
	}
	t, ok := a.cfSelected()
	if !ok {
		return "—", ""
	}
	host := t.Hostname
	if host == "" {
		host = publicHostFromURL(t.PublicURL)
	}
	detail = fmt.Sprintf("%s  %s", firstNonEmpty(t.Mode, "quick"), firstNonEmpty(host, t.LocalURL))
	return t.Name, detail
}

func (a *App) renderCFWizardFieldValue(value string, field int, editable bool) string {
	focused := a.cfWizardField == field
	if !focused {
		return StyleNormal.Render(value)
	}
	if !editable {
		return StyleSelected.Render(value)
	}
	runes := []rune(value)
	cur := a.cfWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return StyleSelected.Render(shown)
}

func (a *App) beginCFWizard(_ *core.Project) {
	if a.cfNewName == "" {
		a.cfNewName = "api"
	}
	if a.cfNewMode == "" {
		a.cfNewMode = "quick"
	}
	if a.cfNewURL == "" {
		a.cfNewURL = cfDefaultURL
	}
	a.cfWizard = true
	a.cfWizardField = cfWizName
	a.cfWizardCursor = len([]rune(a.cfNewName))
}

func (a *App) cfWizardText() string {
	switch a.cfWizardField {
	case cfWizName:
		return a.cfNewName
	case cfWizURL:
		return a.cfNewURL
	case cfWizHostname:
		return a.cfNewHostname
	default:
		return ""
	}
}

func (a *App) setCFWizardText(s string) {
	switch a.cfWizardField {
	case cfWizName:
		a.cfNewName = s
	case cfWizURL:
		a.cfNewURL = s
	case cfWizHostname:
		a.cfNewHostname = s
	}
}

func (a *App) cfWizardFocusField(field int) {
	if field < cfWizName {
		field = cfWizMode
	}
	if field > cfWizMode {
		field = cfWizName
	}
	a.cfWizardField = field
	if field == cfWizMode {
		a.cfWizardCursor = 0
		return
	}
	a.cfWizardCursor = len([]rune(a.cfWizardText()))
}

func (a *App) cycleCFMode() {
	for i, m := range cfutil.Modes {
		if m == a.cfNewMode {
			a.cfNewMode = cfutil.Modes[(i+1)%len(cfutil.Modes)]
			return
		}
	}
	a.cfNewMode = cfutil.Modes[0]
}

func (a *App) cfSelected() (cfutil.Tunnel, bool) {
	if a.cfCursor < 0 || a.cfCursor >= len(a.cfTunnels) {
		return cfutil.Tunnel{}, false
	}
	return a.cfTunnels[a.cfCursor], true
}

func openBrowser(url string) {
	bin := "xdg-open"
	if runtime.GOOS == "darwin" {
		bin = "open"
	}
	_ = exec.Command(bin, url).Start()
}

// cfStarted reporta o resultado do start e já abre a URL pública no browser.
func cfStarted(name, pub string) cfActionMsg {
	if pub == "" {
		return cfActionMsg{out: name + " iniciado — aguardando URL pública"}
	}
	openBrowser(pub)
	return cfActionMsg{out: "online " + pub}
}

// cfLocalTarget resolves the local destination cloudflared should expose.
func cfLocalTarget(t cfutil.Tunnel) string {
	if t.LocalURL != "" {
		return cfutil.NormalizeURL(t.LocalURL)
	}
	if t.Port > 0 {
		return cfutil.NormalizeURL(strconv.Itoa(t.Port))
	}
	return ""
}

func cfTunnelInConfig(cfg cfutil.ProjectConfig, t cfutil.Tunnel) bool {
	for _, c := range cfg.Tunnels {
		if c.Name == t.Name || (c.Port > 0 && c.Port == t.Port) {
			return true
		}
	}
	return false
}

func (a *App) handleCFKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.cfConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			return a, a.cfDeleteSelected(p)
		case "n", "N", "esc":
			a.cfConfirmDelete = false
			return a, nil
		}
		return a, nil
	}
	if a.cfWizard {
		return a.updateCFWizard(msg, p)
	}

	switch msg.String() {
	case "esc":
		return a, a.leaveCFTab()
	case "tab":
		if a.cfSubTab == cfTabTunnels {
			a.cfFocus = (a.cfFocus + 1) % 3 // table → details → logs
		}
	case "0":
		a.cfSubTab = cfTabOverview
	case "1":
		a.cfSubTab = cfTabTunnels
		a.cfFocus = cfFocusTable
	case "2":
		a.cfSubTab = cfTabAccount
	case "3":
		a.cfSubTab = cfTabHistory
	case "4":
		a.cfSubTab = cfTabSetup
	case "5":
		a.cfSubTab = cfTabSettings
	case "up", "k":
		return a, a.cfMove(-1)
	case "down", "j":
		return a, a.cfMove(1)
	case "n":
		a.cfNewName = "api"
		a.cfNewMode = "quick"
		a.cfNewHostname = ""
		a.cfNewURL = ""
		a.beginCFWizard(p)
	case "e":
		if t, ok := a.cfSelected(); ok {
			a.cfNewName = t.Name
			a.cfNewURL = cfLocalTarget(t)
			a.cfNewHostname = t.Hostname
			a.cfNewMode = t.Mode
			if a.cfNewMode == "" {
				a.cfNewMode = "quick"
			}
			a.beginCFWizard(p)
		}
	case "s":
		return a, a.cfStartSelected(p)
	case "x":
		return a, a.cfStopSelected()
	case "r":
		if a.cfFocus == cfFocusTable && a.cfSubTab == cfTabTunnels {
			return a, a.cfRestartSelected(p)
		}
		return a, a.refreshCF(p)
	case "c":
		return a, a.cfCopyURL()
	case "o", "O":
		return a, a.cfOpenBrowser()
	case "I":
		return a, a.cfInstall()
	case "L":
		return a, a.cfLogin()
	case "C":
		return a, a.cfCreateNamed(p)
	case "R":
		return a, a.cfRouteSelected()
	case "A", "shift+a", "shift+A":
		a.cfShowAll = !a.cfShowAll
		if a.cfShowAll {
			a.cfStatus = "mostrando todos os túneis do host"
		} else {
			a.cfStatus = "filtrando túneis do projeto"
		}
		return a, a.refreshCF(p)
	case "d":
		if a.cfSubTab == cfTabAccount {
			if a.cfAcctCursor >= 0 && a.cfAcctCursor < len(a.cfAccount) {
				a.cfConfirmDelete = true
				a.cfStatus = "delete named tunnel da conta?"
			}
			return a, nil
		}
		if t, ok := a.cfSelected(); ok {
			if a.cfShowAll && !cfTunnelInConfig(a.cfCfg, t) {
				a.cfStatus = "túnel externo — só stop (x)"
				return a, nil
			}
			a.cfConfirmDelete = true
			a.cfStatus = "delete túnel da config?"
		}
	case "ctrl+r":
		return a, a.refreshCF(p)
	}
	return a, nil
}

func (a *App) cfMove(delta int) tea.Cmd {
	switch {
	case a.cfSubTab == cfTabAccount:
		a.cfAcctCursor += delta
		if a.cfAcctCursor < 0 {
			a.cfAcctCursor = 0
		}
		if a.cfAcctCursor > len(a.cfAccount)-1 {
			a.cfAcctCursor = maxInt(0, len(a.cfAccount)-1)
		}
	case a.cfFocus == cfFocusDetails:
		a.cfDetailsScroll += delta
		if a.cfDetailsScroll < 0 {
			a.cfDetailsScroll = 0
		}
	case a.cfFocus == cfFocusLogs:
		a.cfLogScroll += delta
		if a.cfLogScroll < 0 {
			a.cfLogScroll = 0
		}
	default:
		prev := a.cfCursor
		a.cfCursor += delta
		if a.cfCursor < 0 {
			a.cfCursor = 0
		}
		if a.cfCursor > len(a.cfTunnels)-1 {
			a.cfCursor = maxInt(0, len(a.cfTunnels)-1)
		}
		if a.cfCursor != prev {
			a.cfDetailsScroll = 0
			a.cfLogScroll = 0
		}
	}
	return nil
}

func (a *App) updateCFWizard(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.cfWizard = false
		return a, nil
	case "enter":
		name := strings.TrimSpace(a.cfNewName)
		if name == "" {
			a.cfStatus = "nome vazio"
			return a, nil
		}
		url := cfutil.NormalizeURL(a.cfNewURL)
		if url == "" {
			a.cfStatus = "URL inválida — ex: http://localhost:4321"
			a.cfWizardField = cfWizURL
			a.cfWizardCursor = len([]rune(a.cfNewURL))
			return a, nil
		}
		if a.cfNewMode == "named" && strings.TrimSpace(a.cfNewHostname) == "" {
			a.cfStatus = "named exige hostname"
			a.cfWizardField = cfWizHostname
			a.cfWizardCursor = len([]rune(a.cfNewHostname))
			return a, nil
		}
		a.cfNewName = name
		a.cfNewURL = url
		a.cfNewHostname = strings.TrimSpace(strings.ToLower(a.cfNewHostname))
		a.cfWizard = false
		return a, a.cfCreateAndStart(p)
	case "tab", "down":
		a.cfWizardFocusField(a.cfWizardField + 1)
		return a, nil
	case "shift+tab", "up":
		a.cfWizardFocusField(a.cfWizardField - 1)
		return a, nil
	case " ":
		if a.cfWizardField == cfWizMode {
			a.cycleCFMode()
		}
		return a, nil
	}

	if a.cfWizardField == cfWizMode {
		switch msg.String() {
		case "left", "right":
			a.cycleCFMode()
		}
		return a, nil
	}

	text := a.cfWizardText()
	runes := []rune(text)
	cur := a.cfWizardCursor
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
			a.setCFWizardText(string(runes))
		}
	case "delete":
		if cur < len(runes) {
			runes = append(runes[:cur], runes[cur+1:]...)
			a.setCFWizardText(string(runes))
		}
	default:
		if len(msg.Runes) > 0 {
			inserted := append([]rune(nil), msg.Runes...)
			runes = append(runes[:cur], append(inserted, runes[cur:]...)...)
			cur += len(inserted)
			a.setCFWizardText(string(runes))
		}
	}
	a.cfWizardCursor = cur
	return a, nil
}

func (a *App) cfCreateAndStart(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	name := strings.TrimSpace(a.cfNewName)
	mode := a.cfNewMode
	hostname := a.cfNewHostname
	url := cfutil.NormalizeURL(a.cfNewURL)
	if url == "" {
		url = cfDefaultURL
	}
	cfg := a.cfCfg
	cfg.Project = p.Name
	cfg.UpsertTunnel(cfutil.TunnelConfig{Name: name, URL: url, Mode: mode, Hostname: hostname})
	_ = cfutil.SaveProject(p.Path, cfg)
	a.cfCfg = cfg
	a.cfLoading = true
	a.cfStatus = "starting " + name + "…"
	return func() tea.Msg {
		if mode == "named" {
			if _, err := cfutil.CreateTunnel(name); err != nil && !strings.Contains(strings.ToLower(err.Error()), "already exists") {
				// continue — tunnel may already exist
				if !cfutil.LoggedIn() {
					return cfActionMsg{err: err.Error()}
				}
			}
			if hostname != "" {
				_ = cfutil.RouteDNS(name, hostname)
			}
		}
		pub, err := cfutil.StartTunnel(name, url, mode, hostname)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		cfg := cfutil.LoadProject(p.Path, p.Name)
		cfg.History = append([]cfutil.HistoryEntry{{
			Name: name, Target: url, Hostname: hostname, Mode: mode, Started: time.Now(), URL: pub,
		}}, cfg.History...)
		if len(cfg.History) > 40 {
			cfg.History = cfg.History[:40]
		}
		_ = cfutil.SaveProject(p.Path, cfg)
		return cfStarted(name, pub)
	}
}

func (a *App) cfStartSelected(p *core.Project) tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		return nil
	}
	if t.Status == "online" {
		a.cfStatus = t.Name + " já online"
		return nil
	}
	target := cfLocalTarget(t)
	a.cfLoading = true
	return func() tea.Msg {
		pub, err := cfutil.StartTunnel(t.Name, target, t.Mode, t.Hostname)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		if p != nil {
			cfg := cfutil.LoadProject(p.Path, p.Name)
			cfg.UpsertTunnel(cfutil.TunnelConfig{Name: t.Name, URL: target, Mode: t.Mode, Hostname: t.Hostname, TunnelID: t.TunnelID})
			cfg.History = append([]cfutil.HistoryEntry{{
				Name: t.Name, Target: target, Hostname: t.Hostname, Mode: t.Mode, Started: time.Now(), URL: pub,
			}}, cfg.History...)
			if len(cfg.History) > 40 {
				cfg.History = cfg.History[:40]
			}
			_ = cfutil.SaveProject(p.Path, cfg)
		}
		return cfStarted(t.Name, pub)
	}
}

func (a *App) cfStopSelected() tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		return nil
	}
	a.cfLoading = true
	return func() tea.Msg {
		err := cfutil.StopTunnel(t.Name)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfActionMsg{out: "stopped " + t.Name}
	}
}

func (a *App) cfRestartSelected(p *core.Project) tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		return nil
	}
	target := cfLocalTarget(t)
	a.cfLoading = true
	return func() tea.Msg {
		_ = cfutil.StopTunnel(t.Name)
		time.Sleep(400 * time.Millisecond)
		pub, err := cfutil.StartTunnel(t.Name, target, t.Mode, t.Hostname)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfStarted(t.Name, pub)
	}
}

func (a *App) cfDeleteSelected(p *core.Project) tea.Cmd {
	a.cfConfirmDelete = false
	if a.cfSubTab == cfTabAccount {
		if a.cfAcctCursor < 0 || a.cfAcctCursor >= len(a.cfAccount) {
			return nil
		}
		t := a.cfAccount[a.cfAcctCursor]
		a.cfLoading = true
		return func() tea.Msg {
			if err := cfutil.DeleteAccountTunnel(t.Name); err != nil {
				return cfActionMsg{err: err.Error()}
			}
			return cfActionMsg{out: "deleted account tunnel " + t.Name}
		}
	}
	t, ok := a.cfSelected()
	if !ok || p == nil {
		return nil
	}
	_ = cfutil.StopTunnel(t.Name)
	cfg := a.cfCfg
	cfg.RemoveTunnel(t.Name)
	_ = cfutil.SaveProject(p.Path, cfg)
	a.cfCfg = cfg
	return a.refreshCF(p)
}

func (a *App) cfInstall() tea.Cmd {
	a.cfLoading = true
	a.cfStatus = "instalando cloudflared…"
	return func() tea.Msg {
		out, err := cfutil.Install()
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfActionMsg{out: out}
	}
}

func (a *App) cfLogin() tea.Cmd {
	a.cfLoading = true
	a.cfStatus = "abrindo login Cloudflare…"
	return func() tea.Msg {
		err := cfutil.Login()
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfActionMsg{out: "login ok — cert.pem salvo"}
	}
}

func (a *App) cfCreateNamed(p *core.Project) tea.Cmd {
	name := "api"
	url := cfDefaultURL
	if t, ok := a.cfSelected(); ok {
		name = t.Name
		if target := cfLocalTarget(t); target != "" {
			url = target
		}
	} else if a.cfNewName != "" {
		name = a.cfNewName
	}
	path, proj := "", ""
	if p != nil {
		path, proj = p.Path, p.Name
	}
	a.cfLoading = true
	a.cfStatus = "criando tunnel " + name + "…"
	return func() tea.Msg {
		created, err := cfutil.CreateTunnel(name)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		if path != "" {
			cfg := cfutil.LoadProject(path, proj)
			cfg.UpsertTunnel(cfutil.TunnelConfig{
				Name: created.Name, URL: url, Mode: "named", TunnelID: created.ID,
			})
			_ = cfutil.SaveProject(path, cfg)
		}
		id := created.ID
		if id == "" {
			id = created.Name
		}
		return cfActionMsg{out: "created " + created.Name + " (" + truncate(id, 12) + ")"}
	}
}

func (a *App) cfRouteSelected() tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		a.cfErr = "selecione um túnel"
		return nil
	}
	hostname := t.Hostname
	if hostname == "" {
		hostname = a.cfNewHostname
	}
	if hostname == "" {
		a.cfErr = "hostname vazio — edite o túnel (e) e preencha Host"
		return nil
	}
	a.cfLoading = true
	return func() tea.Msg {
		err := cfutil.RouteDNS(t.Name, hostname)
		if err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfActionMsg{out: "dns " + hostname + " → " + t.Name}
	}
}

func (a *App) cfCopyURL() tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		return nil
	}
	url := t.PublicURL
	if url == "" {
		url = cfutil.PublicURL(t.Name)
	}
	if url == "" && t.Hostname != "" {
		url = "https://" + t.Hostname
	}
	if url == "" {
		a.cfErr = "URL vazia"
		return nil
	}
	if err := copyToClipboard(url); err != nil {
		a.cfErr = "clipboard: " + err.Error()
		return nil
	}
	a.cfStatus = "copied " + truncate(url, 40)
	return nil
}

func (a *App) cfOpenBrowser() tea.Cmd {
	t, ok := a.cfSelected()
	if !ok {
		a.cfErr = "selecione um túnel"
		return nil
	}
	url := t.PublicURL
	if url == "" {
		url = cfutil.PublicURL(t.Name)
	}
	if url == "" && t.Hostname != "" {
		url = "https://" + t.Hostname
	}
	if url == "" {
		a.cfErr = "sem URL pública"
		return nil
	}
	openBrowser(url)
	a.cfStatus = "abrindo " + truncate(url, 40)
	return nil
}
