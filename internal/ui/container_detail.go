package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

type containerDetailTab int

const (
	containerDetailTabLogs containerDetailTab = iota
	containerDetailTabStats
	containerDetailTabEnv
	containerDetailTabConfig
	containerDetailTabTop
	containerDetailTabCompose
	containerDetailTabFile
)

const containerDetailTabTotal = int(containerDetailTabFile) + 1

func (t containerDetailTab) label() string {
	switch t {
	case containerDetailTabLogs:
		return "Registros"
	case containerDetailTabStats:
		return "Estatísticas"
	case containerDetailTabEnv:
		return "Env"
	case containerDetailTabConfig:
		return "Configuração"
	case containerDetailTabTop:
		return "Topo"
	case containerDetailTabCompose:
		return "Compose"
	case containerDetailTabFile:
		return "File"
	default:
		return "?"
	}
}

func (a *App) renderContainerDetail(p *core.Project) string {
	name := a.containerDetailName
	if name == "" {
		if c, ok := a.selectedContainer(p); ok {
			name = c.Name
		}
	}

	lines := []string{
		StyleSection.Render(name) + "  " + StyleMuted.Render("(monitoramento)"),
		a.renderContainerDetailTabBar(),
		"",
	}

	if a.containerDetailLoading {
		lines = append(lines, StyleMuted.Render("Carregando..."))
	} else {
		contentLines := a.containerDetailLines()
		viewport := a.containerDetailViewport()
		a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(contentLines))
		start := a.containerDetailScroll
		end := minInt(start+viewport, len(contentLines))

		if start > 0 {
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
		}
		for i := start; i < end; i++ {
			line := contentLines[i]
			if line == "" {
				line = " "
			}
			lines = append(lines, a.renderContainerDetailLine(a.containerDetailTab, line))
		}
		if end < len(contentLines) {
			lines = append(lines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", len(contentLines)-end)))
		}
	}

	lines = append(lines, "",
		StyleMuted.Render("←→ tabs  ↑↓ scroll  esc voltar"),
	)

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderContainerDetailTabBar() string {
	var parts []string
	for i := 0; i < containerDetailTabTotal; i++ {
		tab := containerDetailTab(i)
		label := tab.label()
		if tab == a.containerDetailTab {
			parts = append(parts, StyleTabActive.Render(label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	return "─ " + strings.Join(parts, " ─ ") + " ─"
}

func (a *App) renderContainerDetailLine(tab containerDetailTab, line string) string {
	text := "  " + truncate(line, maxInt(a.width-6, 40))
	if tab == containerDetailTabEnv {
		if key, val, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			return "  " + StyleWarning.Render(key) + "=" + StyleNormal.Render(val)
		}
	}
	return StyleNormal.Render(text)
}

func (a *App) containerDetailLines() []string {
	content := a.containerDetailContent
	if content == "" {
		return []string{"(vazio)"}
	}
	lines := strings.Split(content, "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{"(vazio)"}
	}
	return lines
}

func (a *App) containerDetailViewport() int {
	return a.gitListViewport()
}

func (a *App) containerDetailSwitchTab(delta int) tea.Cmd {
	n := int(a.containerDetailTab) + delta
	for n < 0 {
		n += containerDetailTabTotal
	}
	a.containerDetailTab = containerDetailTab(n % containerDetailTabTotal)
	a.containerDetailScroll = 0
	return a.loadContainerDetailTab()
}

func (a *App) containerDetailScrollBy(delta int) {
	lines := a.containerDetailLines()
	viewport := a.containerDetailViewport()
	a.containerDetailScroll = clampScroll(a.containerDetailScroll+delta, viewport, len(lines))
}

func clampScroll(scroll, viewport, total int) int {
	maxScroll := total - viewport
	if maxScroll < 0 {
		return 0
	}
	if scroll < 0 {
		return 0
	}
	if scroll > maxScroll {
		return maxScroll
	}
	return scroll
}

func (a *App) handleContainerDetailKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.containerSubview = containerSubviewList
		a.containerDetailCache = nil
		return a, nil
	case "left", "h":
		return a, a.containerDetailSwitchTab(-1)
	case "right", "l":
		return a, a.containerDetailSwitchTab(1)
	case "up", "k":
		a.containerDetailScrollBy(-1)
	case "down", "j":
		a.containerDetailScrollBy(1)
	case "pgup", "shift+up", "shift+k":
		a.containerDetailScrollBy(-a.containerDetailViewport())
	case "pgdown", "shift+down", "shift+j":
		a.containerDetailScrollBy(a.containerDetailViewport())
	}
	return a, nil
}
