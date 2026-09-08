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
	cfWizMode
	cfWizHostname
)

// cfDefaultURL é o destino local pré-preenchido no wizard. 127.0.0.1 em vez de
// localhost porque localhost pode resolver para ::1 e render 502 na origem.
const cfDefaultURL = "http://127.0.0.1:3000"

type cfSubTab int

// Eram seis: Overview repetia o header e History/Setup/Settings tinham poucas
// linhas cada. Conta é dado real da Cloudflare e continua sozinha.
const (
	cfTabTunnels cfSubTab = iota
	cfTabAccount
	cfTabConfig
	cfTabCount
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
	case cfTabAccount:
		body = a.renderCFAccount(w, bodyH)
	case cfTabConfig:
		body = a.renderCFConfig(p, w, bodyH)
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
	base := "1-3 aba  tab lista/detalhes/logs  n new  s start  x stop  K mata órfãos  I install  L login  C create  R route  c copy  o open  d delete  " + scope + "  esc"
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

// renderCFHeader concentra o que estava espalhado entre header, QUICK STATS e a
// aba OVERVIEW — os três repetiam autenticação, versão e contagem.
func (a *App) renderCFHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabCFTunnel)).Bold(true)
	left := accent.Render("☁ CLOUDFLARE TUNNEL")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 24))
	}
	left += "   " + a.cfAuthChip()

	right := []string{}
	if v := a.cfAuth.Version; v != "" {
		right = append(right, StyleMuted.Render("v"+v))
	}
	if a.cfLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

func (a *App) cfAuthChip() string {
	switch {
	case !a.cfAuth.CLI:
		return StyleUnhealthy.Render("✕ cloudflared não instalado")
	case !a.cfAuth.LoggedIn:
		return StyleWarning.Render("⚠ sem login · L")
	default:
		return StyleHealthy.Render("● autenticado")
	}
}

func (a *App) renderCFNav(width int) string {
	names := []string{"TÚNEIS", "CONTA", "CONFIG"}
	counts := []int{len(a.cfTunnels), len(a.cfAccount), 0}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		label := fmt.Sprintf(" %d %s ", i+1, n)
		if counts[i] > 0 {
			label = fmt.Sprintf(" %d %s %d ", i+1, n, counts[i])
		}
		if cfSubTab(i) == a.cfSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	online, offline := a.cfCounts()
	var chips []string
	if online > 0 {
		chips = append(chips, StyleHealthy.Render(fmt.Sprintf("● %d", online))+StyleMuted.Render(" online"))
	}
	if offline > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("○ %d offline", offline)))
	}
	if a.cfShowAll {
		chips = append(chips, StyleAccent.Render("A todos os projetos"))
	} else if a.cfForeign > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("+%d de outros projetos · A", a.cfForeign)))
	}
	if len(chips) == 0 {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, strings.Join(chips, "  ")+" ", width)
}

func (a *App) cfCounts() (online, offline int) {
	for _, t := range a.cfTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	return
}

// renderCFTunnelsView: a tabela ocupa a largura toda porque a URL pública é o
// que se copia — antes a coluna nem existia (só ST/NAME/PORT/MODE).
func (a *App) renderCFTunnelsView(p *core.Project, width, height int) string {
	_ = p
	if height < 8 {
		height = 8
	}
	tableH := minInt(maxInt(6, height*55/100), len(a.cfTunnels)+4)
	if tableH < 6 {
		tableH = 6
	}
	bottomH := maxInt(5, height-tableH)
	leftW := maxInt(30, width*52/100)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderCFTunnelTable(width, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderCFDetailsPane(leftW, bottomH),
			a.renderCFLogsPane(maxInt(24, width-leftW), bottomH),
		),
	)
}

type cfCols struct{ dot, name, mode, local, url, uptime, auto int }

func cfColumns(width int) cfCols {
	w := maxInt(30, width)
	c := cfCols{dot: 1, name: minInt(18, maxInt(8, w*14/100)), mode: 6, local: 6, uptime: 8, auto: 4}
	if w < 78 {
		c.uptime, c.auto = 0, 0
	}
	used := c.dot + c.name + c.mode + c.local + c.uptime + c.auto + 6
	c.url = maxInt(10, w-used)
	return c
}

func (a *App) renderCFTunnelTable(width, height int) string {
	focus := a.cfFocus == cfFocusTable
	inner := maxInt(20, width-2)
	c := cfColumns(inner - 2)
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
			cell("MODO", c.mode), cell("LOCAL", c.local), cell("URL PÚBLICA", c.url),
			rcell("UPTIME", c.uptime), rcell("AUTO", c.auto))),
		StyleMuted.Render(strings.Repeat("─", inner)),
	}

	n := len(a.cfTunnels)
	viewport := maxInt(1, height-4)
	if n == 0 {
		lines = append(lines, "", "  "+StyleMuted.Render("nenhum túnel neste projeto — ")+
			StyleKey.Render("n")+StyleMuted.Render(" cria o primeiro"))
	} else {
		a.cfScroll = ensureVisible(a.cfCursor, a.cfScroll, viewport, n)
		for i := a.cfScroll; i < minInt(a.cfScroll+viewport, n); i++ {
			lines = append(lines, a.renderCFRow(c, a.cfTunnels[i], i == a.cfCursor, focus))
		}
	}
	return renderApiTitledBox(fmt.Sprintf("TÚNEIS (%d)", n),
		fitExactLines(lines, maxInt(1, height-2)), width, height, focus)
}

func (a *App) renderCFRow(c cfCols, t cfutil.Tunnel, cursor, focus bool) string {
	glyph, dotStyle := cfTunnelDot(t, a.animFrame)
	sel := cursor && focus

	url := firstNonEmpty(publicHostOf(t.PublicURL), t.Hostname)
	urlStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	if url == "" {
		url, urlStyle = emDash, StyleMuted
	}
	auto := emDash
	for _, cfg := range a.cfCfg.Tunnels {
		if cfg.Name == t.Name && cfg.AutoStart {
			auto = "sim"
		}
	}
	local := emDash
	if t.Port > 0 {
		local = fmt.Sprintf(":%d", t.Port)
	}

	row := renderCells(sel, []dashCell{
		{text: glyph, width: c.dot, style: dotStyle},
		{text: t.Name, width: c.name, style: StyleNormal.Bold(true)},
		{text: firstNonEmpty(t.Mode, "quick"), width: c.mode, style: cfModeStyle(t.Mode)},
		{text: local, width: c.local, style: StyleMuted},
		{text: elideLeft(url, maxInt(1, c.url)), width: c.url, style: urlStyle},
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

func cfTunnelDot(t cfutil.Tunnel, frame int) (string, lipgloss.Style) {
	switch t.Status {
	case "online":
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case "starting":
		return pulseGlyph(pulseWarn, frame), StyleWarning
	default:
		return pulseGlyph(pulseBad, frame), StyleMuted
	}
}

// cfModeStyle: named tem hostname estável; quick sorteia um trycloudflare novo
// a cada start.
func cfModeStyle(mode string) lipgloss.Style {
	if mode == "named" {
		return lipgloss.NewStyle().Foreground(ColorAccent)
	}
	return StyleMuted
}

// renderCFDetailsPane mostra o que a tabela não cabe: URLs inteiras, a
// procedência do hostname e o ID do túnel. Status/modo/porta/PID saíram — eram
// quatro mini-caixas repetindo a linha da tabela logo acima.
func (a *App) renderCFDetailsPane(width, height int) string {
	focus := a.cfFocus == cfFocusDetails
	innerW := maxInt(20, width-4)
	t, ok := a.cfSelected()
	if !ok {
		return renderApiTitledBox("DETALHES",
			[]string{StyleMuted.Render("selecione um túnel na lista acima")},
			width, minInt(height, 3), focus)
	}

	label := func(k string) string { return StyleMuted.Render(padRight(k, 11)) }
	valW := maxInt(10, innerW-11)
	raw := []string{
		StyleNormal.Bold(true).Render(truncate(t.Name, innerW-14)) + "  " + tunnelStatusBadge(t.Status, a.animFrame),
		"",
		label("Público") + lipgloss.NewStyle().Foreground(ColorAccent).Render(elideLeft(firstNonEmpty(t.PublicURL, emDash), valW)),
		label("Local") + StyleNormal.Render(elideLeft(firstNonEmpty(t.LocalURL, emDash), valW)),
	}
	if t.Mode == "named" {
		host := firstNonEmpty(t.Hostname, emDash)
		raw = append(raw,
			label("Hostname")+StyleNormal.Render(elideLeft(host, valW)),
			label("")+StyleAccent.Render("named · hostname fixo no seu domínio"))
	} else {
		raw = append(raw, label("Hostname")+StyleMuted.Render("quick · trycloudflare sorteia uma URL a cada start"))
	}
	if t.TunnelID != "" {
		raw = append(raw, label("Tunnel ID")+StyleMuted.Render(elideLeft(t.TunnelID, valW)))
	}
	raw = append(raw, label("Projeto")+StyleMuted.Render(truncate(firstNonEmpty(t.Project, emDash), valW)))
	if t.PID > 0 {
		raw = append(raw, label("PID")+StyleMuted.Render(fmt.Sprintf("%d", t.PID)))
	}
	raw = append(raw, "", StyleKey.Render("c")+StyleMuted.Render(" copia a URL   ")+
		StyleKey.Render("o")+StyleMuted.Render(" abre no browser   ")+
		StyleKey.Render("R")+StyleMuted.Render(" rota DNS"))

	a.cfDetailsScroll = clampScroll(a.cfDetailsScroll, height-2, len(raw))
	end := minInt(a.cfDetailsScroll+height-2, len(raw))
	return renderApiTitledBox("DETALHES", fitExactLines(raw[a.cfDetailsScroll:end], height-2), width, height, focus)
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

// renderCFConfig funde as antigas abas History, Setup e Settings — três paradas
// na navegação para poucas linhas cada.
func (a *App) renderCFConfig(p *core.Project, width, height int) string {
	rightW := minInt(46, maxInt(32, width*34/100))
	leftW := maxInt(36, width-rightW-1)

	setup := a.cfSetupLines(p, leftW-2)
	hist := a.cfHistoryLines(leftW - 2)
	left := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("CLI E PROJETO", setup, leftW, len(setup)+2, false),
		renderApiTitledBox("HISTÓRICO", hist, leftW,
			minInt(maxInt(3, height-len(setup)-2), len(hist)+2), false),
	)

	steps := a.cfStepLines()
	right := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("PRIMEIROS PASSOS", steps, rightW, len(steps)+2, false),
		renderActionsBox(rightW, maxInt(3, height-len(steps)-2),
			[2]string{"I", "instalar cloudflared"},
			[2]string{"L", "login na conta"},
			[2]string{"C", "criar túnel named"},
			[2]string{"R", "rota DNS → túnel"},
			[2]string{"1", "voltar aos túneis"},
		),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (a *App) cfSetupLines(p *core.Project, width int) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 14)) }
	valW := maxInt(10, width-14)
	lines := []string{
		label("cloudflared") + a.cfAuthChip(),
		label("Versão") + StyleNormal.Render(firstNonEmpty(a.cfAuth.Version, emDash)),
		label("Origin cert") + StyleMuted.Render(elideLeft(firstNonEmpty(a.cfAuth.CertPath, emDash), valW)),
		label("Config") + StyleNormal.Render(".devscope/cloudflare.json"),
	}
	if p != nil {
		lines = append(lines, label("Projeto")+StyleMuted.Render(elideLeft(shortenPath(p.Path), valW)))
	}
	lines = append(lines,
		label("Quick")+StyleMuted.Render("trycloudflare.com · sem login, URL muda"),
		label("Named")+StyleMuted.Render("hostname no seu domínio · exige login"))
	return lines
}

func (a *App) cfStepLines() []string {
	step := func(n, title string, done bool, hint string) string {
		mark := StyleMuted.Render("○")
		if done {
			mark = StyleHealthy.Render("✓")
		}
		return mark + " " + StyleNormal.Render(n+" "+title) + StyleMuted.Render("  "+hint)
	}
	return []string{
		step("1", "instalar", a.cfAuth.CLI, "tecla I"),
		step("2", "login", a.cfAuth.LoggedIn, "tecla L"),
		step("3", "criar named", len(a.cfAccount) > 0, "tecla C"),
		step("4", "rota DNS", false, "tecla R"),
		"",
		StyleMuted.Render("ou pule tudo: ") + StyleKey.Render("n") + StyleMuted.Render(" cria um quick tunnel"),
	}
}

func (a *App) cfHistoryLines(width int) []string {
	if len(a.cfCfg.History) == 0 {
		return []string{StyleMuted.Render("(nenhum túnel iniciado ainda)")}
	}
	nameW := minInt(16, maxInt(8, width*20/100))
	out := make([]string, 0, len(a.cfCfg.History))
	for i, h := range a.cfCfg.History {
		if i >= 12 {
			out = append(out, StyleMuted.Render(fmt.Sprintf("+%d anteriores", len(a.cfCfg.History)-12)))
			break
		}
		dur := emDash
		if !h.Stopped.IsZero() && h.Stopped.After(h.Started) {
			dur = formatUptime(h.Stopped.Sub(h.Started))
		}
		host := firstNonEmpty(h.Hostname, publicHostOf(h.URL), emDash)
		out = append(out, StyleNormal.Render(padRight(truncate(h.Name, nameW), nameW))+" "+
			StyleMuted.Render(padRight(truncate(firstNonEmpty(h.Mode, "quick"), 6), 6))+" "+
			StyleMuted.Render(padRight(relTime(h.Started), 6))+
			StyleMuted.Render(padRight(dur, 8))+
			StyleMuted.Render(elideLeft(host, maxInt(8, width-nameW-22))))
	}
	return out
}

// renderCFWizard: formulário alinhado numa caixa só. Eram quatro caixas
// tituladas empilhadas (~20 linhas) mais quatro linhas de documentação fixa.
func (a *App) renderCFWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(58, width*62/100))
	innerW := maxInt(34, boxW-6)
	accent := tabAccentColor(TabCFTunnel)

	lines := tunnelModalChrome("CLOUDFLARE", accent, "Novo túnel", "expor um serviço local", proj, innerW)
	lines = append(lines, "")
	lines = append(lines, a.cfWizardFields(p, innerW)...)
	lines = append(lines, "",
		StyleMuted.Render(strings.Repeat("─", innerW)),
		StyleMuted.Render("$ ")+StyleNormal.Render(truncate("cloudflared "+strings.Join(
			cfutil.TunnelArgs(a.cfWizardSpec()), " "), innerW-2)),
	)
	if a.cfNewMode == "named" && strings.TrimSpace(a.cfNewHostname) != "" {
		lines = append(lines, StyleMuted.Render("  depois: ")+
			StyleKey.Render("R")+StyleMuted.Render(" aponta o DNS de "+truncate(a.cfNewHostname, innerW-24)))
	}
	lines = append(lines, "",
		StyleMuted.Render("↑↓/tab campo  ·  space alterna  ·  enter salva e sobe  ·  esc"))
	boxH := minInt(height-2, len(lines)+4)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) cfWizardFields(p *core.Project, width int) []string {
	labelW := 12
	valW := maxInt(12, minInt(30, width/3))
	row := func(field int, label, value, hint string) string {
		mark := "  "
		key := StyleMuted.Render(padRight(label, labelW))
		if a.cfWizardField == field {
			mark = StyleKey.Render("▌ ")
			key = StyleNormal.Bold(true).Render(padRight(label, labelW))
		}
		hintW := width - 2 - labelW - valW - 2
		out := mark + key + a.cfWizardValue(field, value, valW)
		if hintW >= 4 {
			out += "  " + StyleMuted.Render(truncate(hint, hintW))
		}
		return out
	}

	host := a.cfNewHostname
	hostHint := "hostname no seu domínio"
	if a.cfNewMode != "named" {
		host, hostHint = emDash, "só no modo named"
	} else if strings.TrimSpace(host) == "" && a.cfWizardField != cfWizHostname {
		host, hostHint = emDash, "sem hostname o named não roteia"
	}
	return []string{
		row(cfWizName, "Nome", a.cfNewName, "identifica o túnel"),
		row(cfWizURL, "Destino", a.cfNewURL, a.cfPortHint(p)),
		row(cfWizMode, "Modo", a.cfNewMode, cfModeHint(a.cfNewMode)),
		row(cfWizHostname, "Hostname", host, hostHint),
	}
}

func cfModeHint(mode string) string {
	switch mode {
	case "named":
		return "URL fixa · exige login"
	case "http2":
		return "quick sem QUIC · rede que bloqueia UDP"
	default:
		return "trycloudflare · URL nova a cada start"
	}
}

// cfPortHint mostra as portas que o scanner já achou — digitar a URL inteira de
// cabeça era o passo mais chato.
func (a *App) cfPortHint(p *core.Project) string {
	if p == nil || len(p.Ports) == 0 {
		return "porta ou URL local"
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

func (a *App) cfWizardSpec() cfutil.TunnelConfig {
	return cfutil.TunnelConfig{
		Name:     strings.TrimSpace(a.cfNewName),
		URL:      strings.TrimSpace(a.cfNewURL),
		Hostname: strings.TrimSpace(a.cfNewHostname),
		Mode:     a.cfNewMode,
	}
}

func (a *App) cfWizardValue(field int, value string, width int) string {
	editable := field != cfWizMode
	if a.cfWizardField != field {
		return StyleNormal.Render(padRight(truncate(value, width), width))
	}
	if !editable {
		return StyleSelected.Render(padRight(truncate(value+"  ⟨space⟩", width), width))
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
	return StyleSelected.Render(padRight(truncate(shown, width), width))
}

// cfWizardCycle é o ⟨space⟩: alterna o modo, ou percorre as portas do projeto
// quando o foco está no destino.
func (a *App) cfWizardCycle(p *core.Project) {
	if a.cfWizardField == cfWizMode {
		a.cycleCFMode()
		return
	}
	if a.cfWizardField != cfWizURL || p == nil || len(p.Ports) == 0 {
		return
	}
	cur := parsePortFromURL(a.cfNewURL)
	next := p.Ports[0]
	for i, port := range p.Ports {
		if port == cur {
			next = p.Ports[(i+1)%len(p.Ports)]
			break
		}
	}
	a.cfNewURL = fmt.Sprintf("http://127.0.0.1:%d", next)
	a.cfWizardCursor = len([]rune(a.cfNewURL))
}

func parsePortFromURL(u string) int {
	i := strings.LastIndex(u, ":")
	if i < 0 {
		return 0
	}
	port, _ := strconv.Atoi(strings.TrimSuffix(u[i+1:], "/"))
	return port
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
		host = publicHostOf(t.PublicURL)
	}
	detail = fmt.Sprintf("%s  %s", firstNonEmpty(t.Mode, "quick"), firstNonEmpty(host, t.LocalURL))
	return t.Name, detail
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
		field = cfWizHostname
	}
	if field > cfWizHostname {
		field = cfWizName
	}
	a.cfWizardField = field
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
	case "1":
		a.cfSubTab = cfTabTunnels
		a.cfFocus = cfFocusTable
	case "2":
		a.cfSubTab = cfTabAccount
	case "3":
		a.cfSubTab = cfTabConfig
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
	case "K", "shift+k", "shift+K":
		return a, a.cfKillForeign()
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
		a.cfWizardCycle(p)
		return a, nil
	}

	if a.cfWizardField == cfWizMode {
		switch msg.String() {
		case "left", "right", "[", "]":
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

// cfKillForeign encerra de uma vez todo túnel vivo que não é deste projeto —
// os órfãos que sobram ocupando porta (métricas 20241+) quando o devscope
// reinicia sem derrubar o cloudflared filho.
func (a *App) cfKillForeign() tea.Cmd {
	cfg := a.cfCfg
	a.cfLoading = true
	a.cfStatus = "encerrando túneis externos…"
	return func() tea.Msg {
		n, err := cfutil.StopForeignTunnels(cfg)
		if n == 0 && err != nil {
			return cfActionMsg{err: err.Error()}
		}
		return cfActionMsg{out: fmt.Sprintf("%d túnel(is) externo(s) encerrado(s)", n)}
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
