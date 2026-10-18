package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
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
	a.apiSection = apiSecParams
	a.apiFocus = apiFocusURL
	a.apiRespView = apiRespBody
	a.apiRowCursor = 0
	a.apiRowField = 0
	a.apiRowScroll = 0
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

func (a *App) apiBaseURL(p *core.Project) string {
	u := strings.TrimSpace(a.apiURL)
	if u != "" {
		if i := strings.Index(u, "://"); i >= 0 {
			rest := u[i+3:]
			if slash := strings.IndexByte(rest, '/'); slash >= 0 {
				return u[:i+3+slash]
			}
			return u
		}
	}
	if ports := a.apiProjectPorts(p); len(ports) > 0 {
		return fmt.Sprintf("http://localhost:%d", ports[0])
	}
	return "http://localhost:8080"
}

// leaveApiTab closes the fullscreen client and returns to tab 8 landing.
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
	url := strings.TrimSpace(a.apiURL)
	if url == "" && p != nil {
		if ports := a.apiProjectPorts(p); len(ports) > 0 {
			url = fmt.Sprintf("http://localhost:%d", ports[0])
		}
	}
	ports := ""
	if p != nil {
		ports = collectors.FormatPortsShort(a.apiProjectPorts(p), 6)
	}
	state := StyleMuted.Render("cliente HTTP no contexto do projeto")
	if url != "" {
		state = StyleHealthy.Render("↗ " + truncate(url, 56))
	}
	facts := [][2]string{
		{"url", StyleNormal.Render(firstNonEmpty(url, emDash))},
		{"portas", StyleAccent.Render(firstNonEmpty(ports, emDash))},
	}
	// O histórico é conteúdo de verdade — as últimas chamadas dizem onde você
	// parou. Entra como fato, não como caixa esticada.
	if n := len(a.apiHistory); n == 0 {
		facts = append(facts, [2]string{"últimas", StyleMuted.Render("vazio — as chamadas aparecem aqui")})
	} else {
		for i := 0; i < minInt(4, n); i++ {
			h := a.apiHistory[i]
			label := ""
			if i == 0 {
				label = "últimas"
			}
			facts = append(facts, [2]string{label,
				apiMethodStyle(h.Method).Render(h.Method) + " " + StyleMuted.Render(truncate(h.URL, 60))})
		}
		if n > 4 {
			facts = append(facts, [2]string{"", StyleMuted.Render(fmt.Sprintf("+%d", n-4))})
		}
	}
	return a.renderModuleLanding(p, moduleLanding{
		title:   "API",
		tagline: "cliente HTTP do projeto — monta, envia e lê a resposta",
		state:   state,
		facts:   facts,
		actions: [][2]string{
			{"enter", "abrir cliente"},
			{"=", "descobrir rotas"},
			{"o", "abrir browser"},
			{"esc", "voltar"},
		},
	})
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
	a.apiSyncParamsFromURL()
	a.apiFocus = apiFocusURL
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
	method := a.apiMethodOrGet()
	a.apiMethod = method
	a.apiLoading = true
	a.apiResponseErr = ""
	// Enviar leva o foco pra resposta: é lá que a atenção vai. A aba da
	// requisição fica como estava, pronta pro próximo ajuste.
	a.apiFocus = apiFocusResponse
	a.apiRespView = apiRespBody
	a.apiEditing = false
	a.apiResponseScroll = 0
	a.apiHScroll = 0
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

	editingBody := a.apiFocus == apiFocusSection && a.apiSection == apiSecBody
	editingAuth := a.apiFocus == apiFocusSection && a.apiSection == apiSecAuth
	editingRow := a.apiFocus == apiFocusSection &&
		(a.apiSection == apiSecParams || a.apiSection == apiSecHeaders)
	multiline := editingBody
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
		if a.apiFocus == apiFocusURL {
			// Enter na URL envia, como no Postman.
			a.apiEditing = false
			a.apiClearSel()
			a.apiSyncParamsFromURL()
			return a, a.sendApiRequest()
		}
		if editingRow {
			// Enter conclui a célula e desce pra próxima linha: é assim que se
			// digita uma tabela inteira sem soltar as mãos do teclado.
			a.apiEditing = false
			a.apiClearSel()
			if a.apiRowField == 0 {
				a.apiRowField = 1
				a.beginApiEdit()
				return a, nil
			}
			a.apiRowField = 0
			a.apiRowCursor++
			a.apiClampRows()
			return a, nil
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
		if editingRow {
			// Tab troca de célula sem sair da edição: chave → valor → chave.
			a.apiRowField = 1 - a.apiRowField
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

func apiIsWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '$'
}

func apiIsSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// apiMoveWordLeft jumps like VS Code: skip whitespace, then a word or one punct char.
func apiMoveWordLeft(runes []rune, cursor int) int {
	if cursor <= 0 {
		return 0
	}
	i := cursor
	for i > 0 && apiIsSpace(runes[i-1]) {
		i--
	}
	if i == 0 {
		return 0
	}
	if !apiIsWordChar(runes[i-1]) {
		return i - 1
	}
	for i > 0 && apiIsWordChar(runes[i-1]) {
		i--
	}
	return i
}

func apiMoveWordRight(runes []rune, cursor int) int {
	if cursor >= len(runes) {
		return len(runes)
	}
	i := cursor
	for i < len(runes) && apiIsSpace(runes[i]) {
		i++
	}
	if i >= len(runes) {
		return len(runes)
	}
	if !apiIsWordChar(runes[i]) {
		return i + 1
	}
	for i < len(runes) && apiIsWordChar(runes[i]) {
		i++
	}
	return i
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

// ── Ponte com o editor de texto ───────────────────────────────────────────────
//
// O editor (updateApiEdit) é genérico: ele mexe num texto e num cursor. Estas
// duas funções dizem QUAL texto é esse, conforme o foco — campo da URL, célula
// da tabela, token de auth ou o corpo.

func (a *App) apiCurrentEditText() string {
	if a.apiFocus == apiFocusURL {
		return a.apiURL
	}
	if a.apiFocus != apiFocusSection {
		return ""
	}
	switch a.apiSection {
	case apiSecParams, apiSecHeaders:
		return a.apiRowText()
	case apiSecAuth:
		switch a.apiAuthType {
		case apiAuthBearer:
			return a.apiAuthToken
		case apiAuthBasic:
			if a.apiAuthEditPass {
				return a.apiAuthPass
			}
			return a.apiAuthUser
		}
	case apiSecBody:
		return a.apiBody
	}
	return ""
}

func (a *App) apiSetCurrentEditText(text string) {
	if a.apiFocus == apiFocusURL {
		a.apiURL = text
		// A tabela de params segue a URL a cada tecla: é o que faz a query
		// aparecer como linhas na hora em que você cola um link.
		a.apiSyncParamsFromURL()
		return
	}
	if a.apiFocus != apiFocusSection {
		return
	}
	switch a.apiSection {
	case apiSecParams, apiSecHeaders:
		a.apiRowSetText(text)
	case apiSecAuth:
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
	case apiSecBody:
		a.apiBody = text
	}
}

// beginApiEdit entra em edição no que estiver em foco. Tabela vazia ganha uma
// linha na hora — pedir `n` antes de poder digitar é um passo a mais sem motivo.
func (a *App) beginApiEdit() {
	a.apiClearSel()
	if a.apiFocus == apiFocusResponse {
		return // resposta é leitura
	}
	if a.apiFocus == apiFocusSection {
		switch a.apiSection {
		case apiSecParams, apiSecHeaders:
			if a.apiRowCursor >= len(a.apiRows()) {
				a.apiRowAdd()
			}
		case apiSecAuth:
			if a.apiAuthType == apiAuthNone {
				a.apiAuthType = apiAuthBearer
			}
			a.apiAuthEditPass = false
		case apiSecBody:
			a.apiHScroll = 0
			a.apiEditorScroll = 0
			a.apiEditing = true
			a.apiEditorCursor = apiBodyStartCursor(a.apiBody)
			return
		case apiSecResponse:
			return
		}
	}
	a.apiEditing = true
	a.apiEditorCursor = len([]rune(a.apiCurrentEditText()))
}

// cycleApiMethod gira o método no lugar, como o seletor do Postman.
func (a *App) cycleApiMethod(delta int) {
	cur := 0
	for i, m := range apiMethods {
		if m == a.apiMethodOrGet() {
			cur = i
			break
		}
	}
	n := len(apiMethods)
	a.apiMethod = apiMethods[((cur+delta)%n+n)%n]
	a.statusMsg = "método → " + a.apiMethod
}

// apiSetSection troca de aba. Ela leva o foco pro painel — número de aba que não
// move o foco obriga a apertar tab depois, e ninguém lembra disso.
func (a *App) apiSetSection(sec apiSection) {
	a.apiSection = sec
	a.apiEditing = false
	a.apiClearSel()
	a.apiRowScroll = 0
	if sec == apiSecResponse {
		a.apiFocus = apiFocusResponse
		return
	}
	a.apiFocus = apiFocusSection
	if sec == apiSecParams || sec == apiSecHeaders {
		a.apiRowCursor, a.apiRowField = 0, 0
		a.apiClampRows()
	}
	if sec == apiSecBody {
		a.apiEditorScroll = 0
	}
}

// apiCycleFocus é o `tab`: url → painel da aba → resposta → url. Três paradas,
// sempre na mesma ordem — antes eram quatro caixas à esquerda mais um painel à
// direita, e `tab` fazia coisas diferentes conforme onde você estava.
func (a *App) apiCycleFocus(delta int) {
	order := []apiFocus{apiFocusURL, apiFocusSection, apiFocusResponse}
	if a.apiSection == apiSecResponse {
		order = []apiFocus{apiFocusURL, apiFocusResponse}
	}
	idx := 0
	for i, f := range order {
		if f == a.apiFocus {
			idx = i
			break
		}
	}
	n := len(order)
	a.apiFocus = order[((idx+delta)%n+n)%n]
	a.apiEditing = false
	a.apiClearSel()
}

// apiRecallHistory traz uma chamada anterior de volta pra linha da requisição.
func (a *App) apiRecallHistory(delta int) {
	if len(a.apiHistory) == 0 {
		a.statusMsg = "histórico vazio — envie algo primeiro"
		return
	}
	a.apiHistoryIdx += delta
	if a.apiHistoryIdx < 0 {
		a.apiHistoryIdx = 0
	}
	if a.apiHistoryIdx >= len(a.apiHistory) {
		a.apiHistoryIdx = len(a.apiHistory) - 1
	}
	h := a.apiHistory[a.apiHistoryIdx]
	a.apiMethod = h.Method
	a.apiURL = h.URL
	a.apiSyncParamsFromURL()
	a.apiFocus = apiFocusURL
	a.statusMsg = fmt.Sprintf("histórico %d/%d · %s", a.apiHistoryIdx+1, len(a.apiHistory), h.Method)
}

// ── Teclado ───────────────────────────────────────────────────────────────────
//
// Três camadas, na ordem: modo de digitação (busca, edição) intercepta tudo;
// depois o que depende do foco; depois os atalhos da tela. Cada tecla tratada
// aqui está na barra de comandos e no `?` (DESIGN.md §3).
func (a *App) handleApiKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.apiSearchOn {
		return a.updateApiSearch(msg)
	}
	if a.apiEditing {
		return a.updateApiEdit(msg)
	}

	key := msg.String()

	// Números trocam de aba, sempre — é a única forma, e nada mais pode
	// sequestrar 1–5.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '5' {
		a.apiSetSection(apiSection(key[0] - '1'))
		return a, nil
	}

	switch key {
	case "esc":
		if a.apiSearchQuery != "" {
			a.apiSearchQuery, a.apiSearchIdx = "", 0
			return a, nil
		}
		if a.apiFocus != apiFocusURL {
			a.apiFocus = apiFocusURL
			return a, nil
		}
		return a, a.leaveApiTab()
	case "tab":
		a.apiCycleFocus(1)
		return a, nil
	case "shift+tab":
		a.apiCycleFocus(-1)
		return a, nil
	case "enter", "ctrl+enter":
		// Enter numa tabela edita a célula; em qualquer outro lugar, envia.
		if key == "enter" && a.apiFocus == apiFocusSection &&
			(a.apiSection == apiSecParams || a.apiSection == apiSecHeaders) {
			a.beginApiEdit()
			return a, nil
		}
		return a, a.sendApiRequest()
	case "m":
		a.cycleApiMethod(1)
		return a, nil
	case "M":
		a.cycleApiMethod(-1)
		return a, nil
	case "u":
		a.apiFocus = apiFocusURL
		a.beginApiEdit()
		return a, nil
	case "p":
		a.cycleApiPort(p)
		return a, nil
	case "e":
		a.beginApiEdit()
		return a, nil
	case "y":
		if err := copyToClipboard(a.apiCurlCommand()); err != nil {
			a.statusMsg = "clipboard: " + err.Error()
		} else {
			a.statusMsg = "comando curl no clipboard ✓"
		}
		return a, nil
	case ",":
		a.apiRecallHistory(1)
		return a, nil
	case ".":
		a.apiRecallHistory(-1)
		return a, nil
	case "r":
		return a, a.sendApiRequest()
	}

	// Daqui pra baixo é o que muda com o foco.
	if a.apiFocus == apiFocusSection {
		switch a.apiSection {
		case apiSecParams, apiSecHeaders:
			return a.handleApiRowKeys(msg)
		case apiSecAuth:
			switch key {
			case "a", "right", "l":
				a.cycleApiAuth(1)
				return a, nil
			case "A", "left", "h":
				a.cycleApiAuth(-1)
				return a, nil
			}
		case apiSecBody:
			switch key {
			case "up", "k":
				if a.apiEditorScroll > 0 {
					a.apiEditorScroll--
				}
				return a, nil
			case "down", "j":
				a.apiEditorScroll++
				return a, nil
			}
		}
	}

	// Resposta: rolar, buscar, trocar corpo/cabeçalhos.
	return a.handleApiResponseKeys(msg)
}

// handleApiRowKeys é a navegação da tabela de params/headers.
func (a *App) handleApiRowKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if a.apiRowCursor > 0 {
			a.apiRowCursor--
		}
	case "down", "j":
		a.apiRowCursor++
		a.apiClampRows()
	case "left", "h":
		a.apiRowField = 0
	case "right", "l":
		a.apiRowField = 1
	case " ", "space":
		a.apiRowToggle()
	case "n":
		a.apiRowAdd()
		a.beginApiEdit()
	case "d":
		a.apiRowDelete()
	case "home", "g":
		a.apiRowCursor = 0
	case "end", "G":
		a.apiRowCursor = len(a.apiRows())
	}
	return a, nil
}

// handleApiResponseKeys é a leitura da resposta: rolagem, busca e a troca entre
// corpo e cabeçalhos.
func (a *App) handleApiResponseKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if a.apiResponseScroll > 0 {
			a.apiResponseScroll--
		}
	case "down", "j":
		a.apiResponseScroll++
	case "pgup", "shift+up":
		a.apiResponseScroll = maxInt(0, a.apiResponseScroll-a.apiViewport())
	case "pgdown", "shift+down":
		a.apiResponseScroll += a.apiViewport()
	case "home", "g":
		a.apiResponseScroll = 0
	case "left":
		a.apiHScroll = maxInt(0, a.apiHScroll-8)
	case "right":
		a.apiHScroll += 8
	case "shift+left", "H":
		a.apiHScroll = maxInt(0, a.apiHScroll-40)
	case "shift+right", "L":
		a.apiHScroll += 40
	case "0":
		a.apiHScroll = 0
	case "h":
		if a.apiRespView == apiRespBody {
			a.apiRespView = apiRespHeaders
		} else {
			a.apiRespView = apiRespBody
		}
		a.apiResponseScroll = 0
	case "/":
		a.apiSearchOn = true
		a.apiSearchInput = a.apiSearchQuery
	case "n":
		if a.apiSearchQuery != "" {
			a.jumpApiSearch(1)
		}
	case "N":
		if a.apiSearchQuery != "" {
			a.jumpApiSearch(-1)
		}
	}
	return a, nil
}
