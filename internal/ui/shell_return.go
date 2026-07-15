package ui

import (
	"github.com/charmbracelet/lipgloss"
)

func renderShellReturnMessage(errMsg string) string {
	msg := "Pressione enter para retornar ao DevScope"
	if errMsg != "" {
		msg = errMsg + "\n\n" + msg
	}
	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		StyleHealthy.Render(msg),
		"",
	)
}

func (a *App) renderFullShellReturn(errMsg string) string {
	return "\n\n" + renderShellReturnMessage(errMsg)
}
