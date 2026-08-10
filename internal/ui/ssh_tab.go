package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/sshutil"
)

const (
	sshWizName = iota
	sshWizMode
	sshWizLocalPort
	sshWizBind
	sshWizTarget
	sshWizIdentity
)

type sshSubTab int

const (
	sshTabOverview sshSubTab = iota
	sshTabTunnels
	sshTabHistory
	sshTabSettings
)

type sshFocus int

const (
	sshFocusTable sshFocus = iota
	sshFocusDetails
	sshFocusLogs
)

type sshLoadedMsg struct {
	tunnels []sshutil.Tunnel
	cfg     sshutil.ProjectConfig
	foreign int
	err     string
}

type sshActionMsg struct {
	out string
	err string
}

func (a *App) enterSSHTab(_ *core.Project) {
	a.tab = TabSSH
	a.tabCursor = 0
	a.sshOpen = false
}

func (a *App) openSSHClient(p *core.Project) tea.Cmd {
	a.sshOpen = true
	a.sshSubTab = sshTabTunnels
	a.sshFocus = sshFocusTable
	a.sshCursor = 0
	a.sshScroll = 0
	a.sshLogScroll = 0
	a.sshDetailsScroll = 0
	a.sshErr = ""
	a.sshStatus = ""
	a.sshWizard = false
	a.sshConfirmDelete = false
	a.sshSeedWizard = true
	a.seedSSHDefaults(p)
	return a.refreshSSH(p)
}

// seedSSHDefaults preenche remote (−R) com a porta do projeto: abre no servidor
// e aponta pro serviço local do PC.
func (a *App) seedSSHDefaults(p *core.Project) {
	name, ports, framework, remotes := "app", []int(nil), "", []string(nil)
	if p != nil {
		name = firstNonEmpty(p.Name, "app")
		ports = p.Ports
		framework = p.Framework.Name
		if p.Git != nil {
			if p.Git.Remote != "" {
				remotes = append(remotes, p.Git.Remote)
			}
			for _, r := range p.Git.Remotes {
				if r.URL != "" {
					remotes = append(remotes, r.URL)
				}
			}
		}
	}
	def := sshutil.DefaultRemoteTunnel(name, ports, framework, "")
	a.sshNewMode = sshutil.ModeRemote
	a.sshNewName = def.Name
	a.sshNewLocalPort = def.LocalPort
	a.sshNewLocalPortStr = strconv.Itoa(def.LocalPort)
	a.sshNewBind = fmt.Sprintf("%s:%d", def.RemoteHost, def.RemotePort)

	// Target: último túnel salvo → git remote (VPS) → mantém o que já estava.
	target := ""
	for _, t := range a.sshCfg.Tunnels {
		if strings.TrimSpace(t.Target) != "" {
			target = t.Target
			break
		}
	}
	if target == "" {
		for _, h := range a.sshCfg.History {
			if strings.TrimSpace(h.Target) != "" {
				target = h.Target
				break
			}
		}
	}
	if target == "" {
		target = sshutil.SuggestSSHTarget(remotes...)
	}
	if target != "" {
		a.sshNewTarget = target
	}
}

func (a *App) leaveSSHTab() tea.Cmd {
	a.sshOpen = false
	a.sshWizard = false
	a.sshConfirmDelete = false
	a.tab = TabSSH
	a.tabCursor = 0
	return nil
}

func (a *App) refreshSSH(p *core.Project) tea.Cmd {
	a.sshLoading = true
	path, name := "", "project"
	if p != nil {
		path, name = p.Path, p.Name
	}
	showAll := a.sshShowAll
	return func() tea.Msg {
		cfg := sshutil.LoadProject(path, name)
		live := sshutil.ListLiveTunnels()
		foreign := sshutil.CountForeignLive(cfg, live)
		tunnels := sshutil.MergeTunnels(cfg, live)
		if showAll {
			tunnels = sshutil.MergeTunnelsAll(cfg, live)
		}
		return sshLoadedMsg{tunnels: tunnels, cfg: cfg, foreign: foreign}
	}
}

func (a *App) handleSSHMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case sshLoadedMsg:
		a.sshLoading = false
		a.sshCfg = m.cfg
		a.sshTunnels = m.tunnels
		a.sshForeign = m.foreign
		if m.err != "" {
			a.sshErr = m.err
		} else {
			a.sshErr = ""
		}
		if a.sshCursor >= len(a.sshTunnels) {
			a.sshCursor = maxInt(0, len(a.sshTunnels)-1)
		}
		// Sem túneis no projeto → abre wizard remote já com porta do app.
		if a.sshSeedWizard {
			a.sshSeedWizard = false
			a.seedSSHDefaults(a.currentProject())
			if len(a.sshTunnels) == 0 && !a.sshWizard {
				a.beginSSHWizard(a.currentProject())
			}
		}
	case sshActionMsg:
		a.sshLoading = false
		a.sshConfirmDelete = false
		if m.err != "" {
			a.sshErr = m.err
			a.sshStatus = ""
			return a, nil
		}
		a.sshErr = ""
		a.sshStatus = truncate(m.out, 60)
		return a, a.refreshSSH(a.currentProject())
	}
	return a, nil
}

func (a *App) renderSSHLanding(p *core.Project) string {
	w, h := a.moduleSize()
	available := a.landingSSHAvail
	liveN := a.landingSSHLive
	status := "…"
	if a.landingSSHOK {
		status = "offline"
		if liveN > 0 {
			status = "connected"
		}
	}
	ctx := a.renderModuleContext(p, w, "SSH TUNNEL", status)
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(6, bodyH*35/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("padrão: remote (−R) — abre porta no servidor → seu PC/projeto"),
	}
	openLines = append(openLines, moduleOpenHint()...)
	switch {
	case !a.landingSSHOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando ambiente…"))
	case !available:
		openLines = append(openLines, "", StyleUnhealthy.Render("ssh não encontrado no PATH"))
	default:
		openLines = append(openLines, "", StyleMuted.Render("cliente  ")+StyleNormal.Render(firstNonEmpty(a.landingSSHVer, "OpenSSH")))
		if liveN > 0 {
			openLines = append(openLines, StyleHealthy.Render(fmt.Sprintf("● %d túnel(is) ativo(s) nesta sessão", liveN)))
		} else {
			openLines = append(openLines, StyleMuted.Render("○ nenhum túnel ativo — n cria e s inicia"))
		}
	}
	featLines := []string{
		StyleMuted.Render("remote (−R) padrão · local (−L) · dynamic SOCKS (−D)"),
		StyleMuted.Render("porta do projeto no PC exposta no servidor"),
		StyleMuted.Render("start/stop/restart · logs · copy forward"),
		StyleMuted.Render("config em .devscope/ssh.json"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("SSH TUNNEL", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	cliLabel, liveLabel := "…", "…"
	if a.landingSSHOK {
		cliLabel, liveLabel = boolLabel(available), fmt.Sprintf("%d", liveN)
	}
	details := []string{
		StyleMuted.Render("CLI     ") + StyleNormal.Render(cliLabel),
		StyleMuted.Render("Live    ") + StyleNormal.Render(liveLabel),
	}
	if p != nil && len(p.Ports) > 0 {
		details = append(details, StyleMuted.Render("ports  ")+StyleAccent.Render(fmt.Sprintf("%v", p.Ports)))
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir console"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderSSHTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderSSHHeader(p, w)
	nav := a.renderSSHNav(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(nav)
	bodyH := maxInt(4, h-headerH-2)
	cmdW := actionsCmdWidth(w)
	mainW := w
	if cmdW > 0 {
		mainW = maxInt(36, w-cmdW)
	}

	var body string
	switch a.sshSubTab {
	case sshTabOverview:
		body = a.renderSSHOverview(p, mainW, bodyH)
	case sshTabHistory:
		body = a.renderSSHHistory(mainW, bodyH)
	case sshTabSettings:
		body = a.renderSSHSettings(p, mainW, bodyH)
	default:
		body = a.renderSSHTunnelsView(p, mainW, bodyH)
	}
	if cmdW > 0 {
		side := a.renderSSHCommands(cmdW, bodyH)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, side)
	}
	view := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, a.renderStatusBar(a.sshHints()))
	if a.sshWizard {
		view = overlayCentered(view, a.renderSSHWizard(p, w, h), w, h)
	}
	if a.sshConfirmDelete {
		target, detail := a.sshDeleteConfirmLabels()
		box := renderTunnelDeleteConfirmBox("SSH", tabAccentColor(TabSSH), target, detail, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) renderSSHCommands(width, height int) string {
	return renderActionsBox(width, height,
		[2]string{"n", "novo túnel"},
		[2]string{"e", "editar"},
		[2]string{"s", "start"},
		[2]string{"x", "stop"},
		[2]string{"r", "restart"},
		[2]string{"c", "copy forward"},
		[2]string{"d", "delete"},
		[2]string{"A", "todos/projeto"},
		[2]string{"0-3", "abas"},
		[2]string{"tab", "foco painéis"},
		[2]string{"ctrl+r", "refresh"},
		[2]string{"esc", "voltar"},
	)
}

func (a *App) sshHints() string {
	if a.sshConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.sshWizard {
		return "modal novo túnel  tab campo  ←→ cursor  space mode  enter salvar+start  esc"
	}
	scope := "A todos"
	if a.sshShowAll {
		scope = "A projeto"
	}
	base := "0-3 aba  tab lista/detalhes/logs  n new  s start  x stop  r restart  c copy  d delete  " + scope + "  esc"
	if a.sshLoading {
		base = a.spinner() + " carregando…  " + base
	}
	if a.sshStatus != "" {
		return truncate(a.sshStatus, 36) + "  ·  " + base
	}
	if a.sshErr != "" {
		return StyleUnhealthy.Render(truncate(a.sshErr, 40)) + "  ·  " + base
	}
	return base
}

func (a *App) renderSSHHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabSSH)).Bold(true)
	name := "project"
	if p != nil {
		name = p.Name
	}
	env := projectEnvLabel(p)
	left := accent.Render("devscope") + StyleMuted.Render(" › ssh") +
		StyleMuted.Render("  ·  ") + StyleNormal.Render(name) +
		StyleMuted.Render("  ") + StyleMuted.Render(env)
	online := 0
	for _, t := range a.sshTunnels {
		if t.Status == "online" {
			online++
		}
	}
	right := StyleHealthy.Render(fmt.Sprintf("%d online", online)) +
		StyleMuted.Render(fmt.Sprintf(" / %d", len(a.sshTunnels)))
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right))
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderSSHNav(width int) string {
	names := []string{"Overview", "Tunnels", "History", "Settings"}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf(" %d:%s ", i, n)
		if sshSubTab(i) == a.sshSubTab {
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

func (a *App) renderSSHOverview(p *core.Project, width, height int) string {
	rightW := a.moduleRightWidth(width)
	centerW := maxInt(36, width-rightW-1)
	online, offline := 0, 0
	for _, t := range a.sshTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	sumH := maxInt(8, height*45/100)
	listH := maxInt(6, height-sumH)
	cli := "ausente"
	if sshutil.Available() {
		cli = firstNonEmpty(sshutil.Version(), "ok")
	}
	lines := []string{
		StyleMuted.Render("CLI        ") + StyleNormal.Render(cli),
		StyleMuted.Render("Tunnels    ") + StyleHealthy.Render(fmt.Sprintf("%d online", online)) +
			StyleMuted.Render(" / ") + StyleUnhealthy.Render(fmt.Sprintf("%d offline", offline)),
		StyleMuted.Render("Config     ") + StyleMuted.Render(".devscope/ssh.json"),
		StyleMuted.Render("Default    ") + StyleNormal.Render("remote (−R) · porta do projeto"),
	}
	if p != nil && len(p.Ports) > 0 {
		lines = append(lines, StyleMuted.Render("Proj ports ")+StyleAccent.Render(fmt.Sprintf("%v", p.Ports)))
	}
	evLines := make([]string, 0, listH-2)
	if len(a.sshCfg.History) == 0 {
		evLines = append(evLines, StyleMuted.Render("(sem histórico recente)"))
	} else {
		n := minInt(listH-2, len(a.sshCfg.History))
		for i := 0; i < n; i++ {
			h := a.sshCfg.History[i]
			evLines = append(evLines, StyleMuted.Render(h.Started.Format("15:04"))+" "+
				StyleNormal.Render(fmt.Sprintf("%s  %s  :%d", h.Name, h.Mode, h.LocalPort)))
		}
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("OVERVIEW", fitExactLines(lines, sumH-2), centerW, sumH, false),
		renderApiTitledBox("RECENT", fitExactLines(evLines, listH-2), centerW, listH, false),
	)
	details := []string{
		StyleHealthy.Render(fmt.Sprintf("online   %d", online)),
		StyleUnhealthy.Render(fmt.Sprintf("offline  %d", offline)),
	}
	actions := moduleActionLines(
		[2]string{"1", "túneis"},
		[2]string{"n", "novo túnel"},
		[2]string{"s", "start"},
		[2]string{"x", "stop"},
		[2]string{"e", "editar"},
		[2]string{"d", "delete"},
		[2]string{"r", "refresh"},
	)
	right := a.renderModuleRightRail(rightW, height, details, actions)
	return lipgloss.JoinHorizontal(lipgloss.Top, center, right)
}

func (a *App) renderSSHTunnelsView(p *core.Project, width, height int) string {
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
	left := a.renderSSHTunnelTable(leftW, height)
	right := lipgloss.JoinVertical(lipgloss.Left,
		a.renderSSHDetailsPane(rightW, detailsH),
		a.renderSSHLogsPane(rightW, logsH),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (a *App) renderSSHTunnelTable(width, height int) string {
	focus := a.sshFocus == sshFocusTable
	n := len(a.sshTunnels)
	a.sshScroll = ensureVisible(a.sshCursor, a.sshScroll, height-3, n)
	nameW := maxInt(8, width-24)
	header := fmt.Sprintf("%-3s %-*s %4s %-7s", "ST", nameW, "NAME", "PORT", "MODE")
	lines := []string{StyleMuted.Render(truncate(header, width-2))}
	if n == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhum túnel — n para criar)"))
	} else {
		start := a.sshScroll
		end := minInt(start+height-3, n)
		for i := start; i < end; i++ {
			t := a.sshTunnels[i]
			dot := StyleUnhealthy.Render("●")
			switch t.Status {
			case "online":
				dot = StyleHealthy.Render(a.pulse())
			case "starting":
				dot = StyleWarning.Render(a.spinner())
			}
			row := fmt.Sprintf("%-*s %4d %-7s",
				nameW, truncate(t.Name, nameW), t.LocalPort, truncate(t.Mode, 7),
			)
			prefix := "  "
			style := StyleMuted
			if i == a.sshCursor {
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

func (a *App) renderSSHDetailsPane(width, height int) string {
	focus := a.sshFocus == sshFocusDetails
	var raw []string
	t, ok := a.sshSelected()
	if !ok {
		raw = []string{StyleMuted.Render("(selecione um túnel na lista)")}
	} else {
		fwd := t.Forward
		if fwd == "" {
			fwd = sshutil.FormatForward(t.Mode, t.LocalPort, t.RemoteHost, t.RemotePort)
		}
		raw = []string{
			tunnelStatusBadge(t.Status, a.animFrame),
			"",
			tunnelDetailKV("name", t.Name),
			tunnelDetailKV("mode", t.Mode),
			tunnelDetailKV("local", fmt.Sprintf(":%d", t.LocalPort)),
			tunnelDetailKV("bind", fmt.Sprintf("%s:%d", firstNonEmpty(t.RemoteHost, "127.0.0.1"), t.RemotePort)),
			tunnelDetailKV("target", t.Target),
			tunnelDetailKV("forward", fwd),
			tunnelDetailKV("identity", firstNonEmpty(t.Identity, "(default)")),
			tunnelDetailKV("pid", fmt.Sprintf("%d", t.PID)),
			tunnelDetailKV("uptime", t.Uptime),
		}
	}
	title := "DETALHES"
	if focus {
		title = "> " + title
	}
	a.sshDetailsScroll = ensureVisible(0, a.sshDetailsScroll, height-2, len(raw))
	start := a.sshDetailsScroll
	end := minInt(start+height-2, len(raw))
	if start > end {
		start = 0
	}
	return renderApiTitledBox(title, fitExactLines(raw[start:end], height-2), width, height, focus)
}

func (a *App) renderSSHLogsPane(width, height int) string {
	focus := a.sshFocus == sshFocusLogs
	var lines []string
	t, ok := a.sshSelected()
	if !ok || t.Status != "online" {
		lines = []string{StyleMuted.Render("(logs quando o túnel estiver online)")}
	} else {
		logs := sshutil.RecentLogs(t.Name, 40)
		if len(logs) == 0 {
			lines = []string{StyleMuted.Render("(ssh −N sem saída — forwards silenciosos)")}
		} else {
			for _, l := range logs {
				lines = append(lines, StyleMuted.Render(truncate(l, width-4)))
			}
		}
	}
	title := "LOGS"
	if focus {
		title = "> " + title
	}
	a.sshLogScroll = ensureVisible(0, a.sshLogScroll, height-2, len(lines))
	start := a.sshLogScroll
	end := minInt(start+height-2, len(lines))
	if start > end {
		start = 0
	}
	return renderApiTitledBox(title, fitExactLines(lines[start:end], height-2), width, height, focus)
}

func (a *App) renderSSHHistory(width, height int) string {
	lines := []string{StyleMuted.Render("STARTED  NAME          MODE     PORT  TARGET")}
	if len(a.sshCfg.History) == 0 {
		lines = append(lines, StyleMuted.Render("(vazio — starts ficam registrados aqui)"))
	} else {
		for _, h := range a.sshCfg.History {
			lines = append(lines, StyleNormal.Render(fmt.Sprintf("%s  %-12s  %-7s  %4d  %s",
				h.Started.Format("01-02 15:04"),
				truncate(h.Name, 12),
				truncate(h.Mode, 7),
				h.LocalPort,
				truncate(h.Target, maxInt(8, width-48)),
			)))
		}
	}
	return renderApiTitledBox("HISTORY", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderSSHSettings(p *core.Project, width, height int) string {
	lines := []string{
		StyleMuted.Render("Config file    ") + StyleMuted.Render(".devscope/ssh.json"),
		StyleMuted.Render("CLI            ") + StyleNormal.Render(firstNonEmpty(sshutil.Version(), "(ssh)")),
		StyleMuted.Render("Default mode   ") + StyleNormal.Render("remote (−R)"),
		StyleMuted.Render("Host keys       ") + StyleMuted.Render("StrictHostKeyChecking=accept-new"),
		StyleMuted.Render("Alive           ") + StyleMuted.Render("ServerAliveInterval=30"),
		StyleMuted.Render("Fail forward    ") + StyleMuted.Render("ExitOnForwardFailure=yes"),
	}
	if p != nil {
		lines = append(lines, StyleMuted.Render("Project path   ")+StyleMuted.Render(truncate(p.Path, width-18)))
	}
	return renderApiTitledBox("SETTINGS", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderSSHWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(52, width*58/100))
	boxH := minInt(height-2, maxInt(22, height*60/100))
	innerW := maxInt(28, boxW-6)
	accent := tabAccentColor(TabSSH)

	subtitle := "remote (−R): porta no servidor → app no seu PC"
	switch a.sshNewMode {
	case sshutil.ModeLocal:
		subtitle = "local (−L): porta no PC → serviço no servidor"
	case sshutil.ModeDynamic:
		subtitle = "dynamic (−D): SOCKS no PC"
	}
	lines := tunnelModalChrome("SSH", accent, "Novo túnel", subtitle, proj, innerW)
	lines = append(lines, "")

	nameBox := renderApiTitledBox("nome",
		[]string{a.renderSSHWizardFieldValue(a.sshNewName, sshWizName, true)},
		innerW, 3, a.sshWizardField == sshWizName,
	)
	modeShown := a.sshNewMode
	if a.sshWizardField == sshWizMode {
		modeShown = a.sshNewMode + "  ⟨space⟩"
	}
	modeBox := renderApiTitledBox("mode",
		[]string{a.renderSSHWizardFieldValue(modeShown, sshWizMode, false)},
		innerW, 3, a.sshWizardField == sshWizMode,
	)
	portLabel := "porta no servidor (−R)"
	bindLabel := "destino no PC (host:porta)"
	switch a.sshNewMode {
	case sshutil.ModeLocal:
		portLabel = "porta local (−L)"
		bindLabel = "destino remoto (host:porta)"
	case sshutil.ModeDynamic:
		portLabel = "porta SOCKS local"
		bindLabel = "destino"
	}
	portBox := renderApiTitledBox(portLabel,
		[]string{a.renderSSHWizardFieldValue(a.sshNewLocalPortStr, sshWizLocalPort, true)},
		innerW, 3, a.sshWizardField == sshWizLocalPort,
	)
	bindVal := a.sshNewBind
	if a.sshNewMode == sshutil.ModeDynamic {
		bindVal = "(não usado em dynamic)"
	}
	bindBox := renderApiTitledBox(bindLabel,
		[]string{a.renderSSHWizardFieldValue(bindVal, sshWizBind, a.sshNewMode != sshutil.ModeDynamic)},
		innerW, 3, a.sshWizardField == sshWizBind,
	)
	targetBox := renderApiTitledBox("target (user@host)",
		[]string{a.renderSSHWizardFieldValue(a.sshNewTarget, sshWizTarget, true)},
		innerW, 3, a.sshWizardField == sshWizTarget,
	)
	idBox := renderApiTitledBox("identity (−i, opcional)",
		[]string{a.renderSSHWizardFieldValue(a.sshNewIdentity, sshWizIdentity, true)},
		innerW, 3, a.sshWizardField == sshWizIdentity,
	)

	preview := StyleMuted.Render("preview  ")
	name := strings.TrimSpace(a.sshNewName)
	port := strings.TrimSpace(a.sshNewLocalPortStr)
	target := strings.TrimSpace(a.sshNewTarget)
	if name == "" || target == "" {
		preview += StyleMuted.Render("(preencha nome e target)")
	} else {
		lp, _ := strconv.Atoi(port)
		host, rp, _ := sshutil.ParseBind(a.sshNewBind)
		preview += StyleHealthy.Render(truncate(name, 12)) +
			StyleMuted.Render("  ·  ") +
			StyleWarning.Render(sshutil.FormatForward(a.sshNewMode, lp, host, rp)) +
			StyleMuted.Render("  ·  ") +
			StyleNormal.Render(truncate(target, 20))
	}

	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(modeBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(portBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(bindBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(targetBox, "\n")...)
	lines = append(lines, "")
	lines = append(lines, strings.Split(idBox, "\n")...)
	hint := "no servidor: curl/open localhost:" + firstNonEmpty(port, "?") + " → chega no app deste PC"
	if a.sshNewMode != sshutil.ModeRemote {
		hint = "túnel fica em .devscope/ssh.json deste projeto"
	}
	lines = append(lines, "",
		StyleMuted.Render(hint),
		preview,
		"",
		StyleMuted.Render("tab campo  ·  ←→ cursor  ·  space mode  ·  enter salva e inicia  ·  esc"),
	)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) sshDeleteConfirmLabels() (target, detail string) {
	t, ok := a.sshSelected()
	if !ok {
		return "—", ""
	}
	detail = fmt.Sprintf("%s  :%d  %s", t.Mode, t.LocalPort, t.Target)
	return t.Name, detail
}

func (a *App) renderSSHWizardFieldValue(value string, field int, editable bool) string {
	focused := a.sshWizardField == field
	if !focused {
		return StyleNormal.Render(value)
	}
	if !editable {
		return StyleSelected.Render(value)
	}
	runes := []rune(value)
	cur := a.sshWizardCursor
	if cur < 0 {
		cur = 0
	}
	if cur > len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return StyleSelected.Render(shown)
}

func (a *App) beginSSHWizard(p *core.Project) {
	if a.sshNewMode == "" || a.sshNewLocalPortStr == "" || a.sshNewBind == "" {
		a.seedSSHDefaults(p)
	}
	if a.sshNewName == "" {
		a.sshNewName = "app"
	}
	if a.sshNewMode == "" {
		a.sshNewMode = sshutil.ModeRemote
	}
	if a.sshNewLocalPortStr == "" {
		port := a.sshNewLocalPort
		if port == 0 && p != nil {
			port = sshutil.SuggestPort(p.Ports, p.Framework.Name)
		}
		if port == 0 {
			port = 3000
		}
		a.sshNewLocalPort = port
		a.sshNewLocalPortStr = strconv.Itoa(port)
	}
	if a.sshNewBind == "" {
		a.sshNewBind = fmt.Sprintf("127.0.0.1:%s", firstNonEmpty(a.sshNewLocalPortStr, "3000"))
	}
	a.sshWizard = true
	// Target vazio = foca nele; senão começa no nome.
	if strings.TrimSpace(a.sshNewTarget) == "" {
		a.sshWizardField = sshWizTarget
		a.sshWizardCursor = 0
	} else {
		a.sshWizardField = sshWizName
		a.sshWizardCursor = len([]rune(a.sshNewName))
	}
}

func (a *App) sshWizardText() string {
	switch a.sshWizardField {
	case sshWizName:
		return a.sshNewName
	case sshWizLocalPort:
		return a.sshNewLocalPortStr
	case sshWizBind:
		return a.sshNewBind
	case sshWizTarget:
		return a.sshNewTarget
	case sshWizIdentity:
		return a.sshNewIdentity
	default:
		return ""
	}
}

func (a *App) setSSHWizardText(s string) {
	switch a.sshWizardField {
	case sshWizName:
		a.sshNewName = s
	case sshWizLocalPort:
		a.sshNewLocalPortStr = s
		// remote: mesma porta do projeto no PC por padrão
		if a.sshNewMode == sshutil.ModeRemote {
			if port, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && port > 0 {
				a.sshNewBind = fmt.Sprintf("127.0.0.1:%d", port)
			}
		}
	case sshWizBind:
		a.sshNewBind = s
	case sshWizTarget:
		a.sshNewTarget = s
	case sshWizIdentity:
		a.sshNewIdentity = s
	}
}

func (a *App) sshWizardFocusField(field int) {
	if field < sshWizName {
		field = sshWizIdentity
	}
	if field > sshWizIdentity {
		field = sshWizName
	}
	if field == sshWizBind && a.sshNewMode == sshutil.ModeDynamic {
		if a.sshWizardField < field {
			field = sshWizTarget
		} else {
			field = sshWizLocalPort
		}
	}
	a.sshWizardField = field
	if field == sshWizMode {
		a.sshWizardCursor = 0
		return
	}
	a.sshWizardCursor = len([]rune(a.sshWizardText()))
}

func (a *App) cycleSSHMode() {
	switch a.sshNewMode {
	case sshutil.ModeRemote:
		a.sshNewMode = sshutil.ModeLocal
	case sshutil.ModeLocal:
		a.sshNewMode = sshutil.ModeDynamic
	default:
		a.sshNewMode = sshutil.ModeRemote
	}
}

func (a *App) sshSelected() (sshutil.Tunnel, bool) {
	if a.sshCursor < 0 || a.sshCursor >= len(a.sshTunnels) {
		return sshutil.Tunnel{}, false
	}
	return a.sshTunnels[a.sshCursor], true
}

func (a *App) handleSSHKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.sshConfirmDelete {
		switch msg.String() {
		case "y", "Y":
			return a, a.sshDeleteSelected(p)
		case "n", "N", "esc":
			a.sshConfirmDelete = false
			return a, nil
		}
		return a, nil
	}
	if a.sshWizard {
		return a.updateSSHWizard(msg, p)
	}

	switch msg.String() {
	case "esc":
		return a, a.leaveSSHTab()
	case "tab":
		if a.sshSubTab == sshTabTunnels {
			a.sshFocus = (a.sshFocus + 1) % 3
		}
	case "0":
		a.sshSubTab = sshTabOverview
	case "1":
		a.sshSubTab = sshTabTunnels
		a.sshFocus = sshFocusTable
	case "2":
		a.sshSubTab = sshTabHistory
	case "3":
		a.sshSubTab = sshTabSettings
	case "up", "k":
		return a, a.sshMove(-1)
	case "down", "j":
		return a, a.sshMove(1)
	case "n":
		a.sshNewName = ""
		a.sshNewMode = ""
		a.sshNewLocalPort = 0
		a.sshNewLocalPortStr = ""
		a.sshNewBind = ""
		a.seedSSHDefaults(p)
		a.beginSSHWizard(p)
	case "e":
		if t, ok := a.sshSelected(); ok {
			a.sshNewName = t.Name
			a.sshNewMode = t.Mode
			a.sshNewLocalPort = t.LocalPort
			a.sshNewLocalPortStr = strconv.Itoa(t.LocalPort)
			a.sshNewBind = fmt.Sprintf("%s:%d", firstNonEmpty(t.RemoteHost, "127.0.0.1"), t.RemotePort)
			a.sshNewTarget = t.Target
			a.sshNewIdentity = t.Identity
			a.beginSSHWizard(p)
		}
	case "s":
		return a, a.sshStartSelected(p)
	case "x":
		return a, a.sshStopSelected()
	case "r":
		if a.sshFocus == sshFocusTable {
			return a, a.sshRestartSelected(p)
		}
		return a, a.refreshSSH(p)
	case "ctrl+r":
		return a, a.refreshSSH(p)
	case "c", "C":
		return a, a.sshCopyForward()
	case "d":
		if _, ok := a.sshSelected(); ok {
			a.sshConfirmDelete = true
		}
	case "A":
		a.sshShowAll = !a.sshShowAll
		return a, a.refreshSSH(p)
	}
	return a, nil
}

func (a *App) sshMove(delta int) tea.Cmd {
	switch a.sshFocus {
	case sshFocusDetails:
		a.sshDetailsScroll = maxInt(0, a.sshDetailsScroll+delta)
	case sshFocusLogs:
		a.sshLogScroll = maxInt(0, a.sshLogScroll+delta)
	default:
		n := len(a.sshTunnels)
		if n == 0 {
			return nil
		}
		a.sshCursor = (a.sshCursor + delta + n) % n
		a.sshLogScroll = 0
		a.sshDetailsScroll = 0
	}
	return nil
}

func (a *App) updateSSHWizard(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.sshWizard = false
		return a, nil
	case "tab":
		a.sshWizardFocusField(a.sshWizardField + 1)
		return a, nil
	case "shift+tab":
		a.sshWizardFocusField(a.sshWizardField - 1)
		return a, nil
	case "enter":
		a.sshWizard = false
		return a, a.sshCreateAndStart(p)
	case " ":
		if a.sshWizardField == sshWizMode {
			a.cycleSSHMode()
		}
		return a, nil
	}

	if a.sshWizardField == sshWizMode {
		return a, nil
	}
	if a.sshWizardField == sshWizBind && a.sshNewMode == sshutil.ModeDynamic {
		return a, nil
	}

	runes := []rune(a.sshWizardText())
	cur := a.sshWizardCursor
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
			a.setSSHWizardText(string(runes))
		}
	case "delete":
		if cur < len(runes) {
			runes = append(runes[:cur], runes[cur+1:]...)
			a.setSSHWizardText(string(runes))
		}
	default:
		if len(msg.Runes) > 0 {
			inserted := append([]rune(nil), msg.Runes...)
			if a.sshWizardField == sshWizLocalPort {
				for _, r := range inserted {
					if r < '0' || r > '9' {
						return a, nil
					}
				}
			}
			runes = append(runes[:cur], append(inserted, runes[cur:]...)...)
			cur += len(inserted)
			a.setSSHWizardText(string(runes))
		}
	}
	a.sshWizardCursor = cur
	return a, nil
}

func (a *App) sshTunnelFromWizard() (sshutil.TunnelConfig, error) {
	name := strings.TrimSpace(a.sshNewName)
	mode := sshutil.NormalizeMode(a.sshNewMode)
	port, _ := strconv.Atoi(strings.TrimSpace(a.sshNewLocalPortStr))
	if port == 0 {
		port = a.sshNewLocalPort
	}
	cfg := sshutil.TunnelConfig{
		Name:      name,
		Mode:      mode,
		LocalPort: port,
		Target:    strings.TrimSpace(a.sshNewTarget),
		Identity:  strings.TrimSpace(a.sshNewIdentity),
	}
	if mode != sshutil.ModeDynamic {
		host, rp, err := sshutil.ParseBind(a.sshNewBind)
		if err != nil {
			return cfg, err
		}
		cfg.RemoteHost = host
		cfg.RemotePort = rp
	}
	if cfg.Name == "" {
		return cfg, fmt.Errorf("nome vazio")
	}
	if cfg.Target == "" {
		return cfg, fmt.Errorf("target obrigatório")
	}
	if cfg.LocalPort <= 0 {
		return cfg, fmt.Errorf("porta local inválida")
	}
	return cfg, nil
}

func (a *App) sshCreateAndStart(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	tcfg, err := a.sshTunnelFromWizard()
	if err != nil {
		a.sshErr = err.Error()
		return nil
	}
	cfg := a.sshCfg
	cfg.Project = p.Name
	cfg.UpsertTunnel(tcfg)
	_ = sshutil.SaveProject(p.Path, cfg)
	a.sshCfg = cfg
	a.sshLoading = true
	a.sshStatus = "starting " + tcfg.Name + "…"
	return func() tea.Msg {
		err := sshutil.StartTunnel(tcfg)
		if err != nil {
			return sshActionMsg{err: err.Error()}
		}
		cfg := sshutil.LoadProject(p.Path, p.Name)
		cfg.History = append([]sshutil.HistoryEntry{{
			Name: tcfg.Name, Mode: tcfg.Mode, LocalPort: tcfg.LocalPort,
			Target: tcfg.Target, Started: time.Now(),
		}}, cfg.History...)
		if len(cfg.History) > 40 {
			cfg.History = cfg.History[:40]
		}
		_ = sshutil.SaveProject(p.Path, cfg)
		return sshActionMsg{out: "started " + tcfg.Name}
	}
}

func (a *App) sshStartSelected(p *core.Project) tea.Cmd {
	t, ok := a.sshSelected()
	if !ok {
		return nil
	}
	if t.Status == "online" {
		a.sshStatus = t.Name + " já online"
		return nil
	}
	tcfg := sshutil.TunnelConfig{
		Name: t.Name, Mode: t.Mode, LocalPort: t.LocalPort,
		RemoteHost: t.RemoteHost, RemotePort: t.RemotePort,
		Target: t.Target, Identity: t.Identity,
	}
	a.sshLoading = true
	return func() tea.Msg {
		err := sshutil.StartTunnel(tcfg)
		if err != nil {
			return sshActionMsg{err: err.Error()}
		}
		if p != nil {
			cfg := sshutil.LoadProject(p.Path, p.Name)
			cfg.UpsertTunnel(tcfg)
			cfg.History = append([]sshutil.HistoryEntry{{
				Name: t.Name, Mode: t.Mode, LocalPort: t.LocalPort,
				Target: t.Target, Started: time.Now(),
			}}, cfg.History...)
			if len(cfg.History) > 40 {
				cfg.History = cfg.History[:40]
			}
			_ = sshutil.SaveProject(p.Path, cfg)
		}
		return sshActionMsg{out: "started " + t.Name}
	}
}

func (a *App) sshStopSelected() tea.Cmd {
	t, ok := a.sshSelected()
	if !ok {
		return nil
	}
	a.sshLoading = true
	return func() tea.Msg {
		err := sshutil.StopTunnel(t.Name)
		if err != nil {
			return sshActionMsg{err: err.Error()}
		}
		return sshActionMsg{out: "stopped " + t.Name}
	}
}

func (a *App) sshRestartSelected(p *core.Project) tea.Cmd {
	t, ok := a.sshSelected()
	if !ok {
		return nil
	}
	tcfg := sshutil.TunnelConfig{
		Name: t.Name, Mode: t.Mode, LocalPort: t.LocalPort,
		RemoteHost: t.RemoteHost, RemotePort: t.RemotePort,
		Target: t.Target, Identity: t.Identity,
	}
	a.sshLoading = true
	return func() tea.Msg {
		_ = sshutil.StopTunnel(t.Name)
		time.Sleep(400 * time.Millisecond)
		err := sshutil.StartTunnel(tcfg)
		if err != nil {
			return sshActionMsg{err: err.Error()}
		}
		return sshActionMsg{out: "restarted " + t.Name}
	}
}

func (a *App) sshDeleteSelected(p *core.Project) tea.Cmd {
	t, ok := a.sshSelected()
	if !ok || p == nil {
		a.sshConfirmDelete = false
		return nil
	}
	_ = sshutil.StopTunnel(t.Name)
	cfg := a.sshCfg
	cfg.RemoveTunnel(t.Name)
	_ = sshutil.SaveProject(p.Path, cfg)
	a.sshCfg = cfg
	a.sshConfirmDelete = false
	return a.refreshSSH(p)
}

func (a *App) sshCopyForward() tea.Cmd {
	t, ok := a.sshSelected()
	if !ok {
		return nil
	}
	fwd := t.Forward
	if fwd == "" {
		fwd = sshutil.FormatForward(t.Mode, t.LocalPort, t.RemoteHost, t.RemotePort)
	}
	if err := copyToClipboard(fwd); err != nil {
		a.sshErr = "clipboard: " + err.Error()
		return nil
	}
	a.sshStatus = "copied " + truncate(fwd, 40)
	return nil
}
