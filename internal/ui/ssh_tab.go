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
	// Overview repetia o header; History e Settings tinham poucas linhas cada.
	sshTabTunnels sshSubTab = iota
	sshTabConfig
	sshTabCount
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
	live := a.landingSSHLive
	state := landingToolState(a.landingSSHOK, a.landingSSHAvail, "ssh", "")
	if a.landingSSHOK && a.landingSSHAvail {
		if live > 0 {
			state = StyleHealthy.Render(fmt.Sprintf("%s %d túnel(is) ativo(s) nesta sessão", a.okPulse(), live))
		} else {
			state = StyleWarning.Render("○ nenhum túnel ativo")
		}
	}
	note := ""
	if a.landingSSHOK && a.landingSSHAvail && live == 0 {
		note = StyleMuted.Render("n cria um túnel, s inicia")
	}
	facts := [][2]string{
		{"modos", StyleMuted.Render("−R remote  ·  −L local  ·  −D socks")},
		{"config", StyleMuted.Render(".devscope/ssh.json")},
	}
	if a.landingSSHOK && a.landingSSHAvail {
		facts = append([][2]string{
			{"cliente", StyleNormal.Render(firstNonEmpty(a.landingSSHVer, "OpenSSH"))},
			{"ativos", StyleNormal.Render(fmt.Sprintf("%d", live))},
		}, facts...)
	}
	if p != nil && len(p.Ports) > 0 {
		facts = append(facts, [2]string{"portas", StyleAccent.Render(fmt.Sprintf("%v", p.Ports))})
	}
	return a.renderModuleLanding(p, moduleLanding{
		title:        "SSH TUNNEL",
		tagline:      "porta do projeto exposta no servidor — remote (−R) por padrão",
		state:        state,
		note:         note,
		facts:        facts,
		previewTitle: "O QUE ESTE PROJETO EXPÕE",
		preview:      landingPortRows(p),
		previewEmpty: "nenhuma porta publicada — suba o projeto antes de abrir o túnel",
		previewFoot:  tunnelPortFoot(p, "uma porta no servidor"),
		actions:      [][2]string{{"enter", "abrir console"}, {"esc", "voltar"}},
	})
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
	case sshTabConfig:
		body = a.renderSSHConfig(p, mainW, bodyH)
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
		[2]string{"s", "iniciar"},
		[2]string{"x", "parar"},
		[2]string{"r", "reiniciar"},
		[2]string{"c", "copiar forward"},
		[2]string{"d", "excluir"},
		[2]string{"A", "todos/projeto"},
		[2]string{"1-2", "abas"},
		[2]string{"tab", "foco painéis"},
		[2]string{"ctrl+r", "atualizar"},
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
	left := accent.Render("⇌ SSH TUNNEL")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 24))
	}
	online, _ := a.sshCounts()
	if online > 0 {
		left += "   " + StyleHealthy.Render(fmt.Sprintf("● %d ativo(s)", online))
	} else {
		left += "   " + StyleMuted.Render("○ nenhum túnel ativo")
	}

	right := []string{}
	if a.sshLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

func (a *App) renderSSHNav(width int) string {
	names := []string{"TÚNEIS", "CONFIG"}
	counts := []int{len(a.sshTunnels), 0}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		label := fmt.Sprintf(" %d %s ", i+1, n)
		if counts[i] > 0 {
			label = fmt.Sprintf(" %d %s %d ", i+1, n, counts[i])
		}
		if sshSubTab(i) == a.sshSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	online, offline := a.sshCounts()
	var chips []string
	if online > 0 {
		chips = append(chips, StyleHealthy.Render(fmt.Sprintf("● %d", online))+StyleMuted.Render(" online"))
	}
	if offline > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("○ %d offline", offline)))
	}
	if a.sshShowAll {
		chips = append(chips, StyleAccent.Render("A todos os projetos"))
	} else if a.sshForeign > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("+%d de outros projetos · A", a.sshForeign)))
	}
	if len(chips) == 0 {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, strings.Join(chips, "  ")+" ", width)
}

func (a *App) sshCounts() (online, offline int) {
	for _, t := range a.sshTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	return
}

// renderSSHTunnelsView: a tabela ocupa a largura toda para caber o encaminhamento
// (localhost:5433 → 127.0.0.1:5432), que é a informação central de um túnel SSH
// e não aparecia na lista — só ST/NAME/PORT/MODE.
func (a *App) renderSSHTunnelsView(p *core.Project, width, height int) string {
	_ = p
	if height < 8 {
		height = 8
	}
	tableH := minInt(maxInt(6, height*55/100), len(a.sshTunnels)+4)
	if tableH < 6 {
		tableH = 6
	}
	bottomH := maxInt(5, height-tableH)
	leftW := maxInt(30, width*52/100)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderSSHTunnelTable(width, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderSSHDetailsPane(leftW, bottomH),
			a.renderSSHLogsPane(maxInt(24, width-leftW), bottomH),
		),
	)
}

type sshCols struct{ dot, name, mode, forward, target, uptime, auto int }

func sshColumns(width int) sshCols {
	w := maxInt(30, width)
	c := sshCols{dot: 1, name: minInt(18, maxInt(8, w*14/100)), mode: 9,
		target: minInt(26, maxInt(10, w*20/100)), uptime: 8, auto: 4}
	if w < 84 {
		c.uptime, c.auto = 0, 0
	}
	used := c.dot + c.name + c.mode + c.target + c.uptime + c.auto + 6
	c.forward = maxInt(12, w-used)
	return c
}

func (a *App) renderSSHTunnelTable(width, height int) string {
	focus := a.sshFocus == sshFocusTable
	inner := maxInt(20, width-2)
	c := sshColumns(inner - 2)
	head := StyleMuted.Bold(true)
	cell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return padRight(truncate(t, n), n)
	}
	rcell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return padLeft(truncate(t, n), n)
	}
	lines := []string{
		"  " + head.Render(joinNonEmpty(" ", cell("", c.dot), cell("NOME", c.name),
			cell("MODO", c.mode), cell("ENCAMINHAMENTO", c.forward), cell("SERVIDOR", c.target),
			rcell("UPTIME", c.uptime), rcell("AUTO", c.auto))),
		rule(inner),
	}

	n := len(a.sshTunnels)
	viewport := maxInt(1, height-4)
	if n == 0 {
		lines = append(lines, "", "  "+StyleMuted.Render("nenhum túnel neste projeto — ")+
			StyleKey.Render("n")+StyleMuted.Render(" cria o primeiro"))
	} else {
		a.sshScroll = ensureVisible(a.sshCursor, a.sshScroll, viewport, n)
		for i := a.sshScroll; i < minInt(a.sshScroll+viewport, n); i++ {
			lines = append(lines, a.renderSSHRow(c, a.sshTunnels[i], i == a.sshCursor, focus))
		}
	}
	return panelBox(panelTitle("TÚNEIS", fmt.Sprint(n)),
		fitExactLines(lines, maxInt(1, height-2)), width, height, focus)
}

func (a *App) renderSSHRow(c sshCols, t sshutil.Tunnel, cursor, focus bool) string {
	glyph, dotStyle := sshTunnelDot(t, a.animFrame)
	sel := cursor && focus

	fwd := t.Forward
	if fwd == "" {
		fwd = sshutil.FormatForward(t.Mode, t.LocalPort, t.RemoteHost, t.RemotePort)
	}
	auto := emDash
	for _, cfg := range a.sshCfg.Tunnels {
		if cfg.Name == t.Name && cfg.AutoStart {
			auto = "sim"
		}
	}
	row := renderCells(sel, []dashCell{
		{text: glyph, width: c.dot, style: dotStyle},
		{text: t.Name, width: c.name, style: StyleNormal.Bold(true)},
		{text: sshModeLabel(t.Mode), width: c.mode, style: sshModeStyle(t.Mode)},
		{text: fwd, width: c.forward, style: lipgloss.NewStyle().Foreground(ColorAccent)},
		{text: elideLeft(t.Target, maxInt(1, c.target)), width: c.target, style: StyleMuted},
		{text: firstNonEmpty(t.Uptime, emDash), width: c.uptime, style: StyleMuted, right: true},
		{text: auto, width: c.auto, style: StyleMuted, right: true},
	})
	if sel {
		return StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
	}
	if cursor {
		return StyleKey.Render("▌") + " " + row
	}
	return "  " + row
}

func sshTunnelDot(t sshutil.Tunnel, frame int) (string, lipgloss.Style) {
	switch t.Status {
	case "online":
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case "starting":
		return pulseGlyph(pulseWarn, frame), StyleWarning
	default:
		return pulseGlyph(pulseBad, frame), StyleMuted
	}
}

// sshModeLabel troca local/remote/dynamic pelo flag do ssh — é assim que se
// pensa no encaminhamento.
func sshModeLabel(mode string) string {
	switch mode {
	case sshutil.ModeRemote:
		return "-R remoto"
	case sshutil.ModeDynamic:
		return "-D socks"
	default:
		return "-L local"
	}
}

func sshModeStyle(mode string) lipgloss.Style {
	if mode == sshutil.ModeRemote {
		return StyleWarning // expõe algo do seu PC no servidor
	}
	return StyleMuted
}

// renderSSHDetailsPane mostra o que a tabela não cabe e explica a direção do
// encaminhamento — a confusão clássica entre -L e -R.
func (a *App) renderSSHDetailsPane(width, height int) string {
	focus := a.sshFocus == sshFocusDetails
	innerW := maxInt(20, width-4)
	t, ok := a.sshSelected()
	if !ok {
		return panelBox("DETALHES",
			[]string{StyleMuted.Render("selecione um túnel na lista acima")},
			width, minInt(height, 3), focus)
	}

	label := func(k string) string { return StyleMuted.Render(padRight(k, 12)) }
	valW := maxInt(10, innerW-12)
	fwd := t.Forward
	if fwd == "" {
		fwd = sshutil.FormatForward(t.Mode, t.LocalPort, t.RemoteHost, t.RemotePort)
	}
	raw := []string{
		StyleNormal.Bold(true).Render(truncate(t.Name, innerW-14)) + "  " + tunnelStatusBadge(t.Status, a.animFrame),
		"",
		label("Encaminha") + lipgloss.NewStyle().Foreground(ColorAccent).Render(elideLeft(fwd, valW)),
		label("") + StyleMuted.Render(truncate(sshModeExplain(t.Mode), valW)),
		label("Servidor") + StyleNormal.Render(elideLeft(firstNonEmpty(t.Target, emDash), valW)),
	}
	if t.Identity != "" {
		raw = append(raw, label("Chave")+StyleMuted.Render(elideLeft(t.Identity, valW)))
	}
	raw = append(raw, label("Projeto")+StyleMuted.Render(truncate(firstNonEmpty(t.Project, emDash), valW)))
	if t.PID > 0 {
		raw = append(raw, label("PID")+StyleMuted.Render(fmt.Sprintf("%d", t.PID)))
	}
	raw = append(raw, "", StyleKey.Render("c")+StyleMuted.Render(" copia o encaminhamento   ")+
		StyleKey.Render("e")+StyleMuted.Render(" editar"))

	a.sshDetailsScroll = clampScroll(a.sshDetailsScroll, height-2, len(raw))
	end := minInt(a.sshDetailsScroll+height-2, len(raw))
	return panelBox("DETALHES", fitExactLines(raw[a.sshDetailsScroll:end], height-2), width, height, focus)
}

// sshModeExplain: -L e -R apontam em direções opostas e trocá-los é o erro
// mais comum com túnel SSH.
func sshModeExplain(mode string) string {
	switch mode {
	case sshutil.ModeRemote:
		return "abre a porta no servidor e entrega no seu PC"
	case sshutil.ModeDynamic:
		return "proxy SOCKS local saindo pelo servidor"
	default:
		return "abre a porta no seu PC e entrega no servidor"
	}
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
	return panelBox(title, fitExactLines(lines[start:end], height-2), width, height, focus)
}

// renderSSHConfig funde as antigas abas History e Settings.
func (a *App) renderSSHConfig(p *core.Project, width, height int) string {
	setup := a.sshSetupLines(p, width-2)
	hist := a.sshHistoryLines(width - 2)
	return lipgloss.JoinVertical(lipgloss.Left,
		panelBox("CLIENTE E PROJETO", setup, width, len(setup)+2, false),
		panelBox("HISTÓRICO", hist, width,
			minInt(maxInt(3, height-len(setup)-2), len(hist)+2), false),
	)
}

func (a *App) sshSetupLines(p *core.Project, width int) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 14)) }
	valW := maxInt(10, width-14)
	cli := StyleUnhealthy.Render("✕ não encontrado")
	if sshutil.Available() {
		cli = StyleHealthy.Render("● " + firstNonEmpty(sshutil.Version(), "instalado"))
	}
	lines := []string{
		label("Cliente ssh") + cli,
		label("Config") + StyleNormal.Render(".devscope/ssh.json"),
	}
	if p != nil {
		lines = append(lines, label("Projeto")+StyleMuted.Render(elideLeft(shortenPath(p.Path), valW)))
	}
	lines = append(lines,
		label("-L local")+StyleMuted.Render("porta no seu PC → serviço do servidor"),
		label("-R remoto")+StyleMuted.Render("porta no servidor → app do seu PC"),
		label("-D socks")+StyleMuted.Render("proxy SOCKS saindo pelo servidor"),
		label("Chaves")+StyleMuted.Render("~/.ssh/ · informe em identity ao criar"))
	return lines
}

func (a *App) sshHistoryLines(width int) []string {
	if len(a.sshCfg.History) == 0 {
		return []string{StyleMuted.Render("(nenhum túnel iniciado ainda)")}
	}
	nameW := minInt(16, maxInt(8, width*16/100))
	out := make([]string, 0, len(a.sshCfg.History))
	for i, h := range a.sshCfg.History {
		if i >= 12 {
			out = append(out, StyleMuted.Render(fmt.Sprintf("+%d anteriores", len(a.sshCfg.History)-12)))
			break
		}
		dur := emDash
		if !h.Stopped.IsZero() && h.Stopped.After(h.Started) {
			dur = formatUptime(h.Stopped.Sub(h.Started))
		}
		out = append(out, StyleNormal.Render(padRight(truncate(h.Name, nameW), nameW))+" "+
			StyleMuted.Render(padRight(sshModeLabel(h.Mode), 10))+
			lipgloss.NewStyle().Foreground(ColorAccent).Render(padLeft(fmt.Sprintf(":%d", h.LocalPort), 7))+"  "+
			StyleMuted.Render(padRight(relTime(h.Started), 6))+
			StyleMuted.Render(padRight(dur, 8))+
			StyleMuted.Render(elideLeft(h.Target, maxInt(8, width-nameW-34))))
	}
	return out
}

// renderSSHWizard: formulário alinhado numa caixa só. Eram seis caixas
// tituladas empilhadas — mais de 30 linhas para seis campos.
func (a *App) renderSSHWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(62, width*66/100))
	innerW := maxInt(38, boxW-6)
	accent := tabAccentColor(TabSSH)

	lines := tunnelModalChrome("SSH", accent, "Novo túnel", sshModeExplain(a.sshNewMode), proj, innerW)
	lines = append(lines, "")
	lines = append(lines, a.sshWizardFields(p, innerW)...)
	lines = append(lines, "",
		rule(innerW),
		StyleMuted.Render("$ ")+StyleNormal.Render(truncate("ssh "+strings.Join(
			sshSignificantArgs(sshutil.TunnelArgs(a.sshWizardSpec())), " "), innerW-2)),
		"",
		StyleMuted.Render("↑↓/tab campo  ·  space alterna  ·  enter salva e sobe  ·  esc"),
	)
	boxH := minInt(height-2, len(lines)+4)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

// sshSignificantArgs esconde o -N e os -o de keepalive/host-key do preview: são
// sempre os mesmos e afogavam o -L/-R, que é o que a pessoa quer conferir.
func sshSignificantArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-N":
			continue
		case "-o":
			i++ // pula o valor
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func (a *App) sshWizardFields(p *core.Project, width int) []string {
	labelW := 14
	valW := maxInt(14, minInt(30, width/3))
	row := func(field int, label, value, hint string) string {
		mark := "  "
		key := StyleMuted.Render(padRight(label, labelW))
		if a.sshWizardField == field {
			mark = StyleKey.Render("▌ ")
			key = StyleNormal.Bold(true).Render(padRight(label, labelW))
		}
		hintW := width - 2 - labelW - valW - 2
		out := mark + key + a.sshWizardValue(field, value, valW)
		if hintW >= 4 {
			out += "  " + StyleMuted.Render(truncate(hint, hintW))
		}
		return out
	}

	dynamic := a.sshNewMode == sshutil.ModeDynamic
	portLabel, portHint := "Porta local", a.sshPortHint(p)
	bindLabel, bindHint := "Destino", "host:porta do outro lado"
	switch a.sshNewMode {
	case sshutil.ModeRemote:
		portLabel, portHint = "Porta remota", "porta que abre no servidor"
		bindLabel, bindHint = "Destino", "host:porta aqui no seu PC"
	case sshutil.ModeDynamic:
		portLabel, portHint = "Porta SOCKS", "porta local do proxy"
	}
	bindVal := a.sshNewBind
	if dynamic {
		bindVal, bindHint = emDash, "não se usa em -D"
	}

	return []string{
		row(sshWizName, "Nome", a.sshNewName, "identifica o túnel"),
		row(sshWizMode, "Modo", sshModeLabel(a.sshNewMode), "-L · -R · -D"),
		row(sshWizLocalPort, portLabel, a.sshNewLocalPortStr, portHint),
		row(sshWizBind, bindLabel, bindVal, bindHint),
		row(sshWizTarget, "Servidor", a.sshNewTarget, a.sshTargetHint(p)),
		row(sshWizIdentity, "Chave", firstNonEmpty(a.sshNewIdentity, emDash), "-i · opcional"),
	}
}

// sshPortHint reaproveita as portas que o scanner achou no projeto.
func (a *App) sshPortHint(p *core.Project) string {
	if p == nil || len(p.Ports) == 0 {
		return "porta que abre no seu PC"
	}
	parts := make([]string, 0, 4)
	for i, port := range p.Ports {
		if i == 4 {
			parts = append(parts, fmt.Sprintf("+%d", len(p.Ports)-4))
			break
		}
		parts = append(parts, strconv.Itoa(port))
	}
	return "no projeto: " + strings.Join(parts, " · ") + "  ⟨space⟩"
}

// sshTargetHint sugere o user@host a partir do remote git, quando ele não é um
// host público de código.
func (a *App) sshTargetHint(p *core.Project) string {
	if p != nil && p.Git != nil {
		if t := sshutil.SuggestSSHTarget(p.Git.Remote); t != "" && t != strings.TrimSpace(a.sshNewTarget) {
			return "do git: " + t + "  ⟨space⟩"
		}
	}
	return "user@host"
}

func (a *App) sshWizardSpec() sshutil.TunnelConfig {
	port, _ := strconv.Atoi(strings.TrimSpace(a.sshNewLocalPortStr))
	host, rport, _ := sshutil.ParseBind(a.sshNewBind)
	return sshutil.TunnelConfig{
		Name:       strings.TrimSpace(a.sshNewName),
		Mode:       a.sshNewMode,
		LocalPort:  port,
		RemoteHost: host,
		RemotePort: rport,
		Target:     strings.TrimSpace(a.sshNewTarget),
		Identity:   strings.TrimSpace(a.sshNewIdentity),
	}
}

func (a *App) sshWizardValue(field int, value string, width int) string {
	editable := field != sshWizMode && !(field == sshWizBind && a.sshNewMode == sshutil.ModeDynamic)
	if a.sshWizardField != field {
		return StyleNormal.Render(padRight(truncate(value, width), width))
	}
	if !editable {
		return StyleSelected.Render(padRight(truncate(value+"  ⟨space⟩", width), width))
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
	return StyleSelected.Render(padRight(truncate(shown, width), width))
}

// sshWizardCycle é o ⟨space⟩: alterna o modo, percorre as portas do projeto ou
// aceita o servidor sugerido pelo remote git.
func (a *App) sshWizardCycle(p *core.Project) {
	switch a.sshWizardField {
	case sshWizMode:
		a.cycleSSHMode()
	case sshWizLocalPort:
		if p == nil || len(p.Ports) == 0 {
			return
		}
		cur, _ := strconv.Atoi(strings.TrimSpace(a.sshNewLocalPortStr))
		next := p.Ports[0]
		for i, port := range p.Ports {
			if port == cur {
				next = p.Ports[(i+1)%len(p.Ports)]
				break
			}
		}
		a.sshNewLocalPortStr = strconv.Itoa(next)
		a.sshWizardCursor = len(a.sshNewLocalPortStr)
	case sshWizTarget:
		if p != nil && p.Git != nil {
			if t := sshutil.SuggestSSHTarget(p.Git.Remote); t != "" {
				a.sshNewTarget = t
				a.sshWizardCursor = len([]rune(t))
			}
		}
	}
}

func (a *App) sshDeleteConfirmLabels() (target, detail string) {
	t, ok := a.sshSelected()
	if !ok {
		return "—", ""
	}
	detail = fmt.Sprintf("%s  :%d  %s", t.Mode, t.LocalPort, t.Target)
	return t.Name, detail
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
	case "1":
		a.sshSubTab = sshTabTunnels
		a.sshFocus = sshFocusTable
	case "2":
		a.sshSubTab = sshTabConfig
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
		a.sshWizardCycle(p)
		return a, nil
	}

	if a.sshWizardField == sshWizMode {
		switch msg.String() {
		case "left", "right", "[", "]":
			a.cycleSSHMode()
		}
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
