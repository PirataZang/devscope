package ui

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type containerDetailLoadedMsg struct {
	tab     containerDetailTab
	id      string
	content string
	err     error
}

type containerActionDoneMsg struct {
	action string
	name   string
	err    error
}

type projectLogFollowMsg struct {
	id   string
	logs string
}

type containerShellDoneMsg struct {
	err error
}

type containerShellFallbackMsg struct {
	container core.Container
	step      int
}

func loadContainerDetailTab(tab containerDetailTab, c core.Container, projectPath string) tea.Cmd {
	return func() tea.Msg {
		target := collectors.DockerExecTarget(c)
		var content string
		var err error
		switch tab {
		case containerDetailTabLogs:
			content, err = collectors.DockerLogs(c.ID, 400)
		case containerDetailTabStats:
			content, err = collectors.DockerContainerStats(target)
		case containerDetailTabEnv:
			content, err = collectors.DockerContainerEnv(target)
		case containerDetailTabConfig:
			content, err = collectors.DockerContainerConfig(target)
		case containerDetailTabTop:
			content, err = collectors.DockerContainerTop(target)
		case containerDetailTabCompose:
			var path string
			path, content, err = collectors.ReadComposeForContainer(target, projectPath)
			if err == nil && path != "" {
				content = "# " + path + "\n\n" + content
			}
		case containerDetailTabFile:
			var path string
			path, content, err = collectors.DockerfileForContainer(target, projectPath)
			if err == nil && path != "" {
				content = "# " + path + "\n\n" + content
			}
		}
		if err != nil && content == "" {
			content = "erro: " + err.Error()
		}
		return containerDetailLoadedMsg{tab: tab, id: c.ID, content: content, err: err}
	}
}

func (a *App) loadContainerDetailTab() tea.Cmd {
	if a.containerDetailCache == nil {
		a.containerDetailCache = make(map[containerDetailTab]string)
	}
	if content, ok := a.containerDetailCache[a.containerDetailTab]; ok {
		a.containerDetailContent = content
		a.containerDetailLoading = false
		return nil
	}
	a.containerDetailLoading = true
	a.containerDetailContent = ""
	tab := a.containerDetailTab
	c := core.Container{ID: a.containerDetailID, Name: a.containerDetailName}
	return loadContainerDetailTab(tab, c, a.containerDetailProjectPath)
}

func (a *App) handleContainerDetailLoaded(msg containerDetailLoadedMsg) {
	if msg.id != a.containerDetailID || msg.tab != a.containerDetailTab {
		return
	}
	a.containerDetailContent = msg.content
	a.containerDetailLoading = false
	if a.containerDetailCache == nil {
		a.containerDetailCache = make(map[containerDetailTab]string)
	}
	a.containerDetailCache[msg.tab] = msg.content
}

func followProjectLogs(id string) tea.Cmd {
	return func() tea.Msg {
		logs, _ := collectors.DockerLogsSince(id, 2, 80)
		return projectLogFollowMsg{id: id, logs: logs}
	}
}

func (a *App) beginContainerAction(kind string, c core.Container) bool {
	if a.containerActions == nil {
		a.containerActions = make(map[string]string)
	}
	if _, busy := a.containerActions[c.Name]; busy {
		return false
	}
	a.containerActions[c.Name] = kind
	a.updateContainerStatusSummary()
	return true
}

func (a *App) endContainerAction(name string) {
	if a.containerActions == nil {
		return
	}
	delete(a.containerActions, name)
	if len(a.containerActions) > 0 {
		a.updateContainerStatusSummary()
	}
}

func (a *App) containerActionKind(name string) string {
	if a.containerActions == nil {
		return ""
	}
	return a.containerActions[name]
}

func containerActionPrefix(kind string) string {
	switch kind {
	case "stop":
		return "◌ parando"
	case "start":
		return "▶ iniciando"
	case "restart":
		return "⟳ reiniciando"
	case "pause":
		return "⏸ pausando"
	case "unpause":
		return "▶ retomando"
	default:
		return kind
	}
}

func (a *App) updateContainerStatusSummary() {
	if len(a.containerActions) == 0 {
		return
	}
	order := []string{"stop", "start", "restart", "pause", "unpause"}
	byKind := make(map[string][]string)
	for name, kind := range a.containerActions {
		byKind[kind] = append(byKind[kind], name)
	}
	var parts []string
	for _, kind := range order {
		names := byKind[kind]
		if len(names) == 0 {
			continue
		}
		prefix := containerActionPrefix(kind)
		if len(names) == 1 {
			parts = append(parts, prefix+" "+names[0]+"...")
		} else {
			parts = append(parts, fmt.Sprintf("%s %d containers...", prefix, len(names)))
		}
	}
	a.containerStatusMsg = strings.Join(parts, "  ·  ")
}

func (a *App) containerPause(c core.Container) tea.Cmd {
	action := "pause"
	run := collectors.DockerPause
	if strings.EqualFold(c.Status, "paused") {
		action = "unpause"
		run = collectors.DockerUnpause
	}
	if !a.beginContainerAction(action, c) {
		return nil
	}
	return func() tea.Msg {
		err := run(collectors.DockerExecTarget(c))
		collectors.RefreshProjectsDocker(a.store)
		return containerActionDoneMsg{action: action, name: c.Name, err: err}
	}
}

func (a *App) containerStart(c core.Container) tea.Cmd {
	if !a.beginContainerAction("start", c) {
		return nil
	}
	return func() tea.Msg {
		err := collectors.DockerStart(collectors.DockerExecTarget(c))
		collectors.RefreshProjectsDocker(a.store)
		return containerActionDoneMsg{action: "start", name: c.Name, err: err}
	}
}

func (a *App) containerStop(c core.Container) tea.Cmd {
	if !a.beginContainerAction("stop", c) {
		return nil
	}
	return func() tea.Msg {
		err := collectors.DockerStop(collectors.DockerExecTarget(c))
		collectors.RefreshProjectsDocker(a.store)
		return containerActionDoneMsg{action: "stop", name: c.Name, err: err}
	}
}

func (a *App) containerRestart(c core.Container) tea.Cmd {
	if !a.beginContainerAction("restart", c) {
		return nil
	}
	return func() tea.Msg {
		err := collectors.DockerRestart(collectors.DockerExecTarget(c))
		collectors.RefreshProjectsDocker(a.store)
		return containerActionDoneMsg{action: "restart", name: c.Name, err: err}
	}
}

func (a *App) containerStartOrRestart(c core.Container) tea.Cmd {
	if collectors.IsContainerStopped(c) {
		return a.containerStart(c)
	}
	return a.containerRestart(c)
}

func (a *App) containerRemove(c core.Container) tea.Cmd {
	return func() tea.Msg {
		err := collectors.DockerRemove(collectors.DockerExecTarget(c))
		collectors.RefreshProjectsDocker(a.store)
		return containerActionDoneMsg{action: "remove", name: c.Name, err: err}
	}
}

func (a *App) containerExecShell(c core.Container) tea.Cmd {
	if !collectors.IsContainerRunning(c) {
		return func() tea.Msg {
			return containerShellDoneMsg{err: errContainerNotRunning}
		}
	}
	target := collectors.DockerExecTarget(c)
	cmd := collectors.DockerExecShell(target)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return containerShellFallbackMsg{container: c, step: 0}
		}
		return containerShellDoneMsg{}
	})
}

func (a *App) containerExecShellFallback(msg containerShellFallbackMsg) tea.Cmd {
	target := collectors.DockerExecTarget(msg.container)
	var cmd *exec.Cmd
	switch msg.step {
	case 0:
		cmd = collectors.DockerExecShellBash(target)
	default:
		cmd = collectors.DockerExecShellFallback(target)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil && msg.step < 1 {
			return containerShellFallbackMsg{container: msg.container, step: 1}
		}
		return containerShellDoneMsg{err: err}
	})
}

func (a *App) handleContainerActionDone(msg containerActionDoneMsg) {
	a.endContainerAction(msg.name)
	a.snapshot = a.store.Get()
	if msg.err != nil {
		a.containerStatusMsg = msg.action + " " + msg.name + ": " + msg.err.Error()
	} else if len(a.containerActions) == 0 {
		switch msg.action {
		case "stop":
			a.containerStatusMsg = "◌ " + msg.name + " parado ✓"
		case "start":
			a.containerStatusMsg = "▶ " + msg.name + " iniciado ✓"
		case "restart":
			a.containerStatusMsg = "⟳ " + msg.name + " reiniciado ✓"
		default:
			a.containerStatusMsg = msg.action + " " + msg.name + " ✓"
		}
	}
	containers := a.currentProjectContainers()
	if len(containers) == 0 {
		a.tabCursor = 0
		a.containerScroll = 0
		return
	}
	a.tabCursor = clampCursor(a.tabCursor, len(containers))
	a.syncContainerScroll(len(containers))
}

func (a *App) handleContainerShellDone(msg containerShellDoneMsg) {
	a.containerSubview = containerSubviewShellReturn
	if msg.err != nil {
		a.containerShellExitErr = "shell: " + msg.err.Error()
	} else {
		a.containerShellExitErr = ""
	}
}

func (a *App) openContainerDetail(c core.Container, projectPath string) tea.Cmd {
	a.containerSubview = containerSubviewDetail
	a.containerDetailTab = containerDetailTabLogs
	a.containerDetailID = c.ID
	a.containerDetailName = c.Name
	a.containerDetailProjectPath = projectPath
	a.containerDetailScroll = 0
	a.containerDetailContent = ""
	a.containerDetailCache = nil
	a.containerDetailLoading = true
	return a.loadContainerDetailTab()
}

func (a *App) currentProjectContainers() []core.Container {
	p := a.currentProject()
	if p == nil {
		return nil
	}
	return p.Containers
}

var errContainerNotRunning = errString("container não está running")

type errString string

func (e errString) Error() string { return string(e) }
