package ui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// DISCO na Visão Geral: quanto do projeto é pasta que ele mesmo gera
// (node_modules, vendor, target, .next…) e que pode ir embora. A regra de quem
// entra mora em collectors.ProjectCleanTargets — num repositório git, só pasta
// que o git ignora.

type diskLoadedMsg struct {
	path    string
	targets []collectors.CleanTarget
}

type diskCleanedMsg struct {
	path  string
	freed int64
	err   error
}

func (a *App) loadDisk(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	if a.diskPath != path {
		a.diskPath, a.diskTargets = path, nil
	}
	a.diskLoading = true
	return func() tea.Msg {
		return diskLoadedMsg{path: path, targets: collectors.ProjectCleanTargets(context.Background(), path)}
	}
}

func (a *App) handleDiskLoaded(msg diskLoadedMsg) {
	if msg.path != a.diskPath {
		return
	}
	a.diskLoading = false
	a.diskTargets = msg.targets
}

func (a *App) handleDiskCleaned(msg diskCleanedMsg) tea.Cmd {
	a.diskStatus = "liberados " + formatBytes(msg.freed)
	if msg.err != nil {
		a.diskStatus = "erro: " + lastLine(msg.err.Error())
	}
	if p := a.currentProject(); p != nil && pathsMatch(p.Path, msg.path) {
		return a.loadDisk(p)
	}
	return nil
}

// handleOverviewKeys: as teclas da Visão Geral que não são de navegação.
func (a *App) handleOverviewKeys(msg tea.KeyMsg, p *core.Project) (tea.Cmd, bool) {
	switch msg.String() {
	case "x":
		if a.diskLoading || len(a.diskTargets) == 0 {
			a.diskStatus = "nada para limpar"
			return nil, true
		}
		a.gitConfirmOn = true
		a.gitConfirmAction = "disk-clean"
		a.gitConfirmBranch = p.Path
		return nil, true
	case "r":
		a.snapshot = a.store.Get()
		a.diskStatus = ""
		return a.loadDisk(p), true
	}
	return nil, false
}

func (a *App) cleanDisk(p *core.Project) tea.Cmd {
	path, targets := p.Path, a.diskTargets
	a.diskStatus = "apagando…"
	a.diskLoading = true
	return func() tea.Msg {
		var freed int64
		for _, t := range targets {
			freed += t.Size
		}
		return diskCleanedMsg{path: path, freed: freed, err: collectors.RemoveCleanTargets(path, targets)}
	}
}

func (a *App) diskTotal() int64 {
	var total int64
	for _, t := range a.diskTargets {
		total += t.Size
	}
	return total
}

// overviewDiskLines: uma linha por pasta, maior primeiro, com a tecla no fim.
func (a *App) overviewDiskLines(width int) []string {
	if a.diskLoading && len(a.diskTargets) == 0 {
		return []string{a.loadingText("medindo pastas…")}
	}
	var lines []string
	if a.diskStatus != "" {
		lines = append(lines, StyleMuted.Render(truncate(a.diskStatus, width)))
	}
	if len(a.diskTargets) == 0 {
		return append(lines, StyleHealthy.Render("nada gerado para limpar"))
	}
	nameW := 0
	for _, t := range a.diskTargets {
		nameW = maxInt(nameW, len(t.Name))
	}
	nameW = minInt(nameW, maxInt(8, width-10))
	for _, t := range a.diskTargets {
		lines = append(lines, StyleNormal.Render(padRight(truncate(t.Name, nameW), nameW))+"  "+
			diskSizeStyle(t.Size).Render(padLeft(formatBytes(t.Size), 8)))
	}
	return append(lines, keyHint("x", "limpar tudo (confirma)"))
}

func (a *App) diskTitle() string {
	if len(a.diskTargets) == 0 {
		return "DISCO"
	}
	return panelTitle("DISCO", formatBytes(a.diskTotal()))
}

// diskSizeStyle: o tamanho pinta como alerta só quando pesa de verdade.
func diskSizeStyle(n int64) lipgloss.Style {
	switch {
	case n >= 1<<30:
		return StyleUnhealthy
	case n >= 200<<20:
		return StyleWarning
	}
	return StyleMuted
}

func (a *App) diskConfirmOpts() deleteConfirmOpts {
	names := make([]string, 0, len(a.diskTargets))
	hints := []string{}
	seen := map[string]bool{}
	for _, t := range a.diskTargets {
		names = append(names, t.Name)
		if !seen[t.Hint] {
			seen[t.Hint] = true
			hints = append(hints, t.Hint)
		}
	}
	return deleteConfirmOpts{
		Brand:    "DISCO",
		Color:    tabAccentColor(TabOverview),
		Title:    "Limpar " + formatBytes(a.diskTotal()),
		Subtitle: "pastas que o projeto gera — o comando abaixo recria",
		Label:    "apaga",
		Target:   strings.Join(names, " · "),
		Detail:   "volta com: " + strings.Join(hints, " · "),
	}
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%d MB", n>>20)
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}
