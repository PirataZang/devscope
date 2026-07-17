package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

type containerSubview int

const (
	containerSubviewList containerSubview = iota
	containerSubviewDetail
	containerSubviewShellReturn
)

func (a *App) initContainersTab() {
	a.containerSubview = containerSubviewList
	a.containerScroll = 0
	a.tabCursor = 0
	a.containerDetailCache = nil
	a.containerDetailScroll = 0
	a.containerDetailContent = ""
	a.containerDetailLoading = false
	a.containerStatusMsg = ""
	a.containerActions = nil
}

func (a *App) renderContainersTab(p *core.Project) string {
	switch a.containerSubview {
	case containerSubviewDetail:
		return a.renderContainerDetail(p)
	case containerSubviewShellReturn:
		return renderShellReturnMessage(a.containerShellExitErr)
	default:
		return a.renderContainerList(p)
	}
}

func (a *App) dismissContainerShellReturn() tea.Cmd {
	a.containerSubview = containerSubviewList
	if a.containerShellExitErr != "" {
		a.containerStatusMsg = a.containerShellExitErr
	}
	a.containerShellExitErr = ""
	containers := a.currentProjectContainers()
	if len(containers) > 0 {
		a.tabCursor = clampCursor(a.tabCursor, len(containers))
		a.syncContainerScroll(len(containers))
	}
	return tea.Batch(
		tea.ClearScreen,
		a.refreshDocker(),
	)
}

func (a *App) renderContainerList(p *core.Project) string {
	if a.projectDockerLoading && len(p.Containers) == 0 {
		title := StyleSection.Render("Containers") + "  " +
			StyleMuted.Render(shortenPath(p.Path))
		return StylePanel.Render(title + "\n\n" + StyleMuted.Render("Carregando containers..."))
	}

	containers := p.Containers
	title := StyleSection.Render("Containers") + "  " +
		StyleMuted.Render(shortenPath(p.Path))

	if len(containers) == 0 {
		return StylePanel.Render(
			title + "\n\n" +
				StyleMuted.Render("Nenhum container vinculado a este projeto.\n"+
					"Vinculamos por docker-compose working_dir, config e volume mounts."),
		)
	}

	viewport := a.containerListViewport()
	start := a.containerScroll
	end := minInt(start+viewport, len(containers))

	running := 0
	for _, c := range containers {
		if c.Status == "running" {
			running++
		}
	}

	lines := []string{
		title,
		StyleMuted.Render(fmt.Sprintf("%d containers  •  %d running", len(containers), running)),
	}

	// Linha de status fixa (1 linha) — evita salto de altura
	if a.containerStatusMsg != "" {
		style := StyleWarning
		if strings.Contains(a.containerStatusMsg, "✓") {
			style = StyleHealthy
		}
		lines = append(lines, style.Render(a.containerStatusMsg))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines,
		a.renderContainerHeader(),
		StyleMuted.Render("  "+strings.Repeat("─", maxInt(a.width-12, 30))),
	)

	if start > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
	}

	for i := start; i < end; i++ {
		c := containers[i]
		line := a.renderContainerRow(c, i == a.tabCursor)
		lines = append(lines, line)
	}

	// Preenche linhas vazias até o viewport para manter altura fixa
	for i := end - start; i < viewport; i++ {
		lines = append(lines, "")
	}

	remaining := len(containers) - end
	if remaining > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", remaining)))
	}

	lines = append(lines, "",
		StyleMuted.Render("  Atalhos: enter/m detalhe │ shift+e shell │ s/r/p container │ d remover │ shift+u/d/R compose"),
	)

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderContainerRow(c core.Container, selected bool) string {
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	cols := a.containerColumns()
	gap := lipgloss.NewStyle().Width(1).Render("")
	cell := func(width int, text string) string {
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	}
	state := a.containerStateCell(c, selected)
	parts := []string{
		lipgloss.NewStyle().Width(2).Render(""),
		state,
		gap,
		cell(cols.name, c.Name),
		gap,
		cell(cols.image, c.Image),
	}
	if cols.ports > 0 {
		parts = append(parts, gap, cell(cols.ports, c.Ports))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

type containerCols struct {
	state, name, image, ports int
}

func (a *App) containerColumns() containerCols {
	tableWidth := maxInt(38, a.width-10)
	cols := containerCols{state: 10}
	flexible := tableWidth - 2 - cols.state - 2
	if a.width < 80 {
		cols.name = flexible * 45 / 100
		cols.image = flexible - cols.name
		return cols
	}
	flexible--
	cols.name = flexible * 30 / 100
	cols.image = flexible * 30 / 100
	cols.ports = flexible - cols.name - cols.image
	return cols
}

func (a *App) renderContainerHeader() string {
	cols := a.containerColumns()
	style := StyleTableHeader
	gap := lipgloss.NewStyle().Width(1).Render("")
	parts := []string{
		lipgloss.NewStyle().Width(2).Render(""),
		style.Width(cols.state).Render("STATE"),
		gap,
		style.Width(cols.name).Render("NAME"),
		gap,
		style.Width(cols.image).Render("IMAGE"),
	}
	if cols.ports > 0 {
		parts = append(parts, gap, style.Width(cols.ports).Render("PORTS"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) containerStateCell(c core.Container, selected bool) string {
	width := a.containerColumns().state
	if kind := a.containerActionKind(c.Name); kind != "" {
		var label string
		switch kind {
		case "stop":
			label = "◌ parando"
		case "start":
			label = "▶ iniciando"
		case "restart":
			label = "⟳ reiniciando"
		case "pause":
			label = "⏸ pausando"
		case "unpause":
			label = "▶ retomando"
		default:
			label = kind
		}
		s := StyleWarning.Bold(true)
		if selected {
			s = StyleWarning.Bold(true).Background(lipgloss.Color("#78350F"))
		}
		return s.Width(width).MaxWidth(width).Render(truncate(label, width))
	}
	if selected {
		return styleSelectedState(c.Status, width)
	}
	return containerStateStyled(c.Status, width)
}

func styleSelectedState(status string, width int) string {
	switch strings.ToLower(status) {
	case "running":
		return StyleSelected.Width(width).MaxWidth(width).Render("RUNNING")
	case "exited", "stopped":
		return StyleSelected.Width(width).MaxWidth(width).Render("EXITED")
	case "paused":
		return StyleSelected.Width(width).MaxWidth(width).Render("PAUSED")
	default:
		return StyleSelected.Width(width).MaxWidth(width).Render(strings.ToUpper(truncate(status, width)))
	}
}

func containerStateStyled(status string, width int) string {
	switch strings.ToLower(status) {
	case "running":
		return StyleRunning.Width(width).Render("running")
	case "exited", "stopped":
		return StyleStopped.Width(width).Render("exited")
	case "paused":
		return StyleWarning.Width(width).Render("paused")
	default:
		return StyleMuted.Width(width).Render(truncate(status, width))
	}
}

func (a *App) containerListViewport() int {
	// chrome: title(1) + count(1) + status-or-blank(1) + header(1) + separator(1) + blank(1) + shortcuts(1) = 7
	v := a.contentPanelHeight() - 7
	if v < 4 {
		return 4
	}
	return v
}

func (a *App) syncContainerScroll(count int) {
	viewport := a.containerListViewport()
	a.containerScroll = ensureVisible(a.tabCursor, a.containerScroll, viewport, count)
}

func (a *App) updateContainerCursor(delta int, p *core.Project) {
	if a.containerSubview == containerSubviewDetail {
		a.containerDetailScrollBy(delta)
		return
	}

	containers := p.Containers
	if len(containers) == 0 {
		return
	}
	a.tabCursor = clampCursor(a.tabCursor+delta, len(containers))
	a.syncContainerScroll(len(containers))
}

func (a *App) selectedContainer(p *core.Project) (core.Container, bool) {
	if p == nil || a.tabCursor >= len(p.Containers) {
		return core.Container{}, false
	}
	return p.Containers[a.tabCursor], true
}

func (a *App) containersCount(p *core.Project) int {
	if p == nil {
		return 0
	}
	return len(p.Containers)
}
