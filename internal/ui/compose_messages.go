package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
)

type composeDoneMsg struct {
	action string
	err    error
}

func (a *App) composeUp(path string) tea.Cmd {
	return a.runCompose(path, "up", collectors.ComposeUp)
}

func (a *App) composeDown(path string) tea.Cmd {
	return a.runCompose(path, "down", collectors.ComposeDown)
}

func (a *App) composeRestart(path string) tea.Cmd {
	return a.runCompose(path, "restart", collectors.ComposeRestart)
}

func (a *App) runCompose(path, action string, fn func(string) error) tea.Cmd {
	return func() tea.Msg {
		err := fn(path)
		collectors.RefreshProjectsDocker(a.store)
		return composeDoneMsg{action: action, err: err}
	}
}

func (a *App) handleComposeDone(msg composeDoneMsg) {
	a.snapshot = a.store.Get()
	if msg.err != nil {
		a.statusMsg = "compose " + msg.action + ": " + msg.err.Error()
	} else {
		a.statusMsg = "compose " + msg.action + " ✓"
	}
}
