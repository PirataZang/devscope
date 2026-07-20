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
	wsTabSend
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
	RecvFrames int
	SentFrames int
	RecvBytes  int
	SentBytes  int
	Errors     int
	Disconnects int
	LatencyMin time.Duration
	LatencyMax time.Duration
	LatencySum time.Duration
	LatencyN   int
}

type wsEventMsg struct {
	ev   wsutil.Event
	sess *wsutil.Session
}

type wsConnectedMsg struct {
	sess *wsutil.Session
}

var wsFilterLabels = []string{"All", "Text", "JSON", "Binary", "Errors", "In", "Out"}

// --- lifecycle ---

func (a *App) enterWsTab(_ *core.Project) {
	a.tab = TabWebSocket
	a.tabCursor = 0
	a.wsOpen = false
	a.wsEditing = false
	a.wsSearchOn = false
}

func (a *App) openWsClient(p *core.Project) tea.Cmd {
	a.wsOpen = true
	a.wsEditing = false
	a.wsSearchOn = false
	a.wsSubTab = wsTabOverview
	a.wsFocus = wsFocusMessages
	a.wsEdit = editorState{Anchor: -1}
	a.wsErr = ""
	a.wsStatus = "ready"
	a.wsMsgScroll = 0
	a.wsFilter = wsFilterAll
	a.wsPayloadMode = wsPayloadPretty
	a.wsSendMode = wsSendJSON
	if a.wsHeaders == "" {
		a.wsHeaders = "Origin: http://localhost\n"
	}
	if a.wsSend == "" {
		a.wsSend = "{\n  \"type\": \"ping\"\n}"
	}
	if strings.TrimSpace(a.wsURL) == "" {
		a.wsURL = a.defaultWsURL(p)
	}
	a.rememberWsURL(a.wsURL)
	return nil
}

func (a *App) leaveWsTab() tea.Cmd {
	a.wsCloseSession()
	a.wsOpen = false
	a.wsEditing = false
	a.wsSearchOn = false
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
		if len(out) >= 8 {
			break
		}
	}
	a.wsRecent = out
	a.wsRecentCursor = 0
}

// --- landing ---

func (a *App) renderWsLanding(_ *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabWebSocket)).Bold(true)
	lines := []string{
		accent.Render("⚡  WebSocket Inspector"),
		StyleMuted.Render("observabilidade WS — estilo DevTools + LazyGit"),
		"",
		StyleSection.Render("ABRIR"),
		StyleNormal.Render("  pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para o Overview"),
		"",
		StyleSection.Render("OVERVIEW"),
		StyleMuted.Render("  esquerda  connections · filters"),
		StyleMuted.Render("  centro    messages + send"),
		StyleMuted.Render("  direita   inspector (details / payload / handshake)"),
		"",
		StyleSection.Render("ATALHOS"),
		StyleMuted.Render("  c connect   d disconnect   r reconnect"),
		StyleMuted.Render("  0-4 abas   tab painéis   / search   f filter"),
		StyleMuted.Render("  enter send · ctrl+enter na edição"),
	}
	return StylePanel.Render(strings.Join(lines, "\n"))
}

// --- main render ---

func (a *App) renderWsTab(p *core.Project) string {
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
	header := a.renderWsHeader(w)
	tabs := a.renderWsSubTabs(w)
	headerH := lipgloss.Height(header) + lipgloss.Height(tabs)

	bodyH := maxInt(10, h-headerH-2)
	var body string
	switch a.wsSubTab {
	case wsTabMessages:
		body = a.renderWsMessagesFull(w, bodyH)
	case wsTabSend:
		body = a.renderWsSendFull(w, bodyH)
	case wsTabHistory:
		body = a.renderWsHistory(w, bodyH)
	case wsTabSettings:
		body = a.renderWsSettings(w, bodyH)
	default:
		body = a.renderWsOverview(w, bodyH)
	}

	hints := a.wsHints()
	return lipgloss.JoinVertical(lipgloss.Left, header, tabs, body, a.renderStatusBar(hints))
}

func (a *App) wsHints() string {
	if a.wsEditing {
		return "editando  ctrl+enter send  esc sair"
	}
	if a.wsSearchOn {
		return "search  enter aplicar  esc limpar"
	}
	base := "c connect  d disconnect  r reconnect  tab painel  / search  f filter  0-4 aba  esc"
	if a.wsStatus != "" {
		return a.wsStatus + "  ·  " + base
	}
	return base
}

func (a *App) renderWsHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabWebSocket)).Bold(true)
	url := truncate(strings.TrimSpace(a.wsURL), maxInt(24, width/3))
	left := accent.Render("devscope") + StyleMuted.Render(" › ") + StyleNormal.Render(url)

	badge := StyleMuted.Render("○ Disconnected")
	switch {
	case a.wsStatus == "connecting…" || a.wsStatus == "connecting":
		badge = StyleWarning.Render("● Connecting")
	case a.wsConnected:
		badge = StyleHealthy.Render("● Connected")
	case a.wsErr != "":
		badge = StyleUnhealthy.Render("● Error")
	}

	lat := "—"
	if a.wsLatency > 0 {
		lat = fmt.Sprintf("%dms", a.wsLatency.Milliseconds())
	}
	tls := "Off"
	if a.wsInfo.TLS || strings.HasPrefix(a.wsURL, "wss://") {
		tls = "On"
	}
	comp := "Off"
	if a.wsInfo.Compression {
		comp = "permessage-deflate"
	}
	proto := a.wsInfo.Subprotocol
	if proto == "" {
		proto = "—"
	}
	auto := "Off"
	if a.wsAutoReconnect {
		auto = "On"
	}

	meta := fmt.Sprintf("%s  %s  RFC6455  %s  TLS:%s  Compress:%s  AutoReconnect:%s  ↑%d ↓%d",
		badge, StyleMuted.Render(lat), StyleMuted.Render(proto),
		StyleMuted.Render(tls), StyleMuted.Render(comp), StyleMuted.Render(auto),
		a.wsStats.SentFrames, a.wsStats.RecvFrames,
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

func (a *App) renderWsSubTabs(width int) string {
	names := []string{"Overview", "Messages", "Send", "History", "Settings"}
	var parts []string
	for i, n := range names {
		label := fmt.Sprintf("%d:%s", i, n)
		if wsSubTab(i) == a.wsSubTab {
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
	leftW := maxInt(22, width*22/100)
	if leftW > 34 {
		leftW = 34
	}
	rightW := maxInt(26, width*28/100)
	if rightW > 42 {
		rightW = 42
	}
	centerW := maxInt(30, width-leftW-rightW-2)

	sendH := maxInt(6, height*28/100)
	msgH := maxInt(6, height-sendH)

	left := a.renderWsLeftColumn(leftW, height)
	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderWsMessagesTable(centerW, msgH),
		a.renderWsSendBox(centerW, sendH),
	)
	right := a.renderWsInspector(rightW, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, center, right)
}

func (a *App) renderWsLeftColumn(width, height int) string {
	connH := maxInt(6, height*35/100)
	statsH := maxInt(7, height*30/100)
	filtH := maxInt(5, height-connH-statsH)
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderWsConnections(width, connH),
		a.renderWsStatsBox(width, statsH),
		a.renderWsFiltersBox(width, filtH),
	)
}

func (a *App) renderWsConnections(width, height int) string {
	focus := a.wsFocus == wsFocusConnections
	lines := make([]string, 0, height)
	if len(a.wsRecent) == 0 {
		lines = append(lines, StyleMuted.Render("  (nenhuma ainda)"))
	}
	for i, u := range a.wsRecent {
		host := shortWsHost(u)
		dot := StyleMuted.Render("○")
		if a.wsConnected && strings.TrimSpace(a.wsURL) == u {
			dot = StyleHealthy.Render("●")
		} else if i == 0 {
			dot = lipgloss.NewStyle().Foreground(ColorPrimary).Render("●")
		}
		mark := "  "
		if i == a.wsRecentCursor && focus {
			mark = StyleSelected.Render("▸ ")
		}
		lines = append(lines, mark+dot+" "+StyleNormal.Render(truncate(host, width-6)))
	}
	lines = append(lines, StyleMuted.Render("  + n nova url (settings)"))
	title := "CONNECTIONS"
	if focus {
		title = "> CONNECTIONS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderWsStatsBox(width, height int) string {
	st := a.wsStats
	status := "Disconnected"
	stStyle := StyleMuted
	if a.wsConnected {
		status = "Connected"
		stStyle = StyleHealthy
	}
	up := "—"
	if a.wsConnected && !a.wsConnectedAt.IsZero() {
		up = formatDuration(time.Since(a.wsConnectedAt))
	}
	lat := "—"
	if st.LatencyN > 0 {
		avg := st.LatencySum / time.Duration(st.LatencyN)
		lat = fmt.Sprintf("%dms / %dms / %dms", avg.Milliseconds(), st.LatencyMin.Milliseconds(), st.LatencyMax.Milliseconds())
	}
	kv := []string{
		stStyle.Render("Status  "+status),
		StyleMuted.Render("Uptime  ") + StyleNormal.Render(up),
		StyleMuted.Render("Frames  ") + StyleNormal.Render(fmt.Sprintf("↓%d  ↑%d", st.RecvFrames, st.SentFrames)),
		StyleMuted.Render("Bytes   ") + StyleNormal.Render(fmt.Sprintf("↓%s  ↑%s", humanBytes(st.RecvBytes), humanBytes(st.SentBytes))),
		StyleMuted.Render("Latency ") + StyleWarning.Render(lat),
		StyleMuted.Render("Errors  ") + StyleUnhealthy.Render(fmt.Sprintf("%d", st.Errors)),
	}
	return renderApiTitledBox("STATS", fitExactLines(kv, height-2), width, height, false)
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
	title := "FILTERS"
	if focus {
		title = "> FILTERS"
	}
	return renderApiTitledBox(title, fitExactLines(lines, height-2), width, height, focus)
}

func (a *App) renderWsMessagesTable(width, height int) string {
	focus := a.wsFocus == wsFocusMessages
	viewport := maxInt(1, height-2)
	vis := a.filteredWsFrames()
	a.syncWsFrameCursor(len(vis))

	lines := []string{StyleMuted.Render(fmt.Sprintf("%-8s %-2s %-6s %5s  %s", "TIME", "DIR", "TYPE", "SIZE", "PAYLOAD"))}
	if len(vis) == 0 {
		lines = append(lines, StyleMuted.Render("  sem frames — c conecta, enter envia"))
	} else {
		if a.wsFrameCursor < a.wsMsgScroll {
			a.wsMsgScroll = a.wsFrameCursor
		}
		if a.wsFrameCursor >= a.wsMsgScroll+viewport-1 {
			a.wsMsgScroll = a.wsFrameCursor - (viewport - 2)
		}
		if a.wsMsgScroll < 0 {
			a.wsMsgScroll = 0
		}
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
	title := fmt.Sprintf("MESSAGES (%d)", len(vis))
	if focus {
		title = "> " + title
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) formatWsFrameRow(f wsFrame, width int) string {
	tm := f.Time.Format("15:04:05")
	dir, dirSt := "●", StyleMuted
	switch f.Dir {
	case "in":
		dir, dirSt = "←", lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	case "out":
		dir, dirSt = "→", lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	case "err":
		dir, dirSt = "✕", StyleUnhealthy
	}
	kindSt := StyleMuted
	switch f.Kind {
	case "json":
		kindSt = StyleHealthy
	case "binary":
		kindSt = lipgloss.NewStyle().Foreground(ColorPrimary)
	case "error":
		kindSt = StyleUnhealthy
	case "text":
		kindSt = StyleNormal
	}
	payload := strings.ReplaceAll(f.Payload, "\n", " ")
	payload = truncate(payload, maxInt(8, width-28))
	return StyleMuted.Render(tm+" ") + dirSt.Render(dir+" ") + kindSt.Render(fmt.Sprintf("%-6s", f.Kind)) +
		StyleMuted.Render(fmt.Sprintf(" %4s  ", humanBytes(f.Size))) + StyleNormal.Render(payload)
}

func (a *App) renderWsSendBox(width, height int) string {
	focus := a.wsFocus == wsFocusSend
	modes := []string{"Text", "JSON", "Binary"}
	var modeParts []string
	for i, m := range modes {
		if wsSendMode(i) == a.wsSendMode {
			modeParts = append(modeParts, StyleSelected.Render(m))
		} else {
			modeParts = append(modeParts, StyleMuted.Render(m))
		}
	}
	head := strings.Join(modeParts, StyleMuted.Render(" │ "))
	innerH := maxInt(1, height-3)
	var body []string
	body = append(body, head)
	if a.wsEditing && focus {
		ed := a.wsEdit
		lines := renderEditorLines(a.wsSend, &ed, width-2, innerH, true, a.wsSendMode == wsSendJSON)
		a.wsEdit = ed
		body = append(body, lines...)
	} else {
		raw := strings.Split(strings.ReplaceAll(a.wsSend, "\r\n", "\n"), "\n")
		st := StyleMuted
		if focus {
			st = StyleNormal
		}
		for i := 0; i < innerH && i < len(raw); i++ {
			line := sanitizeTerminalLine(raw[i])
			if a.wsSendMode == wsSendJSON && strings.HasPrefix(strings.TrimSpace(line), "{") {
				body = append(body, renderJSONColumns(line, 0, width-2))
			} else {
				body = append(body, st.Render(truncate(line, width-2)))
			}
		}
	}
	title := "SEND  enter"
	if focus {
		title = "> SEND"
	}
	return renderApiTitledBox(title, fitExactLines(body, height-2), width, height, focus)
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

	details := []string{StyleMuted.Render("selecione um frame")}
	payload := []string{StyleMuted.Render("—")}
	handshake := strings.Split(wsutil.FormatHandshake(a.wsInfo), "\n")
	if !a.wsConnected && a.wsInfo.URL == "" {
		handshake = []string{StyleMuted.Render("handshake após connect")}
	}

	if f != nil {
		details = []string{
			StyleMuted.Render("Timestamp  ") + StyleNormal.Render(f.Time.Format("15:04:05.000")),
			StyleMuted.Render("Direction  ") + StyleNormal.Render(f.Dir),
			StyleMuted.Render("Type       ") + StyleNormal.Render(f.Kind),
			StyleMuted.Render("Size       ") + StyleNormal.Render(humanBytes(f.Size)),
			StyleMuted.Render("Frame #    ") + StyleNormal.Render(fmt.Sprintf("%d", f.ID)),
		}
		if f.Latency > 0 {
			details = append(details, StyleMuted.Render("Latency    ")+StyleWarning.Render(fmt.Sprintf("%dms", f.Latency.Milliseconds())))
		}
		payload = a.renderWsPayloadLines(f, width-2, payH-2)
	}

	modes := []string{"Pretty", "Raw", "Hex"}
	var mp []string
	for i, m := range modes {
		if wsPayloadMode(i) == a.wsPayloadMode {
			mp = append(mp, StyleSelected.Render(m))
		} else {
			mp = append(mp, StyleMuted.Render(m))
		}
	}
	payTitle := "PAYLOAD  " + strings.Join(mp, StyleMuted.Render("|"))
	if focus {
		payTitle = "> " + payTitle
	}

	dTitle := "DETAILS"
	hTitle := "HANDSHAKE"
	if focus {
		dTitle = "> DETAILS"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox(dTitle, fitExactLines(details, detH-2), width, detH, focus),
		renderApiTitledBox(payTitle, fitExactLines(payload, payH-2), width, payH, focus),
		renderApiTitledBox(hTitle, fitExactLines(handshake, hdrH-2), width, hdrH, false),
	)
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

func (a *App) renderWsMessagesFull(width, height int) string {
	a.wsFocus = wsFocusMessages
	return a.renderWsMessagesTable(width, height)
}

func (a *App) renderWsSendFull(width, height int) string {
	a.wsFocus = wsFocusSend
	top := 3
	box := a.renderWsSendBox(width, height-top)
	hint := StyleMuted.Render("m cicla Text/JSON/Binary   e editar   ctrl+enter enviar")
	return lipgloss.JoinVertical(lipgloss.Left, hint, box)
}

func (a *App) renderWsHistory(width, height int) string {
	lines := []string{StyleMuted.Render("payloads enviados — enter reenvia")}
	if len(a.wsHistory) == 0 {
		lines = append(lines, StyleMuted.Render("  (vazio)"))
	}
	for i, h := range a.wsHistory {
		lines = append(lines, fmt.Sprintf("  %2d  %s", i+1, truncate(strings.ReplaceAll(h, "\n", " "), width-8)))
	}
	return renderApiTitledBox("HISTORY", fitExactLines(lines, height-2), width, height, true)
}

func (a *App) renderWsSettings(width, height int) string {
	focusURL := a.wsEditing && a.wsFocus == wsFocusSend // reuse send focus flag loosely — use messages=url
	_ = focusURL
	urlH := 4
	hdrH := maxInt(6, (height-urlH)*45/100)
	optH := maxInt(4, height-urlH-hdrH)

	urlLines := []string{StyleNormal.Render(truncate(a.wsURL, width-4))}
	if a.wsEditing && a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections {
		ed := a.wsEdit
		urlLines = renderEditorLines(a.wsURL, &ed, width-2, 1, true, false)
		a.wsEdit = ed
	}
	hdrLines := strings.Split(strings.ReplaceAll(a.wsHeaders, "\r\n", "\n"), "\n")
	if a.wsEditing && a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusFilters {
		ed := a.wsEdit
		hdrLines = renderEditorLines(a.wsHeaders, &ed, width-2, hdrH-2, true, false)
		a.wsEdit = ed
	}
	auto := "Off"
	if a.wsAutoReconnect {
		auto = "On"
	}
	opts := []string{
		StyleMuted.Render("a  Auto reconnect: ") + StyleNormal.Render(auto),
		StyleMuted.Render("u  Ciclar porta do projeto"),
		StyleMuted.Render("e  Editar URL (focus connections) / Headers (filters)"),
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("URL", fitExactLines(urlLines, urlH-2), width, urlH, a.wsFocus == wsFocusConnections),
		renderApiTitledBox("HEADERS", fitExactLines(hdrLines, hdrH-2), width, hdrH, a.wsFocus == wsFocusFilters),
		renderApiTitledBox("OPTIONS", fitExactLines(opts, optH-2), width, optH, false),
	)
}

// --- keys ---

func (a *App) handleWsKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.wsEditing {
		return a.updateWsEdit(msg, p)
	}
	switch msg.String() {
	case "esc":
		return a, a.leaveWsTab()
	case "0":
		a.wsSubTab = wsTabOverview
	case "1":
		a.wsSubTab = wsTabMessages
	case "2":
		a.wsSubTab = wsTabSend
	case "3":
		a.wsSubTab = wsTabHistory
	case "4":
		a.wsSubTab = wsTabSettings
		a.wsFocus = wsFocusConnections
	case "c":
		if !a.wsConnected {
			return a, a.toggleWsConnect()
		}
	case "d", "x":
		if msg.String() == "x" && a.wsFocus == wsFocusSend {
			a.wsSend = ""
			return a, nil
		}
		if a.wsConnected {
			a.wsCloseSession()
			a.pushWsMeta("disconnected")
			a.wsStatus = "disconnected"
		}
	case "r":
		a.wsCloseSession()
		return a, a.toggleWsConnect()
	case "f":
		a.wsFocus = wsFocusFilters
		a.wsFilter = wsFilterKind((int(a.wsFilter) + 1) % len(wsFilterLabels))
		a.wsFrameCursor = 0
		a.wsStatus = "filter → " + wsFilterLabels[a.wsFilter]
	case "/":
		a.wsSearchOn = true
		a.wsSearchInput = a.wsSearch
		return a, nil
	case "tab":
		a.cycleWsFocus(1)
	case "shift+tab":
		a.cycleWsFocus(-1)
	case "m":
		if a.wsFocus == wsFocusSend || a.wsSubTab == wsTabSend {
			a.wsSendMode = wsSendMode((int(a.wsSendMode) + 1) % 3)
		} else if a.wsFocus == wsFocusInspector {
			a.wsPayloadMode = wsPayloadMode((int(a.wsPayloadMode) + 1) % 3)
		}
	case "[", "]":
		if a.wsFocus == wsFocusInspector {
			delta := 1
			if msg.String() == "[" {
				delta = -1
			}
			a.wsPayloadMode = wsPayloadMode((int(a.wsPayloadMode) + delta + 3) % 3)
		}
	case "a":
		if a.wsSubTab == wsTabSettings {
			a.wsAutoReconnect = !a.wsAutoReconnect
		}
	case "u":
		a.cycleWsPort(p)
	case "e":
		a.beginWsEdit()
	case "enter":
		return a, a.wsEnterAction(p)
	case "up", "k":
		a.wsNav(-1)
	case "down", "j":
		a.wsNav(1)
	case "pgup":
		a.wsFrameCursor = maxInt(0, a.wsFrameCursor-10)
	case "pgdown":
		a.wsFrameCursor = minInt(len(a.filteredWsFrames())-1, a.wsFrameCursor+10)
	case "ctrl+l":
		a.wsFrames = nil
		a.wsFrameCursor = 0
		a.wsMsgScroll = 0
		a.wsStatus = "log limpo"
	}
	return a, nil
}

func (a *App) cycleWsFocus(delta int) {
	order := []wsFocus{wsFocusConnections, wsFocusFilters, wsFocusMessages, wsFocusSend, wsFocusInspector}
	idx := 0
	for i, f := range order {
		if f == a.wsFocus {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(order)) % len(order)
	a.wsFocus = order[idx]
}

func (a *App) wsNav(delta int) {
	switch a.wsFocus {
	case wsFocusConnections:
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
		// no-op
	}
}

func (a *App) wsEnterAction(p *core.Project) tea.Cmd {
	switch a.wsFocus {
	case wsFocusConnections:
		if a.wsRecentCursor >= 0 && a.wsRecentCursor < len(a.wsRecent) {
			a.wsURL = a.wsRecent[a.wsRecentCursor]
			if !a.wsConnected {
				return a.toggleWsConnect()
			}
		}
	case wsFocusFilters:
		a.wsFrameCursor = 0
	case wsFocusSend:
		return a.wsSendFrame()
	case wsFocusMessages, wsFocusInspector:
		a.wsFocus = wsFocusInspector
	}
	if a.wsSubTab == wsTabHistory && a.wsFrameCursor < len(a.wsHistory) {
		// re-send from history by index using frame cursor reused — use recent cursor
	}
	if a.wsSubTab == wsTabHistory && len(a.wsHistory) > 0 {
		idx := clampCursor(a.wsFrameCursor, len(a.wsHistory))
		a.wsSend = a.wsHistory[idx]
		a.wsSubTab = wsTabSend
		return a.wsSendFrame()
	}
	if a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections && !a.wsConnected {
		return a.toggleWsConnect()
	}
	_ = p
	return nil
}

func (a *App) beginWsEdit() {
	switch {
	case a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections:
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsURL)), Anchor: -1}
	case a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusFilters:
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsHeaders)), Anchor: -1}
	case a.wsFocus == wsFocusSend || a.wsSubTab == wsTabSend:
		a.wsFocus = wsFocusSend
		a.wsEditing = true
		a.wsEdit = editorState{Cursor: len([]rune(a.wsSend)), Anchor: -1}
	}
}

func (a *App) updateWsEdit(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.wsEditing = false
		a.wsEdit.clearSel()
		return a, nil
	case "ctrl+enter":
		a.wsEditing = false
		a.wsEdit.clearSel()
		if a.wsFocus == wsFocusSend || a.wsSubTab == wsTabSend {
			return a, a.wsSendFrame()
		}
		return a, nil
	}
	multiline := !(a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections)
	text := a.wsSend
	if a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections {
		text = a.wsURL
	} else if a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusFilters {
		text = a.wsHeaders
	}
	newText, handled := editorApplyKey(msg, text, &a.wsEdit, multiline)
	if !handled {
		return a, nil
	}
	if a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusConnections {
		a.wsURL = newText
	} else if a.wsSubTab == wsTabSettings && a.wsFocus == wsFocusFilters {
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
	if !a.wsConnected || a.wsSess == nil {
		a.wsErr = "conecte com c"
		a.wsStatus = ""
		return nil
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
		return a, a.waitWsEvent()
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
