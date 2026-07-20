package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

// Shared layout chrome for project modules (Overview-style).

func (a *App) moduleSize() (width, height int) {
	return maxInt(60, a.width), maxInt(16, a.projectPanelHeight())
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
		renderApiTitledBox("AÇÕES RÁPIDAS", fitExactLines(actions, actH-2), width, actH, false),
	)
}

func moduleActionLines(items ...[2]string) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, StyleKey.Render(fmt.Sprintf("%-5s", it[0]))+StyleMuted.Render(it[1]))
	}
	return out
}

func moduleOpenHint() []string {
	return []string{
		StyleNormal.Render("pressione ") + StyleKey.Render("enter") + StyleNormal.Render(" para entrar"),
		StyleMuted.Render("esc no cliente volta para esta aba"),
	}
}
