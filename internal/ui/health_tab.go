package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderHealthTab(p *core.Project) string {
	w, h := a.moduleSize()
	ctx := a.renderModuleContext(p, w, "Health", healthPlain(p.Health))
	bodyH := maxInt(12, h-lipgloss.Height(ctx))

	rightW := a.moduleRightWidth(w)
	centerW := maxInt(36, w-rightW-1)

	topH := maxInt(6, bodyH*40/100)
	midH := maxInt(5, bodyH*30/100)
	botH := maxInt(4, bodyH-topH-midH)

	checksW := maxInt(20, centerW*58/100)
	portsW := centerW - checksW
	ctrW := centerW / 2
	sslW := centerW - ctrW

	center := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderHealthChecksBox(p, checksW, topH),
			a.renderHealthPortsBox(p, portsW, topH),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderHealthContainersBox(p, ctrW, midH),
			a.renderHealthWorkersBox(p, sslW, midH),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.renderHealthSSLBox(p, ctrW, botH),
			a.renderHealthDomainsBox(p, sslW, botH),
		),
	)

	ok, bad, unk := 0, 0, 0
	for _, c := range p.HealthChecks {
		switch c.Status {
		case core.HealthHealthy:
			ok++
		case core.HealthUnhealthy:
			bad++
		default:
			unk++
		}
	}
	details := []string{
		StyleMuted.Render("Overall  ") + healthLabel(p.Health),
		StyleMuted.Render("Checks   ") + StyleNormal.Render(fmt.Sprintf("%d", len(p.HealthChecks))),
		StyleHealthy.Render(fmt.Sprintf("ok %d", ok)) + "  " +
			StyleUnhealthy.Render(fmt.Sprintf("bad %d", bad)) + "  " +
			StyleMuted.Render(fmt.Sprintf("n/a %d", unk)),
		StyleMuted.Render("Ports    ") + StyleNormal.Render(fmt.Sprintf("%d", len(p.Ports))),
		StyleMuted.Render("Ctrs     ") + StyleNormal.Render(fmt.Sprintf("%d", len(p.Containers))),
		StyleMuted.Render("SSL      ") + StyleNormal.Render(fmt.Sprintf("%d", len(p.SSL))),
	}
	actions := moduleActionLines(
		[2]string{"r", "refresh scan"},
		[2]string{"6", "ver logs"},
		[2]string{"3", "containers"},
		[2]string{"o", "abrir browser"},
		[2]string{"1", "visão geral"},
	)
	right := a.renderModuleRightRail(rightW, bodyH, details, actions)
	return lipgloss.JoinVertical(lipgloss.Left, ctx, lipgloss.JoinHorizontal(lipgloss.Top, center, right))
}

func (a *App) renderHealthChecksBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.HealthChecks) == 0 {
		lines = append(lines, StyleMuted.Render("nenhum check HTTP/TCP"), StyleMuted.Render("aguarde scan / portas"))
	} else {
		for _, c := range p.HealthChecks {
			st := StyleHealthy
			if c.Status == core.HealthUnhealthy {
				st = StyleUnhealthy
			} else if c.Status == core.HealthUnknown {
				st = StyleMuted
			}
			msg := ""
			if c.Message != "" {
				msg = " · " + c.Message
			}
			lines = append(lines, st.Render(truncate(fmt.Sprintf("%s  %dms%s", c.URL, c.LatencyMS, msg), width-2)))
		}
	}
	return renderApiTitledBox("CHECKS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderHealthPortsBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.Ports) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhuma)"))
	} else {
		lines = append(lines, StyleAccent.Render(collectors.FormatPortsShort(p.Ports, 10)))
		for _, port := range p.Ports {
			if len(lines) >= height-2 {
				break
			}
			lines = append(lines, StyleMuted.Render(fmt.Sprintf(":%d", port)))
		}
	}
	return renderApiTitledBox("PORTAS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderHealthContainersBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.Containers) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum)"))
	} else {
		for _, c := range p.Containers {
			health := c.Health
			if health == "" {
				health = c.State
			}
			st := StyleMuted
			if strings.EqualFold(c.Status, "running") {
				st = StyleRunning
			}
			if strings.EqualFold(health, "unhealthy") {
				st = StyleUnhealthy
			}
			lines = append(lines, st.Render(truncate(fmt.Sprintf("%s [%s] %s", c.Name, c.Status, health), width-2)))
		}
	}
	return renderApiTitledBox("CONTAINERS", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderHealthWorkersBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.Workers) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum PM2)"))
	} else {
		for _, w := range p.Workers {
			st := StyleRunning
			if !strings.EqualFold(w.Status, "online") {
				st = StyleStopped
			}
			lines = append(lines, st.Render(truncate(fmt.Sprintf("%s [%s] CPU %.1f%%", w.Name, w.Status, w.CPU), width-2)))
		}
	}
	return renderApiTitledBox("PM2", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderHealthSSLBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.SSL) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum cert)"))
	} else {
		for _, s := range p.SSL {
			st := StyleHealthy
			if s.DaysLeft < 7 {
				st = StyleWarning
			}
			if s.DaysLeft < 0 {
				st = StyleUnhealthy
			}
			lines = append(lines, st.Render(truncate(fmt.Sprintf("%s  %dd (%s)", s.Domain, s.DaysLeft, s.Issuer), width-2)))
		}
	}
	return renderApiTitledBox("SSL", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderHealthDomainsBox(p *core.Project, width, height int) string {
	lines := make([]string, 0, height-2)
	if len(p.Domains) == 0 {
		lines = append(lines, StyleMuted.Render("(nenhum)"))
	} else {
		for _, d := range p.Domains {
			ssl := ""
			if d.SSL {
				ssl = " HTTPS"
			}
			proxy := ""
			if d.ProxyTo != "" {
				proxy = " → " + d.ProxyTo
			}
			lines = append(lines, StyleNormal.Render(truncate(d.Host+ssl+proxy, width-2)))
		}
	}
	return renderApiTitledBox("DOMAINS", fitExactLines(lines, height-2), width, height, false)
}

func healthLabel(h core.HealthStatus) string {
	switch h {
	case core.HealthHealthy:
		return StyleHealthy.Render(string(h))
	case core.HealthUnhealthy:
		return StyleUnhealthy.Render(string(h))
	default:
		return StyleMuted.Render(string(h))
	}
}

func healthPlain(h core.HealthStatus) string {
	if h == "" {
		return "Unknown"
	}
	return string(h)
}
