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

// shortLabel retorna um nome curto para uso na barra de abas compacta.
func (t containerDetailTab) shortLabel() string {
	switch t {
	case containerDetailTabLogs:
		return "Logs"
	case containerDetailTabStats:
		return "Stats"
	case containerDetailTabEnv:
		return "Env"
	case containerDetailTabConfig:
		return "Config"
	case containerDetailTabTop:
		return "Top"
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

	viewport := a.containerDetailViewport()

	// Constrói o conteúdo sempre com exatamente viewport linhas.
	// Os indicadores de scroll CONSOMEM do orçamento em vez de serem adicionados por cima.
	var contentLines []string
	if a.containerDetailLoading {
		contentLines = append(contentLines, StyleMuted.Render("Carregando..."))
	} else {
		allLines := a.containerDetailLines()
		a.containerDetailScroll = clampScroll(a.containerDetailScroll, viewport, len(allLines))
		start := a.containerDetailScroll
		available := viewport // linhas disponíveis para usar

		// Reserva 1 linha para indicador de scroll-up, se necessário
		if start > 0 {
			contentLines = append(contentLines, StyleMuted.Render(fmt.Sprintf("  ↑ %d acima", start)))
			available--
		}

		// Verifica se haverá linhas abaixo e reserva 1 linha para o indicador
		remaining := len(allLines) - start
		showScrollDown := remaining > available
		if showScrollDown {
			available-- // reserva 1 linha para o indicador de scroll-down
		}

		end := minInt(start+available, len(allLines))
		for i := start; i < end; i++ {
			line := allLines[i]
			if line == "" {
				line = " "
			}
			contentLines = append(contentLines, a.renderContainerDetailLine(a.containerDetailTab, line))
		}

		if showScrollDown {
			contentLines = append(contentLines, StyleMuted.Render(fmt.Sprintf("  ↓ %d abaixo", len(allLines)-end)))
		}
	}

	// Garante exatamente viewport linhas — preenche se curto, corta se longo (segurança)
	for len(contentLines) < viewport {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > viewport {
		contentLines = contentLines[:viewport]
	}

	// Mostra [X/total] no cabeçalho para orientação rápida
	tabCounter := StyleMuted.Render(fmt.Sprintf("[%d/%d]", int(a.containerDetailTab)+1, containerDetailTabTotal))

	lines := []string{
		StyleSection.Render(name) + "  " + tabCounter,
		a.renderContainerDetailTabBar(),
		"",
		strings.Join(contentLines, "\n"),
		"",
		StyleMuted.Render("←→ tabs  ↑↓ scroll  esc voltar"),
	}

	return StylePanel.Render(strings.Join(lines, "\n"))
}

func (a *App) renderContainerDetailTabBar() string {
	var parts []string
	for i := 0; i < containerDetailTabTotal; i++ {
		tab := containerDetailTab(i)
		label := tab.shortLabel()
		if tab == a.containerDetailTab {
			parts = append(parts, StyleTabActive.Render("▶ "+label))
		} else {
			parts = append(parts, StyleMuted.Render(label))
		}
	}
	return strings.Join(parts, "  │  ")
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
	// Panel chrome: name(1) + tabbar(1) + blank(1) + blank(1) + hints(1) = 5 lines
	v := a.contentPanelHeight() - 5
	if v < 8 {
		return 8
	}
	return v
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
