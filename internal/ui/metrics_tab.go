package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderMetricsTab(p *core.Project) string {
	w, h := a.moduleSize()
	cpu, memoryMB := projectRuntimeMetrics(p)
	ctx := a.renderModuleContext(p, w, "Metrics", fmt.Sprintf("CPU %.1f%% · RAM %d MB", cpu, memoryMB))
	bodyH := maxInt(12, h-lipgloss.Height(ctx))

	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	sumH := maxInt(6, bodyH*28/100)
	listH := maxInt(6, bodyH-sumH)
	ctrW := centerW / 2
	wrkW := centerW - ctrW

	center := lipgloss.JoinVertical(lipgloss.Left,
		a.renderMetricsSummaryBox(p, cpu, memoryMB, centerW, sumH),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderMetricsContainersBox(p, ctrW, listH),
			a.renderMetricsWorkersBox(p, wrkW, listH),
		),
	)

	host := a.snapshot.HostMetrics
	details := []string{
		StyleMuted.Render("CPU      ") + StyleMetricCPU.Render(fmt.Sprintf("%.1f%%", cpu)),
		StyleMuted.Render("RAM      ") + StyleMetricRAM.Render(fmt.Sprintf("%d MB", memoryMB)),
		StyleMuted.Render("Ctrs     ") + StyleNormal.Render(fmt.Sprintf("%d", p.ContainerCount)),
		StyleMuted.Render("Workers  ") + StyleNormal.Render(fmt.Sprintf("%d", p.WorkerCount)),
		StyleMuted.Render("Host CPU ") + StyleMuted.Render(fmt.Sprintf("%.0f%%", host.CPUPercent)),
		StyleMuted.Render("Host RAM ") + StyleMuted.Render(fmt.Sprintf("%.0f%%", host.MemoryPercent)),
	}
	actions := moduleActionLines(
		[2]string{"3", "containers"},
		[2]string{"5", "health"},
		[2]string{"6", "logs"},
		[2]string{"1", "visão geral"},
		[2]string{"r", "refresh"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderMetricsSummaryBox(p *core.Project, cpu float64, memoryMB int64, width, height int) string {
	hostRAM := a.snapshot.HostMetrics.MemoryTotalMB
	if hostRAM <= 0 {
		hostRAM = 8192
	}
	ramPct := float64(memoryMB) * 100 / float64(hostRAM)
	lines := []string{
		StyleMuted.Render("CPU ") + meterBar(cpu, 12) + StyleMuted.Render(fmt.Sprintf(" %.1f%%", cpu)),
		StyleMuted.Render("RAM ") + meterBar(ramPct, 12) + StyleMuted.Render(fmt.Sprintf(" %d / %d MB", memoryMB, hostRAM)),
		StyleMuted.Render(fmt.Sprintf("Containers %d   Workers %d", p.ContainerCount, p.WorkerCount)),
	}
	if len(p.Ports) > 0 {
		lines = append(lines, StyleAccent.Render(fmt.Sprintf("Ports  %v", p.Ports)))
	}
	return renderApiTitledBox("SUMMARY", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderMetricsContainersBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	lines = append(lines, StyleMuted.Render(truncate(fmt.Sprintf("%-18s %-8s %6s %7s", "NAME", "STATUS", "CPU", "RAM"), width-2)))
	if len(p.Containers) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum)"))
	} else {
		for _, c := range p.Containers {
			lines = append(lines, StyleNormal.Render(truncate(fmt.Sprintf("%-18s %-8s %5.1f%% %5dM",
				truncate(c.Name, 18), truncate(c.Status, 8), c.CPU, c.Memory/(1024*1024)), width-2)))
		}
	}
	return renderApiTitledBox(fmt.Sprintf("CONTAINERS (%d)", len(p.Containers)), fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderMetricsWorkersBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	lines = append(lines, StyleMuted.Render(truncate(fmt.Sprintf("%-18s %-8s %6s %7s", "NAME", "STATUS", "CPU", "RAM"), width-2)))
	if len(p.Workers) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum)"))
	} else {
		for _, w := range p.Workers {
			st := StyleMuted
			if strings.EqualFold(w.Status, "online") {
				st = StyleHealthy
			}
			lines = append(lines, st.Render(truncate(fmt.Sprintf("%-18s %-8s %5.1f%% %5dM",
				truncate(w.Name, 18), truncate(w.Status, 8), w.CPU, w.Memory/(1024*1024)), width-2)))
		}
	}
	return renderApiTitledBox(fmt.Sprintf("WORKERS (%d)", len(p.Workers)), fitExactLines(lines, height-2), width, height, false)
}

func projectRuntimeMetrics(p *core.Project) (float64, int64) {
	var cpu float64
	var memory int64
	for _, c := range p.Containers {
		cpu += c.CPU
		memory += c.Memory
	}
	for _, w := range p.Workers {
		if strings.EqualFold(w.Status, "online") {
			cpu += w.CPU
			memory += w.Memory
		}
	}
	return cpu, memory / (1024 * 1024)
}
