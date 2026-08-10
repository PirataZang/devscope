package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func (a *App) openContainerPorts(c core.Container) tea.Cmd {
	a.containerSubview = containerSubviewPorts
	a.resetContainerPortsView()
	a.containerPreviewID = c.ID
	ports := collectors.ParseContainerPortMappings(c.Ports)
	if len(ports) == 1 {
		return a.loadContainerPortPreview(ports[0].HostPort)
	}
	return nil
}

func (a *App) selectedContainerPort(p *core.Project) (collectors.PortMapping, bool) {
	c, ok := a.selectedContainer(p)
	if !ok {
		return collectors.PortMapping{}, false
	}
	ports := collectors.ParseContainerPortMappings(c.Ports)
	if len(ports) == 0 {
		return collectors.PortMapping{}, false
	}
	a.containerPortCursor = clampCursor(a.containerPortCursor, len(ports))
	return ports[a.containerPortCursor], true
}

func (a *App) renderContainerPorts(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())
	c, ok := a.selectedContainer(p)
	if !ok {
		return renderApiTitledBox("PORTAS", fitExactLines([]string{StyleMuted.Render("container não encontrado")}, h-2), w, h, true)
	}
	ports := collectors.ParseContainerPortMappings(c.Ports)
	listTitle := fmt.Sprintf("PORTAS · %s", truncate(sanitizeTerminalLine(c.Name), 24))

	listW := maxInt(18, w*36/100)
	prevW := maxInt(20, w-listW)
	innerH := maxInt(4, h-2)

	listLines := make([]string, 0, innerH)
	if len(ports) == 0 {
		listLines = append(listLines, StyleMuted.Render("(nenhuma porta publicada)"))
	} else {
		a.containerPortCursor = clampCursor(a.containerPortCursor, len(ports))
		for i, port := range ports {
			label := fmt.Sprintf(":%d → %d/%s", port.HostPort, port.ContainerPort, port.Proto)
			if port.HostIP != "" && port.HostIP != "0.0.0.0" {
				label = port.HostIP + label
			}
			st := StyleNormal
			prefix := "  "
			if i == a.containerPortCursor {
				st = StyleSelected
				prefix = "▶ "
			}
			listLines = append(listLines, st.Render(truncate(prefix+label, maxInt(1, listW-4))))
		}
	}
	listLines = append(listLines, "",
		StyleKey.Render("enter")+StyleMuted.Render(" preview"),
		StyleKey.Render("o")+StyleMuted.Render(" browser"),
		StyleKey.Render("x")+StyleMuted.Render(" fechar porta"),
		StyleKey.Render("m")+StyleMuted.Render(" logs/stats/env"),
		StyleKey.Render("esc")+StyleMuted.Render(" voltar"),
	)

	prevLines := a.containerPortPreviewLines(maxInt(1, prevW-4), innerH)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderApiTitledBox(listTitle, fitExactLines(listLines, innerH), listW, h, true),
		renderApiTitledBox(a.containerPortPreviewTitle(), fitExactLines(prevLines, innerH), prevW, h, true),
	)
}

func (a *App) containerPortPreviewTitle() string {
	if a.containerPortLoading {
		return "PREVIEW · …"
	}
	if a.containerPortPreviewPort > 0 {
		return fmt.Sprintf("PREVIEW · :%d", a.containerPortPreviewPort)
	}
	return "PREVIEW"
}

func (a *App) containerPortPreviewLines(width, maxLines int) []string {
	if a.containerPortLoading {
		return []string{StyleMuted.Render(a.loadingText("carregando…"))}
	}
	if strings.TrimSpace(a.containerPortPreview) == "" {
		return []string{
			StyleMuted.Render("enter · abrir telinha HTTP"),
			StyleMuted.Render("o · abrir no browser"),
			StyleMuted.Render("x · fechar porta no docker"),
		}
	}
	raw := strings.Split(strings.TrimRight(a.containerPortPreview, "\n"), "\n")
	lines := make([]string, 0, maxLines)
	for _, line := range raw {
		if len(lines) >= maxLines {
			break
		}
		line = sanitizeTerminalLine(line)
		st := StyleNormal
		low := strings.ToLower(line)
		switch {
		case strings.HasPrefix(low, "http 2"), strings.HasPrefix(low, "http 3"):
			st = StyleHealthy
		case strings.HasPrefix(low, "http 4"), strings.HasPrefix(low, "http 5"):
			st = StyleUnhealthy
		case strings.HasPrefix(low, "get "):
			st = StyleAccent
		}
		lines = append(lines, st.Render(truncate(line, width)))
	}
	return lines
}

func (a *App) handleContainerPortsKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.containerConfirmClosePort {
		return a, nil
	}
	ports := []collectors.PortMapping{}
	if c, ok := a.selectedContainer(p); ok {
		ports = collectors.ParseContainerPortMappings(c.Ports)
	}
	switch msg.String() {
	case "esc":
		if a.containerPortPreview != "" || a.containerPortLoading {
			a.containerPortPreview = ""
			a.containerPortPreviewPort = 0
			a.containerPortLoading = false
			a.containerPortGen++
			return a, nil
		}
		a.containerSubview = containerSubviewList
		a.resetContainerPortsView()
		return a, a.requestContainerPreview()
	case "up", "k":
		if len(ports) > 0 {
			a.containerPortCursor = clampCursor(a.containerPortCursor-1, len(ports))
		}
		return a, nil
	case "down", "j":
		if len(ports) > 0 {
			a.containerPortCursor = clampCursor(a.containerPortCursor+1, len(ports))
		}
		return a, nil
	case "enter":
		port, ok := a.selectedContainerPort(p)
		if !ok {
			a.containerStatusMsg = "nenhuma porta publicada"
			return a, nil
		}
		return a, a.loadContainerPortPreview(port.HostPort)
	case "o", "O":
		port, ok := a.selectedContainerPort(p)
		if !ok {
			a.containerStatusMsg = "nenhuma porta publicada"
			return a, nil
		}
		url := fmt.Sprintf("http://127.0.0.1:%d/", port.HostPort)
		openBrowser(url)
		a.containerStatusMsg = "browser · " + url
		return a, nil
	case "x":
		if _, ok := a.selectedContainerPort(p); ok {
			a.containerConfirmClosePort = true
			a.containerStatusMsg = "fechar porta  y confirma  n/esc cancela"
		}
		return a, nil
	case "m":
		if c, ok := a.selectedContainer(p); ok {
			a.resetContainerPortsView()
			return a, a.openContainerDetail(c, p.Path)
		}
	}
	return a, nil
}

func (a *App) loadContainerPortPreview(port int) tea.Cmd {
	a.containerPortLoading = true
	a.containerPortPreviewPort = port
	a.containerPortPreview = ""
	a.containerPortGen++
	gen := a.containerPortGen
	return func() tea.Msg {
		return containerPortPreviewMsg{port: port, gen: gen, body: collectors.ProbePortPreview(port)}
	}
}

func (a *App) handleContainerPortPreview(msg containerPortPreviewMsg) {
	if msg.gen != a.containerPortGen {
		return
	}
	a.containerPortLoading = false
	a.containerPortPreviewPort = msg.port
	a.containerPortPreview = msg.body
}

func (a *App) containerClosePort(c core.Container, hostPort int) tea.Cmd {
	if !a.beginContainerAction("close-port", c) {
		return nil
	}
	path := a.containerActionProjectPath(c)
	store := a.store
	healthCfg := a.cfg.Health
	target := collectors.DockerExecTarget(c)
	return func() tea.Msg {
		err := collectors.DockerCloseHostPort(target, hostPort)
		collectors.RefreshProjectDocker(store, path, healthCfg)
		return containerActionDoneMsg{action: "close-port", name: c.Name, err: err}
	}
}
