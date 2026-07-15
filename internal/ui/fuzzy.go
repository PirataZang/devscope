package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (a *App) updateFuzzy(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.fuzzyOn = false
		a.fuzzyInput = ""
	case "enter":
		a.fuzzyOn = false
		a.filter = a.fuzzyInput
		a.fuzzyInput = ""
		a.dashboardScroll = 0
		a.cursor = 0
	case "backspace":
		if len(a.fuzzyInput) > 0 {
			a.fuzzyInput = a.fuzzyInput[:len(a.fuzzyInput)-1]
		}
	default:
		if len(msg.String()) == 1 {
			a.fuzzyInput += msg.String()
		}
	}
	return a, nil
}

func (a *App) updateDeployConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		a.deployConfirm = false
		if p := a.currentProject(); p != nil {
			return a, a.runDeploy(p)
		}
	case "esc", "n", "N":
		a.deployConfirm = false
		a.statusMsg = "deploy cancelado"
	}
	return a, nil
}

func (a *App) updateContainerRemoveConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := a.currentProject()
	switch msg.String() {
	case "y", "Y":
		a.containerConfirmRemove = false
		if p != nil {
			if c, ok := a.selectedContainer(p); ok {
				return a, a.containerRemove(c)
			}
		}
	case "esc", "n", "N":
		a.containerConfirmRemove = false
		a.containerStatusMsg = "remove cancelado"
	}
	return a, nil
}

func (a *App) renderFuzzyPrompt() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderHeader(),
		"",
		StylePanel.Render("Fuzzy search: "+a.fuzzyInput+"█"),
		a.renderStatusBar("busca em nome, path, framework, branch | enter confirm | esc cancel"),
	)
}
