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
	ngrokWizDomain
	ngrokWizRegion
	ngrokWizAuto
	ngrokWizCount
)

// ngrokRegions são as regiões de edge do ngrok; a latência do túnel é a
// distância até a edge, então escolher errado dobra o tempo de resposta.
var ngrokRegions = []string{"us", "eu", "sa", "ap", "au", "in", "jp"}

var ngrokProtos = []string{"http", "https", "tcp", "tls"}

type ngrokSubTab int

// Eram seis abas: Overview repetia o header, History e Domains tinham duas
// linhas cada, Settings era estático. Sobraram as três que se usa.
const (
	ngrokTabTunnels ngrokSubTab = iota
	ngrokTabRequests
	ngrokTabConfig
	ngrokTabCount
)

type ngrokFocus int

const (
	ngrokFocusTable ngrokFocus = iota
	ngrokFocusDetails
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
	a.ngrokDetailsScroll = 0
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
	available := a.landingNgrokAvail
	agent := a.landingNgrokAgent
	status := "…"
	if a.landingNgrokOK {
		status = "offline"
		if agent.Connected {
			status = "connected"
		}
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
	switch {
	case !a.landingNgrokOK:
		openLines = append(openLines, "", StyleMuted.Render("detectando ambiente…"))
	case !available:
		openLines = append(openLines, "", StyleUnhealthy.Render("ngrok não encontrado no PATH"))
	default:
		openLines = append(openLines, "", StyleMuted.Render("versão  ")+StyleNormal.Render(a.landingNgrokVer))
		if agent.Connected {
			openLines = append(openLines, a.livePulse("agente local online (:4040)"))
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
	cliLabel, agentLabel := "…", "…"
	if a.landingNgrokOK {
		cliLabel, agentLabel = boolLabel(available), boolLabel(agent.Connected)
	}
	details := []string{
		StyleMuted.Render("CLI     ") + StyleNormal.Render(cliLabel),
		StyleMuted.Render("Agent   ") + StyleNormal.Render(agentLabel),
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
	switch a.ngrokSubTab {
	case ngrokTabRequests:
		body = a.renderNgrokRequestsFull(w, bodyH)
	case ngrokTabConfig:
		body = a.renderNgrokConfig(p, w, bodyH)
	default:
		body = a.renderNgrokTunnelsView(p, w, bodyH)
	}
	view := lipgloss.JoinVertical(lipgloss.Left, header, nav, body, a.renderStatusBar(a.ngrokHints()))
	if a.ngrokWizard {
		view = overlayCentered(view, a.renderNgrokWizard(p, w, h), w, h)
	}
	if a.ngrokConfirmDelete {
		target, detail := a.ngrokDeleteConfirmLabels()
		box := renderTunnelDeleteConfirmBox("NGROK", tabAccentColor(TabNgrok), target, detail, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) ngrokHints() string {
	if a.ngrokConfirmDelete {
		return "modal delete  y confirma  n/esc cancela"
	}
	if a.ngrokWizard {
		return "novo túnel  ·  ↑↓/tab campo  ·  space alterna  ·  enter salva e sobe  ·  esc"
	}
	scope := "A todos os projetos"
	if a.ngrokShowAll {
		scope = "A só este projeto"
	}
	base := "1-3 aba  ·  tab painel  ·  n novo  ·  s subir  ·  x parar  ·  r reiniciar  ·  c copiar URL  ·  o abrir  ·  e editar  ·  d apagar  ·  " + scope + "  ·  esc"
	if a.ngrokLoading {
		base = a.spinner() + " carregando…  " + base
	}
	if a.ngrokStatus != "" {
		return truncate(a.ngrokStatus, 36) + "  ·  " + base
	}
	if a.ngrokErr != "" {
		return StyleUnhealthy.Render(truncate(a.ngrokErr, 40)) + "  ·  " + base
	}
	return base
}

// renderNgrokHeader carrega o que estava espalhado entre header, QUICK STATS e
// a aba OVERVIEW — os três repetiam região, versão e contagem de túneis.
func (a *App) renderNgrokHeader(p *core.Project, width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabNgrok)).Bold(true)
	left := accent.Render("⇪ NGROK")
	if p != nil && p.Name != "" {
		left += StyleMuted.Render("   " + truncate(p.Name, 26))
	}
	left += "   " + a.ngrokAgentChip()

	right := []string{StyleMuted.Render("região " + cfgRegionOr(a.ngrokCfg, "us"))}
	if v := a.ngrokAgent.Version; v != "" {
		right = append(right, StyleMuted.Render("v"+v))
	}
	if a.ngrokLoading {
		right = append(right, a.loadingMuted("carregando…"))
	}
	right = append(right, StyleMuted.Render(a.now.Format("15:04:05")))
	return joinWithSpacer(truncateVisible(left, width), strings.Join(right, StyleMuted.Render("  ·  ")), width)
}

func (a *App) ngrokAgentChip() string {
	if a.ngrokAgent.Connected {
		return StyleHealthy.Render("● agente conectado")
	}
	return StyleMuted.Render("○ agente offline")
}

func (a *App) renderNgrokNav(width int) string {
	names := []string{"TÚNEIS", "REQUESTS", "CONFIG"}
	counts := []int{len(a.ngrokTunnels), len(a.ngrokRequests), 0}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		label := fmt.Sprintf(" %d %s ", i+1, n)
		if counts[i] > 0 {
			label = fmt.Sprintf(" %d %s %d ", i+1, n, counts[i])
		}
		if ngrokSubTab(i) == a.ngrokSubTab {
			parts = append(parts, StyleSelected.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	left := strings.Join(parts, StyleMuted.Render("│"))

	online, offline := a.ngrokCounts()
	var chips []string
	if online > 0 {
		chips = append(chips, StyleHealthy.Render(fmt.Sprintf("● %d", online))+StyleMuted.Render(" online"))
	}
	if offline > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("○ %d offline", offline)))
	}
	if a.ngrokShowAll {
		chips = append(chips, StyleAccent.Render("A todos os projetos"))
	} else if a.ngrokForeign > 0 {
		chips = append(chips, StyleMuted.Render(fmt.Sprintf("+%d de outros projetos · A", a.ngrokForeign)))
	}
	if len(chips) == 0 {
		return padRightVisible(left, width)
	}
	return joinWithSpacer(left, strings.Join(chips, "  ")+" ", width)
}

func (a *App) ngrokCounts() (online, offline int) {
	for _, t := range a.ngrokTunnels {
		if t.Status == "online" {
			online++
		} else {
			offline++
		}
	}
	return
}

func ngrokStatusLabel(ok bool, frame int) string {
	if ok {
		return StyleHealthy.Render(animPulse(frame) + " Connected")
	}
	return StyleMuted.Render("○ Offline")
}

// renderNgrokTunnelsView: a tabela ocupa a largura inteira porque a URL pública
// é o que se copia — antes ela ficava numa coluna de 40% e nem aparecia.
func (a *App) renderNgrokTunnelsView(p *core.Project, width, height int) string {
	_ = p
	if height < 8 {
		height = 8
	}
	tableH := minInt(maxInt(6, height*55/100), len(a.ngrokTunnels)+4)
	if tableH < 6 {
		tableH = 6
	}
	bottomH := maxInt(5, height-tableH)
	leftW := maxInt(30, width*52/100)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderNgrokTunnelTable(width, tableH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderNgrokDetailsPane(leftW, bottomH),
			a.renderNgrokLogsPane(maxInt(24, width-leftW), bottomH),
		),
	)
}

type ngrokCols struct{ dot, name, proto, local, url, reqs, uptime, auto int }

func ngrokColumns(width int) ngrokCols {
	w := maxInt(30, width)
	c := ngrokCols{dot: 1, name: minInt(18, maxInt(8, w*14/100)), proto: 5, local: 6, reqs: 6, uptime: 8, auto: 4}
	if w < 78 {
		c.uptime, c.auto = 0, 0
	}
	if w < 60 {
		c.reqs = 0
	}
	used := c.dot + c.name + c.proto + c.local + c.reqs + c.uptime + c.auto + 7
	c.url = maxInt(10, w-used)
	return c
}

func (a *App) renderNgrokTunnelTable(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusTable
	inner := maxInt(20, width-2)
	c := ngrokColumns(inner - 2)
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
			cell("PROTO", c.proto), cell("LOCAL", c.local), cell("URL PÚBLICA", c.url),
			rcell("REQS", c.reqs), rcell("UPTIME", c.uptime), rcell("AUTO", c.auto))),
		StyleMuted.Render(strings.Repeat("─", inner)),
	}

	n := len(a.ngrokTunnels)
	viewport := maxInt(1, height-4)
	if n == 0 {
		lines = append(lines, "", "  "+StyleMuted.Render("nenhum túnel neste projeto — ")+
			StyleKey.Render("n")+StyleMuted.Render(" cria o primeiro"))
	} else {
		a.ngrokScroll = ensureVisible(a.ngrokCursor, a.ngrokScroll, viewport, n)
		for i := a.ngrokScroll; i < minInt(a.ngrokScroll+viewport, n); i++ {
			lines = append(lines, a.renderNgrokRow(c, a.ngrokTunnels[i], i == a.ngrokCursor, focus))
		}
	}
	title := fmt.Sprintf("TÚNEIS (%d)", n)
	return renderApiTitledBox(title, fitExactLines(lines, maxInt(1, height-2)), width, height, focus)
}

func (a *App) renderNgrokRow(c ngrokCols, t ngrokutil.Tunnel, cursor, focus bool) string {
	glyph, dotStyle := ngrokTunnelDot(t, a.animFrame)
	sel := cursor && focus

	url := firstNonEmpty(publicHostOf(t.PublicURL), t.Domain)
	urlStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	if url == "" {
		url, urlStyle = emDash, StyleMuted
	}
	auto := emDash
	for _, cfg := range a.ngrokCfg.Tunnels {
		if cfg.Name == t.Name && cfg.AutoStart {
			auto = "sim"
		}
	}
	reqs := emDash
	if t.Requests > 0 {
		reqs = fmt.Sprintf("%d", t.Requests)
	}

	cells := []dashCell{
		{text: glyph, width: c.dot, style: dotStyle},
		{text: t.Name, width: c.name, style: StyleNormal.Bold(true)},
		{text: t.Proto, width: c.proto, style: StyleMuted},
		{text: fmt.Sprintf(":%d", t.Port), width: c.local, style: StyleMuted},
		{text: elideLeft(url, maxInt(1, c.url)), width: c.url, style: urlStyle},
		{text: reqs, width: c.reqs, style: StyleMuted, right: true},
		{text: firstNonEmpty(t.Uptime, emDash), width: c.uptime, style: StyleMuted, right: true},
		{text: auto, width: c.auto, style: StyleMuted, right: true},
	}
	row := renderCells(sel, cells)
	if sel {
		return StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
	}
	if cursor {
		return StyleKey.Render("▌") + " " + row
	}
	return "  " + row
}

func ngrokTunnelDot(t ngrokutil.Tunnel, frame int) (string, lipgloss.Style) {
	switch t.Status {
	case "online":
		return pulseGlyph(pulseOK, frame), StyleHealthy
	case "starting":
		return pulseGlyph(pulseWarn, frame), StyleWarning
	case "offline":
		return pulseGlyph(pulseBad, frame), StyleMuted
	default:
		return pulseGlyph(pulseIdle, frame), StyleMuted
	}
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

// renderNgrokDetailsPane mostra o que a tabela não cabe: as URLs inteiras (é o
// que se copia) e a procedência do domínio. Status/porta/reqs/uptime saíram —
// já estão na linha da tabela, a duas linhas de distância.
func (a *App) renderNgrokDetailsPane(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusDetails
	innerW := maxInt(20, width-4)
	t, ok := a.ngrokSelected()
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
		label("Local") + StyleNormal.Render(elideLeft(firstNonEmpty(t.LocalURL, fmt.Sprintf("http://localhost:%d", t.Port)), valW)),
	}

	host := firstNonEmpty(t.Domain, publicHostOf(t.PublicURL))
	if host != "" {
		origem := StyleMuted.Render("efêmero · muda no próximo restart")
		if ngrokDomainIsReserved(a.ngrokCfg, t.Name, host) {
			origem = StyleAccent.Render("reservado · estável")
		}
		raw = append(raw, label("Domínio")+StyleNormal.Render(elideLeft(host, valW)), label("")+origem)
	} else {
		raw = append(raw, label("Domínio")+StyleMuted.Render("— · defina um ao criar para a URL não mudar"))
	}

	raw = append(raw,
		label("Região")+StyleNormal.Render(firstNonEmpty(t.Region, cfgRegionOr(a.ngrokCfg, "us"))),
		label("Projeto")+StyleMuted.Render(truncate(firstNonEmpty(t.Project, emDash), valW)),
	)
	if t.PID > 0 {
		raw = append(raw, label("PID")+StyleMuted.Render(fmt.Sprintf("%d", t.PID)))
	}
	if t.BytesIn > 0 || t.BytesOut > 0 {
		raw = append(raw, label("Tráfego")+StyleMuted.Render(
			fmt.Sprintf("↓ %s   ↑ %s", formatMiB(t.BytesIn), formatMiB(t.BytesOut))))
	}
	raw = append(raw, "", StyleKey.Render("c")+StyleMuted.Render(" copia a URL   ")+
		StyleKey.Render("o")+StyleMuted.Render(" abre no browser"))

	a.ngrokDetailsScroll = clampScroll(a.ngrokDetailsScroll, height-2, len(raw))
	end := minInt(a.ngrokDetailsScroll+height-2, len(raw))
	return renderApiTitledBox("DETALHES", fitExactLines(raw[a.ngrokDetailsScroll:end], height-2), width, height, focus)
}

// renderNgrokRequestsPane: ganhou TÚNEL (com três túneis abertos, saber qual
// recebeu é o essencial), LAT e IP. E o status voltou a ter cor — o código
// antigo pintava e depois passava stripANSI na linha inteira.
func (a *App) renderNgrokRequestsPane(width, height int) string {
	focus := a.ngrokFocus == ngrokFocusRequests
	inner := maxInt(20, width-2)

	hostW := 0
	latW, ipW := 0, 0
	if inner >= 66 {
		hostW = minInt(18, maxInt(8, inner*16/100))
	}
	if inner >= 52 {
		latW = 7
	}
	if inner >= 96 {
		ipW = 15
	}
	pathW := maxInt(10, inner-2-9-5-4-hostW-latW-ipW-7)

	head := StyleMuted.Bold(true)
	cell := func(t string, n int) string {
		if n <= 0 {
			return ""
		}
		return padRight(truncate(t, n), n)
	}
	lines := []string{
		"  " + head.Render(joinNonEmpty(" ", cell("HORA", 8), cell("MÉT", 4), cell("ST", 3),
			cell("TÚNEL", hostW), cell("CAMINHO", pathW), padLeft("LAT", latW), cell("IP", ipW))),
		StyleMuted.Render(strings.Repeat("─", inner)),
	}

	if len(a.ngrokRequests) == 0 {
		lines = append(lines, "", "  "+StyleMuted.Render("nenhuma requisição capturada ainda"),
			"  "+StyleMuted.Render("o agente registra a partir do momento em que o túnel sobe"))
	} else {
		viewport := maxInt(1, height-4)
		a.ngrokReqScroll = ensureVisible(a.ngrokReqCursor, a.ngrokReqScroll, viewport, len(a.ngrokRequests))
		for i := a.ngrokReqScroll; i < minInt(a.ngrokReqScroll+viewport, len(a.ngrokRequests)); i++ {
			r := a.ngrokRequests[i]
			sel := i == a.ngrokReqCursor && focus
			lat := emDash
			if r.LatencyMS > 0 {
				lat = fmt.Sprintf("%dms", r.LatencyMS)
			}
			row := renderCells(sel, []dashCell{
				{text: r.Time.Format("15:04:05"), width: 8, style: StyleMuted},
				{text: r.Method, width: 4, style: ngrokMethodStyle(r.Method)},
				{text: fmt.Sprintf("%d", r.Status), width: 3, style: ngrokStatusStyle(r.Status)},
				{text: shortTunnelHost(r.Host), width: hostW, style: StyleMuted},
				{text: r.Path, width: pathW, style: StyleNormal},
				{text: lat, width: latW, style: ngrokLatencyStyle(r.LatencyMS), right: true},
				{text: r.IP, width: ipW, style: StyleMuted},
			})
			if sel {
				lines = append(lines, StyleKey.Render("▌")+lipgloss.NewStyle().Background(ColorSelBg).Render(" ")+row)
			} else {
				lines = append(lines, "  "+row)
			}
		}
	}
	return renderApiTitledBox(fmt.Sprintf("REQUISIÇÕES (%d)", len(a.ngrokRequests)),
		fitExactLines(lines, maxInt(1, height-2)), width, height, focus)
}

func ngrokStatusStyle(code int) lipgloss.Style {
	switch {
	case code >= 500:
		return StyleUnhealthy
	case code >= 400:
		return StyleWarning
	case code >= 300:
		return StyleMuted
	case code > 0:
		return StyleHealthy
	default:
		return StyleMuted
	}
}

func ngrokMethodStyle(m string) lipgloss.Style {
	switch strings.ToUpper(m) {
	case "GET":
		return StyleMuted
	case "DELETE":
		return StyleUnhealthy
	case "POST", "PUT", "PATCH":
		return lipgloss.NewStyle().Foreground(ColorAccent)
	default:
		return StyleMuted
	}
}

// ngrokLatencyStyle: acima de 1s a requisição é o problema, não o detalhe.
func ngrokLatencyStyle(ms int64) lipgloss.Style {
	switch {
	case ms >= 1000:
		return StyleUnhealthy
	case ms >= 300:
		return StyleWarning
	default:
		return StyleMuted
	}
}

// shortTunnelHost corta o sufixo do provedor — o que distingue é o subdomínio.
func shortTunnelHost(host string) string {
	host = publicHostOf(host)
	for _, suffix := range []string{".ngrok.app", ".ngrok-free.app", ".ngrok.io", ".ngrok.dev"} {
		if strings.HasSuffix(host, suffix) {
			return strings.TrimSuffix(host, suffix)
		}
	}
	return host
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

// renderNgrokConfig funde as antigas abas History, Domains e Settings — as três
// mostravam duas linhas cada e custavam três paradas na navegação.
func (a *App) renderNgrokConfig(p *core.Project, width, height int) string {
	rightW := minInt(42, maxInt(30, width*32/100))
	leftW := maxInt(36, width-rightW-1)

	setup := a.ngrokSetupLines(p, leftW-2)
	hist := a.ngrokHistoryLines(leftW - 2)
	left := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("AGENTE E PROJETO", setup, leftW, len(setup)+2, false),
		renderApiTitledBox("HISTÓRICO", hist, leftW, minInt(maxInt(3, height-len(setup)-2), len(hist)+2), false),
	)
	dom := a.ngrokDomainLines(rightW - 2)
	right := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DOMÍNIOS", dom, rightW, len(dom)+2, false),
		renderActionsBox(rightW, maxInt(3, height-len(dom)-2),
			[2]string{"n", "novo túnel"},
			[2]string{"e", "editar"},
			[2]string{"r", "refresh"},
			[2]string{"A", "todos os projetos"},
			[2]string{"1", "voltar aos túneis"},
		),
	)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (a *App) ngrokSetupLines(p *core.Project, width int) []string {
	label := func(k string) string { return StyleMuted.Render(padRight(k, 14)) }
	valW := maxInt(10, width-14)
	lines := []string{
		label("Agente") + a.ngrokAgentChip() + StyleMuted.Render("  "+a.ngrokAgent.URI),
		label("CLI") + StyleNormal.Render(firstNonEmpty("ngrok "+a.ngrokAgent.Version, "ngrok")),
		label("Região padrão") + StyleNormal.Render(cfgRegionOr(a.ngrokCfg, "us")),
		label("Config") + StyleNormal.Render(elideLeft(".devscope/ngrok.json", valW)),
	}
	if p != nil {
		lines = append(lines, label("Projeto")+StyleMuted.Render(elideLeft(shortenPath(p.Path), valW)))
	}
	lines = append(lines,
		label("Authtoken")+StyleMuted.Render("ngrok config add-authtoken <token>"))
	return lines
}

func (a *App) ngrokHistoryLines(width int) []string {
	if len(a.ngrokCfg.History) == 0 {
		return []string{StyleMuted.Render("(nenhum túnel iniciado ainda)")}
	}
	nameW := minInt(16, maxInt(8, width*22/100))
	out := make([]string, 0, len(a.ngrokCfg.History))
	for i, h := range a.ngrokCfg.History {
		if i >= 12 {
			out = append(out, StyleMuted.Render(fmt.Sprintf("+%d anteriores", len(a.ngrokCfg.History)-12)))
			break
		}
		dur := emDash
		if !h.Stopped.IsZero() && h.Stopped.After(h.Started) {
			dur = formatUptime(h.Stopped.Sub(h.Started))
		}
		out = append(out, StyleNormal.Render(padRight(truncate(h.Name, nameW), nameW))+" "+
			lipgloss.NewStyle().Foreground(ColorAccent).Render(padLeft(fmt.Sprintf(":%d", h.Port), 6))+"  "+
			StyleMuted.Render(padRight(relTime(h.Started), 6))+
			StyleMuted.Render(padRight(dur, 8))+
			StyleMuted.Render(padLeft(fmt.Sprintf("%d req", h.Requests), 10)))
	}
	return out
}

func (a *App) ngrokDomainLines(width int) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range a.ngrokTunnels {
		host := firstNonEmpty(t.Domain, publicHostOf(t.PublicURL))
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		reserved := ngrokDomainIsReserved(a.ngrokCfg, t.Name, host)
		glyph, st := pulseGlyph(pulseBad, a.animFrame), StyleMuted
		if t.Status == "online" {
			glyph, st = pulseGlyph(pulseOK, a.animFrame), StyleHealthy
		}
		tag := StyleMuted.Render(" efêmero")
		if reserved {
			tag = StyleAccent.Render(" reservado")
		}
		hostW := maxInt(10, width-12)
		out = append(out, st.Render(glyph)+" "+
			StyleNormal.Render(padRight(elideLeft(host, hostW), hostW))+tag)
	}
	if len(out) == 0 {
		return []string{
			StyleMuted.Render("(nenhum domínio ativo)"),
			StyleMuted.Render("reserve um em dashboard.ngrok.com"),
			StyleMuted.Render("e preencha o campo Domínio ao criar"),
		}
	}
	return out
}

// ngrokDomainIsReserved: domínio que veio da config do projeto é reservado; o
// que o agente sorteou some no próximo restart.
func ngrokDomainIsReserved(cfg ngrokutil.ProjectConfig, name, host string) bool {
	for _, c := range cfg.Tunnels {
		if c.Name == name && strings.EqualFold(strings.TrimSpace(c.Domain), host) {
			return true
		}
	}
	return false
}

func publicHostOf(u string) string {
	u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	u = strings.TrimPrefix(u, "tcp://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func (a *App) renderNgrokWizard(p *core.Project, width, height int) string {
	proj := ""
	if p != nil {
		proj = p.Name
	}
	boxW := minInt(width-4, maxInt(58, width*62/100))
	innerW := maxInt(34, boxW-6)
	accent := tabAccentColor(TabNgrok)

	lines := tunnelModalChrome("NGROK", accent, "Novo túnel", "expor uma porta local", proj, innerW)
	lines = append(lines, "")
	lines = append(lines, a.ngrokWizardFields(p, innerW)...)
	lines = append(lines, "",
		StyleMuted.Render(strings.Repeat("─", innerW)),
		// O que se vê é o que roda: mesma linha que StartArgs monta.
		StyleMuted.Render("$ ")+StyleNormal.Render(truncate("ngrok "+strings.Join(
			ngrokutil.StartArgs(a.ngrokWizardSpec()), " "), innerW-2)),
		"",
		StyleMuted.Render("↑↓/tab campo  ·  space alterna  ·  enter salva e sobe  ·  esc"),
	)
	boxH := minInt(height-2, len(lines)+4)
	return tunnelModalBox(lines, boxW, boxH, accent)
}

func (a *App) ngrokWizardFields(p *core.Project, width int) []string {
	labelW := 12
	valW := maxInt(12, minInt(26, width/3))
	row := func(field int, label, value, hint string) string {
		mark := "  "
		key := StyleMuted.Render(padRight(label, labelW))
		if a.ngrokWizardField == field {
			mark = StyleKey.Render("▌ ")
			key = StyleNormal.Bold(true).Render(padRight(label, labelW))
		}
		// A dica encolhe com a caixa — sem isso a linha quebrava em duas.
		hintW := width - 2 - labelW - valW - 2
		if hintW < 4 {
			return mark + key + a.ngrokWizardValue(field, value, valW)
		}
		return mark + key + a.ngrokWizardValue(field, value, valW) + "  " +
			StyleMuted.Render(truncate(hint, hintW))
	}

	auto := "não"
	if a.ngrokNewAuto {
		auto = "sim"
	}
	domain := a.ngrokNewDomain
	domainHint := "opcional · domínio reservado da conta"
	if strings.TrimSpace(domain) == "" && a.ngrokWizardField != ngrokWizDomain {
		domain = emDash
		domainHint = "sem domínio a URL muda a cada restart"
	}
	return []string{
		row(ngrokWizName, "Nome", a.ngrokNewName, "identifica o túnel no agente"),
		row(ngrokWizPort, "Porta", a.ngrokNewPortStr, a.ngrokPortHint(p)),
		row(ngrokWizProto, "Proto", a.ngrokNewProto, strings.Join(ngrokProtos, " · ")),
		row(ngrokWizDomain, "Domínio", domain, domainHint),
		row(ngrokWizRegion, "Região", a.ngrokWizardRegion(), "edge mais perto = menos latência"),
		row(ngrokWizAuto, "Auto-start", auto, "sobe junto com o projeto"),
	}
}

// ngrokPortHint mostra as portas que o scanner já achou no projeto — digitar a
// porta de cabeça era o passo mais chato de criar um túnel.
func (a *App) ngrokPortHint(p *core.Project) string {
	if p == nil || len(p.Ports) == 0 {
		return "porta local a expor"
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

func (a *App) ngrokWizardRegion() string {
	if r := strings.TrimSpace(a.ngrokNewRegion); r != "" {
		return r
	}
	return cfgRegionOr(a.ngrokCfg, "us")
}

// ngrokWizardSpec é o túnel que o formulário descreve agora — alimenta tanto o
// preview do comando quanto o enter.
func (a *App) ngrokWizardSpec() ngrokutil.TunnelConfig {
	port, _ := strconv.Atoi(strings.TrimSpace(a.ngrokNewPortStr))
	return ngrokutil.TunnelConfig{
		Name:      strings.TrimSpace(a.ngrokNewName),
		Port:      port,
		Proto:     a.ngrokNewProto,
		Domain:    strings.TrimSpace(a.ngrokNewDomain),
		Region:    a.ngrokWizardRegion(),
		AutoStart: a.ngrokNewAuto,
	}
}

func (a *App) ngrokWizardValue(field int, value string, width int) string {
	editable := field == ngrokWizName || field == ngrokWizPort || field == ngrokWizDomain
	if a.ngrokWizardField != field {
		return StyleNormal.Render(padRight(truncate(value, width), width))
	}
	if !editable {
		return StyleSelected.Render(padRight(truncate(value+"  ⟨space⟩", width), width))
	}
	runes := []rune(value)
	cur := clampCursor(a.ngrokWizardCursor, len(runes)+1)
	if a.ngrokWizardCursor >= len(runes) {
		cur = len(runes)
	}
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return StyleSelected.Render(padRight(truncate(shown, width), width))
}

func (a *App) ngrokDeleteConfirmLabels() (target, detail string) {
	t, ok := a.ngrokSelected()
	if !ok {
		return "—", ""
	}
	detail = fmt.Sprintf("%s  :%d  %s", firstNonEmpty(t.Proto, "http"), t.Port, firstNonEmpty(t.Domain, t.PublicURL))
	return t.Name, detail
}

func (a *App) renderNgrokWizardFieldValue(value string, field int, editable bool) string {
	focused := a.ngrokWizardField == field
	if !focused {
		return StyleNormal.Render(value)
	}
	if !editable {
		return StyleSelected.Render(value)
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
	return StyleSelected.Render(shown)
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
	if a.ngrokNewRegion == "" {
		a.ngrokNewRegion = cfgRegionOr(a.ngrokCfg, "us")
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
	case ngrokWizDomain:
		return a.ngrokNewDomain
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
	case ngrokWizDomain:
		a.ngrokNewDomain = s
	}
}

func (a *App) ngrokWizardFocusField(field int) {
	if field < ngrokWizName {
		field = ngrokWizCount - 1
	}
	if field >= ngrokWizCount {
		field = ngrokWizName
	}
	a.ngrokWizardField = field
	a.ngrokWizardCursor = len([]rune(a.ngrokWizardText()))
}

// ngrokWizardCycle é o ⟨space⟩ de cada campo: nos campos de escolha alterna o
// valor, na porta percorre as portas que o projeto já expõe.
func (a *App) ngrokWizardCycle(p *core.Project) {
	switch a.ngrokWizardField {
	case ngrokWizProto:
		a.ngrokNewProto = cycleFrom(ngrokProtos, a.ngrokNewProto)
	case ngrokWizRegion:
		a.ngrokNewRegion = cycleFrom(ngrokRegions, a.ngrokWizardRegion())
	case ngrokWizAuto:
		a.ngrokNewAuto = !a.ngrokNewAuto
	case ngrokWizPort:
		if p == nil || len(p.Ports) == 0 {
			return
		}
		cur, _ := strconv.Atoi(strings.TrimSpace(a.ngrokNewPortStr))
		next := p.Ports[0]
		for i, port := range p.Ports {
			if port == cur {
				next = p.Ports[(i+1)%len(p.Ports)]
				break
			}
		}
		a.ngrokNewPortStr = strconv.Itoa(next)
		a.ngrokWizardCursor = len(a.ngrokNewPortStr)
	}
}

func cycleFrom(options []string, current string) string {
	for i, o := range options {
		if o == current {
			return options[(i+1)%len(options)]
		}
	}
	if len(options) == 0 {
		return current
	}
	return options[0]
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
		} else if a.ngrokSubTab == ngrokTabTunnels {
			a.ngrokFocus = (a.ngrokFocus + 1) % 3 // table → details → logs
		}
	case "1":
		a.ngrokSubTab = ngrokTabTunnels
		a.ngrokFocus = ngrokFocusTable
	case "2":
		a.ngrokSubTab = ngrokTabRequests
		a.ngrokFocus = ngrokFocusRequests
	case "3":
		a.ngrokSubTab = ngrokTabConfig
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
	case ngrokFocusDetails:
		a.ngrokDetailsScroll += delta
		if a.ngrokDetailsScroll < 0 {
			a.ngrokDetailsScroll = 0
		}
	case ngrokFocusLogs:
		a.ngrokLogScroll += delta
		if a.ngrokLogScroll < 0 {
			a.ngrokLogScroll = 0
		}
	default:
		prev := a.ngrokCursor
		a.ngrokCursor += delta
		if a.ngrokCursor < 0 {
			a.ngrokCursor = 0
		}
		if a.ngrokCursor > len(a.ngrokTunnels)-1 {
			a.ngrokCursor = maxInt(0, len(a.ngrokTunnels)-1)
		}
		if a.ngrokCursor != prev {
			a.ngrokDetailsScroll = 0
			a.ngrokLogScroll = 0
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
		if a.ngrokNewRegion == "" {
			a.ngrokNewRegion = a.ngrokWizardRegion()
		}
		a.ngrokWizard = false
		return a, a.ngrokCreateAndStart(p)
	case "tab", "down":
		a.ngrokWizardFocusField(a.ngrokWizardField + 1)
		return a, nil
	case "shift+tab", "up":
		a.ngrokWizardFocusField(a.ngrokWizardField - 1)
		return a, nil
	case " ":
		a.ngrokWizardCycle(p)
		return a, nil
	}

	// Campos de escolha não têm texto para editar.
	switch a.ngrokWizardField {
	case ngrokWizProto, ngrokWizRegion, ngrokWizAuto:
		switch msg.String() {
		case "left", "right", "[", "]":
			a.ngrokWizardCycle(p)
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
	region := strings.TrimSpace(a.ngrokNewRegion)
	if region == "" {
		region = cfgRegionOr(a.ngrokCfg, "us")
	}
	spec := ngrokutil.TunnelConfig{
		Name: name, Port: port, Proto: proto,
		Domain: strings.TrimSpace(a.ngrokNewDomain), Region: region, AutoStart: a.ngrokNewAuto,
	}
	cfg := a.ngrokCfg
	cfg.Project = p.Name
	cfg.UpsertTunnel(spec)
	_ = ngrokutil.SaveProject(p.Path, cfg)
	a.ngrokCfg = cfg
	a.ngrokLoading = true
	a.ngrokStatus = "subindo " + name + "…"
	return func() tea.Msg {
		err := ngrokutil.StartTunnel(spec)
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
	spec := a.ngrokSpecFor(t)
	a.ngrokLoading = true
	return func() tea.Msg {
		err := ngrokutil.StartTunnel(spec)
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
	spec := a.ngrokSpecFor(t)
	a.ngrokLoading = true
	return func() tea.Msg {
		_ = ngrokutil.StopTunnel(t.Name)
		time.Sleep(400 * time.Millisecond)
		err := ngrokutil.StartTunnel(spec)
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

// ngrokSpecFor recupera a config salva do túnel (domínio reservado, região)
// para religá-lo igual. Sem isso, restart devolvia uma URL nova.
func (a *App) ngrokSpecFor(t ngrokutil.Tunnel) ngrokutil.TunnelConfig {
	for _, c := range a.ngrokCfg.Tunnels {
		if c.Name == t.Name {
			if c.Port == 0 {
				c.Port = t.Port
			}
			if c.Proto == "" {
				c.Proto = t.Proto
			}
			if c.Region == "" {
				c.Region = a.ngrokCfg.Region
			}
			return c
		}
	}
	return ngrokutil.TunnelConfig{
		Name: t.Name, Port: t.Port, Proto: t.Proto,
		Domain: t.Domain, Region: firstNonEmpty(t.Region, a.ngrokCfg.Region),
	}
}

func cfgRegionOr(cfg ngrokutil.ProjectConfig, fallback string) string {
	if cfg.Region != "" {
		return cfg.Region
	}
	return fallback
}
