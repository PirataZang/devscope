package ui

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/wsutil"
)

// --- model ---

type wsSubTab int

const (
	wsTabOverview wsSubTab = iota
	wsTabMessages
	wsTabHistory
	wsTabSettings
)

type wsFocus int

const (
	wsFocusConnections wsFocus = iota
	wsFocusFilters
	wsFocusMessages
	wsFocusSend
	wsFocusInspector
)

type wsFilterKind int

const (
	wsFilterAll wsFilterKind = iota
	wsFilterText
	wsFilterJSON
	wsFilterBinary
	wsFilterErrors
	wsFilterIn
	wsFilterOut
)

type wsPayloadMode int

const (
	wsPayloadPretty wsPayloadMode = iota
	wsPayloadRaw
	wsPayloadHex
)

type wsSendMode int

const (
	wsSendText wsSendMode = iota
	wsSendJSON
	wsSendBinary
)

type wsFrame struct {
	ID      int
	Time    time.Time
	Dir     string // in | out | meta | err
	Kind    string // text | json | binary | error | meta
	Size    int
	Latency time.Duration
	Payload string
}

type wsStats struct {
	RecvFrames  int
	SentFrames  int
	RecvBytes   int
	SentBytes   int
	Errors      int
	Disconnects int
	LatencyMin  time.Duration
	LatencyMax  time.Duration
	LatencySum  time.Duration
	LatencyN    int
}

type wsEventMsg struct {
	ev   wsutil.Event
	sess *wsutil.Session
}

type wsConnectedMsg struct {
	sess *wsutil.Session
}

var wsFilterLabels = []string{"Todas", "Texto", "JSON", "Binário", "Erros", "Recebidas", "Enviadas"}

// --- lifecycle ---

func (a *App) enterWsTab(_ *core.Project) {
	a.tab = TabWebSocket
	a.tabCursor = 0
	a.wsOpen = false
	a.wsComposeOn = false
	a.wsEditing = false
	a.wsSearchOn = false
	a.wsShowAll = false
}

func (a *App) openWsClient(p *core.Project) tea.Cmd {
	a.wsOpen = true
	a.wsComposeOn = false
	a.wsEditing = false
	a.wsSearchOn = false
	a.wsShowAll = false
	a.wsEditSourceIdx = -1
	a.wsSubTab = wsTabOverview
	a.wsFocus = wsFocusConnections
	a.wsEdit = editorState{Anchor: -1}
	a.wsErr = ""
	a.wsStatus = "ready"
	a.wsMsgScroll = 0
	a.wsMsgHScroll = 0
	a.wsSendVScroll = 0
	a.wsSendHScroll = 0
	a.wsFilter = wsFilterAll
	a.wsPayloadMode = wsPayloadPretty
	a.wsSendMode = wsSendText
	if a.wsHeaders == "" {
		a.wsHeaders = "Origin: http://localhost\n"
	}
	if a.wsSend == "" {
		a.wsSend = "{\n  \"type\": \"ping\"\n}"
	}
	a.loadWsProjectConns(p)
	if strings.TrimSpace(a.wsURL) == "" {
		if len(a.wsRecent) > 0 {
			a.wsURL = a.wsRecent[0]
		} else {
			a.wsURL = a.defaultWsURL(p)
		}
	}
	a.rememberWsURL(a.wsURL)
	return nil
}

func (a *App) leaveWsTab() tea.Cmd {
	a.wsCloseSession()
	a.wsOpen = false
	a.wsComposeOn = false
	a.wsEditing = false
	a.wsSearchOn = false
	a.wsShowAll = false
	a.tab = TabWebSocket
	a.tabCursor = 0
	return nil
}

func (a *App) defaultWsURL(p *core.Project) string {
	port := 8080
	if p != nil {
		if ports := a.apiProjectPorts(p); len(ports) > 0 {
			port = ports[0]
		}
	}
	return fmt.Sprintf("ws://localhost:%d/ws", port)
}

func (a *App) rememberWsURL(u string) {
	u = strings.TrimSpace(u)
	if u == "" {
		return
	}
	out := []string{u}
	for _, r := range a.wsRecent {
		if r == u {
			continue
		}
		out = append(out, r)
		if len(out) >= 24 {
			break
		}
	}
	a.wsRecent = out
	a.wsRecentCursor = 0
	a.persistWsProjectConns()
}

func (a *App) loadWsProjectConns(p *core.Project) {
	if p == nil {
		return
	}
	a.wsRecent = wsutil.LoadProject(p.Path).URLs
	a.wsRecentCursor = 0
}

func (a *App) persistWsProjectConns() {
	p := a.currentProject()
	if p == nil {
		return
	}
	_ = wsutil.SaveProject(p.Path, wsutil.ProjectConfig{URLs: a.wsRecent})
}

// --- landing ---

func (a *App) renderWsLanding(p *core.Project) string {
	w, h := a.moduleSize()
	ctx := a.renderModuleContext(p, w, "WEBSOCKET", "ready")
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)
	openH := maxInt(7, bodyH*40/100)
	featH := maxInt(6, bodyH-openH)
	openLines := []string{
		StyleMuted.Render("Conversa ao vivo com o servidor."),
	}
	openLines = append(openLines, moduleOpenHint()...)
	featLines := []string{
		StyleMuted.Render("1.  na esquerda, ↑↓ escolhe o servidor"),
		StyleMuted.Render("2.  enter troca / conecta"),
		StyleMuted.Render("3.  m  escreve e envia"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("WEBSOCKET", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("COMO USAR", fitExactLines(featLines, featH-2), centerW, featH, false),
	)
	details := []string{
		StyleMuted.Render("O que é  ") + StyleNormal.Render("bate-papo em tempo real"),
		StyleMuted.Render("Local    ") + StyleMuted.Render("ws://localhost"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir"},
		[2]string{"n", "novo endereço"},
		[2]string{"c", "conectar"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

// --- main render ---

func (a *App) renderWsTab(p *core.Project) string {
	w := a.screenWidth()
	h := a.screenHeight()
	header := a.renderWsHeader(w)
	tabs := a.renderWsSubTabs(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(tabs)

	bodyH := maxInt(10, h-headerH-2)
	var body string
	switch a.wsSubTab {
	case wsTabSettings, wsTabHistory:
		body = a.renderWsSettings(w, bodyH)
	default:
		body = a.renderWsOverview(w, bodyH)
	}

	hints := a.wsHints()
	view := lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, a.renderStatusBar(hints))
	if a.wsComposeOn {
		return a.renderWsCompose()
	}
	return view
}

func (a *App) wsHints() string {
	if a.wsShowAll {
		return "↑↓ servidor  enter troca  A este projeto  esc"
	}
	if a.wsComposeOn {
		return "tab tipo  enter no Enviar  esc cancela"
	}
	if a.wsEditing {
		if a.wsFocus == wsFocusConnections {
			return "editando endereço  enter salva  esc cancela"
		}
		return "escrevendo  esc sai"
	}
	if a.wsSearchOn {
		return "buscar no texto  enter aplica  esc limpa"
	}
	base := "c conectar  d desligar  m mensagem  esc"
	if a.wsOnSettings() {
		base = "c conectar  n novo  e editar  0 conversa  esc"
	}
	switch a.wsFocus {
	case wsFocusConnections:
		base = "↑↓ servidor  enter troca  c liga  n novo  tab conversa  esc"
	case wsFocusMessages:
		base = "↑↓ conversa  m mensagem  tab servidores  esc"
	}
	if a.wsStatus != "" {
		return a.wsStatus + "  ·  " + base
	}
	return base
}

func (a *App) renderWsHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabWebSocket)).Bold(true)
	proj := "ws"
	if p := a.currentProject(); p != nil && p.Name != "" {
		proj = p.Name
	}
	showURL := strings.TrimSpace(a.wsURL)
	if live := a.liveWsURL(); live != "" {
		showURL = live
	}
	url := truncate(showURL, maxInt(20, width/3))
	left := accent.Render("devscope") + StyleMuted.Render(" › ") +
		StyleNormal.Render(proj) + StyleMuted.Render(" › ") + StyleNormal.Render(url)

	badge := StyleMuted.Render("○ Desconectado")
	switch {
	case a.wsStatus == "connecting…" || a.wsStatus == "connecting":
		badge = StyleWarning.Render(a.spinner() + " Conectando…")
	case a.wsConnected:
		badge = a.livePulse("Conectado")
	case a.wsErr != "":
		badge = StyleUnhealthy.Render("● Erro")
	}

	lat := ""
	if a.wsLatency > 0 {
		lat = StyleMuted.Render(fmt.Sprintf("  demora %dms", a.wsLatency.Milliseconds()))
	}
	meta := fmt.Sprintf("%s%s  %s",
		badge, lat,
		StyleMuted.Render(fmt.Sprintf("recebeu %d  enviou %d", a.wsStats.RecvFrames, a.wsStats.SentFrames)),
	)
	if a.wsErr != "" && !a.wsConnected {
		meta += "  " + StyleUnhealthy.Render(truncate(a.wsErr, 28))
	}
	gap := width - lipgloss.Width(stripANSI(left)) - 2
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", minInt(2, gap)) + "\n" + meta
}

func (a *App) wsOnSettings() bool {
	return a.wsSubTab == wsTabSettings || a.wsSubTab == wsTabHistory
}

func (a *App) renderWsSubTabs(width int) string {
	names := []string{"Conversa", "Ajustes"}
	active := 0
	if a.wsOnSettings() {
		active = 1
	}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf("%d:%s", i, n)
		if i == active {
			parts = append(parts, StyleSelected.Render(" "+label+" "))
		} else {
			parts = append(parts, StyleMuted.Render(" "+label+" "))
		}
	}
	line := strings.Join(parts, StyleMuted.Render("│"))
	help := StyleMuted.Render(" ?")
	pad := width - lipgloss.Width(stripANSI(line)) - 2
	if pad < 1 {
		pad = 1
	}
	return line + strings.Repeat(" ", pad) + help
}

// --- overview 3-column ---

func (a *App) renderWsOverview(width, height int) string {
	leftW := maxInt(40, width*52/100)
	if leftW > width-26 {
		leftW = maxInt(36, width-26)
	}
	rightW := maxInt(24, width-leftW-1)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		a.renderWsConnections(leftW, height),
		a.renderWsMessagesTable(rightW, height),
	)
}

func (a *App) renderWsLeftColumn(width, height int) string {
	connH := maxInt(8, height*38/100)
	statsH := maxInt(6, height*28/100)
	filtH := maxInt(5, height-connH-statsH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderWsConnections(width, connH),
		a.renderWsStatsBox(width, statsH),
		a.renderWsFiltersBox(width, filtH),
	)
}

func (a *App) wsServerEntries() []wsAllEntry {
	if a.wsShowAll {
		return a.wsAllConnEntries()
	}
	proj := ""
	if p := a.currentProject(); p != nil {
		proj = p.Name
	}
	out := make([]wsAllEntry, 0, len(a.wsRecent))
	for _, u := range a.wsRecent {
		out = append(out, wsAllEntry{Project: proj, URL: u})
	}
	return out
}

func (a *App) wsServerCursor() int {
	if a.wsShowAll {
		return a.wsAllCursor
	}
	return a.wsRecentCursor
}

type wsServerCols struct{ state, project, name int }

func (a *App) wsServerColumns(width int) wsServerCols {
	inner := maxInt(24, width-6)
	c := wsServerCols{state: 8}
	rest := inner - 1 - c.state - 1
	if a.wsShowAll {
		c.project = maxInt(8, minInt(16, rest*28/100))
		rest -= c.project + 1
	}
	c.name = maxInt(8, rest)
	return c
}

func (a *App) renderWsConnections(width, height int) string {
	focus := a.wsFocus == wsFocusConnections
	entries := a.wsServerEntries()
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	live := a.liveWsURL()
	cur := a.wsServerCursor()
	if cur >= len(entries) {
		cur = maxInt(0, len(entries)-1)
	}

	lines := []string{a.renderWsServerHeader(width), StyleMuted.Render(strings.Repeat("─", maxInt(16, width-6)))}
	if a.wsEditing && focus && !a.wsShowAll {
		ed := a.wsEdit
		editLines := renderEditorLines(a.wsURL, &ed, maxInt(8, width-6), 1, true, false)
		a.wsEdit = ed
		if len(editLines) > 0 {
			lines = append(lines, StyleWarning.Render("✎ ")+editLines[0])
			viewport = maxInt(1, viewport-1)
		}
	}
	if len(entries) == 0 {
		lines = append(lines, StyleMuted.Render("  nenhum — n adiciona"))
	} else {
		start := 0
		if cur >= viewport {
			start = cur - viewport + 1
		}
		end := minInt(start+viewport, len(entries))
		if start > 0 {
			lines[1] = StyleMuted.Render(fmt.Sprintf("↑ %d  ", start) + strings.Repeat("─", maxInt(8, width-14)))
		}
		for i := start; i < end; i++ {
			lines = append(lines, a.renderWsServerTableRow(entries[i], i == cur && focus && !a.wsEditing, live, width))
		}
		for i := end - start; i < viewport; i++ {
			lines = append(lines, "")
		}
		if rem := len(entries) - end; rem > 0 {
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("↓ %d abaixo", rem)))
		}
	}

	title := fmt.Sprintf("SERVIDORES (%d)", len(entries))
	if a.wsShowAll {
		title = fmt.Sprintf("SERVIDORES · TODOS (%d)", len(entries))
	}
	return renderApiTitledBox(title, fitExactLines(lines, inner), width, height, focus)
}

func (a *App) renderWsServerHeader(width int) string {
	cols := a.wsServerColumns(width)
	h := StyleTableHeader
	gap := lipgloss.NewStyle().Width(1).Render("")
	parts := []string{
		lipgloss.NewStyle().Width(1).Render(""),
		h.Width(cols.state).Render("STATE"),
		gap,
	}
	if cols.project > 0 {
		parts = append(parts, h.Width(cols.project).Render("PROJECT"), gap)
	}
	parts = append(parts, h.Width(cols.name).Render("NAME"))
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) renderWsServerTableRow(e wsAllEntry, selected bool, live string, width int) string {
	cols := a.wsServerColumns(width)
	gap := lipgloss.NewStyle().Width(1).Render("")
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	cell := func(w int, text string) string {
		return style.Width(w).MaxWidth(w).Render(truncate(text, w))
	}
	parts := []string{
		lipgloss.NewStyle().Width(1).Render(""),
		a.wsServerStateCell(e, selected, live, cols.state),
		gap,
	}
	if cols.project > 0 {
		projSt := style
		if !selected {
			if p := a.currentProject(); p != nil && p.Name == e.Project {
				projSt = StyleAccent
			} else {
				projSt = StyleWarning
			}
		}
		proj := e.Project
		if proj == "" {
			proj = "—"
		}
		parts = append(parts, projSt.Width(cols.project).MaxWidth(cols.project).Render(truncate(proj, cols.project)), gap)
	}
	parts = append(parts, cell(cols.name, shortWsLabel(e.URL)))
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) wsServerStateCell(e wsAllEntry, selected bool, live string, width int) string {
	on := live != "" && live == e.URL
	err := a.wsErr != "" && strings.TrimSpace(a.wsURL) == e.URL
	if selected {
		label := "PARADO"
		if on {
			label = "LIGADO"
		} else if err {
			label = "ERRO"
		}
		return StyleSelected.Width(width).MaxWidth(width).Render(label)
	}
	if on {
		return StyleRunning.Width(width).Render("ligado")
	}
	if err {
		return StyleUnhealthy.Width(width).Render("erro")
	}
	return StyleStopped.Width(width).Render("parado")
}

func (a *App) renderWsStatsBox(width, height int) string {
	st := a.wsStats
	status := "Desconectado"
	stStyle := StyleMuted
	if a.wsConnected {
		status = "Conectado"
		stStyle = StyleHealthy
	}
	up := "—"
	if a.wsConnected && !a.wsConnectedAt.IsZero() {
		up = formatDuration(time.Since(a.wsConnectedAt))
	}
	kv := []string{
		stStyle.Render(status),
		StyleMuted.Render("Há       ") + StyleNormal.Render(up),
		StyleMuted.Render("Recebeu  ") + StyleNormal.Render(fmt.Sprintf("%d msgs", st.RecvFrames)),
		StyleMuted.Render("Enviou   ") + StyleNormal.Render(fmt.Sprintf("%d msgs", st.SentFrames)),
		StyleMuted.Render("Erros    ") + StyleUnhealthy.Render(fmt.Sprintf("%d", st.Errors)),
	}
	return renderApiTitledBox("RESUMO", fitExactLines(kv, height-2), width, height, false)
}

func (a *App) renderWsFiltersBox(width, height int) string {
	focus := a.wsFocus == wsFocusFilters
	counts := a.wsFilterCounts()
	lines := make([]string, 0, len(wsFilterLabels))
	for i, label := range wsFilterLabels {
		mark := StyleMuted.Render("[ ] ")
		if wsFilterKind(i) == a.wsFilter {
			mark = StyleHealthy.Render("[✓] ")
		}
		n := counts[i]
		line := mark + StyleNormal.Render(label) + StyleMuted.Render(fmt.Sprintf(" (%d)", n))
		if focus && wsFilterKind(i) == a.wsFilter {
			line = StyleSelected.Render("▸ ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if q := strings.TrimSpace(a.wsSearch); q != "" {
		lines = append(lines, StyleMuted.Render("search: "+truncate(q, width-12)))
	}
	title := "MOSTRAR"
	if focus {
		title = "> MOSTRAR"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderWsMessagesTable(width, height int) string {
	focus := a.wsFocus == wsFocusMessages
	viewport := maxInt(1, height-2)
	vis := a.filteredWsFrames()
	a.syncWsFrameCursor(len(vis))

	hdr := fmt.Sprintf("%-8s %-9s  %s", "HORA", "QUEM", "TEXTO")
	if a.wsMsgHScroll > 0 {
		hdr = sliceColumns(hdr, a.wsMsgHScroll, maxInt(8, width-4))
	}
	lines := []string{StyleMuted.Render(hdr)}
	if len(vis) == 0 {
		lines = append(lines, StyleMuted.Render("  nada ainda — c conecta, m escreve"))
	} else {
		a.wsMsgScroll = ensureVisible(a.wsFrameCursor, a.wsMsgScroll, maxInt(1, viewport-1), len(vis))
		end := minInt(a.wsMsgScroll+viewport-1, len(vis))
		for i := a.wsMsgScroll; i < end; i++ {
			f := vis[i]
			row := a.formatWsFrameRow(f, width-4)
			if i == a.wsFrameCursor {
				row = StyleSelected.Render("▸ " + stripANSI(row))
			} else {
				row = "  " + row
			}
			lines = append(lines, row)
		}
	}
	title := fmt.Sprintf("MENSAGENS (%d)", len(vis))
	if focus {
		title = "> " + title
		if a.wsMsgHScroll > 0 {
			title += fmt.Sprintf("  ←%d", a.wsMsgHScroll)
		}
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) formatWsFrameRow(f wsFrame, width int) string {
	tm := f.Time.Format("15:04:05")
	who, whoSt := "info", StyleMuted
	switch f.Dir {
	case "in":
		who, whoSt = "servidor", StyleHealthy
	case "out":
		who, whoSt = "você", StyleNormal
	case "err":
		who, whoSt = "erro", StyleUnhealthy
	}
	payload := strings.ReplaceAll(f.Payload, "\n", " ")
	payloadW := maxInt(8, width-22)
	payload = sliceColumns(payload, a.wsMsgHScroll, payloadW)
	return StyleMuted.Render(tm+"  ") + whoSt.Render(fmt.Sprintf("%-9s", who)) + StyleNormal.Render(payload)
}

func (a *App) renderWsInspector(width, height int) string {
	focus := a.wsFocus == wsFocusInspector
	detH := maxInt(8, height*32/100)
	payH := maxInt(6, height*40/100)
	hdrH := maxInt(4, height-detH-payH)

	vis := a.filteredWsFrames()
	var f *wsFrame
	if len(vis) > 0 && a.wsFrameCursor >= 0 && a.wsFrameCursor < len(vis) {
		f = &vis[a.wsFrameCursor]
	}

	details := []string{StyleMuted.Render("escolha uma mensagem na lista")}
	payload := []string{StyleMuted.Render("—")}
	handshake := a.wsConnPlainLines()

	if f != nil {
		dir := "info"
		switch f.Dir {
		case "in":
			dir = "chegou do servidor"
		case "out":
			dir = "você enviou"
		case "err":
			dir = "deu erro"
		}
		details = []string{
			StyleMuted.Render("Quando    ") + StyleNormal.Render(f.Time.Format("15:04:05")),
			StyleMuted.Render("Quem      ") + StyleNormal.Render(dir),
			StyleMuted.Render("Tipo      ") + StyleNormal.Render(f.Kind),
			StyleMuted.Render("Tamanho   ") + StyleNormal.Render(humanBytes(f.Size)),
		}
		if f.Latency > 0 {
			details = append(details, StyleMuted.Render("Demora    ")+StyleWarning.Render(fmt.Sprintf("%dms", f.Latency.Milliseconds())))
		}
		payload = a.renderWsPayloadLines(f, width-2, payH-2)
	}

	modes := []string{"Legível", "Cru", "Hex"}
	var mp []string
	for i, m := range modes {
		if wsPayloadMode(i) == a.wsPayloadMode {
			mp = append(mp, StyleSelected.Render(m))
		} else {
			mp = append(mp, StyleMuted.Render(m))
		}
	}
	payTitle := "CONTEÚDO  " + strings.Join(mp, StyleMuted.Render("|"))
	if focus {
		payTitle = "> " + payTitle
	}

	dTitle := "ESTA MENSAGEM"
	hTitle := "CONEXÃO"
	if focus {
		dTitle = "> ESTA MENSAGEM"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox(dTitle, fitExactLines(details, detH-2), width, detH, focus),
		renderApiTitledBox(payTitle, fitExactLines(payload, payH-2), width, payH, focus),
		renderApiTitledBox(hTitle, fitExactLines(handshake, hdrH-2), width, hdrH, false),
	)
}

func (a *App) wsConnPlainLines() []string {
	if !a.wsConnected && a.wsInfo.URL == "" {
		return []string{StyleMuted.Render("aparece depois de conectar")}
	}
	url := firstNonEmpty(a.wsInfo.URL, a.wsURL)
	seguro := "não (ws)"
	if a.wsInfo.TLS || strings.HasPrefix(url, "wss://") {
		seguro = "sim (wss)"
	}
	estado := "desligado"
	if a.wsConnected {
		estado = "ligado"
	}
	lines := []string{
		StyleMuted.Render("Endereço  ") + StyleNormal.Render(truncate(url, 28)),
		StyleMuted.Render("Estado    ") + StyleNormal.Render(estado),
		StyleMuted.Render("Seguro    ") + StyleNormal.Render(seguro),
	}
	if a.wsInfo.Subprotocol != "" {
		lines = append(lines, StyleMuted.Render("Acordo    ")+StyleNormal.Render(a.wsInfo.Subprotocol))
	}
	return lines
}

func (a *App) renderWsPayloadLines(f *wsFrame, width, height int) []string {
	switch a.wsPayloadMode {
	case wsPayloadHex:
		h := hex.Dump([]byte(f.Payload))
		return strings.Split(strings.TrimRight(h, "\n"), "\n")
	case wsPayloadRaw:
		return strings.Split(f.Payload, "\n")
	default:
		if f.Kind == "json" || json.Valid([]byte(f.Payload)) {
			var v any
			if json.Unmarshal([]byte(f.Payload), &v) == nil {
				if b, err := json.MarshalIndent(v, "", "  "); err == nil {
					return strings.Split(string(b), "\n")
				}
			}
		}
		return strings.Split(f.Payload, "\n")
	}
}

// --- other subtabs ---

func (a *App) renderWsHistory(width, height int) string {
	lines := []string{StyleMuted.Render("o que você já mandou — enter manda de novo")}
	if len(a.wsHistory) == 0 {
		lines = append(lines, StyleMuted.Render("  (ainda não enviou nada)"))
	}
	for i, h := range a.wsHistory {
		lines = append(lines, fmt.Sprintf("  %2d  %s", i+1, truncate(strings.ReplaceAll(h, "\n", " "), width-8)))
	}
	return renderApiTitledBox("JÁ ENVIADAS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderWsSettings(width, height int) string {
	connH := maxInt(8, height*40/100)
	histH := maxInt(6, height*28/100)
	optH := maxInt(5, height-connH-histH)
	auto := "não"
	if a.wsAutoReconnect {
		auto = "sim"
	}
	hdrLines := strings.Split(strings.ReplaceAll(a.wsHeaders, "\r\n", "\n"), "\n")
	if a.wsEditing && a.wsOnSettings() && a.wsFocus == wsFocusFilters {
		ed := a.wsEdit
		hdrLines = renderEditorLines(a.wsHeaders, &ed, width-2, maxInt(2, optH-6), true, false)
		a.wsEdit = ed
	}
	opts := []string{
		StyleMuted.Render("a  Reconectar sozinho: ") + StyleNormal.Render(auto),
		StyleMuted.Render("u  Trocar porta do projeto"),
		StyleMuted.Render("A  Servidores de todos os projetos"),
		StyleMuted.Render("e  Editar endereço ou cabeçalhos"),
		StyleMuted.Render("cabeçalhos"),
	}
	opts = append(opts, hdrLines...)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderWsConnections(width, connH),
		a.renderWsHistory(width, histH),
		renderApiTitledBox("OPÇÕES", fitExactLines(opts, optH-2), width, optH, a.wsFocus == wsFocusFilters),
	)
}

type wsAllEntry struct {
	Project string
	URL     string
}

func (a *App) wsAllConnEntries() []wsAllEntry {
	out := make([]wsAllEntry, 0)
	curPath := ""
	if p := a.currentProject(); p != nil {
		curPath = p.Path
	}
	for _, p := range a.snapshot.Projects {
		urls := wsutil.LoadProject(p.Path).URLs
		if curPath != "" && pathsMatch(p.Path, curPath) {
			urls = a.wsRecent
		}
		for _, u := range urls {
			out = append(out, wsAllEntry{Project: p.Name, URL: u})
		}
	}
	return out
}

// --- keys ---

func (a *App) handleWsKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.wsComposeOn {
		return a.updateWsCompose(msg)
	}
	if a.wsEditing {
		return a.updateWsEdit(msg, p)
	}
	switch msg.String() {
	case "esc":
		if a.wsShowAll {
			a.wsShowAll = false
			a.wsStatus = "este projeto"
			return a, nil
		}
		return a, a.leaveWsTab()
	case "0":
		a.wsSubTab = wsTabOverview
		if a.wsFocus != wsFocusSend {
			a.wsFocus = wsFocusMessages
		}
	case "1", "2", "3":
		a.wsSubTab = wsTabSettings
		a.wsFocus = wsFocusConnections
	case "c":
		if a.wsFocus == wsFocusConnections {
			return a, a.connectSelectedWsURL()
		}
		if !a.wsConnected {
			return a, a.toggleWsConnect()
		}
	case "d":
		a.disconnectSelectedWsURL()
		return a, nil
	case "x", "delete", "backspace":
		if a.wsFocus == wsFocusConnections {
			a.deleteSelectedWsURL()
			return a, nil
		}
		if msg.String() == "x" && a.wsFocus == wsFocusSend {
			a.wsSend = ""
			a.wsSendVScroll = 0
			a.wsSendHScroll = 0
			return a, nil
		}
		if msg.String() == "x" && a.wsConnected {
			a.wsCloseSession()
			a.pushWsMeta("disconnected")
			a.wsStatus = "disconnected"
		}
	case "r":
		if a.wsFocus == wsFocusConnections {
			if u, ok := a.selectedWsURL(); ok {
				a.wsURL = u
			}
		}
		a.wsCloseSession()
		return a, a.toggleWsConnect()
	case "f":
		a.wsFocus = wsFocusFilters
		a.wsFilter = wsFilterKind((int(a.wsFilter) + 1) % len(wsFilterLabels))
		a.wsFrameCursor = 0
		a.wsStatus = "mostrar → " + wsFilterLabels[a.wsFilter]
	case "/":
		a.wsSearchOn = true
		a.wsSearchInput = a.wsSearch
		return a, nil
	case "tab":
		a.cycleWsFocus(1)
	case "shift+tab":
		a.cycleWsFocus(-1)
	case "m", "M":
		a.startWsCompose()
		return a, nil
	case "[", "]":
		if a.wsFocus == wsFocusInspector {
			delta := 1
			if msg.String() == "[" {
				delta = -1
			}
			a.wsPayloadMode = wsPayloadMode((int(a.wsPayloadMode) + delta + 3) % 3)
		}
	case "a":
		if a.wsOnSettings() {
			a.wsAutoReconnect = !a.wsAutoReconnect
		}
	case "A", "shift+a", "shift+A":
		a.toggleWsShowAll()
		return a, nil
	case "u":
		a.cycleWsPort(p)
	case "n", "N":
		a.startNewWsURL(p)
		return a, nil
	case "e":
		if a.wsFocus == wsFocusConnections {
			a.startEditSelectedWsURL()
			return a, nil
		}
		if a.wsOnSettings() && a.wsFocus == wsFocusFilters {
			a.beginWsEdit()
			return a, nil
		}
		a.startWsCompose()
		return a, nil
	case "enter":
		return a, a.wsEnterAction(p)
	case "left", "h":
		a.wsPan(-4, 0)
	case "right", "l":
		a.wsPan(4, 0)
	case "up", "k":
		if a.wsFocus == wsFocusSend {
			a.wsPan(0, -1)
		} else {
			a.wsNav(-1)
		}
	case "down", "j":
		if a.wsFocus == wsFocusSend {
			a.wsPan(0, 1)
		} else {
			a.wsNav(1)
		}
	case "pgup":
		a.wsFrameCursor = maxInt(0, a.wsFrameCursor-10)
	case "pgdown":
		a.wsFrameCursor = minInt(len(a.filteredWsFrames())-1, a.wsFrameCursor+10)
	case "ctrl+l":
		a.wsFrames = nil
		a.wsFrameCursor = 0
		a.wsMsgScroll = 0
		a.wsMsgHScroll = 0
		a.wsStatus = "log limpo"
	}
	return a, nil
}

func (a *App) wsPan(dx, dy int) {
	switch a.wsFocus {
	case wsFocusMessages:
		a.wsMsgHScroll = maxInt(0, a.wsMsgHScroll+dx)
		if dy != 0 {
			a.wsNav(dy)
		}
	case wsFocusSend:
		a.wsSendHScroll = maxInt(0, a.wsSendHScroll+dx)
		a.wsSendVScroll = maxInt(0, a.wsSendVScroll+dy)
	case wsFocusInspector:
		if dy != 0 {
			a.wsNav(dy)
		}
	}
}

func (a *App) toggleWsShowAll() {
	a.wsShowAll = !a.wsShowAll
	a.wsFocus = wsFocusConnections
	a.wsAllCursor = 0
	if a.wsShowAll {
		a.wsStatus = "todos os projetos"
	} else {
		a.wsStatus = "este projeto"
	}
}

func (a *App) pickWsAllEntry() tea.Cmd {
	entries := a.wsAllConnEntries()
	if a.wsAllCursor < 0 || a.wsAllCursor >= len(entries) {
		a.wsStatus = "nenhuma connection"
		return nil
	}
	e := entries[a.wsAllCursor]
	a.wsURL = e.URL
	a.rememberWsURL(e.URL)
	a.wsFocus = wsFocusConnections
	a.wsSubTab = wsTabOverview
	a.wsStatus = e.Project + " · " + shortWsHost(e.URL)
	if live := a.liveWsURL(); live != "" && live == e.URL {
		a.wsStatus = "já conectado"
		return nil
	}
	if a.wsConnected {
		a.wsCloseSession()
		a.pushWsMeta("switched")
	}
	return a.toggleWsConnect()
}

func (a *App) cycleWsFocus(delta int) {
	if a.wsOnSettings() {
		if a.wsFocus == wsFocusConnections {
			a.wsFocus = wsFocusFilters
		} else {
			a.wsFocus = wsFocusConnections
		}
		return
	}
	if a.wsFocus == wsFocusConnections {
		a.wsFocus = wsFocusMessages
	} else {
		a.wsFocus = wsFocusConnections
	}
	_ = delta
}

func (a *App) wsNav(delta int) {
	switch a.wsFocus {
	case wsFocusConnections:
		if a.wsShowAll {
			a.wsAllCursor = clampCursor(a.wsAllCursor+delta, len(a.wsAllConnEntries()))
			return
		}
		if len(a.wsRecent) == 0 {
			return
		}
		a.wsRecentCursor = clampCursor(a.wsRecentCursor+delta, len(a.wsRecent))
	case wsFocusFilters:
		a.wsFilter = wsFilterKind(clampCursor(int(a.wsFilter)+delta, len(wsFilterLabels)))
		a.wsFrameCursor = 0
	case wsFocusMessages, wsFocusInspector:
		a.wsFrameCursor = clampCursor(a.wsFrameCursor+delta, len(a.filteredWsFrames()))
	case wsFocusSend:
		a.wsSendVScroll = maxInt(0, a.wsSendVScroll+delta)
	}
}

func (a *App) wsEnterAction(p *core.Project) tea.Cmd {
	switch a.wsFocus {
	case wsFocusConnections:
		if a.wsShowAll {
			return a.pickWsAllEntry()
		}
		return a.connectSelectedWsURL()
	case wsFocusMessages:
		a.startWsCompose()
		return nil
	}
	if a.wsSubTab == wsTabHistory && len(a.wsHistory) > 0 {
		idx := clampCursor(a.wsFrameCursor, len(a.wsHistory))
		a.wsSend = a.wsHistory[idx]
		a.wsSubTab = wsTabOverview
		a.wsFocus = wsFocusSend
		return a.wsSendFrame()
	}
	_ = p
	return nil
}

func (a *App) beginWsEdit() {
	switch {
	case a.wsFocus == wsFocusConnections:
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsURL)), Anchor: -1}
	case a.wsOnSettings() && a.wsFocus == wsFocusFilters:
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsHeaders)), Anchor: -1}
	case a.wsFocus == wsFocusSend || a.wsSubTab == wsTabMessages:
		a.wsFocus = wsFocusSend
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsSend)), Anchor: -1}
	}
}

func (a *App) startNewWsURL(p *core.Project) {
	a.wsEditSourceIdx = -1
	a.wsSubTab = wsTabOverview
	a.wsFocus = wsFocusConnections
	a.wsURL = a.defaultWsURL(p)
	a.wsEditing = true
	a.wsEdit = editorState{Cursor: len([]rune(a.wsURL)), Anchor: -1}
	a.wsStatus = "nova url — enter salva"
}

func (a *App) startEditSelectedWsURL() {
	u, ok := a.selectedWsURL()
	if !ok {
		a.wsStatus = "nenhuma connection"
		return
	}
	a.wsEditSourceIdx = a.wsRecentCursor
	a.wsURL = u
	a.wsSubTab = wsTabOverview
	a.wsFocus = wsFocusConnections
	a.wsEditing = true
	a.wsEdit = editorState{Cursor: len([]rune(a.wsURL)), Anchor: -1}
	a.wsStatus = "editando url — enter salva"
}

func (a *App) selectedWsURL() (string, bool) {
	if a.wsShowAll {
		entries := a.wsAllConnEntries()
		if a.wsAllCursor < 0 || a.wsAllCursor >= len(entries) {
			return "", false
		}
		return entries[a.wsAllCursor].URL, true
	}
	if a.wsRecentCursor < 0 || a.wsRecentCursor >= len(a.wsRecent) {
		return "", false
	}
	return a.wsRecent[a.wsRecentCursor], true
}

// liveWsURL is the URL of the open socket — not the list cursor / editor draft.
func (a *App) liveWsURL() string {
	if !a.wsConnected {
		return ""
	}
	if u := strings.TrimSpace(a.wsInfo.URL); u != "" {
		return u
	}
	return strings.TrimSpace(a.wsURL)
}

func (a *App) connectSelectedWsURL() tea.Cmd {
	u, ok := a.selectedWsURL()
	if !ok {
		u = strings.TrimSpace(a.wsURL)
	}
	u = strings.TrimSpace(u)
	if u == "" {
		a.wsStatus = "nenhuma url"
		return nil
	}
	if live := a.liveWsURL(); live != "" && live == u {
		a.wsURL = u
		a.wsStatus = "já conectado"
		return nil
	}
	if a.wsConnected {
		a.wsCloseSession()
		a.pushWsMeta("switched")
	}
	a.wsURL = u
	a.rememberWsURL(u)
	return a.toggleWsConnect()
}

func (a *App) disconnectSelectedWsURL() {
	if a.wsFocus == wsFocusConnections {
		u, ok := a.selectedWsURL()
		if !ok {
			a.wsStatus = "nenhuma connection"
			return
		}
		if a.liveWsURL() != u {
			a.wsStatus = "não está conectada"
			return
		}
	}
	if !a.wsConnected {
		a.wsStatus = "já desconectado"
		return
	}
	a.wsCloseSession()
	a.pushWsMeta("disconnected")
	a.wsStatus = "disconnected"
}

func (a *App) deleteSelectedWsURL() {
	idx := a.wsRecentCursor
	if idx < 0 || idx >= len(a.wsRecent) {
		a.wsStatus = "nenhuma connection"
		return
	}
	u := a.wsRecent[idx]
	if a.wsConnected && strings.TrimSpace(a.wsURL) == u {
		a.wsCloseSession()
		a.pushWsMeta("disconnected")
	}
	a.wsRecent = append(a.wsRecent[:idx], a.wsRecent[idx+1:]...)
	a.persistWsProjectConns()
	if len(a.wsRecent) == 0 {
		a.wsRecentCursor = 0
		a.wsStatus = "connection removida"
		return
	}
	a.wsRecentCursor = clampCursor(idx, len(a.wsRecent))
	if strings.TrimSpace(a.wsURL) == u {
		a.wsURL = a.wsRecent[a.wsRecentCursor]
	}
	a.wsStatus = "connection removida"
}

func (a *App) saveWsURLFromEditor() {
	u := strings.TrimSpace(a.wsURL)
	if u == "" {
		a.wsStatus = "url vazia"
		return
	}
	a.wsURL = u
	if a.wsEditSourceIdx >= 0 && a.wsEditSourceIdx < len(a.wsRecent) {
		old := a.wsRecent[a.wsEditSourceIdx]
		a.wsRecent[a.wsEditSourceIdx] = u
		out := make([]string, 0, len(a.wsRecent))
		seen := map[string]bool{}
		for _, r := range a.wsRecent {
			if r == "" || seen[r] {
				continue
			}
			seen[r] = true
			out = append(out, r)
		}
		a.wsRecent = out
		for i, r := range a.wsRecent {
			if r == u {
				a.wsRecentCursor = i
				break
			}
		}
		a.persistWsProjectConns()
		if a.wsConnected && old != u && strings.TrimSpace(a.wsInfo.URL) == old {
			a.wsStatus = "url atualizada — reconecte com c"
		} else {
			a.wsStatus = "url atualizada"
		}
	} else {
		a.rememberWsURL(u)
		a.wsStatus = "url salva"
	}
	a.wsEditSourceIdx = -1
	a.wsSubTab = wsTabOverview
	a.wsFocus = wsFocusConnections
}

func (a *App) updateWsEdit(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		wasURL := a.wsFocus == wsFocusConnections
		a.wsEditing = false
		a.wsEdit.clearSel()
		a.wsEditSourceIdx = -1
		if wasURL {
			if live := a.liveWsURL(); live != "" {
				a.wsURL = live
			}
		}
		return a, nil
	case "tab":
		a.wsEditing = false
		a.cycleWsFocus(1)
		return a, nil
	case "shift+tab":
		a.wsEditing = false
		a.cycleWsFocus(-1)
		return a, nil
	case "enter", "ctrl+enter":
		if a.wsFocus == wsFocusConnections {
			a.wsEditing = false
			a.wsEdit.clearSel()
			a.saveWsURLFromEditor()
			return a, nil
		}
		if a.wsFocus != wsFocusFilters {
			return a, a.wsSendFrame()
		}
	case "shift+enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	multiline := a.wsFocus != wsFocusConnections
	text := a.wsSend
	if a.wsFocus == wsFocusConnections {
		text = a.wsURL
	} else if a.wsOnSettings() && a.wsFocus == wsFocusFilters {
		text = a.wsHeaders
	}
	newText, handled := editorApplyKey(msg, text, &a.wsEdit, multiline)
	if !handled {
		return a, nil
	}
	if a.wsFocus == wsFocusConnections {
		a.wsURL = newText
	} else if a.wsOnSettings() && a.wsFocus == wsFocusFilters {
		a.wsHeaders = newText
	} else {
		a.wsSend = newText
	}
	return a, nil
}

func (a *App) updateWsSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.wsSearchOn = false
		a.wsSearchInput = ""
		a.wsSearch = ""
		a.wsFrameCursor = 0
	case "enter":
		a.wsSearchOn = false
		a.wsSearch = strings.TrimSpace(a.wsSearchInput)
		a.wsSearchInput = ""
		a.wsFrameCursor = 0
	case "backspace":
		if len(a.wsSearchInput) > 0 {
			r := []rune(a.wsSearchInput)
			a.wsSearchInput = string(r[:len(r)-1])
		}
		a.wsSearch = strings.TrimSpace(a.wsSearchInput)
	default:
		if len(msg.String()) == 1 {
			a.wsSearchInput += msg.String()
			a.wsSearch = strings.TrimSpace(a.wsSearchInput)
			a.wsFrameCursor = 0
		}
	}
	return a, nil
}

// --- connect / send / events ---

func (a *App) cycleWsPort(p *core.Project) {
	ports := a.apiProjectPorts(p)
	if len(ports) == 0 {
		a.wsStatus = "nenhuma porta no projeto"
		return
	}
	a.wsPortIndex = (a.wsPortIndex + 1) % len(ports)
	port := ports[a.wsPortIndex]
	path := "/ws"
	u := strings.TrimSpace(a.wsURL)
	scheme := "ws"
	if i := strings.Index(u, "://"); i >= 0 {
		scheme = u[:i]
		rest := u[i+3:]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			path = rest[slash:]
		}
	}
	a.wsURL = fmt.Sprintf("%s://localhost:%d%s", scheme, port, path)
	a.wsStatus = fmt.Sprintf("porta → %d", port)
}

func (a *App) toggleWsConnect() tea.Cmd {
	if a.wsConnected {
		a.wsCloseSession()
		a.pushWsMeta("disconnected")
		a.wsStatus = "disconnected"
		return nil
	}
	url := strings.TrimSpace(a.wsURL)
	headers := a.wsHeaders
	a.rememberWsURL(url)
	a.wsErr = ""
	a.wsStatus = "connecting…"
	return func() tea.Msg {
		sess, err := wsutil.Dial(url, headers)
		if err != nil {
			return wsEventMsg{ev: wsutil.Event{Kind: "error", Text: err.Error()}, sess: nil}
		}
		return wsConnectedMsg{sess: sess}
	}
}

func (a *App) wsSendFrame() tea.Cmd {
	if strings.TrimSpace(a.wsSend) == "" {
		a.wsStatus = "escreva e aperte enter"
		return nil
	}
	if !a.wsConnected || a.wsSess == nil {
		a.wsErr = "conecte com c"
		a.wsStatus = ""
		return nil
	}
	live := a.liveWsURL()
	// Keep draft URL aligned with the live socket so the UI can't lie.
	if live != "" {
		a.wsURL = live
	}
	text := a.wsSend
	if a.wsSendMode == wsSendJSON {
		var v any
		if err := json.Unmarshal([]byte(text), &v); err != nil {
			a.wsErr = "JSON inválido: " + err.Error()
			return nil
		}
		if b, err := json.Marshal(v); err == nil {
			text = string(b)
		}
	}
	sess := a.wsSess
	mode := a.wsSendMode
	a.wsLastSendAt = time.Now()
	a.pushWsHistory(a.wsSend)
	a.wsStatus = "→ " + shortWsHost(live)
	return func() tea.Msg {
		var err error
		if mode == wsSendBinary {
			err = sess.SendBinary([]byte(text))
		} else {
			err = sess.Send(text)
		}
		if err != nil {
			return wsEventMsg{ev: wsutil.Event{Kind: "error", Text: err.Error()}, sess: sess}
		}
		return wsEventMsg{ev: wsutil.Event{
			Kind: "message", Text: text, Inbound: false,
			Binary: mode == wsSendBinary, ByteSize: len(text),
		}, sess: sess}
	}
}

func (a *App) wsCloseSession() {
	if a.wsSess != nil {
		a.wsSess.Close()
		a.wsSess = nil
	}
	a.wsConnected = false
}

func (a *App) waitWsEvent() tea.Cmd {
	sess := a.wsSess
	if sess == nil {
		return nil
	}
	ch := sess.Events()
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return wsEventMsg{ev: wsutil.Event{Kind: "disconnected", Text: "closed"}, sess: sess}
		}
		return wsEventMsg{ev: ev, sess: sess}
	}
}

func (a *App) handleWsMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case wsConnectedMsg:
		a.wsCloseSession()
		a.wsSess = m.sess
		a.wsConnected = true
		a.wsErr = ""
		a.wsStatus = "connected"
		a.wsInfo = m.sess.Info
		a.wsConnectedAt = time.Now()
		a.pushWsMeta("connected " + m.sess.Info.URL)
		if !a.wsOnSettings() {
			a.wsFocus = wsFocusMessages
		}
		cmds := []tea.Cmd{a.waitWsEvent()}
		if a.wsComposePending && a.wsComposeOn {
			a.wsComposePending = false
			if cmd := a.submitWsCompose(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)
	case wsEventMsg:
		if m.sess != nil && m.sess != a.wsSess {
			return a, nil
		}
		ev := m.ev
		switch ev.Kind {
		case "message":
			dir := "out"
			if ev.Inbound {
				dir = "in"
			}
			kind := "text"
			if ev.Binary {
				kind = "binary"
			} else if json.Valid([]byte(ev.Text)) {
				kind = "json"
			}
			var lat time.Duration
			if ev.Inbound && !a.wsLastSendAt.IsZero() {
				lat = time.Since(a.wsLastSendAt)
				a.wsLatency = lat
				a.recordWsLatency(lat)
				a.wsLastSendAt = time.Time{}
			}
			a.pushWsFrame(dir, kind, ev.Text, ev.ByteSize, lat)
			if ev.Inbound {
				a.wsStats.RecvFrames++
				a.wsStats.RecvBytes += ev.ByteSize
				a.wsStatus = "← frame"
				return a, a.waitWsEvent()
			}
			a.wsStats.SentFrames++
			a.wsStats.SentBytes += len(ev.Text)
			a.wsStatus = "→ sent"
			return a, nil
		case "error":
			a.wsComposePending = false
			a.wsErr = ev.Text
			a.wsStats.Errors++
			a.pushWsFrame("err", "error", ev.Text, len(ev.Text), 0)
			a.wsStatus = ""
			return a, nil
		case "disconnected":
			a.wsConnected = false
			a.wsSess = nil
			a.wsStats.Disconnects++
			a.wsStatus = "disconnected"
			detail := "disconnected"
			if ev.Text != "" && ev.Text != "closed" {
				detail = "disconnected: " + ev.Text
				a.wsErr = ev.Text
				a.wsStats.Errors++
			}
			a.pushWsMeta(detail)
			if a.wsAutoReconnect {
				a.wsStatus = "reconnecting…"
				return a, a.toggleWsConnect()
			}
			return a, nil
		}
	}
	return a, nil
}

// --- frames / filters ---

func (a *App) pushWsFrame(dir, kind, payload string, size int, lat time.Duration) {
	if size <= 0 {
		size = len(payload)
	}
	a.wsFrameSeq++
	a.wsFrames = append(a.wsFrames, wsFrame{
		ID: a.wsFrameSeq, Time: time.Now(), Dir: dir, Kind: kind,
		Size: size, Latency: lat, Payload: payload,
	})
	if len(a.wsFrames) > 1000 {
		a.wsFrames = a.wsFrames[len(a.wsFrames)-1000:]
	}
	a.wsFrameCursor = len(a.filteredWsFrames()) - 1
	if a.wsFrameCursor < 0 {
		a.wsFrameCursor = 0
	}
}

func (a *App) pushWsMeta(text string) {
	a.pushWsFrame("meta", "meta", text, len(text), 0)
}

func (a *App) pushWsHistory(payload string) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return
	}
	out := []string{payload}
	for _, h := range a.wsHistory {
		if h == payload {
			continue
		}
		out = append(out, h)
		if len(out) >= 50 {
			break
		}
	}
	a.wsHistory = out
}

func (a *App) recordWsLatency(d time.Duration) {
	if d <= 0 {
		return
	}
	st := &a.wsStats
	if st.LatencyN == 0 || d < st.LatencyMin {
		st.LatencyMin = d
	}
	if d > st.LatencyMax {
		st.LatencyMax = d
	}
	st.LatencySum += d
	st.LatencyN++
}

func (a *App) filteredWsFrames() []wsFrame {
	q := strings.ToLower(strings.TrimSpace(a.wsSearch))
	out := make([]wsFrame, 0, len(a.wsFrames))
	for _, f := range a.wsFrames {
		if !a.wsFrameMatchesFilter(f) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(f.Payload), q) &&
			!strings.Contains(strings.ToLower(f.Kind), q) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func (a *App) wsFrameMatchesFilter(f wsFrame) bool {
	switch a.wsFilter {
	case wsFilterText:
		return f.Kind == "text"
	case wsFilterJSON:
		return f.Kind == "json"
	case wsFilterBinary:
		return f.Kind == "binary"
	case wsFilterErrors:
		return f.Kind == "error" || f.Dir == "err"
	case wsFilterIn:
		return f.Dir == "in"
	case wsFilterOut:
		return f.Dir == "out"
	default:
		return true
	}
}

func (a *App) wsFilterCounts() []int {
	counts := make([]int, len(wsFilterLabels))
	counts[0] = len(a.wsFrames)
	for _, f := range a.wsFrames {
		switch f.Kind {
		case "text":
			counts[1]++
		case "json":
			counts[2]++
		case "binary":
			counts[3]++
		case "error":
			counts[4]++
		}
		if f.Dir == "err" {
			counts[4]++
		}
		if f.Dir == "in" {
			counts[5]++
		}
		if f.Dir == "out" {
			counts[6]++
		}
	}
	return counts
}

func (a *App) syncWsFrameCursor(n int) {
	if n <= 0 {
		a.wsFrameCursor = 0
		return
	}
	if a.wsFrameCursor >= n {
		a.wsFrameCursor = n - 1
	}
	if a.wsFrameCursor < 0 {
		a.wsFrameCursor = 0
	}
}

// --- helpers ---

func shortWsHost(u string) string {
	u = strings.TrimPrefix(u, "ws://")
	u = strings.TrimPrefix(u, "wss://")
	if i := strings.IndexByte(u, '/'); i >= 0 {
		return u[:i]
	}
	return u
}

func shortWsLabel(u string) string {
	u = strings.TrimPrefix(strings.TrimSpace(u), "ws://")
	u = strings.TrimPrefix(u, "wss://")
	return u
}

func humanBytes(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	return fmt.Sprintf("%.1fK", float64(n)/1024)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
