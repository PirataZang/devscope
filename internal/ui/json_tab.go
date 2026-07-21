package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jsonutil"
)

type jsonPane int

const (
	jsonPaneInput jsonPane = iota
	jsonPaneOutput
)

func (a *App) enterJsonTab(_ *core.Project) {
	a.tab = TabJSON
	a.tabCursor = 0
	a.jsonOpen = false
	a.jsonEditing = false
	a.jsonSearchOn = false
}

func (a *App) openJsonClient(_ *core.Project) tea.Cmd {
	a.jsonOpen = true
	a.jsonEditing = false
	a.jsonSearchOn = false
	a.jsonClearSel()
	a.jsonPane = jsonPaneInput
	a.jsonScrollIn = 0
	a.jsonScrollOut = 0
	a.jsonErr = ""
	a.jsonStatus = ""
	if a.jsonInput == "" {
		a.jsonInput = "{\n  \"hello\": \"devscope\"\n}\n"
	}
	a.jsonEditorCursor = len([]rune(a.jsonInput))
	return nil
}

func (a *App) leaveJsonTab() tea.Cmd {
	a.jsonOpen = false
	a.jsonEditing = false
	a.jsonSearchOn = false
	a.jsonClearSel()
	a.tab = TabJSON
	a.tabCursor = 0
	return nil
}

func (a *App) renderJsonLanding(p *core.Project) string {
	w, h := a.moduleSize()
	ctx := a.renderModuleContext(p, w, "JSON", "utils")
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(5, bodyH*28/100)
	featH := maxInt(5, bodyH*28/100)
	keysH := maxInt(6, bodyH-openH-featH)
	openLines := append([]string{StyleMuted.Render("workspace de JSON — format, convert, validate, diff")}, moduleOpenHint()...)
	featLines := []string{
		StyleMuted.Render("pretty / minify / validate / sort keys"),
		StyleMuted.Render("YAML · TOML · XML  ·  strip nulls"),
		StyleMuted.Render("diff lado a lado  ·  busca por chave"),
		StyleMuted.Render("syntax highlight no editor e no output"),
	}
	keyLines := []string{
		StyleMuted.Render("p Pretty   m Minify   v Validate   s Sort"),
		StyleMuted.Render("w YAML     t TOML     x XML        d Diff"),
		StyleMuted.Render("n strip nulls   / buscar   c copiar"),
		StyleMuted.Render("e editar   tab painel   esc sair"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("JSON", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("CAPACIDADES", fitExactLines(featLines, featH-2), centerW, featH, false),
		renderApiTitledBox("ATALHOS", fitExactLines(keyLines, keysH-2), centerW, keysH, false),
	)
	details := []string{
		StyleMuted.Render("In   ") + StyleNormal.Render(fmt.Sprintf("%d B", len(a.jsonInput))),
		StyleMuted.Render("Out  ") + StyleNormal.Render(fmt.Sprintf("%d B", len(a.jsonOutput))),
		StyleMuted.Render("Fmt  ") + StyleMuted.Render("JSON/YAML/TOML/XML"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir cliente"},
		[2]string{"tab", "módulo"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderJsonTab(_ *core.Project) string {
	w := maxInt(72, a.width)
	h := maxInt(18, a.height-2)
	header := a.renderJsonHeader(w)
	cards := a.renderJsonStatsCards(w)
	chromeH := lipgloss.Height(header) + lipgloss.Height(cards) + 2
	bodyH := maxInt(8, h-chromeH)

	rightW := maxInt(22, w*24/100)
	if rightW > 32 {
		rightW = 32
	}
	panesW := w - rightW - 1
	leftW := maxInt(28, (panesW-1)/2)
	midW := maxInt(28, panesW-leftW-1)

	left := a.renderJsonPane("INPUT", a.jsonInput, leftW, bodyH, a.jsonPane == jsonPaneInput, a.jsonScrollIn, a.jsonEditing && a.jsonPane == jsonPaneInput)
	mid := a.renderJsonPane("OUTPUT", a.jsonOutput, midW, bodyH, a.jsonPane == jsonPaneOutput, a.jsonScrollOut, false)
	rail := a.renderJsonActionRail(rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, mid, rail)

	hints := "p/m/v/s/w/t/x/d  e edit  tab painéis  n nulls  / busca  c copia  esc"
	if a.jsonSearchOn {
		hints = "buscar chave: " + a.jsonSearchInput + "█  enter  esc"
	} else if a.jsonEditing {
		hints = "editando  shift/ctrl+←→↑↓  ctrl+a/c/x/v  esc sair"
	}
	if a.jsonStatus != "" {
		hints = a.jsonStatus + "  ·  " + hints
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, cards, body, a.renderStatusBar(hints))
}

func (a *App) renderJsonHeader(width int) string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabJSON)).Bold(true)
	left := accent.Render("devscope") + StyleMuted.Render(" › json") +
		StyleMuted.Render("  workspace")
	right := StyleMuted.Render("p m v s · w t x d")
	if a.jsonErr != "" {
		right = StyleUnhealthy.Render(truncate(a.jsonErr, 40))
	} else if a.jsonStatus != "" {
		right = StyleHealthy.Render(truncate(a.jsonStatus, 32))
	} else if a.jsonEditing {
		right = StyleWarning.Render("EDIT")
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderJsonStatsCards(width int) string {
	inB, inL, inK, inOK := jsonDocStats(a.jsonInput)
	outB, outL, outK, outOK := jsonDocStats(a.jsonOutput)
	status := "idle"
	stStyle := StyleMuted
	switch {
	case a.jsonErr != "":
		status = "error"
		stStyle = StyleUnhealthy
	case a.jsonStatus != "":
		status = a.jsonStatus
		stStyle = StyleHealthy
	case inOK:
		status = "valid"
		stStyle = StyleHealthy
	case strings.TrimSpace(a.jsonInput) != "":
		status = "invalid"
		stStyle = StyleWarning
	}
	boxW := maxInt(12, width/5)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderStatsCard("INPUT", fmt.Sprintf("%d B", inB), StyleMuted.Render(fmt.Sprintf("%d ln · ~%d keys", inL, inK)), StyleAccent, boxW, 3),
		" ",
		renderStatsCard("OUTPUT", fmt.Sprintf("%d B", outB), StyleMuted.Render(fmt.Sprintf("%d ln · ~%d keys", outL, outK)), StyleHealthy, boxW, 3),
		" ",
		renderStatsCard("STATUS", status, stStyle.Render(boolLabel(inOK || outOK)), stStyle, boxW, 3),
		" ",
		renderStatsCard("PANE", jsonPaneName(a.jsonPane), StyleMuted.Render("tab cicla"), StyleWarning, boxW, 3),
		" ",
		renderStatsCard("MODE", jsonEditMode(a), StyleMuted.Render("e toggle"), StyleNormal, boxW, 3),
	)
}

func jsonPaneName(p jsonPane) string {
	if p == jsonPaneOutput {
		return "output"
	}
	return "input"
}

func jsonEditMode(a *App) string {
	if a.jsonEditing {
		return "edit"
	}
	if a.jsonSearchOn {
		return "search"
	}
	return "view"
}

func jsonDocStats(s string) (bytes, lines, keys int, valid bool) {
	bytes = len(s)
	if s == "" {
		return 0, 0, 0, false
	}
	lines = strings.Count(s, "\n") + 1
	keys = strings.Count(s, `":`) + strings.Count(s, `": `)
	valid = jsonutil.Validate(s) == nil
	return
}

func (a *App) renderJsonActionRail(width, height int) string {
	actH := maxInt(8, height*55/100)
	tipH := maxInt(5, height-actH)
	actions := moduleActionLines(
		[2]string{"p", "pretty"},
		[2]string{"m", "minify"},
		[2]string{"v", "validate"},
		[2]string{"s", "sort keys"},
		[2]string{"w/t/x", "yaml/toml/xml"},
		[2]string{"d", "diff"},
		[2]string{"n", "strip null"},
		[2]string{"c", "copiar out"},
		[2]string{"/", "buscar"},
		[2]string{"e", "editar"},
	)
	tips := []string{
		StyleMuted.Render("input → ação → output"),
		StyleMuted.Render("syntax color no painel"),
		StyleMuted.Render("diff compara in/out"),
	}
	if a.jsonErr != "" {
		tips = append([]string{StyleUnhealthy.Render(truncate(a.jsonErr, width-4))}, tips...)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("AÇÕES", fitExactLines(actions, actH-2), width, actH, false),
		renderApiTitledBox("DICAS", fitExactLines(tips, tipH-2), width, tipH, false),
	)
}

func (a *App) renderJsonPane(title, content string, width, height int, focus bool, scroll int, editing bool) string {
	viewport := maxInt(1, height-2)
	innerW := maxInt(8, width-4)
	label := title
	if focus {
		label = "> " + title
	}
	if editing {
		lines := a.renderJsonMultilineEdit(content, innerW, viewport)
		return renderApiTitledBox(label, lines, width, height, focus)
	}
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(raw) == 1 && raw[0] == "" {
		raw = []string{"(vazio — e para editar · p pretty)"}
	}
	maxScroll := maxInt(0, len(raw)-viewport)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if title == "INPUT" {
		a.jsonScrollIn = scroll
	} else {
		a.jsonScrollOut = scroll
	}
	start := scroll
	end := minInt(start+viewport, len(raw))
	lines := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		plain := sanitizeTerminalLine(raw[i])
		num := StyleMuted.Render(fmt.Sprintf("%3d ", i+1))
		colored := renderJSONColumns(plain, 0, innerW)
		if !focus && strings.TrimSpace(plain) != "" && !strings.HasPrefix(strings.TrimSpace(plain), "(") {
			// dim non-focused pane slightly by using muted for empty-looking lines only
		}
		_ = focus
		lines = append(lines, num+colored)
	}
	return renderApiTitledBox(label, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) renderJsonMultilineEdit(content string, width, height int) []string {
	runes := []rune(content)
	cursor := a.jsonEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	selLo, selHi, hasSel := a.jsonSelRange()
	kinds := jsonKindsForRunes(content)

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
	a.jsonScrollIn = ensureVisible(cursorLine, a.jsonScrollIn, height, len(spans))
	from := a.jsonScrollIn
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
			case i < len(kinds):
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

func (a *App) jsonClearSel() {
	a.jsonEditorAnchor = -1
}

func (a *App) jsonSelRange() (lo, hi int, ok bool) {
	if !a.jsonEditing || a.jsonEditorAnchor < 0 {
		return 0, 0, false
	}
	lo, hi = a.jsonEditorAnchor, a.jsonEditorCursor
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return 0, 0, false
	}
	return lo, hi, true
}

func (a *App) jsonDeleteSelection() bool {
	lo, hi, ok := a.jsonSelRange()
	if !ok {
		return false
	}
	runes := []rune(a.jsonInput)
	if lo > len(runes) {
		lo = len(runes)
	}
	if hi > len(runes) {
		hi = len(runes)
	}
	a.jsonInput = string(append(runes[:lo], runes[hi:]...))
	a.jsonEditorCursor = lo
	a.jsonClearSel()
	return true
}

func (a *App) handleJsonKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.jsonSearchOn {
		return a.updateJsonSearch(msg)
	}

	if a.jsonEditing {
		return a.updateJsonEdit(msg)
	}

	switch msg.String() {
	case "esc":
		return a, a.leaveJsonTab()
	case "p", "m", "v", "s", "w", "t", "x", "d":
		a.runJsonAction(msg.String())
		return a, nil
	case "tab", "right":
		if a.jsonPane == jsonPaneInput {
			a.jsonPane = jsonPaneOutput
		} else {
			a.jsonPane = jsonPaneInput
		}
	case "left":
		a.jsonPane = jsonPaneInput
	case "e":
		a.jsonPane = jsonPaneInput
		a.jsonEditing = true
		a.jsonClearSel()
		a.jsonEditorCursor = len([]rune(a.jsonInput))
	case "n":
		a.runJsonStripNulls()
	case "/":
		a.jsonSearchOn = true
		a.jsonSearchInput = ""
	case "c":
		a.copyJsonOutput()
	case "up", "k":
		a.jsonScrollDelta(-1)
	case "down", "j":
		a.jsonScrollDelta(1)
	case "pgup":
		a.jsonScrollDelta(-10)
	case "pgdown":
		a.jsonScrollDelta(10)
	}
	_ = p
	return a, nil
}

func (a *App) jsonScrollDelta(d int) {
	if a.jsonPane == jsonPaneOutput {
		a.jsonScrollOut = maxInt(0, a.jsonScrollOut+d)
		return
	}
	a.jsonScrollIn = maxInt(0, a.jsonScrollIn+d)
}

func (a *App) updateJsonSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.jsonSearchOn = false
	case "enter":
		a.jsonSearchOn = false
		out, err := jsonutil.SearchKey(a.jsonInput, a.jsonSearchInput)
		if err != nil {
			a.jsonErr = err.Error()
			a.jsonStatus = ""
			return a, nil
		}
		a.jsonErr = ""
		a.jsonOutput = out
		a.jsonStatus = "busca: " + a.jsonSearchInput
		a.jsonPane = jsonPaneOutput
		a.jsonScrollOut = 0
	case "backspace":
		if len(a.jsonSearchInput) > 0 {
			r := []rune(a.jsonSearchInput)
			a.jsonSearchInput = string(r[:len(r)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			a.jsonSearchInput += string(msg.Runes)
		} else if s := msg.String(); len(s) == 1 {
			a.jsonSearchInput += s
		}
	}
	return a, nil
}

func (a *App) updateJsonEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	runes := []rune(a.jsonInput)
	cursor := a.jsonEditorCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	key := msg.String()

	move := func(next int, extend bool) {
		if next < 0 {
			next = 0
		}
		if next > len(runes) {
			next = len(runes)
		}
		if extend {
			if a.jsonEditorAnchor < 0 {
				a.jsonEditorAnchor = cursor
			}
		} else {
			a.jsonClearSel()
		}
		cursor = next
	}

	switch key {
	case "esc":
		a.jsonEditing = false
		a.jsonClearSel()
		return a, nil
	case "ctrl+a":
		if len(runes) == 0 {
			a.jsonClearSel()
			return a, nil
		}
		a.jsonEditorAnchor = 0
		cursor = len(runes)
		a.jsonEditorCursor = cursor
		return a, nil
	case "ctrl+c":
		if lo, hi, ok := a.jsonSelRange(); ok {
			_ = copyToClipboard(string(runes[lo:hi]))
			a.jsonStatus = "seleção copiada ✓"
		}
		return a, nil
	case "ctrl+x":
		if lo, hi, ok := a.jsonSelRange(); ok {
			_ = copyToClipboard(string(runes[lo:hi]))
			a.jsonDeleteSelection()
			a.jsonStatus = "recortado ✓"
		}
		return a, nil
	case "ctrl+v":
		text, err := readClipboard()
		if err != nil {
			a.jsonErr = "paste: " + err.Error()
			return a, nil
		}
		if a.jsonDeleteSelection() {
			runes = []rune(a.jsonInput)
			cursor = a.jsonEditorCursor
		}
		ins := []rune(text)
		runes = append(runes[:cursor], append(ins, runes[cursor:]...)...)
		cursor += len(ins)
		a.jsonInput = string(runes)
		a.jsonEditorCursor = cursor
		a.jsonClearSel()
		a.jsonStatus = "colado ✓"
		return a, nil
	case "enter":
		if a.jsonDeleteSelection() {
			runes = []rune(a.jsonInput)
			cursor = a.jsonEditorCursor
		}
		indent := apiLineIndent(runes, cursor)
		prev := apiRuneBefore(runes, cursor)
		next := apiRuneAfter(runes, cursor)
		if (prev == '{' && next == '}') || (prev == '[' && next == ']') {
			insert := []rune("\n" + indent + "  \n" + indent)
			runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
			cursor += len([]rune("\n" + indent + "  "))
			a.jsonInput = string(runes)
			a.jsonEditorCursor = cursor
			a.jsonClearSel()
			return a, nil
		}
		if prev == '{' || prev == '[' {
			indent += "  "
		}
		insert := []rune("\n" + indent)
		runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
		cursor += len(insert)
		a.jsonInput = string(runes)
		a.jsonEditorCursor = cursor
		a.jsonClearSel()
		return a, nil
	case "left":
		if lo, _, ok := a.jsonSelRange(); ok {
			move(lo, false)
		} else {
			move(cursor-1, false)
		}
	case "right":
		if _, hi, ok := a.jsonSelRange(); ok {
			move(hi, false)
		} else {
			move(cursor+1, false)
		}
	case "shift+left":
		move(cursor-1, true)
	case "shift+right":
		move(cursor+1, true)
	case "ctrl+left":
		move(apiMoveWordLeft(runes, cursor), false)
	case "ctrl+right":
		move(apiMoveWordRight(runes, cursor), false)
	case "ctrl+shift+left":
		move(apiMoveWordLeft(runes, cursor), true)
	case "ctrl+shift+right":
		move(apiMoveWordRight(runes, cursor), true)
	case "up":
		move(apiMoveLine(runes, cursor, -1), false)
	case "down":
		move(apiMoveLine(runes, cursor, 1), false)
	case "shift+up":
		move(apiMoveLine(runes, cursor, -1), true)
	case "shift+down":
		move(apiMoveLine(runes, cursor, 1), true)
	case "home":
		move(apiLineStart(runes, cursor), false)
	case "end":
		move(apiLineEnd(runes, cursor), false)
	case "shift+home":
		move(apiLineStart(runes, cursor), true)
	case "shift+end":
		move(apiLineEnd(runes, cursor), true)
	case "ctrl+home":
		move(0, false)
	case "ctrl+end":
		move(len(runes), false)
	case "ctrl+shift+home":
		move(0, true)
	case "ctrl+shift+end":
		move(len(runes), true)
	case "backspace":
		if a.jsonDeleteSelection() {
			return a, nil
		}
		if cursor > 0 {
			runes = append(runes[:cursor-1], runes[cursor:]...)
			cursor--
			a.jsonInput = string(runes)
		}
	case "delete":
		if a.jsonDeleteSelection() {
			return a, nil
		}
		if cursor < len(runes) {
			runes = append(runes[:cursor], runes[cursor+1:]...)
			a.jsonInput = string(runes)
		}
	case "tab":
		if a.jsonDeleteSelection() {
			runes = []rune(a.jsonInput)
			cursor = a.jsonEditorCursor
		}
		insert := []rune("  ")
		runes = append(runes[:cursor], append(insert, runes[cursor:]...)...)
		cursor += len(insert)
		a.jsonInput = string(runes)
		a.jsonEditorCursor = cursor
		a.jsonClearSel()
		return a, nil
	case "shift+tab":
		a.jsonClearSel()
		n := apiUnindentAt(runes, cursor)
		if n > 0 {
			runes = append(runes[:cursor-n], runes[cursor:]...)
			cursor -= n
			a.jsonInput = string(runes)
			a.jsonEditorCursor = cursor
		}
		return a, nil
	default:
		var inserted []rune
		if len(msg.Runes) > 0 {
			inserted = msg.Runes
		} else if s := key; len(s) == 1 {
			inserted = []rune(s)
		}
		if len(inserted) > 0 {
			if a.jsonDeleteSelection() {
				runes = []rune(a.jsonInput)
				cursor = a.jsonEditorCursor
			}
			runes = append(runes[:cursor], append(inserted, runes[cursor:]...)...)
			cursor += len(inserted)
			a.jsonInput = string(runes)
			a.jsonClearSel()
		}
	}
	a.jsonEditorCursor = cursor
	return a, nil
}

func (a *App) runJsonAction(key string) {
	in := a.jsonInput
	a.jsonErr = ""
	switch key {
	case "p":
		out, err := jsonutil.Pretty(in)
		a.setJsonResult(out, "pretty", err)
	case "m":
		out, err := jsonutil.Minify(in)
		a.setJsonResult(out, "minify", err)
	case "v":
		if err := jsonutil.Validate(in); err != nil {
			a.jsonErr = err.Error()
			a.jsonStatus = ""
			a.jsonOutput = err.Error() + "\n"
			a.jsonPane = jsonPaneOutput
			return
		}
		a.jsonErr = ""
		a.jsonStatus = "JSON válido ✓"
		a.jsonOutput = "OK — JSON válido\n"
		a.jsonPane = jsonPaneOutput
	case "s":
		out, err := jsonutil.SortKeys(in)
		a.setJsonResult(out, "sort keys", err)
	case "w":
		out, label, err := jsonutil.ConvertToggle(in, "yaml")
		a.setJsonResult(out, label, err)
	case "t":
		out, label, err := jsonutil.ConvertToggle(in, "toml")
		a.setJsonResult(out, label, err)
	case "x":
		out, label, err := jsonutil.ConvertToggle(in, "xml")
		a.setJsonResult(out, label, err)
	case "d":
		if strings.TrimSpace(a.jsonOutput) == "" {
			a.jsonErr = "output vazio — rode p/m/v/s/w/t/x antes"
			a.jsonStatus = ""
			return
		}
		a.jsonOutput = jsonutil.DiffText(in, a.jsonOutput)
		a.jsonStatus = "diff input ↔ output"
		a.jsonErr = ""
		a.jsonPane = jsonPaneOutput
		a.jsonScrollOut = 0
	}
}

func (a *App) runJsonStripNulls() {
	out, err := jsonutil.StripNulls(a.jsonInput)
	a.setJsonResult(out, "strip nulls", err)
}

func (a *App) setJsonResult(out, status string, err error) {
	if err != nil {
		a.jsonErr = err.Error()
		a.jsonStatus = ""
		a.jsonOutput = err.Error() + "\n"
		a.jsonPane = jsonPaneOutput
		a.jsonScrollOut = 0
		return
	}
	a.jsonErr = ""
	a.jsonStatus = status
	a.jsonOutput = out
	a.jsonPane = jsonPaneOutput
	a.jsonScrollOut = 0
}

func (a *App) copyJsonOutput() {
	text := a.jsonOutput
	if strings.TrimSpace(text) == "" {
		a.jsonErr = "nada para copiar"
		return
	}
	if err := copyToClipboard(text); err != nil {
		a.jsonErr = "clipboard: " + err.Error()
		a.jsonStatus = ""
		return
	}
	a.jsonErr = ""
	a.jsonStatus = "copiado ✓"
}

func copyToClipboard(text string) error {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	var last error
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err != nil {
			last = err
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last == nil {
		last = fmt.Errorf("wl-copy/xclip/xsel não encontrados")
	}
	return last
}

func readClipboard() (string, error) {
	candidates := [][]string{
		{"wl-paste", "-n"},
		{"xclip", "-selection", "clipboard", "-o"},
		{"xsel", "--clipboard", "--output"},
	}
	var last error
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err != nil {
			last = err
			continue
		}
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			last = err
			continue
		}
		return string(out), nil
	}
	if last == nil {
		last = fmt.Errorf("wl-paste/xclip/xsel não encontrados")
	}
	return "", last
}
