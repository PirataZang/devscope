package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/core"
)

// Left stacked blocks (lazydocker-style).
type apiBlock int

const (
	apiBlockRequest apiBlock = iota // [1]
	apiBlockURL                     // [2]
	apiBlockHeaders                 // [3]
	apiBlockAuth                    // [4]
	apiBlockRight                   // right pane focus
)

type apiRightTab int

const (
	apiRightBody apiRightTab = iota
	apiRightResponse
)

type apiAuthType int

const (
	apiAuthNone apiAuthType = iota
	apiAuthBearer
	apiAuthBasic
)

type apiHistoryItem struct {
	Method string
	URL    string
}

var apiMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

func (a *App) initApiTab(p *core.Project) {
	if a.apiMethod == "" {
		a.apiMethod = "GET"
	}
	if a.apiHeaders == "" {
		a.apiHeaders = "Accept: application/json\n"
	}
	a.apiBlock = apiBlockRequest
	a.apiRightTab = apiRightResponse
	a.apiEditing = false
	a.apiEditorCursor = 0
	a.apiEditorAnchor = -1
	a.apiEditorScroll = 0
	a.apiResponseScroll = 0
	a.apiHScroll = 0
	a.apiSearchOn = false
	a.apiSearchQuery = ""
	a.apiSearchIdx = 0
	a.apiPortIndex = 0
	a.syncApiMethodCursor()
	if strings.TrimSpace(a.apiURL) == "" {
		if ports := a.apiProjectPorts(p); len(ports) > 0 {
			a.apiURL = fmt.Sprintf("http://localhost:%d", ports[0])
		} else {
			a.apiURL = "http://localhost:8080"
		}
	}
}

// enterApiTab selects TOOLS → API landing (sidebar + right pane). Enter opens the client.
func (a *App) enterApiTab(_ *core.Project) {
	a.tab = TabAPI
	a.tabCursor = 0
	a.apiOpen = false
	a.apiEditing = false
}

func (a *App) openApiClient(p *core.Project) {
	a.apiOpen = true
	if p != nil {
		a.initApiTab(p)
	}
}

// leaveApiTab closes the fullscreen client and returns to tab 7 landing.
func (a *App) leaveApiTab() tea.Cmd {
	a.apiOpen = false
	a.apiEditing = false
	a.apiAuthEditPass = false
	a.apiClearSel()
	a.apiSearchOn = false
	a.tab = TabAPI
	a.tabCursor = 0
	return nil
}

func (a *App) renderApiLanding(p *core.Project) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabAPI)).Bold(true)
	url := strings.TrimSpace(a.apiURL)
	if url == "" && p != nil {
		if ports := a.apiProjectPorts(p); len(ports) > 0 {
			url = fmt.Sprintf("http://localhost:%d", ports[0])
		}
	}
	lines := []string{
		accent.Render("↯  API"),
		StyleMuted.Render("cliente HTTP no contexto do projeto"),
		"",
		StyleSection.Render("ABRIR"),
		StyleNormal.Render("  pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para entrar"),
		StyleMuted.Render("  esc na API volta para esta aba"),
	}
	if url != "" {
		lines = append(lines, "",
			StyleSection.Render("ÚLTIMA URL"),
			StyleMuted.Render("  "+truncate(url, 48)),
		)
	}
	if len(a.apiHistory) > 0 {
		lines = append(lines, "", StyleSection.Render("HISTÓRICO"))
		n := minInt(5, len(a.apiHistory))
		for i := 0; i < n; i++ {
			h := a.apiHistory[i]
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  %-6s %s", h.Method, truncate(h.URL, 40))))
		}
	}
	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) syncApiMethodCursor() {
	a.apiMethodCursor = 0
	for i, m := range apiMethods {
		if m == a.apiMethod {
			a.apiMethodCursor = i
			return
		}
	}
}

func (a *App) apiProjectPorts(p *core.Project) []int {
	if p == nil {
		return nil
	}
	seen := map[int]bool{}
	var ports []int
	for _, port := range p.Ports {
		if port > 0 && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports
}

func (a *App) apiSidebarWidth() int {
	if a.width <= 0 {
		return 28
	}
	w := a.width * 32 / 100
	if w < 22 {
		return 22
	}
	if w > 40 {
		return 40
	}
	return w
}

func (a *App) renderApiTab(p *core.Project) string {
	height := maxInt(14, a.height-2)
	panelW := maxInt(20, a.width)
	innerW := maxInt(16, panelW-2)
	chrome := a.renderApiChrome(innerW)
	chromeH := lipgloss.Height(chrome)
	bodyHeight := maxInt(8, height-chromeH-2) // chrome + footer
	sideW := a.apiSidebarWidth()
	if sideW+28 > innerW {
		sideW = maxInt(20, innerW/3)
	}
	mainW := maxInt(24, innerW-sideW)

	left := a.renderApiLeftColumn(p, sideW, bodyHeight)
	right := a.renderApiRightColumn(mainW, bodyHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	footer := a.renderApiFooterLine(innerW)

	content := chrome + "\n" + body + "\n" + footer
	panel := clampRenderedHeight(content, height)

	method := a.apiMethod
	if method == "" {
		method = "GET"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		panel,
		a.renderStatusBar("api · "+method),
	)
}

func (a *App) renderApiChrome(width int) string {
	method := a.apiMethod
	if method == "" {
		method = "GET"
	}
	brand := StyleSection.Render("⚡ API")
	methodBadge := apiMethodStyle(method).Render(" " + method + " ")
	url := strings.TrimSpace(a.apiURL)
	if url == "" {
		url = "—"
	}
	url = fitApiFieldWindow(url, 0, maxInt(12, width/2), false)

	meta := StyleMuted.Render("scratchpad")
	if a.apiLoading {
		meta = StyleWarning.Render("● enviando…")
	} else if a.apiResponseErr != "" {
		meta = StyleUnhealthy.Render("● erro")
	} else if a.apiResponseStatus != "" {
		meta = a.apiStatusStyle().Render("● "+a.apiResponseStatus) + "  " +
			StyleMuted.Render(a.apiResponseTime.Round(time.Millisecond).String())
	}

	line1 := lipgloss.JoinHorizontal(lipgloss.Top, brand, "  ", methodBadge, "  ", meta)
	line2 := StyleMuted.Render("↗ ") + StyleNormal.Render(truncate(url, maxInt(10, width-4)))
	sep := StyleMuted.Render(strings.Repeat("─", maxInt(8, width)))
	return truncate(line1, width) + "\n" + truncate(line2, width) + "\n" + sep
}

func (a *App) renderApiFooterLine(width int) string {
	return StyleMuted.Render(truncate(a.apiFooter(), width))
}

func (a *App) renderApiLeftColumn(p *core.Project, width, height int) string {
	// Allocate heights like lazydocker stacked panels.
	reqH := 7 // border+5 methods
	urlH := 3
	remain := height - reqH - urlH
	if remain < 6 {
		// shrink request if terminal is short
		reqH = 5
		remain = height - reqH - urlH
	}
	if remain < 4 {
		remain = 4
		total := reqH + urlH + remain
		if total > height {
			reqH = maxInt(3, height-urlH-remain)
		}
	}
	headersH := remain / 2
	authH := remain - headersH
	if headersH < 3 {
		headersH = 3
	}
	if authH < 3 {
		authH = 3
	}
	// Fit exactly into height.
	used := reqH + urlH + headersH + authH
	if used > height {
		authH = maxInt(3, authH-(used-height))
	} else if used < height {
		authH += height - used
	}

	reqFocus := a.apiBlock == apiBlockRequest
	urlFocus := a.apiBlock == apiBlockURL
	headersFocus := a.apiBlock == apiBlockHeaders
	authFocus := a.apiBlock == apiBlockAuth
	// Titles must be plain text — ANSI inside borders breaks box width.
	req := renderApiTitledBox("[1]-Request", a.renderApiRequestBlockLines(width-2), width, reqH, reqFocus)
	url := renderApiTitledBox("[2]-URL", a.renderApiURLBlockLines(width-2), width, urlH, urlFocus)
	headers := renderApiTitledBox("[3]-Headers", a.renderApiHeadersBlockLines(width-2, headersH-2), width, headersH, headersFocus)
	auth := renderApiTitledBox("[4]-Auth", a.renderApiAuthBlockLines(width-2, authH-2), width, authH, authFocus)
	_ = p
	return lipgloss.JoinVertical(lipgloss.Left, req, url, headers, auth)
}

func (a *App) renderApiRightColumn(width, height int) string {
	title := a.renderApiRightTitle()
	lines := a.renderApiRightBodyLines(height-2, width-2)
	return renderApiTitledBox(title, lines, width, height, a.apiBlock == apiBlockRight)
}

func (a *App) renderApiRightTitle() string {
	// Plain ASCII only — used inside box border.
	bodyLabel := "Body"
	respLabel := "Response"
	if a.apiRightTab == apiRightBody {
		if a.apiEditing {
			bodyLabel = "* Body"
		} else {
			bodyLabel = "> Body"
		}
	} else {
		respLabel = "> Response"
	}
	meta := ""
	if a.apiLoading {
		meta = " · enviando..."
	} else if a.apiResponseErr != "" {
		meta = " · falha"
	} else if a.apiResponseStatus != "" {
		meta = " · " + a.apiResponseStatus
	}
	return bodyLabel + " | " + respLabel + meta
}

func renderApiTitledBox(title string, lines []string, width, height int, focused bool) string {
	if height < 3 {
		height = 3
	}
	innerW := maxInt(4, width-2)
	innerH := maxInt(1, height-2)

	body := make([]string, innerH)
	for i := 0; i < innerH; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		// Strip styles for width fit of content that already has ANSI.
		if lipgloss.Width(line) > innerW {
			line = truncate(line, innerW)
		}
		body[i] = padRightVisible(line, innerW)
	}

	borderColor := ColorBorder
	if focused {
		borderColor = ColorAccent
	}

	// Title is plain text only (no ANSI), so border math stays exact.
	title = stripANSI(title)
	title = truncate(title, maxInt(1, innerW-2))
	titleW := lipgloss.Width(title)
	pad := innerW - 1 - titleW
	if pad < 0 {
		pad = 0
	}
	topPlain := "─" + title + strings.Repeat("─", pad)
	if lipgloss.Width(topPlain) > innerW {
		topPlain = truncate(topPlain, innerW)
	}
	topPlain = padRightVisible(topPlain, innerW)

	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(borderColor)
	if focused {
		titleStyle = titleStyle.Bold(true)
	}
	b.WriteString(titleStyle.Render("┌" + topPlain + "┐"))
	b.WriteByte('\n')
	side := lipgloss.NewStyle().Foreground(borderColor)
	for _, line := range body {
		b.WriteString(side.Render("│"))
		b.WriteString(line)
		b.WriteString(side.Render("│"))
		b.WriteByte('\n')
	}
	b.WriteString(side.Render("└" + strings.Repeat("─", innerW) + "┘"))
	return b.String()
}

func stripANSI(s string) string {
	var b strings.Builder
	inESC := false
	for _, r := range s {
		if r == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inESC = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func apiMethodStyle(method string) lipgloss.Style {
	switch strings.ToUpper(method) {
	case "GET":
		return lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	case "POST":
		return lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	case "PUT":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true) // ciano
	case "PATCH":
		return lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	case "DELETE":
		return lipgloss.NewStyle().Foreground(ColorDanger).Bold(true)
	default:
		return StyleNormal.Bold(true)
	}
}

func (a *App) renderApiRequestBlockLines(width int) []string {
	lines := make([]string, 0, len(apiMethods))
	for i, m := range apiMethods {
		selected := m == a.apiMethod
		focused := a.apiBlock == apiBlockRequest && i == a.apiMethodCursor
		switch {
		case focused || selected:
			prefix := "● "
			if focused {
				prefix = "▶ "
			}
			lines = append(lines, apiMethodStyle(m).Render(truncate(prefix+m, width)))
		default:
			lines = append(lines, StyleMuted.Render(truncate("  "+m, width)))
		}
	}
	return lines
}

func (a *App) renderApiURLBlockLines(width int) []string {
	editing := a.apiEditing && a.apiBlock == apiBlockURL
	selLo, selHi, _ := a.apiSelRange()
	url := fitApiFieldWindowSel(a.apiURL, a.apiEditorCursor, width, editing, selLo, selHi)
	if a.apiBlock == apiBlockURL {
		return []string{StyleSelected.Render(padRightVisible(url, width))}
	}
	return []string{StyleNormal.Render(padRightVisible(url, width))}
}

// fitApiFieldWindow keeps the cursor (or the end of the text) visible in a fixed width.
func fitApiFieldWindow(text string, cursor, width int, showCursor bool) string {
	return fitApiFieldWindowSel(text, cursor, width, showCursor, -1, -1)
}

// fitApiFieldWindowSel is fitApiFieldWindow with an optional selection highlight [selLo,selHi).
func fitApiFieldWindowSel(text string, cursor, width int, showCursor bool, selLo, selHi int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if selLo < 0 || selHi <= selLo {
		selLo, selHi = -1, -1
	} else if selHi > len(runes) {
		selHi = len(runes)
	}

	displayLen := len(runes)
	if showCursor {
		displayLen++
	}
	start := 0
	if displayLen > width {
		if showCursor {
			start = cursor - width + 1
			if start < 0 {
				start = 0
			}
		} else {
			start = len(runes) - width
			if start < 0 {
				start = 0
			}
		}
	}

	var b strings.Builder
	cols := 0
	i := start
	for cols < width {
		if showCursor && i == cursor {
			b.WriteRune('█')
			cols++
			if cols >= width {
				break
			}
		}
		if i >= len(runes) {
			break
		}
		s := string(runes[i])
		if selLo >= 0 && i >= selLo && i < selHi {
			s = StyleApiSel.Render(s)
		}
		b.WriteString(s)
		cols++
		i++
	}
	out := b.String()
	if start > 0 && !showCursor {
		plain := []rune(ansi.Strip(out))
		if len(plain) > 0 {
			rest := ""
			if len(plain) > 1 {
				rest = string(plain[1:])
			}
			out = "…" + rest
		}
	}
	return out
}

func (a *App) renderApiHeadersBlockLines(width, height int) []string {
	if a.apiEditing && a.apiBlock == apiBlockHeaders {
		return a.renderApiMultilineEdit(a.apiHeaders, width, height, false)
	}
	content := a.apiHeaders
	if strings.TrimSpace(content) == "" {
		content = "Key: Value"
	}
	raw := strings.Split(content, "\n")
	if a.apiBlock == apiBlockHeaders {
		a.apiEditorScroll = clampScroll(a.apiEditorScroll, height, len(raw))
	}
	start := 0
	if a.apiBlock == apiBlockHeaders {
		start = a.apiEditorScroll
	}
	end := minInt(start+height, len(raw))
	lines := make([]string, 0, height)
	style := StyleMuted
	if a.apiBlock == apiBlockHeaders {
		style = StyleNormal
	}
	for _, line := range raw[start:end] {
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), width)))
	}
	return fitExactLines(lines, height)
}

func (a *App) renderApiAuthBlockLines(width, height int) []string {
	editing := a.apiEditing && a.apiBlock == apiBlockAuth
	var lines []string

	switch a.apiAuthType {
	case apiAuthBearer:
		lines = append(lines, StyleTabActive.Render("Bearer Token"))
		token := strings.TrimSpace(a.apiAuthToken)
		switch {
		case editing:
			selLo, selHi, _ := a.apiSelRange()
			view := fitApiFieldWindowSel(a.apiAuthToken, a.apiEditorCursor, width, true, selLo, selHi)
			lines = append(lines, StyleSelected.Render(padRightVisible(view, width)))
			lines = append(lines, StyleMuted.Render("ctrl+a tudo  shift+←→  esc"))
		case token == "":
			lines = append(lines, StyleMuted.Render("(sem token)"))
			lines = append(lines, StyleMuted.Render("e  editar"))
		default:
			// Idle: show end of token (like URL), not a dead truncate from the start.
			view := fitApiFieldWindow(apiMaskSecret(token, width*2), 0, width, false)
			lines = append(lines, StyleNormal.Render(padRightVisible(view, width)))
			lines = append(lines, StyleMuted.Render("e  editar"))
		}
	case apiAuthBasic:
		lines = append(lines, StyleTabActive.Render("Basic Auth"))
		user := a.apiAuthUser
		passShow := apiMaskSecret(a.apiAuthPass, width)
		if editing {
			selLo, selHi, _ := a.apiSelRange()
			if a.apiAuthEditPass {
				bullets := strings.Repeat("•", len([]rune(a.apiAuthPass)))
				passShow = fitApiFieldWindowSel(bullets, a.apiEditorCursor, width, true, selLo, selHi)
				lines = append(lines,
					StyleMuted.Render("user  "+truncate(user, maxInt(4, width-6))),
					StyleMuted.Render("pass"),
					StyleSelected.Render(padRightVisible(passShow, width)),
					StyleMuted.Render("ctrl+a  shift+←→  tab user"),
				)
			} else {
				userView := fitApiFieldWindowSel(user, a.apiEditorCursor, width, true, selLo, selHi)
				lines = append(lines,
					StyleMuted.Render("user"),
					StyleSelected.Render(padRightVisible(userView, width)),
					StyleMuted.Render("pass  "+passShow),
					StyleMuted.Render("ctrl+a  shift+←→  tab pass"),
				)
			}
		} else {
			if strings.TrimSpace(user) == "" {
				user = "(vazio)"
			}
			if strings.TrimSpace(a.apiAuthPass) == "" {
				passShow = "(vazio)"
			}
			lines = append(lines,
				StyleMuted.Render("user  "+truncate(user, maxInt(4, width-6))),
				StyleMuted.Render("pass  "+truncate(passShow, maxInt(4, width-6))),
				StyleMuted.Render("e  editar"),
			)
		}
	default:
		lines = append(lines,
			StyleMuted.Render("Sem autenticação"),
			StyleMuted.Render(""),
			StyleMuted.Render("a / ↑↓  trocar tipo"),
		)
	}

	// Compact type switcher at the bottom when there's room.
	if height >= len(lines)+2 {
		lines = append(lines, "")
		lines = append(lines, StyleMuted.Render(apiAuthTypeBar(a.apiAuthType, width)))
	}
	return fitExactLines(lines, height)
}

func apiAuthTypeBar(cur apiAuthType, width int) string {
	parts := []string{"none", "bearer", "basic"}
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(" · ")
		}
		if apiAuthType(i) == cur {
			b.WriteString(">" + p + "<")
		} else {
			b.WriteString(p)
		}
	}
	return truncate(b.String(), width)
}

func apiMaskSecret(s string, width int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	n := len([]rune(s))
	if n <= 4 {
		return strings.Repeat("•", n)
	}
	// show last 4 chars for recognition
	runes := []rune(s)
	tail := string(runes[n-4:])
	masked := strings.Repeat("•", minInt(n-4, maxInt(4, width-6))) + tail
	return masked
}

func (a *App) cycleApiAuth(delta int) {
	n := 3
	cur := int(a.apiAuthType) + delta
	for cur < 0 {
		cur += n
	}
	a.apiAuthType = apiAuthType(cur % n)
	a.apiAuthEditPass = false
	a.apiEditing = false
}

func (a *App) renderApiRightBodyLines(height, width int) []string {
	if a.apiRightTab == apiRightBody {
		return a.renderApiTextContent(a.apiBody, height, width, "Body raw JSON", a.apiEditing && a.apiBlock == apiBlockRight)
	}
	return a.renderApiResponsePanel(height, width)
}

func (a *App) renderApiTextContent(content string, viewport, width int, hint string, editing bool) []string {
	innerW := maxInt(8, width-2)
	if editing {
		return a.renderApiMultilineEdit(content, innerW, viewport, true)
	}
	if strings.TrimSpace(content) == "" {
		return fitExactLines([]string{StyleMuted.Render(hint)}, viewport)
	}
	raw := strings.Split(content, "\n")
	a.apiEditorScroll = clampScroll(a.apiEditorScroll, viewport, len(raw))
	start := a.apiEditorScroll
	end := minInt(start+viewport, len(raw))
	lines := make([]string, 0, viewport)
	hScroll := a.apiHScroll
	for _, line := range raw[start:end] {
		plain := sanitizeTerminalLine(line)
		display := renderJSONColumns(plain, hScroll, innerW)
		lines = append(lines, display)
	}
	return fitExactLines(lines, viewport)
}

// renderApiMultilineEdit renders Body/Headers with cursor + selection (ANSI-safe truncate).
func (a *App) renderApiMultilineEdit(content string, width, height int, highlightJSON bool) []string {
	runes := []rune(content)
	cursor := a.apiEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	selLo, selHi, hasSel := a.apiSelRange()
	var kinds []uint8
	if highlightJSON {
		kinds = jsonKindsForRunes(content)
	}

	type lineSpan struct{ start, end int }
	var spans []lineSpan
	start := 0
	for i, r := range runes {
		if r == '\n' {
			spans = append(spans, lineSpan{start, i})
			start = i + 1
		}
	}
	spans = append(spans, lineSpan{start, len(runes)})

	cursorLine := 0
	for i, sp := range spans {
		if cursor <= sp.end {
			cursorLine = i
			break
		}
		if i == len(spans)-1 {
			cursorLine = i
		}
	}
	a.apiEditorScroll = ensureVisible(cursorLine, a.apiEditorScroll, height, len(spans))
	from := a.apiEditorScroll
	to := minInt(from+height, len(spans))

	out := make([]string, 0, height)
	for _, sp := range spans[from:to] {
		var b strings.Builder
		if cursor == sp.start && cursor == sp.end {
			b.WriteRune('█')
		}
		for i := sp.start; i < sp.end; i++ {
			if i == cursor {
				b.WriteRune('█')
			}
			s := string(runes[i])
			switch {
			case hasSel && i >= selLo && i < selHi:
				s = StyleApiSel.Render(s)
			case highlightJSON && i < len(kinds):
				s = styleJSONRune(kinds[i], s)
			}
			b.WriteString(s)
		}
		if cursor == sp.end && (sp.end == len(runes) || (sp.end < len(runes) && runes[sp.end] == '\n')) {
			if sp.start != sp.end {
				b.WriteRune('█')
			}
		}
		out = append(out, ansi.Truncate(b.String(), width, "…"))
	}
	return fitExactLines(out, height)
}

// renderJSONColumns windows a JSON line by column, then applies syntax colors.
func renderJSONColumns(line string, start, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(line)
	kinds := jsonKindsForRunes(line)
	if start < 0 {
		start = 0
	}
	if start >= len(runes) {
		return strings.Repeat(" ", width)
	}
	end := start + width
	if end > len(runes) {
		end = len(runes)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(styleJSONRune(kinds[i], string(runes[i])))
	}
	pad := width - (end - start)
	if pad > 0 {
		b.WriteString(strings.Repeat(" ", pad))
	}
	return b.String()
}

func (a *App) apiStatusStyle() lipgloss.Style {
	code := a.apiResponseCode
	switch {
	case code >= 200 && code < 300:
		return StyleHealthy
	case code >= 400:
		return StyleUnhealthy
	case code >= 300:
		return StyleWarning
	default:
		return StyleMuted
	}
}

func (a *App) renderApiResponsePanel(viewport, width int) []string {
	if a.apiLoading {
		return fitExactLines([]string{StyleMuted.Render("Aguardando resposta...")}, viewport)
	}
	if a.apiResponseErr != "" {
		return fitExactLines(wrapAPIErrorLines(a.apiResponseErr, width), viewport)
	}
	if a.apiResponseBody == "" && a.apiResponseStatus == "" {
		return fitExactLines([]string{
			StyleSection.Render("Pronto para enviar"),
			StyleMuted.Render("monte o request à esquerda"),
			StyleMuted.Render("e pressione enter"),
			"",
			StyleMuted.Render("dica: https://httpbin.org/get"),
		}, viewport)
	}

	header := []string{
		a.apiStatusStyle().Render(truncate(a.apiResponseStatus, width)) + "  " +
			StyleMuted.Render(a.apiResponseTime.Round(time.Millisecond).String()),
	}
	if a.apiSearchQuery != "" {
		matches := a.apiSearchMatches()
		if len(matches) == 0 {
			header = append(header, StyleMuted.Render("/0"))
		} else {
			header = append(header, StyleAccent.Render(fmt.Sprintf("/%d/%d", a.apiSearchIdx+1, len(matches))))
		}
	}

	bodyLines := strings.Split(a.apiResponseBody, "\n")
	if a.apiResponseHeaders != "" && a.apiShowResponseHeaders {
		bodyLines = append(strings.Split(a.apiResponseHeaders, "\n"), "", "---", "")
		bodyLines = append(bodyLines, strings.Split(a.apiResponseBody, "\n")...)
	}

	avail := maxInt(1, viewport-len(header))
	a.apiResponseScroll = clampScroll(a.apiResponseScroll, avail, len(bodyLines))
	start := a.apiResponseScroll
	end := minInt(start+avail, len(bodyLines))
	matchSet := a.apiMatchLineSet()
	current := -1
	if matches := a.apiSearchMatches(); len(matches) > 0 {
		current = matches[a.apiSearchIdx]
	}

	out := append([]string{}, header...)
	for i := start; i < end; i++ {
		display := sliceColumns(sanitizeTerminalLine(bodyLines[i]), a.apiHScroll, maxInt(8, width-2))
		switch {
		case i == current:
			out = append(out, StyleDiffMatch.Render(display))
		case matchSet[i]:
			out = append(out, StyleWarning.Render(display))
		default:
			out = append(out, StyleNormal.Render(display))
		}
	}
	return fitExactLines(out, viewport)
}

func (a *App) apiFooter() string {
	if a.apiEditing {
		if a.apiBlock == apiBlockRight && a.apiRightTab == apiRightBody {
			return "body  ctrl+a tudo  shift+←→ sel  tab indent  esc  ctrl+enter send"
		}
		return "editando  ctrl+a tudo  shift+←→ sel  esc  enter send"
	}
	switch a.apiBlock {
	case apiBlockRequest:
		return "↑↓ método  tab blocos  → Body/Resp  [] abas  enter send  esc abas"
	case apiBlockURL:
		return "digite a URL  tab próximo  → Body/Resp  [] abas  enter send  esc abas"
	case apiBlockHeaders:
		return "digite para editar  tab próximo  → Body/Resp  [] abas  enter send  esc abas"
	case apiBlockAuth:
		if a.apiEditing {
			return "editando auth  tab user/pass  esc sair  enter send"
		}
		return "a/↑↓ tipo  e editar  enter send  esc abas"
	default:
		base := "e editar body  [] Body/Resp  tab Request  / buscar  enter send"
		if a.apiSearchQuery != "" {
			base += "  N/P match"
		}
		return base + "  esc abas"
	}
}

// apiBlockEditable: auto-edit on typing. Body and Auth require explicit `e`.
func (a *App) apiBlockEditable() bool {
	switch a.apiBlock {
	case apiBlockURL, apiBlockHeaders:
		return true
	default:
		return false
	}
}

func (a *App) apiCycleLeftBlock(forward bool) {
	order := []apiBlock{apiBlockRequest, apiBlockURL, apiBlockHeaders, apiBlockAuth}
	idx := 0
	for i, b := range order {
		if b == a.apiBlock {
			idx = i
			break
		}
	}
	if a.apiBlock == apiBlockRight {
		if forward {
			a.apiBlock = apiBlockRequest
		} else {
			a.apiBlock = apiBlockAuth
		}
	} else if forward {
		a.apiBlock = order[(idx+1)%len(order)]
	} else {
		a.apiBlock = order[(idx-1+len(order))%len(order)]
	}
	if a.apiBlock == apiBlockRequest {
		a.syncApiMethodCursor()
	}
	if a.apiBlock == apiBlockHeaders {
		a.apiEditorScroll = 0
	}
	a.apiEditing = false
}

func apiPrintableKey(msg tea.KeyMsg) bool {
	if len(msg.Runes) > 0 {
		return true
	}
	s := msg.String()
	return len(s) == 1 && s >= " " && s <= "~"
}

func renderApiCursor(text string, cursor int) string {
	runes := []rune(text)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	return string(runes[:cursor]) + "█" + string(runes[cursor:])
}

func (a *App) apiClearSel() {
	a.apiEditorAnchor = -1
}

// apiSelRange returns [lo,hi) when a selection is active.
func (a *App) apiSelRange() (lo, hi int, ok bool) {
	if !a.apiEditing || a.apiEditorAnchor < 0 {
		return 0, 0, false
	}
	lo, hi = a.apiEditorAnchor, a.apiEditorCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return 0, 0, false
	}
	return lo, hi, true
}

func (a *App) apiDeleteSelection() bool {
	lo, hi, ok := a.apiSelRange()
	if !ok {
		return false
	}
	runes := []rune(a.apiCurrentEditText())
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	runes = append(runes[:lo], runes[hi:]...)
	a.apiSetCurrentEditText(string(runes))
	a.apiEditorCursor = lo
	a.apiClearSel()
	return true
}

func (a *App) apiCurrentEditText() string {
	switch a.apiBlock {
	case apiBlockURL:
		return a.apiURL
	case apiBlockHeaders:
		return a.apiHeaders
	case apiBlockAuth:
		switch a.apiAuthType {
		case apiAuthBearer:
			return a.apiAuthToken
		case apiAuthBasic:
			if a.apiAuthEditPass {
				return a.apiAuthPass
			}
			return a.apiAuthUser
		}
		return ""
	case apiBlockRight:
		if a.apiRightTab == apiRightBody {
			return a.apiBody
		}
	}
	return ""
}

func (a *App) apiSetCurrentEditText(text string) {
	switch a.apiBlock {
	case apiBlockURL:
		a.apiURL = text
	case apiBlockHeaders:
		a.apiHeaders = text
	case apiBlockAuth:
		switch a.apiAuthType {
		case apiAuthBearer:
			a.apiAuthToken = text
		case apiAuthBasic:
			if a.apiAuthEditPass {
				a.apiAuthPass = text
			} else {
				a.apiAuthUser = text
			}
		}
	case apiBlockRight:
		if a.apiRightTab == apiRightBody {
			a.apiBody = text
		}
	}
}

func (a *App) beginApiEdit() {
	a.apiClearSel()
	switch a.apiBlock {
	case apiBlockRequest:
		a.apiMethod = apiMethods[a.apiMethodCursor]
		return
	case apiBlockURL, apiBlockHeaders:
		a.apiEditing = true
		a.apiEditorCursor = len([]rune(a.apiCurrentEditText()))
	case apiBlockAuth:
		if a.apiAuthType == apiAuthNone {
			a.apiAuthType = apiAuthBearer
		}
		a.apiEditing = true
		a.apiAuthEditPass = false
		a.apiEditorCursor = len([]rune(a.apiCurrentEditText()))
	case apiBlockRight:
		if a.apiRightTab != apiRightBody {
			return
		}
		a.apiEditing = true
		a.apiHScroll = 0
		a.apiEditorScroll = 0
		a.apiEditorCursor = apiBodyStartCursor(a.apiBody)
	}
}

// apiBodyStartCursor places the caret sensibly: inside {} / [] or at 0 if empty.
func apiBodyStartCursor(body string) int {
	trim := strings.TrimSpace(body)
	if trim == "" {
		return 0
	}
	runes := []rune(body)
	if trim == "{}" || trim == "[]" {
		// Cursor between the brackets (first non-space + 1).
		for i, r := range runes {
			if r == '{' || r == '[' {
				return i + 1
			}
		}
	}
	return len(runes)
}

func (a *App) cycleApiMethod() {
	a.apiMethodCursor = (a.apiMethodCursor + 1) % len(apiMethods)
	a.apiMethod = apiMethods[a.apiMethodCursor]
}

func (a *App) applyApiPort(port int) {
	path := ""
	if u := strings.TrimSpace(a.apiURL); u != "" {
		if i := strings.Index(u, "://"); i >= 0 {
			rest := u[i+3:]
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				path = rest[slash:]
			}
		}
	}
	a.apiURL = fmt.Sprintf("http://localhost:%d%s", port, path)
	a.statusMsg = "URL → porta " + strconv.Itoa(port)
}

func (a *App) cycleApiPort(p *core.Project) {
	ports := a.apiProjectPorts(p)
	if len(ports) == 0 {
		a.statusMsg = "nenhuma porta detectada no projeto"
		return
	}
	a.apiPortIndex = (a.apiPortIndex + 1) % len(ports)
	a.applyApiPort(ports[a.apiPortIndex])
	a.apiBlock = apiBlockURL
}

func (a *App) pushApiHistory(method, url string) {
	item := apiHistoryItem{Method: method, URL: url}
	out := []apiHistoryItem{item}
	for _, h := range a.apiHistory {
		if h.Method == item.Method && h.URL == item.URL {
			continue
		}
		out = append(out, h)
		if len(out) >= 10 {
			break
		}
	}
	a.apiHistory = out
}

func (a *App) sendApiRequest() tea.Cmd {
	if a.apiLoading {
		return nil
	}
	// Prefer the highlighted method in the Request list.
	if a.apiMethodCursor >= 0 && a.apiMethodCursor < len(apiMethods) {
		a.apiMethod = apiMethods[a.apiMethodCursor]
	}
	method := strings.ToUpper(strings.TrimSpace(a.apiMethod))
	if method == "" {
		method = "GET"
		a.apiMethod = method
		a.syncApiMethodCursor()
	}
	a.apiLoading = true
	a.apiResponseErr = ""
	a.apiRightTab = apiRightResponse
	a.apiBlock = apiBlockRight
	a.apiEditing = false
	a.apiResponseScroll = 0
	a.pushApiHistory(method, a.apiURL)
	return sendAPIRequest(apiRequest{
		Method:   method,
		URL:      a.apiURL,
		Headers:  a.apiHeaders,
		AuthType: a.apiAuthType,
		Token:    a.apiAuthToken,
		User:     a.apiAuthUser,
		Pass:     a.apiAuthPass,
		Body:     a.apiBody,
	})
}

func (a *App) handleApiResponse(msg apiResponseMsg) {
	a.apiLoading = false
	if msg.err != nil {
		a.apiResponseErr = msg.err.Error()
		a.apiResponseStatus = ""
		a.apiResponseCode = 0
		a.apiResponseBody = ""
		a.apiResponseHeaders = ""
		a.apiResponseTime = msg.duration
		return
	}
	a.apiResponseErr = ""
	a.apiResponseStatus = msg.status
	a.apiResponseCode = msg.statusCode
	a.apiResponseTime = msg.duration
	a.apiResponseHeaders = msg.headers
	a.apiResponseBody = msg.body
	a.apiResponseScroll = 0
	a.apiHScroll = 0
}

func (a *App) apiSearchMatches() []int {
	q := strings.ToLower(strings.TrimSpace(a.apiSearchQuery))
	if q == "" {
		return nil
	}
	var matches []int
	for i, line := range strings.Split(a.apiResponseBody, "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			matches = append(matches, i)
		}
	}
	return matches
}

func (a *App) apiMatchLineSet() map[int]bool {
	matches := a.apiSearchMatches()
	if len(matches) == 0 {
		return nil
	}
	set := make(map[int]bool, len(matches))
	for _, i := range matches {
		set[i] = true
	}
	return set
}

func (a *App) jumpApiSearch(delta int) {
	matches := a.apiSearchMatches()
	if len(matches) == 0 {
		return
	}
	a.apiSearchIdx = (a.apiSearchIdx + delta) % len(matches)
	if a.apiSearchIdx < 0 {
		a.apiSearchIdx += len(matches)
	}
	a.apiRightTab = apiRightResponse
	a.apiBlock = apiBlockRight
	a.apiResponseScroll = ensureVisible(matches[a.apiSearchIdx], a.apiResponseScroll, maxInt(1, a.apiViewport()-3), len(strings.Split(a.apiResponseBody, "\n")))
}

func (a *App) apiViewport() int {
	return maxInt(4, maxInt(14, a.height-2)-5)
}

func (a *App) updateApiEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	text := a.apiCurrentEditText()
	runes := []rune(text)
	cursor := a.apiEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	editingBody := a.apiBlock == apiBlockRight && a.apiRightTab == apiRightBody
	editingAuth := a.apiBlock == apiBlockAuth
	multiline := editingBody || a.apiBlock == apiBlockHeaders
	key := msg.String()

	move := func(next int, extend bool) {
		if next < 0 {
			next = 0
		}
		if next > len(runes) {
			next = len(runes)
		}
		if extend {
			if a.apiEditorAnchor < 0 {
				a.apiEditorAnchor = cursor
			}
		} else {
			a.apiClearSel()
		}
		cursor = next
	}

	switch key {
	case "esc":
		a.apiEditing = false
		a.apiAuthEditPass = false
		a.apiClearSel()
		return a, nil
	case "ctrl+a":
		if len(runes) == 0 {
			a.apiClearSel()
			return a, nil
		}
		a.apiEditorAnchor = 0
		cursor = len(runes)
		a.apiEditorCursor = cursor
		return a, nil
	case "enter":
		if a.apiBlock == apiBlockURL {
			a.apiEditing = false
			a.apiClearSel()
			return a, a.sendApiRequest()
		}
		if editingAuth {
			a.apiEditing = false
			a.apiAuthEditPass = false
			a.apiClearSel()
			return a, nil
		}
		if a.apiDeleteSelection() {
			text = a.apiCurrentEditText()
			runes = []rune(text)
			cursor = a.apiEditorCursor
		}
		indent := ""
		if multiline {
			indent = apiLineIndent(runes, cursor)
		}
		prev := apiRuneBefore(runes, cursor)
		next := apiRuneAfter(runes, cursor)
		if editingBody && ((prev == '{' && next == '}') || (prev == '[' && next == ']')) {
			insert := []rune("\n" + indent + "  \n" + indent)
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			cursor += len([]rune("\n" + indent + "  "))
			a.apiSetCurrentEditText(string(runes))
			a.apiEditorCursor = cursor
			a.apiClearSel()
			return a, nil
		}
		if editingBody && (prev == '{' || prev == '[') {
			indent += "  "
		}
		insert := []rune("\n" + indent)
		runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
		cursor += len(insert)
		a.apiSetCurrentEditText(string(runes))
		a.apiEditorCursor = cursor
		a.apiClearSel()
		return a, nil
	case "ctrl+enter":
		a.apiEditing = false
		a.apiClearSel()
		return a, a.sendApiRequest()
	case "left":
		if lo, _, ok := a.apiSelRange(); ok {
			move(lo, false)
		} else {
			move(cursor-1, false)
		}
	case "right":
		if _, hi, ok := a.apiSelRange(); ok {
			move(hi, false)
		} else {
			move(cursor+1, false)
		}
	case "shift+left":
		move(cursor-1, true)
	case "shift+right":
		move(cursor+1, true)
	case "up":
		if multiline {
			move(apiMoveLine(runes, cursor, -1), false)
		}
	case "down":
		if multiline {
			move(apiMoveLine(runes, cursor, 1), false)
		}
	case "shift+up":
		if multiline {
			move(apiMoveLine(runes, cursor, -1), true)
		}
	case "shift+down":
		if multiline {
			move(apiMoveLine(runes, cursor, 1), true)
		}
	case "home":
		move(apiLineStart(runes, cursor), false)
	case "end":
		move(apiLineEnd(runes, cursor), false)
	case "shift+home":
		move(apiLineStart(runes, cursor), true)
	case "shift+end":
		move(apiLineEnd(runes, cursor), true)
	case "backspace":
		if a.apiDeleteSelection() {
			return a, nil
		}
		if cursor > 0 {
			runes = append(runes[:cursor-1], runes[cursor:]...)
			cursor--
			a.apiSetCurrentEditText(string(runes))
		}
	case "delete":
		if a.apiDeleteSelection() {
			return a, nil
		}
		if cursor < len(runes) {
			runes = append(runes[:cursor], runes[cursor+1:]...)
			a.apiSetCurrentEditText(string(runes))
		}
	case "tab":
		if editingAuth && a.apiAuthType == apiAuthBasic {
			a.apiAuthEditPass = !a.apiAuthEditPass
			a.apiClearSel()
			a.apiEditorCursor = len([]rune(a.apiCurrentEditText()))
			return a, nil
		}
		if multiline {
			if a.apiDeleteSelection() {
				text = a.apiCurrentEditText()
				runes = []rune(text)
				cursor = a.apiEditorCursor
			}
			insert := []rune("  ")
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			cursor += len(insert)
			a.apiSetCurrentEditText(string(runes))
			a.apiEditorCursor = cursor
			a.apiClearSel()
			return a, nil
		}
		a.apiEditing = false
		a.apiClearSel()
		return a, nil
	case "shift+tab":
		if multiline {
			a.apiClearSel()
			n := apiUnindentAt(runes, cursor)
			if n > 0 {
				runes = append(runes[:cursor-n], runes[cursor:]...)
				cursor -= n
				a.apiSetCurrentEditText(string(runes))
				a.apiEditorCursor = cursor
			}
			return a, nil
		}
		a.apiEditing = false
		a.apiClearSel()
		return a, nil
	default:
		var inserted []rune
		if len(msg.Runes) > 0 {
			inserted = msg.Runes
		} else if s := key; len(s) == 1 {
			inserted = []rune(s)
		}
		if len(inserted) > 0 {
			if a.apiDeleteSelection() {
				text = a.apiCurrentEditText()
				runes = []rune(text)
				cursor = a.apiEditorCursor
			}
			runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
			cursor += len(inserted)
			a.apiSetCurrentEditText(string(runes))
			a.apiClearSel()
		}
	}
	a.apiEditorCursor = cursor
	return a, nil
}

func apiLineIndent(runes []rune, cursor int) string {
	start := cursor
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	i := start
	for i < cursor && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	return string(runes[start:i])
}

func apiRuneBefore(runes []rune, cursor int) rune {
	i := cursor - 1
	for i >= 0 && (runes[i] == ' ' || runes[i] == '\t') {
		i--
	}
	if i < 0 {
		return 0
	}
	return runes[i]
}

func apiRuneAfter(runes []rune, cursor int) rune {
	i := cursor
	for i < len(runes) && (runes[i] == ' ' || runes[i] == '\t') {
		i++
	}
	if i >= len(runes) {
		return 0
	}
	return runes[i]
}

func apiLineStart(runes []rune, cursor int) int {
	for cursor > 0 && runes[cursor-1] != '\n' {
		cursor--
	}
	return cursor
}

func apiLineEnd(runes []rune, cursor int) int {
	for cursor < len(runes) && runes[cursor] != '\n' {
		cursor++
	}
	return cursor
}

func apiMoveLine(runes []rune, cursor, delta int) int {
	if len(runes) == 0 {
		return 0
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	col := cursor - apiLineStart(runes, cursor)
	// Find target line start.
	lineStart := apiLineStart(runes, cursor)
	if delta < 0 {
		if lineStart == 0 {
			return cursor
		}
		prevEnd := lineStart - 1
		prevStart := apiLineStart(runes, prevEnd)
		prevLen := prevEnd - prevStart
		if col > prevLen {
			col = prevLen
		}
		return prevStart + col
	}
	nextStart := apiLineEnd(runes, cursor)
	if nextStart >= len(runes) {
		return cursor
	}
	nextStart++ // skip newline
	nextEnd := apiLineEnd(runes, nextStart)
	nextLen := nextEnd - nextStart
	if col > nextLen {
		col = nextLen
	}
	return nextStart + col
}

func apiUnindentAt(runes []rune, cursor int) int {
	n := 0
	for n < 2 && cursor-n > 0 {
		r := runes[cursor-n-1]
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

func (a *App) updateApiSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.apiSearchOn = false
		return a, nil
	case tea.KeyEnter:
		a.apiSearchQuery = strings.TrimSpace(a.apiSearchInput)
		a.apiSearchIdx = 0
		a.apiSearchOn = false
		if a.apiSearchQuery != "" {
			a.jumpApiSearch(0)
		}
		return a, nil
	case tea.KeyBackspace:
		if a.apiSearchInput != "" {
			r := []rune(a.apiSearchInput)
			a.apiSearchInput = string(r[:len(r)-1])
		}
	case tea.KeyRunes:
		a.apiSearchInput += string(msg.Runes)
	}
	return a, nil
}

func (a *App) renderApiSearchPrompt() string {
	content := a.renderApiTab(a.currentProject())
	prompt := StylePanel.Render("Buscar na response: " + a.apiSearchInput + "█")
	return lipgloss.JoinVertical(lipgloss.Left,
		content,
		"",
		prompt,
		a.renderStatusBar("enter buscar | esc cancelar"),
	)
}

func (a *App) handleApiKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.apiSearchOn {
		return a.updateApiSearch(msg)
	}
	if a.apiEditing {
		return a.updateApiEdit(msg)
	}

	// Auth: `a` / ↑↓ cycle type before printable keys steal `a` into the token field.
	if a.apiBlock == apiBlockAuth {
		switch msg.String() {
		case "a", "down", "j":
			a.cycleApiAuth(1)
			return a, nil
		case "up", "k":
			a.cycleApiAuth(-1)
			return a, nil
		case "e":
			a.beginApiEdit()
			return a, nil
		}
	}

	// Typing on URL/Headers/Auth starts editing — Body requires `e`.
	if a.apiBlockEditable() && apiPrintableKey(msg) {
		a.beginApiEdit()
		return a.updateApiEdit(msg)
	}

	switch msg.String() {
	case "esc":
		if a.apiSearchQuery != "" {
			a.apiSearchQuery = ""
			a.apiSearchIdx = 0
			return a, nil
		}
		if a.apiBlock == apiBlockRight {
			a.apiEditing = false
			a.apiBlock = apiBlockRequest
			a.syncApiMethodCursor()
			return a, nil
		}
		// Leave fullscreen client → tab 7 landing (TOOLS hub).
		return a, a.leaveApiTab()
	case "1":
		a.apiEditing = false
		a.apiBlock = apiBlockRequest
		a.syncApiMethodCursor()
	case "2":
		a.apiEditing = false
		a.apiBlock = apiBlockURL
	case "3":
		a.apiEditing = false
		a.apiBlock = apiBlockHeaders
		a.apiEditorScroll = 0
	case "4":
		a.apiEditing = false
		a.apiBlock = apiBlockAuth
	case "m":
		a.apiBlock = apiBlockRequest
		a.cycleApiMethod()
	case "a":
		// From any left block, jump to Auth and cycle type.
		a.apiBlock = apiBlockAuth
		a.cycleApiAuth(1)
	case "u":
		if a.apiBlock == apiBlockURL || a.apiBlock == apiBlockRequest {
			a.cycleApiPort(p)
		}
	case "H":
		a.apiShowResponseHeaders = !a.apiShowResponseHeaders
		a.apiRightTab = apiRightResponse
		a.apiBlock = apiBlockRight
		a.apiEditing = false
	case "e":
		if a.apiBlock == apiBlockRight && a.apiRightTab == apiRightResponse {
			return a, nil
		}
		a.beginApiEdit()
	case "tab":
		// Tab only cycles Request → URL → Headers → Auth.
		a.apiCycleLeftBlock(true)
	case "shift+tab":
		a.apiCycleLeftBlock(false)
	case "[":
		// Outside Body editor: switch to Body tab.
		a.apiEditing = false
		a.apiBlock = apiBlockRight
		a.apiRightTab = apiRightBody
		a.apiEditorScroll = 0
	case "]":
		// Outside Body editor: switch to Response tab.
		a.apiEditing = false
		a.apiBlock = apiBlockRight
		a.apiRightTab = apiRightResponse
		a.apiResponseScroll = 0
	case "left":
		a.apiEditing = false
		a.apiBlock = apiBlockRequest
		a.syncApiMethodCursor()
	case "right":
		// → goes to Body/Response pane (does not start editing).
		a.apiEditing = false
		a.apiBlock = apiBlockRight
		if a.apiRightTab != apiRightBody && a.apiRightTab != apiRightResponse {
			a.apiRightTab = apiRightBody
		}
	case "ctrl+enter", "enter":
		return a, a.sendApiRequest()
	case "r":
		return a, a.sendApiRequest()
	case "up", "k":
		switch a.apiBlock {
		case apiBlockRequest:
			if a.apiMethodCursor > 0 {
				a.apiMethodCursor--
				a.apiMethod = apiMethods[a.apiMethodCursor]
			}
		case apiBlockHeaders:
			if a.apiEditorScroll > 0 {
				a.apiEditorScroll--
			}
		case apiBlockRight:
			if a.apiRightTab == apiRightResponse {
				if a.apiResponseScroll > 0 {
					a.apiResponseScroll--
				}
			} else if a.apiEditorScroll > 0 {
				a.apiEditorScroll--
			}
		}
	case "down", "j":
		switch a.apiBlock {
		case apiBlockRequest:
			if a.apiMethodCursor < len(apiMethods)-1 {
				a.apiMethodCursor++
				a.apiMethod = apiMethods[a.apiMethodCursor]
			}
		case apiBlockHeaders:
			a.apiEditorScroll++
		case apiBlockRight:
			if a.apiRightTab == apiRightResponse {
				a.apiResponseScroll++
			} else {
				a.apiEditorScroll++
			}
		}
	case "pgup":
		if a.apiBlock == apiBlockRight && a.apiRightTab == apiRightResponse {
			a.apiResponseScroll -= a.apiViewport()
			if a.apiResponseScroll < 0 {
				a.apiResponseScroll = 0
			}
		}
	case "pgdown":
		if a.apiBlock == apiBlockRight && a.apiRightTab == apiRightResponse {
			a.apiResponseScroll += a.apiViewport()
		}
	case ",":
		if a.apiBlock == apiBlockRight {
			if a.apiHScroll > 0 {
				a.apiHScroll -= 8
				if a.apiHScroll < 0 {
					a.apiHScroll = 0
				}
			}
		}
	case ".":
		if a.apiBlock == apiBlockRight {
			a.apiHScroll += 8
		}
	case "/":
		// Search only in Body/Response pane — never on Request/URL/Headers/Auth.
		if a.apiBlock == apiBlockRight {
			a.apiSearchOn = true
			a.apiSearchInput = a.apiSearchQuery
		}
	case "N":
		if a.apiBlock == apiBlockRight && a.apiSearchQuery != "" {
			a.jumpApiSearch(1)
		}
	case "P":
		if a.apiBlock == apiBlockRight && a.apiSearchQuery != "" {
			a.jumpApiSearch(-1)
		}
	}
	return a, nil
}
