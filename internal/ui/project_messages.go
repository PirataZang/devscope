package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
)

type projectShellDoneMsg struct {
	err error
}

func (a *App) projectExecShell(path string) tea.Cmd {
	cmd := collectors.ProjectShell(path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return projectShellDoneMsg{err: err}
	})
}

func (a *App) handleProjectShellDone(msg projectShellDoneMsg) {
	a.dashboardSubview = dashboardSubviewShellReturn
	if msg.err != nil {
		a.projectShellExitErr = "terminal: " + msg.err.Error()
	} else {
		a.projectShellExitErr = ""
	}
}

func (a *App) dismissProjectShellReturn() {
	a.dashboardSubview = dashboardSubviewList
	a.projectShellExitErr = ""
	a.snapshot = a.store.Get()
	projects := filterNestedProjects(sortProjects(a.filteredProjects()))
	a.syncDashboardScroll(len(projects))
}
