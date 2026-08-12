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
	store := a.store
	healthCfg := a.cfg.Health
	return func() tea.Msg {
		err := fn(path)
		collectors.RefreshProjectsDocker(store, path, healthCfg)
		return composeDoneMsg{action: action, err: err}
	}
}

func (a *App) noteCompose(msg string) {
	a.statusMsg = msg
	if a.tab == TabContainers {
		a.containerStatusMsg = msg
	}
}

func (a *App) handleComposeDone(msg composeDoneMsg) {
	a.snapshot = a.store.Get()
	text := "compose " + msg.action + " ✓"
	if msg.err != nil {
		text = "compose " + msg.action + ": " + msg.err.Error()
	}
	a.noteCompose(text)
}
