package ui

import (
	"fmt"
	"strings"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
	tea "github.com/charmbracelet/bubbletea"
)

type projectLogsLoadedMsg struct {
	source string
	logs   string
	err    error
}

func (a *App) renderLogsTab(p *core.Project) string {
	if a.projectLogsLoading {
		return StylePanel.Render("Carregando logs...")
	}

	if a.projectLogsFollow {
		hint := StyleMuted.Render("follow ON — p pause  f stop follow")
		if a.projectLogsPaused {
			hint = StyleWarning.Render("follow PAUSED — p resume")
		}
		content := a.projectLogs
		if content == "" {
			content = "(aguardando logs...)"
		}
		lines := wrapText(content, maxInt(a.width-10, 40))
		if len(lines) > 40 {
			start := len(lines) - 40
			lines = lines[start:]
		}
		return StylePanel.Render(hint + "\n\n" + strings.Join(lines, "\n"))
	}

	if a.projectLogs == "" {
		hint := "Carregando logs..."
		if p.ContainerCount == 0 && !p.HasDockerCompose {
			hint = "Nenhum container ou docker-compose detectado neste projeto"
		}
		return StylePanel.Render(StyleMuted.Render(hint + "\n\nf follow  r refresh  esc voltar"))
	}

	header := ""
	if a.projectLogSource != "" {
		header = StyleSection.Render("SOURCE: "+a.projectLogSource) + "\n\n"
	}
	lines := wrapText(a.projectLogs, maxInt(a.width-10, 40))
	return StylePanel.Render(header + strings.Join(lines, "\n"))
}

func (a *App) initLogsTab(p *core.Project) tea.Cmd {
	a.projectLogs = ""
	a.projectLogsLoading = true
	a.projectLogsFollow = false
	a.projectLogsPaused = false
	a.projectLogContainerID = ""
	a.projectLogSource = ""
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
