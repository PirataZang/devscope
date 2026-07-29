package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
	"github.com/mattn/go-runewidth"
)

// Shared layout chrome for project modules (Overview-style).

func (a *App) moduleSize() (width, height int) {
	// Don't invent floors larger than the real content pane — that overflows
	// short VS Code terminals and narrow project shells.
	w := a.width
	if w < 40 {
		w = 40
	}
	h := a.projectPanelHeight()
	if h < 6 {
		h = 6
	}
	return w, h
}

func (a *App) renderModuleContext(p *core.Project, width int, module, status string) string {
	name := "project"
	if p != nil && p.Name != "" {
		name = p.Name
	}
	env := "local"
	if p != nil {
		env = projectEnvLabel(p)
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "—"
	}
	left := StyleMuted.Render("Projeto ") + StyleNormal.Render(truncate(name, 18)) +
		StyleMuted.Render("  Ambiente ") + StyleWarning.Render(env) +
		StyleMuted.Render("  Módulo ") + StyleNormal.Render(module)
	if status == "" && p != nil {
		status = string(p.Status)
	}
	right := StyleMuted.Render(truncate(host, 14))
	if status != "" {
		right = StyleNormal.Render(truncate(status, 28)) + StyleMuted.Render("  ") + right
	}
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderModuleShell(p *core.Project, width, height int, module, status string, center, right string) string {
	ctx := a.renderModuleContext(p, width, module, status)
	body := lipgloss.JoinHorizontal(lipgloss.Top, center, right)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, body)
}

func (a *App) moduleRightWidth(width int) int {
	w := maxInt(22, width*26/100)
	if w > 36 {
		w = 36
	}
	return w
}

func (a *App) renderModuleRightRail(width, height int, details, actions []string) string {
	detH := maxInt(6, height*45/100)
	actH := maxInt(5, height-detH)
	return lipgloss.JoinVertical(lipgloss.Left,
		renderApiTitledBox("DETALHES", fitExactLines(details, detH-2), width, detH, false),
		renderApiTitledBox("AÇÕES", fitExactLines(actions, actH-2), width, actH, false),
	)
}

func moduleActionLines(items ...[2]string) []string {
	return moduleActionLinesWidth(0, items...)
}

// moduleActionLinesWidth alinha tecla + descrição por largura visual (runes
// tipo ↑↓ / ←→ não quebram a coluna). Se innerW > 0, corta só a descrição.
func moduleActionLinesWidth(innerW int, items ...[2]string) []string {
	keyW := 0
	for _, it := range items {
		if w := runewidth.StringWidth(it[0]); w > keyW {
			keyW = w
		}
	}
	if keyW < 4 {
		keyW = 4
	}
	if keyW > 8 {
		keyW = 8
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		key := it[0]
		if runewidth.StringWidth(key) > keyW {
			key = runewidth.Truncate(key, keyW, "")
		}
		pad := keyW - runewidth.StringWidth(key)
		desc := strings.TrimSpace(it[1])
		if innerW > 0 {
			maxDesc := innerW - keyW - 1
			if maxDesc < 1 {
				maxDesc = 1
			}
			if runewidth.StringWidth(desc) > maxDesc {
				desc = runewidth.Truncate(desc, maxDesc, "…")
			}
		}
		out = append(out, StyleKey.Render(key)+strings.Repeat(" ", pad+1)+StyleMuted.Render(desc))
	}
	return out
}

// renderActionsBox is the compact titled commands panel used across screens.
// Empty when width is 0 (narrow pane) so callers can JoinHorizontal safely.
// height is a ceiling — the box shrinks to the number of actions (no torre vazia).
func renderActionsBox(width, height int, items ...[2]string) string {
	if width < 12 {
		return ""
	}
	innerW := maxInt(4, width-2)
	lines := moduleActionLinesWidth(innerW, items...)
	need := len(lines) + 2
	if height < 3 {
		height = 3
	}
	if need < height {
		height = need
	}
	return renderApiTitledBox("AÇÕES", fitExactLines(lines, height-2), width, height, false)
}

// actionsCmdWidth is the column reserved for AÇÕES next to main content.
// Returns 0 when the pane is too narrow — callers should omit the AÇÕES column.
func actionsCmdWidth(total int) int {
	if total < 64 {
		return 0
	}
	w := 20
	if total >= 110 {
		w = 22
	}
	if total >= 150 {
		w = 24
	}
	if w > total/4 {
		w = maxInt(18, total/4)
	}
	return w
}

// screenHeight is the usable height for full-screen module views (ngrok, cf, …).
// Soft floor only — never invent rows taller than the real terminal.
func (a *App) screenHeight() int {
	if a.height <= 0 {
		return 18
	}
	return maxInt(6, a.height-2)
}

// screenWidth soft-floors module width for narrow VS Code panes.
func (a *App) screenWidth() int {
	if a.width <= 0 {
		return 72
	}
	return maxInt(40, a.width)
}

func moduleOpenHint() []string {
	return []string{
		StyleNormal.Render("pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para entrar"),
		StyleMuted.Render("esc no cliente volta para esta aba"),
	}
}
