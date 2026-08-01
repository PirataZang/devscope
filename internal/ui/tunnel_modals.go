package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func tunnelModalChrome(brand string, brandColor lipgloss.Color, title, subtitle, project string, ruleW int) []string {
	if ruleW < 40 {
		ruleW = 40
	}
	brandS := lipgloss.NewStyle().Bold(true).Foreground(brandColor).Render(brand)
	lines := []string{
		brandS + StyleMuted.Render("  ·  ") + StyleNormal.Render(title),
		StyleMuted.Render(subtitle),
		StyleMuted.Render(strings.Repeat("─", minInt(ruleW, 52))),
	}
	if project != "" {
		lines = append(lines, StyleMuted.Render("projeto  ")+StyleNormal.Render(truncate(project, maxInt(12, ruleW-10))))
	}
	return lines
}

func tunnelModalBox(lines []string, boxW, boxH int, border lipgloss.Color) string {
	// Prefer full content over clipping: small height args (unit tests) still show fields.
	if boxH < len(lines) {
		boxH = len(lines)
	}
	return StylePanel.
		Width(boxW).
		BorderForeground(border).
		Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}

func renderTunnelDeleteConfirmBox(brand string, brandColor lipgloss.Color, target, detail string, width, height int) string {
	boxW := minInt(width-4, maxInt(44, width*50/100))
	boxH := minInt(height-2, maxInt(12, 14))
	innerW := maxInt(28, boxW-6)
	lines := tunnelModalChrome(brand, brandColor, "Excluir túnel", "remove da config do projeto", "", innerW)
	lines = append(lines, "")
	nameBox := renderApiTitledBox("túnel",
		[]string{StyleWarning.Bold(true).Render(truncate(target, innerW-2))},
		innerW, 3, true,
	)
	lines = append(lines, strings.Split(nameBox, "\n")...)
	if detail != "" {
		lines = append(lines, "", StyleMuted.Render(truncate(detail, innerW)))
	}
	lines = append(lines, "",
		StyleMuted.Render("y confirma  ·  n/esc cancela"),
	)
	return tunnelModalBox(lines, boxW, boxH, brandColor)
}

func tunnelStatusBadge(status string) string {
	switch status {
	case "online":
		return StyleHealthy.Render("● online")
	case "starting":
		return StyleWarning.Render("● starting")
	default:
		return StyleUnhealthy.Render("● offline")
	}
}

func tunnelDetailKV(label, value string) string {
	if value == "" {
		value = "—"
	}
	return StyleMuted.Render(fmt.Sprintf("%-10s ", label)) + StyleNormal.Render(value)
}

func tunnelMetricRow(cells [][2]string, width int) string {
	n := len(cells)
	if n == 0 || width < 20 {
		return ""
	}
	gap := 1
	cellW := maxInt(9, (width-(n-1)*gap)/n)
	parts := make([]string, 0, n*2)
	for i, c := range cells {
		if i > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, renderApiTitledBox(c[0],
			[]string{StyleNormal.Bold(true).Render(truncate(c[1], cellW-2))},
			cellW, 3, false,
		))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}
