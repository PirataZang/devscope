package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type depNode struct {
	Name     string
	Children []*depNode
}

type depTreeLine struct {
	text string // pre-rendered tree glyphs + connector, service name NOT included
	name string
}

func (a *App) openContainerDeps(p *core.Project) tea.Cmd {
	a.containerSubview = containerSubviewDeps
	a.containerDepsCursor = 0
	a.containerDepsScroll = 0
	a.containerDeps = collectors.ParseComposeDependencies(p.Path)
	return nil
}

// buildDepForest groups services into roots (services nobody else depends on)
// with their dependencies nested underneath — same shape as `npm ls`.
func buildDepForest(deps []collectors.ComposeDependency) []*depNode {
	if len(deps) == 0 {
		return nil
	}
	nodes := make(map[string]*depNode, len(deps))
	for _, d := range deps {
		nodes[d.Service] = &depNode{Name: d.Service}
	}
	isDependency := make(map[string]bool)
	for _, d := range deps {
		for _, dep := range d.DependsOn {
			isDependency[dep] = true
		}
	}
	for _, d := range deps {
		n := nodes[d.Service]
		for _, depName := range d.DependsOn {
			if child, ok := nodes[depName]; ok {
				n.Children = append(n.Children, child)
			} else {
				n.Children = append(n.Children, &depNode{Name: depName})
			}
		}
	}
	var roots []*depNode
	for _, d := range deps {
		if !isDependency[d.Service] {
			roots = append(roots, nodes[d.Service])
		}
	}
	if len(roots) == 0 { // every service is someone's dependency (cyclical / all-referenced) — show them all
		for _, d := range deps {
			roots = append(roots, nodes[d.Service])
		}
	}
	return roots
}

func flattenDepTree(roots []*depNode) []depTreeLine {
	var lines []depTreeLine
	var visit func(n *depNode, prefix string, isLast, isRoot bool, ancestors map[string]bool)
	visit = func(n *depNode, prefix string, isLast, isRoot bool, ancestors map[string]bool) {
		if isRoot {
			lines = append(lines, depTreeLine{text: "", name: n.Name})
		} else {
			connector := "├── "
			if isLast {
				connector = "└── "
			}
			lines = append(lines, depTreeLine{text: prefix + connector, name: n.Name})
		}
		if ancestors[n.Name] {
			return // cycle guard — compose itself forbids this, defensive only
		}
		childAncestors := make(map[string]bool, len(ancestors)+1)
		for k := range ancestors {
			childAncestors[k] = true
		}
		childAncestors[n.Name] = true

		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}
		for i, c := range n.Children {
			visit(c, childPrefix, i == len(n.Children)-1, false, childAncestors)
		}
	}
	for i, r := range roots {
		visit(r, "", i == len(roots)-1, true, map[string]bool{})
	}
	return lines
}

// depServiceStatus best-effort matches a compose service name to a running
// container — compose names containers "{project}-{service}-{n}" by default,
// but this stays a loose substring match since custom container_name/naming
// schemes vary.
func depServiceStatus(name string, containers []core.Container) (status string, found bool) {
	nl := strings.ToLower(name)
	for _, c := range containers {
		cn := strings.ToLower(c.Name)
		if cn == nl || strings.Contains(cn, "-"+nl+"-") || strings.HasSuffix(cn, "-"+nl) || strings.HasPrefix(cn, nl+"-") {
			return c.Status, true
		}
	}
	return "", false
}

func (a *App) handleContainerDepsKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	lines := flattenDepTree(buildDepForest(a.containerDeps))
	switch msg.String() {
	case "esc", "q":
		a.containerSubview = containerSubviewList
		return a, a.requestContainerPreview()
	case "up", "k":
		if a.containerDepsCursor > 0 {
			a.containerDepsCursor--
		}
	case "down", "j":
		if a.containerDepsCursor < len(lines)-1 {
			a.containerDepsCursor++
		}
	case "r":
		return a, a.openContainerDeps(p)
	}
	return a, nil
}

func (a *App) renderContainerDeps(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())

	forest := buildDepForest(a.containerDeps)
	lines := flattenDepTree(forest)

	title := panelTitle("DEPENDÊNCIAS", p.Name)
	if len(lines) == 0 {
		msg := []string{
			StyleMuted.Render("Nenhum depends_on encontrado no compose deste projeto."),
			StyleMuted.Render("(ou o projeto não tem docker-compose.yml)"),
		}
		body := panelBox(title, fitExactLines(msg, h-4), w, h-2, true)
		bottom := a.renderDepsActionsBox(w, 3)
		return lipgloss.JoinVertical(lipgloss.Left, body, bottom)
	}

	inner := h - 4
	viewport := maxInt(1, inner-2)
	a.containerDepsCursor = clampCursor(a.containerDepsCursor, len(lines))
	a.containerDepsScroll = ensureVisible(a.containerDepsCursor, a.containerDepsScroll, viewport, len(lines))
	start := a.containerDepsScroll
	end := minInt(start+viewport, len(lines))

	rendered := make([]string, 0, viewport)
	for i := start; i < end; i++ {
		rendered = append(rendered, a.renderDepLine(lines[i], i == a.containerDepsCursor, p))
	}
	for len(rendered) < viewport {
		rendered = append(rendered, "")
	}

	body := panelBox(title, fitExactLines(rendered, inner), w, h-2, true)
	bottom := a.renderDepsActionsBox(w, 3)
	return lipgloss.JoinVertical(lipgloss.Left, body, bottom)
}

func (a *App) renderDepLine(line depTreeLine, selected bool, p *core.Project) string {
	status, found := depServiceStatus(line.name, p.Containers)
	nameStyle := StyleMuted
	badge := "  ○ não criado"
	switch {
	case !found:
		nameStyle, badge = StyleMuted, "  ○ não criado"
	case strings.EqualFold(status, "running"):
		nameStyle, badge = StyleHealthy, "  ● running"
	case strings.EqualFold(status, "missing"):
		nameStyle, badge = StyleWarning, "  ◌ missing"
	default:
		nameStyle, badge = StyleStopped, "  ◼ "+strings.ToLower(status)
	}
	row := StyleMuted.Render(line.text) + nameStyle.Bold(true).Render(line.name) + StyleMuted.Render(badge)
	if selected {
		row = StyleSelected.Render(line.text+line.name) + StyleMuted.Render(badge)
	}
	return row
}

func (a *App) depsActionItems() [][2]string {
	return [][2]string{
		{"↑↓", "navegar"},
		{"r", "atualizar"},
		{"esc", "voltar"},
	}
}

func (a *App) renderDepsActionsBox(width, height int) string {
	innerW := maxInt(4, width-2)
	lines := moduleActionLinesWidth(innerW, a.depsActionItems()...)
	if height < len(lines)+2 {
		height = len(lines) + 2
	}
	return panelBox("AÇÕES", fitExactLines(lines, height-2), width, height, false)
}
