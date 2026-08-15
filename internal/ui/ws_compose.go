package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

type wsComposeFocus int

const (
	wsComposeFocusEditor wsComposeFocus = iota
	wsComposeFocusType
	wsComposeFocusSend
	wsComposeFocusCancel
)

func (a *App) startWsCompose() {
	a.wsComposeOn = true
	a.wsComposeFocus = wsComposeFocusEditor
	a.wsEditing = false
	a.wsEdit = editorState{Cursor: len([]rune(a.wsSend)), Anchor: -1}
	a.wsStatus = "mensagem · tab tipo · enter no Enviar"
}

func (a *App) closeWsCompose() {
	a.wsComposeOn = false
	a.wsComposePending = false
	a.wsComposeFocus = wsComposeFocusEditor
	a.wsEdit = editorState{Anchor: -1}
}

func (a *App) updateWsCompose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.closeWsCompose()
		a.wsStatus = "envio cancelado"
		return a, nil
	case "tab":
		a.wsComposeTab(1)
		return a, nil
	case "shift+tab":
		a.wsComposeTab(-1)
		return a, nil
	case "enter", "ctrl+m", "ctrl+j":
		switch a.wsComposeFocus {
		case wsComposeFocusSend:
			return a, a.submitWsCompose()
		case wsComposeFocusCancel:
			a.closeWsCompose()
			a.wsStatus = "envio cancelado"
			return a, nil
		case wsComposeFocusType:
			a.wsComposeFocus = wsComposeFocusEditor
			return a, nil
		}
	}

	if a.wsComposeFocus != wsComposeFocusEditor {
		return a, nil
	}
	newText, handled := editorApplyKey(msg, a.wsSend, &a.wsEdit, true)
	if handled {
		a.wsSend = newText
	}
	return a, nil
}

func (a *App) wsComposeTab(delta int) {
	switch a.wsComposeFocus {
	case wsComposeFocusEditor:
		if delta > 0 {
			a.wsComposeFocus = wsComposeFocusType
		} else {
			a.wsComposeFocus = wsComposeFocusCancel
		}
	case wsComposeFocusType:
		next := int(a.wsSendMode) + delta
		if next < 0 {
			a.wsComposeFocus = wsComposeFocusEditor
			return
		}
		if next > int(wsSendBinary) {
			a.wsComposeFocus = wsComposeFocusSend
			return
		}
		a.wsSendMode = wsSendMode(next)
	case wsComposeFocusSend:
		if delta > 0 {
			a.wsComposeFocus = wsComposeFocusCancel
		} else {
			a.wsComposeFocus = wsComposeFocusType
			a.wsSendMode = wsSendBinary
		}
	case wsComposeFocusCancel:
		if delta > 0 {
			a.wsComposeFocus = wsComposeFocusEditor
		} else {
			a.wsComposeFocus = wsComposeFocusSend
		}
	}
}

func (a *App) submitWsCompose() tea.Cmd {
	if strings.TrimSpace(a.wsSend) == "" {
		a.wsErr = "escreva a mensagem"
		a.wsComposeFocus = wsComposeFocusEditor
		return nil
	}
	if !a.wsConnected {
		a.wsComposePending = true
		a.wsErr = ""
		a.wsStatus = "conectando…"
		return a.toggleWsConnect()
	}
	if a.wsSess == nil {
		a.wsErr = "sem conexão — esc e c"
		return nil
	}
	cmd := a.wsSendFrame()
	if cmd == nil {
		return nil
	}
	a.closeWsCompose()
	return cmd
}

func (a *App) renderWsCompose() string {
	background := a.renderWsTabBackground()
	boxW := minInt(a.width-4, maxInt(58, a.width*72/100))
	boxH := minInt(a.height-2, maxInt(20, a.height*68/100))
	innerW := maxInt(32, boxW-6)
	accent := tabAccentColor(TabWebSocket)

	proj := ""
	if p := a.currentProject(); p != nil {
		proj = p.Name
	}

	editing := a.wsComposeFocus == wsComposeFocusEditor
	editorH := maxInt(6, boxH-18)
	ed := a.wsEdit
	bodyLines := renderEditorLines(a.wsSend, &ed, innerW-2, editorH, editing, a.wsSendMode == wsSendJSON)
	a.wsEdit = ed

	lines := tunnelModalChrome("WS", accent, "Nova mensagem", "escrever e enviar para o servidor", proj, innerW)
	lines = append(lines, "")
	lines = append(lines, a.renderWsComposeTypes(innerW))
	lines = append(lines, "")

	msgTitle := "mensagem"
	if editing {
		msgTitle = "mensagem · enter nova linha"
	}
	msgBox := renderApiTitledBox(msgTitle, bodyLines, innerW, editorH+2, editing)
	lines = append(lines, strings.Split(msgBox, "\n")...)
	lines = append(lines, "")

	preview := StyleMuted.Render("preview  ")
	first := strings.TrimSpace(strings.SplitN(a.wsSend, "\n", 2)[0])
	if first == "" {
		preview += StyleMuted.Render("(digite a mensagem)")
	} else {
		preview += StyleHealthy.Render(truncate(first, maxInt(20, innerW-12)))
	}
	lines = append(lines, preview)

	sendBtn := "  Enviar  "
	cancelBtn := "  Cancelar  "
	switch a.wsComposeFocus {
	case wsComposeFocusSend:
		sendBtn = StyleSelected.Render("▸ Enviar ◂")
		cancelBtn = StyleMuted.Render(cancelBtn)
	case wsComposeFocusCancel:
		sendBtn = StyleMuted.Render(sendBtn)
		cancelBtn = StyleSelected.Render("▸ Cancelar ◂")
	default:
		sendBtn = StyleMuted.Render(sendBtn)
		cancelBtn = StyleMuted.Render(cancelBtn)
	}
	errLine := ""
	if a.wsStatus == "connecting…" || a.wsStatus == "conectando…" {
		errLine = StyleWarning.Render("conectando…")
	} else if a.wsErr != "" {
		errLine = StyleUnhealthy.Render(a.wsErr)
	}
	lines = append(lines, "",
		sendBtn+StyleMuted.Render("    ")+cancelBtn,
	)
	if errLine != "" {
		lines = append(lines, errLine)
	}
	lines = append(lines, "",
		StyleMuted.Render("tab troca tipo  ·  enter no Enviar manda  ·  esc cancela"),
	)

	if boxH < len(lines) {
		boxH = len(lines)
	}
	box := StylePanel.
		Width(boxW).
		BorderForeground(accent).
		Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
	return overlayCentered(background, box, a.width, a.height)
}

func (a *App) renderWsComposeTypes(width int) string {
	names := []string{"Texto", "JSON", "Binário"}
	var parts []string
	for i, n := range names {
		on := wsSendMode(i) == a.wsSendMode
		focus := a.wsComposeFocus == wsComposeFocusType && on
		switch {
		case focus:
			parts = append(parts, StyleSelected.Render("▸ "+n+" ◂"))
		case on:
			parts = append(parts, StyleHealthy.Render(" "+n+" "))
		default:
			parts = append(parts, StyleMuted.Render(" "+n+" "))
		}
	}
	row := strings.Join(parts, StyleMuted.Render("│"))
	_ = width
	return StyleMuted.Render("tipo  ") + row
}

func (a *App) renderWsTabBackground() string {
	p := a.currentProject()
	if p == nil {
		p = &core.Project{}
	}
	on := a.wsComposeOn
	a.wsComposeOn = false
	view := a.renderWsTab(p)
	a.wsComposeOn = on
	return view
}
