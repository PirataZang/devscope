package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type projectLogsLoadedMsg struct {
	source string
	logs   string
	err    error
}

func (a *App) renderLogsTab(p *core.Project) string {
	w, h := a.moduleSize()
	status := "idle"
	if a.projectLogsLoading {
		status = a.spinner() + " carregando…"
	} else if a.projectLogsFollow {
		if a.projectLogsPaused {
			status = "follow PAUSED"
		} else {
			status = "follow ON"
		}
	} else if a.projectLogSource != "" {
		status = a.projectLogSource
	}
	ctx := a.renderModuleContext(p, w, "Logs", status)

	// Padrão de tela lista+detalhe (docs/DESIGN.md §1.3): os fatos ficam sem
	// moldura e só o conteúdo que rola ganha caixa. Era um trilho vertical de
	// DETALHES + AÇÕES roubando 36 colunas do log.
	head := []string{
		ctx,
		factLine("origem", []string{
			StyleNormal.Render(firstNonEmpty(a.projectLogSource, emDash)),
			StyleMuted.Render(truncate(a.projectLogContainerID, 20)),
		}, w),
		factLine("follow", []string{
			followLabel(a.projectLogsFollow, a.projectLogsPaused),
			StyleMuted.Render(fmt.Sprintf("%d containers", p.ContainerCount)),
			StyleMuted.Render("compose " + boolLabel(p.HasDockerCompose)),
		}, w),
	}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"f", "follow"},
		[2]string{"p", "pausar"},
		[2]string{"r", "recarregar"},
		[2]string{"↑↓", "rolar"},
		[2]string{"3", "containers"},
		[2]string{"esc", "voltar"},
	))
	bodyH := maxInt(5, h-len(head)-lipgloss.Height(cmdBar))
	body := a.renderLogsBodyBox(p, w, bodyH)
	return lipgloss.JoinVertical(lipgloss.Left, append(head, body, cmdBar)...)
}

func followLabel(on, paused bool) string {
	if !on {
		return StyleMuted.Render("off")
	}
	if paused {
		return StyleWarning.Render("paused")
	}
	return StyleHealthy.Render("on")
}

func boolLabel(v bool) string {
	if v {
		return "sim"
	}
	return "não"
}

func (a *App) renderLogsBodyBox(p *core.Project, width, height int) string {
	innerW := maxInt(20, width-2)
	var content string
	title := "LOGS"
	switch {
	case a.projectLogsLoading:
		content = "Carregando logs..."
	case a.projectLogsFollow:
		hint := "follow ON — p pause  f stop follow"
		if a.projectLogsPaused {
			hint = "follow PAUSED — p resume"
		}
		body := a.projectLogs
		if body == "" {
			body = "(aguardando logs...)"
		}
		content = hint + "\n\n" + body
		title = "LOGS · FOLLOW"
	case a.projectLogs == "":
		hint := "Carregando logs..."
		if p.ContainerCount == 0 && !p.HasDockerCompose {
			hint = "Nenhum container ou docker-compose detectado neste projeto"
		}
		content = hint + "\n\nf follow  r refresh  esc voltar"
	default:
		if a.projectLogSource != "" {
			content = "SOURCE: " + a.projectLogSource + "\n\n" + a.projectLogs
		} else {
			content = a.projectLogs
		}
	}

	raw := wrapText(content, innerW)
	// Keep tail visible in follow mode; otherwise respect projectContentScroll.
	start := 0
	viewH := maxInt(1, height-2)
	if a.projectLogsFollow {
		if len(raw) > viewH {
			start = len(raw) - viewH
		}
	} else {
		maxScroll := maxInt(0, len(raw)-viewH)
		if a.projectContentScroll > maxScroll {
			a.projectContentScroll = maxScroll
		}
		start = a.projectContentScroll
	}
	end := minInt(start+viewH, len(raw))
	lines := make([]string, 0, viewH)
	for _, line := range raw[start:end] {
		lines = append(lines, a.colorLogLine(truncate(sanitizeTerminalLine(line), innerW)))
	}
	return panelBox(title, fitExactLines(lines, viewH), width, height, true)
}

func (a *App) colorLogLine(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
		return StyleUnhealthy.Render(line)
	case strings.Contains(lower, "warn"):
		return StyleWarning.Render(line)
	default:
		return StyleMuted.Render(line)
	}
}

func (a *App) initLogsTab(p *core.Project) tea.Cmd {
	a.projectLogs = ""
	a.projectLogsLoading = true
	a.projectLogsFollow = false
	a.projectLogsPaused = false
	a.projectLogContainerID = ""
	a.projectLogSource = ""
	a.projectContentScroll = 0
	for _, c := range p.Containers {
		if collectors.IsContainerRunning(c) {
			a.projectLogContainerID = c.ID
			break
		}
	}
	return a.loadProjectLogs(p)
}

func (a *App) loadProjectLogs(p *core.Project) tea.Cmd {
	return func() tea.Msg {
		if a.projectLogContainerID != "" {
			logs, err := collectors.DockerLogs(a.projectLogContainerID, 300)
			name := a.projectLogContainerID
			for _, c := range p.Containers {
				if c.ID == a.projectLogContainerID {
					name = c.Name
					break
				}
			}
			return projectLogsLoadedMsg{source: "container:" + name, logs: logs, err: err}
		}
		if p.HasDockerCompose || collectors.ComposeFile(p.Path) != "" {
			logs, err := collectors.ComposeLogs(p.Path, 300)
			return projectLogsLoadedMsg{source: "docker compose", logs: logs, err: err}
		}
		return projectLogsLoadedMsg{err: fmt.Errorf("nenhuma fonte de logs disponível")}
	}
}

func (a *App) handleProjectLogsLoaded(msg projectLogsLoadedMsg) {
	a.projectLogsLoading = false
	a.projectLogSource = msg.source
	if msg.err != nil && msg.logs == "" {
		a.projectLogs = "erro: " + msg.err.Error()
	} else {
		a.projectLogs = msg.logs
	}
}

func (a *App) startProjectLogsFollow() tea.Cmd {
	if a.projectLogContainerID == "" {
		return nil
	}
	a.projectLogsFollow = true
	a.projectLogsPaused = false
	return followProjectLogs(a.projectLogContainerID)
}
