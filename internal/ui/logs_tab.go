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
		status = "carregando…"
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
	bodyH := maxInt(12, h-lipgloss.Height(ctx))

	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	center := a.renderLogsBodyBox(p, centerW, bodyH)
	details := []string{
		StyleMuted.Render("Source   ") + StyleNormal.Render(truncate(a.projectLogSource, rightW-12)),
		StyleMuted.Render("Container") + " " + StyleMuted.Render(truncate(a.projectLogContainerID, rightW-12)),
		StyleMuted.Render("Follow   ") + followLabel(a.projectLogsFollow, a.projectLogsPaused),
		StyleMuted.Render("Ctrs     ") + StyleNormal.Render(fmt.Sprintf("%d", p.ContainerCount)),
		StyleMuted.Render("Compose  ") + StyleNormal.Render(boolLabel(p.HasDockerCompose)),
	}
	actions := moduleActionLines(
		[2]string{"f", "follow (container)"},
		[2]string{"p", "pause/resume"},
		[2]string{"r", "refresh"},
		[2]string{"3", "containers"},
		[2]string{"5", "health"},
		[2]string{"↑↓", "scroll"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
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
		return "yes"
	}
	return "no"
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
	return renderApiTitledBox(title, fitExactLines(lines, viewH), width, height, true)
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
