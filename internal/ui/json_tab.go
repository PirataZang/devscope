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

	openH := maxInt(5, bodyH*30/100)
	keysH := maxInt(6, bodyH-openH)
	openLines := append([]string{StyleMuted.Render("pretty, minify, validate, convert e diff")}, moduleOpenHint()...)
	keyLines := []string{
		StyleMuted.Render("p Pretty   m Minify   v Validate   s Sort"),
		StyleMuted.Render("w YAML     t TOML     x XML        d Diff"),
		StyleMuted.Render("n strip nulls   / buscar   c copiar"),
		StyleMuted.Render("e editar   tab painel   esc sair"),
		StyleMuted.Render("edição: seleção · ctrl+a/c/x/v"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("JSON", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("ATALHOS NO CLIENTE", fitExactLines(keyLines, keysH-2), centerW, keysH, false),
	)
	details := []string{
		StyleMuted.Render("Formato  JSON / YAML / TOML / XML"),
		StyleMuted.Render("Diff     lado a lado"),
		StyleMuted.Render("Busca    por chave"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir cliente"},
		[2]string{"-", "abrir JWT"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderJsonTab(_ *core.Project) string {
	w := maxInt(60, a.width)
	h := maxInt(18, a.height-2)
	leftW := maxInt(28, (w-1)/2)
	rightW := maxInt(28, w-leftW-1)
	bodyH := h - 4

	header := a.renderJsonHeader()
	left := a.renderJsonPane("input", a.jsonInput, leftW, bodyH, a.jsonPane == jsonPaneInput, a.jsonScrollIn, a.jsonEditing && a.jsonPane == jsonPaneInput)
	right := a.renderJsonPane("output", a.jsonOutput, rightW, bodyH, a.jsonPane == jsonPaneOutput, a.jsonScrollOut, false)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	hints := "p/m/v/s/w/t/x/d ações  e edit  tab painéis  n nulls  / busca  c copia  esc"
	if a.jsonSearchOn {
		hints = "buscar chave: " + a.jsonSearchInput + "█  enter  esc"
	} else if a.jsonEditing {
		hints = "editando  shift/ctrl+←→↑↓  ctrl+a/c/x/v  esc sair"
	}
	if a.jsonStatus != "" {
		hints = a.jsonStatus + "  ·  " + hints
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, a.renderStatusBar(hints))
}

func (a *App) renderJsonHeader() string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabJSON)).Bold(true)
	line := accent.Render("{ } JSON") + StyleMuted.Render("  p pretty · m minify · v validate · s sort · w yaml · t toml · x xml · d diff")
	if a.jsonErr != "" {
		line += "  " + StyleUnhealthy.Render(truncate(a.jsonErr, 48))
	} else if a.jsonStatus != "" {
		line += "  " + StyleHealthy.Render(truncate(a.jsonStatus, 36))
	}
	return line
}

func (a *App) renderJsonPane(title, content string, width, height int, focus bool, scroll int, editing bool) string {
	viewport := maxInt(1, height-2)
	innerW := maxInt(8, width-2)
	if editing {
		lines := a.renderJsonMultilineEdit(content, innerW, viewport)
		return renderApiTitledBox("["+title+"]", lines, width, height, focus)
	}
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	maxScroll := maxInt(0, len(raw)-viewport)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if title == "input" {
		a.jsonScrollIn = scroll
	} else {
		a.jsonScrollOut = scroll
	}
	start := scroll
	end := minInt(start+viewport, len(raw))
	lines := make([]string, 0, viewport)
	for _, line := range raw[start:end] {
		style := StyleMuted
		if focus {
			style = StyleNormal
		}
		lines = append(lines, style.Render(truncate(sanitizeTerminalLine(line), innerW)))
	}
	return renderApiTitledBox("["+title+"]", fitExactLines(lines, viewport), width, height, focus)
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
