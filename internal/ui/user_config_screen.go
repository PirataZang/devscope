package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
)

// Painel de preferências: os mesmos três blocos do user_config.txt, agora em
// campos. Editar o arquivo cru obrigava a saber o formato — errar uma aspa
// derrubava o bloco inteiro. Aqui o tema é uma lista que gira com as setas, os
// dez atalhos são dez caixas de nome+comando, e a IA é um campo. O arquivo
// continua sendo o armazenamento, escrito com os comentários de sempre.

// Campos: 0 = tema, 1..20 = os dez slots (nome, comando), 21 = IA.
const (
	prefFieldTheme = 0
	prefFieldsPer  = 2 // nome e comando por slot
)

func prefFieldCount() int  { return 1 + len(config.AppSlots)*prefFieldsPer + 1 }
func prefFieldAI() int     { return prefFieldCount() - 1 }
func prefSlotOf(f int) int { return (f - 1) / prefFieldsPer }
func prefIsName(f int) bool {
	return f >= 1 && f < prefFieldAI() && (f-1)%prefFieldsPer == 0
}

func (a *App) openUserConfig() {
	ais := collectors.DetectAITools()
	// A cópia de fábrica acompanha a versão do binário, não a que criou o
	// arquivo meses atrás.
	_, _ = config.WriteDefaultUserConfig(ais)

	path, content, err := config.EnsureUserConfig(CurrentTheme(), ais)
	prefs, _ := config.ParseUserConfig(content)
	if prefs.Commands == nil {
		prefs.Commands = map[string]config.AppShortcut{}
	}

	a.userCfgPath = path
	a.userCfgPrefs = prefs
	a.userCfgAIs = ais
	a.userCfgField = prefFieldTheme
	a.userCfgCursor = 0
	a.userCfgScroll = 0
	a.userCfgOn = true
	a.userCfgDirty = false
	a.userCfgConfirmReset = false
	a.userCfgMsg = ""
	if err != nil {
		a.userCfgMsg = "não consegui criar o arquivo: " + err.Error()
	}
}

func (a *App) updateUserConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.userCfgConfirmReset {
		return a.updateUserConfigReset(msg)
	}
	switch msg.String() {
	case "esc":
		a.userCfgOn = false
		if a.userCfgDirty {
			a.statusMsg = "preferências fechadas sem salvar"
		}
		return a, nil
	case "enter", "ctrl+s":
		a.saveUserConfig()
		return a, nil
	case "ctrl+r":
		a.userCfgConfirmReset = true
		return a, nil
	case "tab", "down":
		a.moveUserCfgField(1)
		return a, nil
	case "shift+tab", "up":
		a.moveUserCfgField(-1)
		return a, nil
	}

	// O tema é uma lista: as setas giram entre os temas e já mostram o
	// resultado na tela atrás do painel.
	if a.userCfgField == prefFieldTheme {
		switch msg.String() {
		case "left", "right", " ", "space":
			delta := 1
			if msg.String() == "left" {
				delta = -1
			}
			a.shiftUserCfgTheme(delta)
			return a, nil
		}
		return a, nil
	}
	return a, a.editUserCfgText(msg)
}

func (a *App) moveUserCfgField(delta int) {
	n := prefFieldCount()
	a.userCfgField = (a.userCfgField + delta + n) % n
	a.userCfgCursor = len([]rune(a.userCfgFieldValue()))
}

// shiftUserCfgTheme troca o tema e aplica na hora: a lista só é útil se der
// para ver a cor mudando enquanto se escolhe.
func (a *App) shiftUserCfgTheme(delta int) {
	i := (ThemeIndex(a.userCfgPrefs.Theme) + delta + len(Themes)) % len(Themes)
	a.userCfgPrefs.Theme = Themes[i].ID
	a.userCfgDirty = true
	a.userCfgMsg = ""
	ApplyTheme(a.userCfgPrefs.Theme)
}

func (a *App) userCfgFieldValue() string {
	switch {
	case a.userCfgField == prefFieldTheme:
		return a.userCfgPrefs.Theme
	case a.userCfgField == prefFieldAI():
		return a.userCfgPrefs.AI
	}
	app := a.userCfgPrefs.Commands[config.AppSlots[prefSlotOf(a.userCfgField)]]
	if prefIsName(a.userCfgField) {
		return app.Name
	}
	return app.Command
}

func (a *App) setUserCfgFieldValue(v string) {
	switch {
	case a.userCfgField == prefFieldAI():
		a.userCfgPrefs.AI = v
		return
	case a.userCfgField == prefFieldTheme:
		a.userCfgPrefs.Theme = v
		return
	}
	key := config.AppSlots[prefSlotOf(a.userCfgField)]
	app := a.userCfgPrefs.Commands[key]
	if prefIsName(a.userCfgField) {
		app.Name = v
	} else {
		app.Command = v
	}
	a.userCfgPrefs.Commands[key] = app
}

// editUserCfgText usa o editor de uma linha compartilhado, o mesmo dos outros
// formulários do projeto.
func (a *App) editUserCfgText(msg tea.KeyMsg) tea.Cmd {
	state := editorState{Cursor: a.userCfgCursor, Anchor: -1}
	text, handled := editorApplyKey(msg, a.userCfgFieldValue(), &state, false)
	if !handled {
		return nil
	}
	if text != a.userCfgFieldValue() {
		a.userCfgDirty = true
		a.userCfgMsg = ""
	}
	a.setUserCfgFieldValue(text)
	a.userCfgCursor = state.Cursor
	return nil
}

func (a *App) saveUserConfig() {
	content := config.RenderUserConfig(a.userCfgPrefs, a.userCfgAIs)
	if err := config.SaveUserConfig(content); err != nil {
		a.userCfgMsg = "erro ao salvar: " + err.Error()
		return
	}
	a.userCfgDirty = false
	if a.cfg != nil {
		a.cfg.ApplyUserPrefs(a.userCfgPrefs)
	}
	if a.userCfgPrefs.Theme != "" && ThemeExists(a.userCfgPrefs.Theme) {
		ApplyTheme(a.userCfgPrefs.Theme)
	}
	a.userCfgMsg = "salvo e aplicado"
}

// updateUserConfigReset: reconfigurar tudo é irreversível para quem já ajustou,
// então passa por confirmação — e mesmo assim o antigo vai para o .bak.
func (a *App) updateUserConfigReset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.userCfgConfirmReset = false
		content, err := config.ResetUserConfig(a.userCfgAIs)
		if err != nil {
			a.userCfgMsg = "não consegui restaurar: " + err.Error()
			return a, nil
		}
		prefs, _ := config.ParseUserConfig(content)
		a.userCfgPrefs = prefs
		a.userCfgDirty = false
		if a.cfg != nil {
			a.cfg.ApplyUserPrefs(prefs)
		}
		if ThemeExists(prefs.Theme) {
			ApplyTheme(prefs.Theme)
		}
		a.userCfgMsg = "tudo voltou ao padrão · o anterior está em user_config.bak.txt"
	case "n", "N", "esc":
		a.userCfgConfirmReset = false
	}
	return a, nil
}

// ─── desenho ────────────────────────────────────────────────────────────────

func (a *App) renderUserConfigScreen(background string) string {
	boxW := maxInt(56, minInt(a.width-6, 104))
	boxH := maxInt(18, minInt(a.height-4, 42))
	innerW := maxInt(40, boxW-6)
	accent := lipgloss.Color(ColorPrimary)

	lines := tunnelModalChrome("PREFERÊNCIAS", accent, "Configurações do DevScope",
		"tema, atalhos de programas e agente de IA", "", innerW)
	lines = append(lines, "")
	lines = append(lines, a.renderUserCfgFields(innerW, boxH-len(lines)-2)...)
	lines = append(lines,
		StyleMuted.Render("tab campo  ·  ←→ tema  ·  enter salva  ·  ctrl+r padrão  ·  esc fecha"),
		a.userCfgFooter(innerW),
	)

	view := overlayCentered(background, tunnelModalBox(lines, boxW, boxH, accent), a.width, a.height)
	if a.userCfgConfirmReset {
		confirm := renderDeleteConfirmBox(deleteConfirmOpts{
			Brand:    "PREFERÊNCIAS",
			Color:    ColorWarning,
			Title:    "Voltar tudo ao padrão",
			Subtitle: "tema, atalhos e agente de IA de uma vez",
			Label:    "arquivo",
			Target:   "user_config.txt",
			Detail:   "o atual vai para user_config.bak.txt antes",
		}, a.width, a.height)
		view = overlayCentered(view, confirm, a.width, a.height)
	}
	return view
}

func (a *App) userCfgFooter(width int) string {
	left := StyleMuted.Render(elideLeft(a.userCfgPath, maxInt(20, width-24)))
	switch {
	case a.userCfgMsg != "":
		right := StyleWarning.Render(truncate(a.userCfgMsg, 46))
		if strings.HasPrefix(a.userCfgMsg, "salvo") || strings.HasPrefix(a.userCfgMsg, "tudo") {
			right = StyleHealthy.Render(truncate(a.userCfgMsg, 46))
		}
		return joinWithSpacer(left, right, width)
	case a.userCfgDirty:
		return joinWithSpacer(left, StyleWarning.Render("alterado · enter salva"), width)
	}
	return left
}

// renderUserCfgFields desenha o tema, os dez slots e a IA. Os slots vão em duas
// colunas quando cabem: dez caixas empilhadas não entram em terminal nenhum.
// A rolagem acompanha o campo em foco pela posição real dele — estimar a linha
// deixava a última caixa cortada justo quando o cursor chegava nela.
func (a *App) renderUserCfgFields(width, height int) []string {
	var out []string
	focusStart, focusEnd := -1, -1
	add := func(focused bool, block []string) {
		if focused {
			focusStart, focusEnd = len(out), len(out)+len(block)-1
		}
		out = append(out, block...)
	}

	add(a.userCfgField == prefFieldTheme, strings.Split(a.renderUserCfgTheme(width), "\n"))
	out = append(out, "")
	out = append(out, StyleMuted.Render("ATALHOS — tecla, nome na tela e comando. \"&\" no fim abre solto."))

	cols := 1
	if width >= 92 {
		cols = 2
	}
	colW := (width - (cols-1)*2) / cols
	rows := (len(config.AppSlots) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		var cells []string
		focused := false
		for c := 0; c < cols; c++ {
			i := r + c*rows
			if i >= len(config.AppSlots) {
				continue
			}
			cells = append(cells, a.renderUserCfgSlot(i, colW))
			if s := prefSlotOf(a.userCfgField); a.userCfgField >= 1 && a.userCfgField < prefFieldAI() && s == i {
				focused = true
			}
		}
		add(focused, strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, joinCells(cells, "  ")...), "\n"))
	}

	out = append(out, "")
	add(a.userCfgField == prefFieldAI(), strings.Split(a.renderUserCfgAI(width), "\n"))

	if height <= 0 || len(out) <= height {
		a.userCfgScroll = 0
		return out
	}
	// Mantém a caixa inteira do campo em foco na tela, não só a primeira linha.
	if focusEnd >= 0 {
		if focusEnd >= a.userCfgScroll+height {
			a.userCfgScroll = focusEnd - height + 1
		}
		if focusStart < a.userCfgScroll {
			a.userCfgScroll = focusStart
		}
	}
	a.userCfgScroll = clampScroll(a.userCfgScroll, height, len(out))
	return out[a.userCfgScroll:minInt(a.userCfgScroll+height, len(out))]
}

func joinCells(cells []string, sep string) []string {
	if len(cells) < 2 {
		return cells
	}
	out := []string{cells[0]}
	for _, c := range cells[1:] {
		out = append(out, sep, c)
	}
	return out
}

func (a *App) renderUserCfgTheme(width int) string {
	value := StyleNormal.Render(a.userCfgPrefs.Theme)
	if a.userCfgField == prefFieldTheme {
		value = StyleSelected.Render(" "+a.userCfgPrefs.Theme+" ") +
			StyleMuted.Render("   ←→ troca  ·  "+fmt.Sprintf("%d de %d", ThemeIndex(a.userCfgPrefs.Theme)+1, len(Themes)))
	}
	return panelBox("TEMA — as cores do DevScope (shift+T também troca)",
		[]string{value}, width, 3, a.userCfgField == prefFieldTheme)
}

func (a *App) renderUserCfgSlot(i, width int) string {
	key := config.AppSlots[i]
	app := a.userCfgPrefs.Commands[key]
	nameField := 1 + i*prefFieldsPer
	cmdField := nameField + 1

	// A caixa come 2 colunas de moldura, o prefixo da tecla 4 e o separador 3.
	avail := maxInt(16, width-9)
	half := avail / 2
	name := a.userCfgValueCell(app.Name, nameField, half, "nome na tela")
	cmd := a.userCfgValueCell(app.Command, cmdField, avail-half, "comando")
	line := StyleKey.Render(" "+key+" ") + StyleMuted.Render(" ") + name + StyleMuted.Render(" │ ") + cmd

	focused := a.userCfgField == nameField || a.userCfgField == cmdField
	return panelBox("", []string{line}, width, 3, focused)
}

// userCfgValueCell desenha um campo de texto com cursor quando está em foco e
// um rótulo apagado quando está vazio — slot livre tem que parecer livre.
func (a *App) userCfgValueCell(value string, field, width int, placeholder string) string {
	if a.userCfgField != field {
		if strings.TrimSpace(value) == "" {
			return StyleMuted.Render(padRight(truncate(placeholder, width), width))
		}
		return StyleNormal.Render(padRight(truncate(value, width), width))
	}
	runes := []rune(value)
	cur := minInt(maxInt(a.userCfgCursor, 0), len(runes))
	shown := string(runes[:cur]) + "█" + string(runes[cur:])
	return StyleSelected.Render(padRight(truncate(shown, width), width))
}

func (a *App) renderUserCfgAI(width int) string {
	detected := "nenhuma encontrada no PATH"
	if len(a.userCfgAIs) > 0 {
		detected = strings.Join(a.userCfgAIs, ", ")
	}
	title := "AGENTE DE IA — ctrl+O abre na pasta do projeto  ·  achados: " + detected
	return panelBox(truncate(title, maxInt(20, width-2)),
		[]string{a.userCfgValueCell(a.userCfgPrefs.AI, prefFieldAI(), width-2,
			"vazio = usa o primeiro do PATH")},
		width, 3, a.userCfgField == prefFieldAI())
}
