package ui

import (
	"fmt"
	"strings"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) renderHealthTab(p *core.Project) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Overall:  %s", healthLabel(p.Health)), "")

	if len(p.HealthChecks) == 0 {
		lines = append(lines, StyleMuted.Render("  Nenhum check HTTP/TCP ainda — portas ou domínios não detectados"))
	} else {
		lines = append(lines, StyleSection.Render("CHECKS"))
		for _, c := range p.HealthChecks {
			st := StyleHealthy
			if c.Status == core.HealthUnhealthy {
				st = StyleUnhealthy
			} else if c.Status == core.HealthUnknown {
				st = StyleMuted
			}
			line := fmt.Sprintf("  %s  %dms", c.URL, c.LatencyMS)
			if c.Message != "" {
				line += " — " + c.Message
			}
			lines = append(lines, st.Render(line))
		}
	}

	if len(p.Ports) > 0 {
		lines = append(lines, "", StyleSection.Render("PORTAS"))
		lines = append(lines, fmt.Sprintf("  %s", collectors.FormatPortsShort(p.Ports, 8)))
	}

	if len(p.Containers) > 0 {
		lines = append(lines, "", StyleSection.Render("CONTAINERS"))
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
			lines = append(lines, st.Render(fmt.Sprintf("  %s  [%s]  %s", c.Name, c.Status, health)))
		}
	}

	if len(p.Workers) > 0 {
		lines = append(lines, "", StyleSection.Render("PM2"))
		for _, w := range p.Workers {
			st := StyleRunning
			if !strings.EqualFold(w.Status, "online") {
				st = StyleStopped
			}
			lines = append(lines, st.Render(fmt.Sprintf("  %s  [%s]  CPU %.1f%%", w.Name, w.Status, w.CPU)))
		}
	}

	if len(p.SSL) > 0 {
		lines = append(lines, "", StyleSection.Render("SSL"))
		for _, s := range p.SSL {
			st := StyleHealthy
			if s.DaysLeft < 7 {
				st = StyleWarning
			}
			if s.DaysLeft < 0 {
				st = StyleUnhealthy
			}
			lines = append(lines, st.Render(fmt.Sprintf("  %s  expires in %dd (%s)", s.Domain, s.DaysLeft, s.Issuer)))
		}
	}

	if len(p.Domains) > 0 {
		lines = append(lines, "", StyleSection.Render("DOMAINS"))
		for _, d := range p.Domains {
			ssl := ""
			if d.SSL {
				ssl = " (HTTPS)"
			}
			proxy := ""
			if d.ProxyTo != "" {
				proxy = " → " + d.ProxyTo
			}
			lines = append(lines, fmt.Sprintf("  %s%s%s", d.Host, ssl, proxy))
		}
	}

	if len(lines) <= 2 {
		lines = append(lines, StyleMuted.Render("  Aguarde o próximo refresh ou verifique docker-compose / .env (PORT)"))
	}

	return StylePanel.Render(strings.Join(lines, "\n"))
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
