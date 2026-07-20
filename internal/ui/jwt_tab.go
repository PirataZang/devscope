package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jwtutil"
)

type jwtPane int

const (
	jwtPaneSecret jwtPane = iota
	jwtPaneInput
	jwtPaneOutput
)

func (a *App) enterJwtTab(_ *core.Project) {
	a.tab = TabJWT
	a.tabCursor = 0
	a.jwtOpen = false
	a.jwtEditing = false
	a.jwtEdit.clearSel()
}

func (a *App) openJwtClient(_ *core.Project) tea.Cmd {
	a.jwtOpen = true
	a.jwtEditing = false
	a.jwtEdit = editorState{Anchor: -1}
	a.jwtPane = jwtPaneInput
	a.jwtErr = ""
	a.jwtStatus = ""
	if a.jwtAlg == "" {
		a.jwtAlg = "HS256"
	}
	if a.jwtSecret == "" {
		a.jwtSecret = "your-256-bit-secret"
	}
	if a.jwtInput == "" {
		a.jwtInput = sampleJWT()
	}
	a.rememberJwtToken(a.jwtInput)
	// Auto-decode so the right pane isn't empty.
	if out, err := jwtutil.DecodePretty(a.jwtInput); err == nil {
		a.jwtOutput = out
		a.jwtStatus = "decode"
	}
	return nil
}

func sampleJWT() string {
	tok, err := jwtutil.Sign(jwtutil.GenerateClaims(), "your-256-bit-secret", "HS256")
	if err != nil {
		return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0."
	}
	return tok
}

func (a *App) leaveJwtTab() tea.Cmd {
	a.jwtOpen = false
	a.jwtEditing = false
	a.jwtEdit.clearSel()
	a.tab = TabJWT
	a.tabCursor = 0
	return nil
}

func (a *App) renderJwtLanding(p *core.Project) string {
	w, h := a.moduleSize()
	ctx := a.renderModuleContext(p, w, "JWT", "utils")
	bodyH := maxInt(12, h-lipgloss.Height(ctx))
	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	openH := maxInt(5, bodyH*30/100)
	keysH := maxInt(6, bodyH-openH)
	openLines := append([]string{StyleMuted.Render("decode, verify, generate e sign — estilo jwt.io")}, moduleOpenHint()...)
	keyLines := []string{
		StyleMuted.Render("d Decode   v Verify   g Generate   s Sign"),
		StyleMuted.Render("y Copy token   Y Copy result   c Claims"),
		StyleMuted.Render("x Export   [] alg   tab painéis"),
		StyleMuted.Render("e editar · ctrl+y token · ctrl+a/c/x/v"),
	}
	center := lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("JWT", fitExactLines(openLines, openH-2), centerW, openH, true),
		renderApiTitledBox("ATALHOS", fitExactLines(keyLines, keysH-2), centerW, keysH, false),
	)
	details := []string{
		StyleMuted.Render("Algs   HS* / RS* / ES* / EdDSA"),
		StyleMuted.Render("Modos  decode · verify · sign"),
		StyleMuted.Render("Extra  claims · export"),
	}
	actions := moduleActionLines(
		[2]string{"enter", "abrir cliente"},
		[2]string{"0", "abrir JSON"},
		[2]string{"esc", "voltar"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderJwtTab(_ *core.Project) string {
	w := maxInt(60, a.width)
	h := maxInt(18, a.height-2)
	leftW := maxInt(28, (w-1)/2)
	rightW := maxInt(28, w-leftW-1)
	bodyH := h - 5

	header := a.renderJwtHeader()
	secretLine := a.renderJwtSecretLine(w)
	left := a.renderJwtInputPane(leftW, bodyH)
	right := a.renderJwtOutputPane(rightW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)

	hints := "d/v/g/s  y token  Y result  c claims  x export  [] alg  ←→  e  tab  esc"
	if a.jwtEditing {
		hints = "editando  ctrl+y token  shift/ctrl+←→↑↓  ctrl+a/c/x/v  esc sair"
	}
	if a.jwtStatus != "" {
		hints = a.jwtStatus + "  ·  " + hints
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, secretLine, body, a.renderStatusBar(hints))
}

func (a *App) renderJwtHeader() string {
	accent := lipgloss.NewStyle().Foreground(tabAccentColor(TabJWT)).Bold(true)
	algs := jwtutil.Algs()
	var parts []string
	for _, alg := range algs {
		if alg == a.jwtAlg {
			parts = append(parts, StyleSelected.Render(">"+alg+"<"))
		} else {
			parts = append(parts, StyleMuted.Render(alg))
		}
	}
	line := accent.Render("⚿ JWT") + "  " + strings.Join(parts, " ")
	if a.jwtErr != "" {
		line += "  " + StyleUnhealthy.Render(truncate(a.jwtErr, 40))
	} else if a.jwtStatus != "" {
		line += "  " + StyleHealthy.Render(truncate(a.jwtStatus, 32))
	}
	return line
}

func (a *App) renderJwtSecretLine(width int) string {
	focus := a.jwtPane == jwtPaneSecret
	label := StyleMuted.Render("secret ")
	innerW := maxInt(12, width-10)
	val := a.jwtSecret
	h := a.jwtHScrollSecret
	if a.jwtEditing && focus {
		ed := a.jwtEdit
		ed.HScroll = h
		lines := renderEditorLines(val, &ed, innerW, 1, true, false)
		a.jwtEdit.HScroll = ed.HScroll
		a.jwtHScrollSecret = ed.HScroll
		show := ""
		if len(lines) > 0 {
			show = lines[0]
		}
		return label + StyleSelected.Render(show)
	}
	runes := []rune(val)
	if h > len(runes) {
		h = len(runes)
	}
	a.jwtHScrollSecret = h
	show := string(runes[h:])
	show = truncate(show, innerW)
	titleExtra := ""
	if h > 0 {
		titleExtra = StyleMuted.Render(" ←" + strconv.Itoa(h) + " ")
	}
	if focus {
		return label + titleExtra + StyleSelected.Render(show)
	}
	return label + titleExtra + StyleNormal.Render(show)
}

func (a *App) renderJwtInputPane(width, height int) string {
	focus := a.jwtPane == jwtPaneInput
	viewport := maxInt(1, height-2)
	innerW := maxInt(8, width-2)
	editing := a.jwtEditing && focus
	title := paneTitleWithHScroll("token / claims", a.jwtHScrollIn)

	ed := a.jwtEdit
	ed.VScroll = a.jwtScrollIn
	ed.HScroll = a.jwtHScrollIn
	var lines []string
	if editing {
		highlight := strings.HasPrefix(strings.TrimSpace(a.jwtInput), "{")
		lines = renderEditorLines(a.jwtInput, &ed, innerW, viewport, true, highlight)
		a.jwtEdit = ed
		a.jwtScrollIn = ed.VScroll
		a.jwtHScrollIn = ed.HScroll
	} else {
		ed.Anchor = -1
		lines = a.renderJwtStaticLines(a.jwtInput, a.jwtScrollIn, a.jwtHScrollIn, innerW, viewport, focus, true)
	}
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) renderJwtOutputPane(width, height int) string {
	focus := a.jwtPane == jwtPaneOutput
	viewport := maxInt(1, height-2)
	innerW := maxInt(8, width-2)
	title := paneTitleWithHScroll("result", a.jwtHScrollOut)
	lines := a.renderJwtStaticLines(a.jwtOutput, a.jwtScrollOut, a.jwtHScrollOut, innerW, viewport, focus, false)
	return renderApiTitledBox(title, fitExactLines(lines, viewport), width, height, focus)
}

func (a *App) renderJwtStaticLines(content string, vScroll, hScroll, width, height int, focus, colorJWT bool) []string {
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	maxScroll := maxInt(0, len(raw)-height)
	if vScroll > maxScroll {
		vScroll = maxScroll
	}
	if a.jwtPane == jwtPaneOutput {
		a.jwtScrollOut = vScroll
	} else if a.jwtPane == jwtPaneInput && !a.jwtEditing {
		a.jwtScrollIn = vScroll
	}
	start := vScroll
	end := minInt(start+height, len(raw))
	lines := make([]string, 0, height)
	for _, line := range raw[start:end] {
		plain := sanitizeTerminalLine(line)
		display := sliceColumns(plain, hScroll, width)
		if colorJWT && strings.Count(plain, ".") >= 2 && !strings.HasPrefix(strings.TrimSpace(plain), "{") {
			display = colorJWTTokenLine(plain, hScroll, width)
		} else if focus && strings.HasPrefix(strings.TrimSpace(plain), "{") {
			display = renderJSONColumns(plain, hScroll, width)
		} else {
			style := StyleMuted
			if focus {
				style = StyleNormal
			}
			display = style.Render(display)
		}
		lines = append(lines, display)
	}
	return lines
}

func colorJWTTokenLine(line string, hScroll, width int) string {
	return colorJWTVisible(sliceColumns(line, hScroll, width))
}

func colorJWTVisible(vis string) string {
	parts := strings.SplitN(vis, ".", 3)
	styles := []lipgloss.Style{
		lipgloss.NewStyle().Foreground(ColorAccent),
		lipgloss.NewStyle().Foreground(ColorPink),
		lipgloss.NewStyle().Foreground(ColorMuted),
	}
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(StyleMuted.Render("."))
		}
		st := StyleNormal
		if i < len(styles) {
			st = styles[i]
		}
		b.WriteString(st.Render(p))
	}
	return b.String()
}

func (a *App) handleJwtKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.jwtEditing {
		return a.updateJwtEdit(msg)
	}
	switch msg.String() {
	case "esc":
		return a, a.leaveJwtTab()
	case "d":
		a.runJwtDecode()
	case "v":
		a.runJwtVerify()
	case "g":
		a.runJwtGenerate()
	case "s":
		a.runJwtSign()
	case "c":
		a.runJwtCopyClaims()
	case "x":
		a.runJwtExport()
	case "y":
		a.runJwtCopyToken()
	case "Y", "shift+y":
		a.runJwtCopyResult()
	case "[":
		a.cycleJwtAlg(-1)
	case "]":
		a.cycleJwtAlg(1)
	case "tab":
		a.jwtPane = jwtPane((int(a.jwtPane) + 1) % 3)
	case "shift+tab":
		a.jwtPane = jwtPane((int(a.jwtPane) + 2) % 3)
	case "e", "enter":
		if a.jwtPane == jwtPaneOutput {
			a.jwtPane = jwtPaneInput
		}
		a.jwtEditing = true
		a.jwtEdit.clearSel()
		if a.jwtPane == jwtPaneSecret {
			a.jwtEdit.Cursor = len([]rune(a.jwtSecret))
			a.jwtEdit.HScroll = a.jwtHScrollSecret
		} else {
			a.jwtEdit.Cursor = len([]rune(a.jwtInput))
			a.jwtEdit.HScroll = a.jwtHScrollIn
			a.jwtEdit.VScroll = a.jwtScrollIn
		}
	case "up", "k":
		a.jwtScrollDelta(-1)
	case "down", "j":
		a.jwtScrollDelta(1)
	case "pgup":
		a.jwtScrollDelta(-10)
	case "pgdown":
		a.jwtScrollDelta(10)
	case "left", "h":
		a.jwtHScrollDelta(-8)
	case "right", "l":
		a.jwtHScrollDelta(8)
	case "home":
		a.jwtSetHScroll(0)
	}
	_ = p
	return a, nil
}

func (a *App) jwtScrollDelta(d int) {
	if a.jwtPane == jwtPaneOutput {
		a.jwtScrollOut = maxInt(0, a.jwtScrollOut+d)
		return
	}
	if a.jwtPane == jwtPaneSecret {
		return
	}
	a.jwtScrollIn = maxInt(0, a.jwtScrollIn+d)
}

func (a *App) jwtHScrollDelta(d int) {
	width := maxInt(20, a.width/2-4)
	switch a.jwtPane {
	case jwtPaneSecret:
		a.jwtHScrollSecret = hScrollDelta(a.jwtHScrollSecret, d, maxLineRuneLen(a.jwtSecret), width)
	case jwtPaneOutput:
		a.jwtHScrollOut = hScrollDelta(a.jwtHScrollOut, d, maxLineRuneLen(a.jwtOutput), width)
	default:
		a.jwtHScrollIn = hScrollDelta(a.jwtHScrollIn, d, maxLineRuneLen(a.jwtInput), width)
	}
}

func (a *App) jwtSetHScroll(v int) {
	switch a.jwtPane {
	case jwtPaneSecret:
		a.jwtHScrollSecret = maxInt(0, v)
	case jwtPaneOutput:
		a.jwtHScrollOut = maxInt(0, v)
	default:
		a.jwtHScrollIn = maxInt(0, v)
	}
}

func (a *App) cycleJwtAlg(delta int) {
	algs := jwtutil.Algs()
	idx := 0
	for i, alg := range algs {
		if alg == a.jwtAlg {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(algs)) % len(algs)
	a.jwtAlg = algs[idx]
	a.jwtStatus = "alg → " + a.jwtAlg
}

func (a *App) updateJwtEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		a.jwtEditing = false
		a.jwtEdit.clearSel()
		return a, nil
	}
	// Copy token without leaving edit (selection copy stays on ctrl+c).
	if msg.String() == "ctrl+y" {
		a.runJwtCopyToken()
		return a, nil
	}
	// Enter on secret finishes edit (single-line field).
	if a.jwtPane == jwtPaneSecret && msg.String() == "enter" {
		a.jwtEditing = false
		a.jwtEdit.clearSel()
		return a, nil
	}

	text := a.jwtInput
	multiline := true
	if a.jwtPane == jwtPaneSecret {
		text = a.jwtSecret
		multiline = false
	}
	newText, handled := editorApplyKey(msg, text, &a.jwtEdit, multiline)
	if !handled {
		return a, nil
	}
	if a.jwtPane == jwtPaneSecret {
		a.jwtSecret = newText
		a.jwtHScrollSecret = a.jwtEdit.HScroll
	} else {
		a.jwtInput = newText
		a.rememberJwtToken(newText)
		a.jwtScrollIn = a.jwtEdit.VScroll
		a.jwtHScrollIn = a.jwtEdit.HScroll
	}
	return a, nil
}

func (a *App) runJwtDecode() {
	a.rememberJwtToken(a.jwtInput)
	out, err := jwtutil.DecodePretty(a.jwtSourceToken())
	a.setJwtResult(out, "decode", err)
	a.jwtHScrollOut = 0
}

func (a *App) runJwtVerify() {
	a.rememberJwtToken(a.jwtInput)
	out, err := jwtutil.Verify(a.jwtSourceToken(), a.jwtSecret, a.jwtAlg)
	a.setJwtResult(out, "verify", err)
	a.jwtHScrollOut = 0
}

func (a *App) runJwtGenerate() {
	a.jwtInput = jwtutil.GenerateClaims()
	a.jwtPane = jwtPaneInput
	a.jwtEditing = true
	a.jwtEdit = editorState{Cursor: len([]rune(a.jwtInput)), Anchor: -1}
	a.jwtErr = ""
	a.jwtStatus = "claims — edite e pressione s para sign"
	a.jwtOutput = "Pressione s para assinar com o secret atual.\n"
	a.jwtHScrollIn = 0
	a.jwtScrollIn = 0
}

func (a *App) runJwtSign() {
	tok, err := jwtutil.Sign(a.jwtInput, a.jwtSecret, a.jwtAlg)
	if err != nil {
		a.setJwtResult("", "sign", err)
		return
	}
	a.jwtInput = tok
	a.jwtLastToken = tok
	a.jwtEditing = false
	a.jwtEdit.clearSel()
	pretty, _ := jwtutil.DecodePretty(tok)
	a.jwtOutput = "TOKEN\n" + tok + "\n\n" + pretty
	a.jwtErr = ""
	a.jwtPane = jwtPaneOutput
	a.jwtScrollOut = 0
	a.jwtHScrollOut = 0
	a.jwtHScrollIn = 0
	if err := copyToClipboard(tok); err != nil {
		a.jwtStatus = "signed ✓ · y copia token"
		return
	}
	a.jwtStatus = "signed · token no clipboard ✓"
}

func (a *App) runJwtCopyClaims() {
	src := a.jwtSourceToken()
	claims, err := jwtutil.ClaimsJSON(src)
	if err != nil {
		in := strings.TrimSpace(a.jwtInput)
		if in != "" && strings.HasPrefix(in, "{") {
			claims = a.jwtInput
			err = nil
		}
	}
	if err != nil {
		a.jwtErr = err.Error()
		a.jwtStatus = ""
		return
	}
	if err := copyToClipboard(claims); err != nil {
		a.jwtErr = "clipboard: " + err.Error()
		a.jwtStatus = ""
		return
	}
	a.jwtErr = ""
	a.jwtStatus = "claims no clipboard ✓"
}

func (a *App) runJwtExport() {
	a.rememberJwtToken(a.jwtInput)
	out, err := jwtutil.ExportJSON(a.jwtSourceToken())
	a.setJwtResult(out, "export json", err)
	a.jwtHScrollOut = 0
}

func (a *App) runJwtCopyToken() {
	tok := a.jwtSourceToken()
	if !lookLikeJWT(tok) {
		a.jwtErr = "nenhum JWT para copiar (sign ou cole um token)"
		a.jwtStatus = ""
		return
	}
	a.jwtLastToken = tok
	if err := copyToClipboard(tok); err != nil {
		a.jwtErr = "clipboard: " + err.Error()
		a.jwtStatus = ""
		return
	}
	a.jwtErr = ""
	a.jwtStatus = "token no clipboard ✓"
}

func (a *App) runJwtCopyResult() {
	text := strings.TrimSpace(a.jwtOutput)
	if text == "" {
		a.jwtErr = "result vazio"
		a.jwtStatus = ""
		return
	}
	if err := copyToClipboard(text); err != nil {
		a.jwtErr = "clipboard: " + err.Error()
		a.jwtStatus = ""
		return
	}
	a.jwtErr = ""
	a.jwtStatus = "result no clipboard ✓"
}

func lookLikeJWT(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \n\t") {
		return false
	}
	parts := strings.Split(s, ".")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}

func extractJWT(text string) string {
	text = strings.TrimSpace(text)
	if lookLikeJWT(text) {
		return text
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if lookLikeJWT(line) {
			return line
		}
		for _, field := range strings.Fields(line) {
			if lookLikeJWT(field) {
				return field
			}
		}
	}
	return ""
}

func (a *App) rememberJwtToken(s string) {
	if tok := extractJWT(s); tok != "" {
		a.jwtLastToken = tok
	}
}

// jwtSourceToken returns the best available JWT: input, cached last token, or output scrape.
func (a *App) jwtSourceToken() string {
	if lookLikeJWT(a.jwtInput) {
		return strings.TrimSpace(a.jwtInput)
	}
	if lookLikeJWT(a.jwtLastToken) {
		return a.jwtLastToken
	}
	return extractJWT(a.jwtOutput)
}

func (a *App) setJwtResult(out, status string, err error) {
	if err != nil {
		a.jwtErr = err.Error()
		a.jwtStatus = ""
		a.jwtOutput = err.Error() + "\n"
		a.jwtPane = jwtPaneOutput
		a.jwtScrollOut = 0
		return
	}
	a.jwtErr = ""
	a.jwtStatus = status
	a.jwtOutput = out
	a.jwtPane = jwtPaneOutput
	a.jwtScrollOut = 0
}
