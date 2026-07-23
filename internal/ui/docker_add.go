package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

type dockerAddStep int

const (
	dockerAddStepSearch dockerAddStep = iota
	dockerAddStepResults
	dockerAddStepEdit
)

type dockerAddFocus int

const (
	dockerAddFocusEditor dockerAddFocus = iota
	dockerAddFocusSave
	dockerAddFocusCancel
)

type dockerAddSearchFocus int

const (
	dockerAddSearchQuery dockerAddSearchFocus = iota
	dockerAddSearchRefuse
)

type dockerHubSearchDoneMsg struct {
	query   string
	results []collectors.DockerHubRepo
	err     error
}

type dockerAddSavedMsg struct {
	path string
	err  error
}

func (a *App) containersTabReady(p *core.Project) bool {
	return a.tab == TabContainers && a.containerSubview == containerSubviewList && p != nil && !a.dockerAddOn
}

func (a *App) startDockerAdd(p *core.Project) {
	if p == nil {
		return
	}
	a.dockerAddOn = true
	a.dockerAddStep = dockerAddStepSearch
	a.dockerAddQuery = ""
	a.dockerAddCursor = 0
	a.dockerAddResults = nil
	a.dockerAddEdit = ""
	a.dockerAddEditState = editorState{Cursor: 0, Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.dockerAddSearchFocus = dockerAddSearchQuery
	a.dockerAddLoading = false
	a.containerStatusMsg = "Docker Hub · tab recusar"
}

func (a *App) closeDockerAdd() {
	a.dockerAddOn = false
	a.dockerAddStep = dockerAddStepSearch
	a.dockerAddQuery = ""
	a.dockerAddCursor = 0
	a.dockerAddResults = nil
	a.dockerAddEdit = ""
	a.dockerAddEditState = editorState{Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.dockerAddSearchFocus = dockerAddSearchQuery
	a.dockerAddLoading = false
}

func (a *App) openDockerAddManualEdit() {
	a.dockerAddStep = dockerAddStepEdit
	a.dockerAddEdit = collectors.ComposeServiceTemplate("")
	a.dockerAddEditState = editorState{Cursor: len([]rune(a.dockerAddEdit)), Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.containerStatusMsg = "edite o YAML · tab Salvar"
}

func (a *App) openDockerAddEditFromImage(image string) {
	a.dockerAddStep = dockerAddStepEdit
	a.dockerAddEdit = collectors.ComposeServiceTemplate(image)
	a.dockerAddEditState = editorState{Cursor: len([]rune(a.dockerAddEdit)), Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.containerStatusMsg = "edite o YAML · tab Salvar"
}

func (a *App) updateDockerAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.dockerAddStep {
	case dockerAddStepSearch:
		return a.updateDockerAddSearch(msg)
	case dockerAddStepResults:
		return a.updateDockerAddResults(msg)
	default:
		return a.updateDockerAddEdit(msg)
	}
}

func (a *App) updateDockerAddSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.closeDockerAdd()
		a.containerStatusMsg = "novo serviço cancelado"
		return a, nil
	case "tab", "shift+tab", "down", "up":
		if a.dockerAddSearchFocus == dockerAddSearchQuery {
			a.dockerAddSearchFocus = dockerAddSearchRefuse
		} else {
			a.dockerAddSearchFocus = dockerAddSearchQuery
		}
		return a, nil
	case "enter":
		if a.dockerAddSearchFocus == dockerAddSearchRefuse {
			a.openDockerAddManualEdit()
			return a, nil
		}
		q := strings.TrimSpace(a.dockerAddQuery)
		if q == "" {
			a.containerStatusMsg = "digite um termo ou tab → recusar"
			return a, nil
		}
		a.dockerAddLoading = true
		a.containerStatusMsg = "buscando no Docker Hub…"
		query := q
		return a, func() tea.Msg {
			results, err := collectors.SearchDockerHub(query, 15)
			return dockerHubSearchDoneMsg{query: query, results: results, err: err}
		}
	case "backspace":
		if a.dockerAddSearchFocus != dockerAddSearchQuery {
			return a, nil
		}
		runes := []rune(a.dockerAddQuery)
		if len(runes) > 0 {
			a.dockerAddQuery = string(runes[:len(runes)-1])
		}
		return a, nil
	case "ctrl+u":
		if a.dockerAddSearchFocus == dockerAddSearchQuery {
			a.dockerAddQuery = ""
		}
		return a, nil
	}
	if a.dockerAddSearchFocus == dockerAddSearchQuery && msg.Type == tea.KeyRunes {
		a.dockerAddQuery += string(msg.Runes)
	}
	return a, nil
}

func (a *App) updateDockerAddResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.dockerAddStep = dockerAddStepSearch
		a.dockerAddResults = nil
		a.dockerAddCursor = 0
		a.dockerAddSearchFocus = dockerAddSearchQuery
		a.containerStatusMsg = "Docker Hub · tab recusar"
		return a, nil
	case "up", "k":
		if a.dockerAddCursor > 0 {
			a.dockerAddCursor--
		}
		return a, nil
	case "down", "j":
		if a.dockerAddCursor < len(a.dockerAddResults)-1 {
			a.dockerAddCursor++
		}
		return a, nil
	case "enter":
		if a.dockerAddCursor < 0 || a.dockerAddCursor >= len(a.dockerAddResults) {
			return a, nil
		}
		a.openDockerAddEditFromImage(a.dockerAddResults[a.dockerAddCursor].Name)
		return a, nil
	}
	return a, nil
}

func (a *App) updateDockerAddEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.closeDockerAdd()
		a.containerStatusMsg = "novo serviço cancelado"
		return a, nil
	case "tab":
		a.dockerAddFocus = (a.dockerAddFocus + 1) % 3
		return a, nil
	case "shift+tab":
		a.dockerAddFocus = (a.dockerAddFocus + 2) % 3
		return a, nil
	case "enter":
		switch a.dockerAddFocus {
		case dockerAddFocusSave:
			return a, a.submitDockerAdd()
		case dockerAddFocusCancel:
			a.closeDockerAdd()
			a.containerStatusMsg = "novo serviço cancelado"
			return a, nil
		}
	}
	if a.dockerAddFocus != dockerAddFocusEditor {
		return a, nil
	}
	newText, handled := editorApplyKey(msg, a.dockerAddEdit, &a.dockerAddEditState, true)
	if handled {
		a.dockerAddEdit = newText
	}
	return a, nil
}

func (a *App) submitDockerAdd() tea.Cmd {
	text := strings.TrimSpace(a.dockerAddEdit)
	if text == "" {
		a.containerStatusMsg = "YAML vazio"
		a.dockerAddFocus = dockerAddFocusEditor
		return nil
	}
	p := a.currentProject()
	if p == nil {
		a.closeDockerAdd()
		return nil
	}
	path := p.Path
	yamlText := a.dockerAddEdit
	a.dockerAddLoading = true
	a.containerStatusMsg = "salvando no compose…"
	a.closeDockerAdd()
	return func() tea.Msg {
		out, err := collectors.MergeComposeYAML(path, yamlText)
		return dockerAddSavedMsg{path: out, err: err}
	}
}

func (a *App) handleDockerHubSearchDone(msg dockerHubSearchDoneMsg) {
	a.dockerAddLoading = false
	if !a.dockerAddOn {
		return
	}
	if msg.err != nil {
		a.containerStatusMsg = "docker hub: " + msg.err.Error()
		return
	}
	if len(msg.results) == 0 {
		a.containerStatusMsg = "nenhum resultado para " + msg.query
		return
	}
	a.dockerAddResults = msg.results
	a.dockerAddCursor = 0
	a.dockerAddStep = dockerAddStepResults
	a.containerStatusMsg = fmt.Sprintf("%d resultados · enter seleciona", len(msg.results))
}

func (a *App) handleDockerAddSaved(msg dockerAddSavedMsg) tea.Cmd {
	a.dockerAddLoading = false
	if msg.err != nil {
		a.containerStatusMsg = "compose: " + msg.err.Error()
		return nil
	}
	a.containerStatusMsg = "serviço adicionado em " + shortenPath(msg.path)
	return a.refreshDocker()
}

func (a *App) renderDockerAdd() string {
	background := a.renderProject()
	var box string
	switch a.dockerAddStep {
	case dockerAddStepSearch:
		box = a.renderDockerAddSearchBox()
	case dockerAddStepResults:
		box = a.renderDockerAddResultsBox()
	default:
		box = a.renderDockerAddEditBox()
	}
	return overlayCentered(background, box, a.width, a.height)
}

func (a *App) renderDockerAddSearchBox() string {
	input := a.dockerAddQuery + "█"
	if a.dockerAddLoading {
		input = a.dockerAddQuery + " …"
	}
	queryLine := StyleMuted.Render("  " + input)
	refuseLine := StyleMuted.Render("  Recusar buscar do docker hub")
	if a.dockerAddSearchFocus == dockerAddSearchQuery {
		queryLine = StyleSelected.Render("▸ " + input)
	} else {
		refuseLine = StyleSelected.Render("▸ Recusar buscar do docker hub")
	}
	lines := []string{
		StyleSection.Render("Novo serviço Docker"),
		StyleMuted.Render("buscar imagem no Docker Hub"),
		"",
		StyleMuted.Render("termo"),
		queryLine,
		"",
		refuseLine,
		"",
		StyleMuted.Render("enter confirma  ·  tab alterna  ·  esc cancela"),
	}
	boxW := minInt(64, maxInt(40, a.width-8))
	return StylePanel.Width(boxW).Background(ColorBgPanel).Render(strings.Join(lines, "\n"))
}

func (a *App) renderDockerAddResultsBox() string {
	boxW := minInt(72, maxInt(48, a.width*85/100))
	boxH := minInt(22, maxInt(12, a.height*60/100))
	viewport := maxInt(4, boxH-6)
	start := 0
	if a.dockerAddCursor >= viewport {
		start = a.dockerAddCursor - viewport + 1
	}
	end := minInt(start+viewport, len(a.dockerAddResults))

	lines := []string{
		StyleSection.Render("Docker Hub"),
		StyleMuted.Render(fmt.Sprintf("%d imagens · ↑↓ seleciona", len(a.dockerAddResults))),
		"",
	}
	for i := start; i < end; i++ {
		r := a.dockerAddResults[i]
		mark := "  "
		style := StyleMuted
		if i == a.dockerAddCursor {
			mark = "▸ "
			style = StyleSelected
		}
		badge := ""
		if r.Official {
			badge = StyleHealthy.Render(" official")
		}
		desc := truncate(r.Description, maxInt(16, boxW-30))
		line := style.Render(mark+r.Name) + StyleMuted.Render(fmt.Sprintf(" ★%d", r.Stars)) + badge
		if desc != "" {
			line += StyleMuted.Render("  " + desc)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "",
		StyleMuted.Render("enter usa imagem  ·  esc volta à busca"),
	)
	return StylePanel.Width(boxW).Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}

func (a *App) renderDockerAddEditBox() string {
	boxW := minInt(a.width-4, maxInt(56, a.width*90/100))
	boxH := minInt(a.height-2, maxInt(16, a.height*75/100))
	editorH := maxInt(6, boxH-10)

	editing := a.dockerAddFocus == dockerAddFocusEditor
	ed := a.dockerAddEditState
	body := renderEditorLines(a.dockerAddEdit, &ed, boxW-4, editorH, editing, false)
	a.dockerAddEditState = ed

	saveBtn := StyleMuted.Render("  Salvar no compose  ")
	cancelBtn := StyleMuted.Render("  Cancelar  ")
	switch a.dockerAddFocus {
	case dockerAddFocusSave:
		saveBtn = StyleSelected.Render("▸ Salvar no compose ◂")
	case dockerAddFocusCancel:
		cancelBtn = StyleSelected.Render("▸ Cancelar ◂")
	}

	lines := []string{
		StyleSection.Render("Editar serviço"),
		StyleMuted.Render("conteúdo será mesclado no docker-compose do projeto"),
		"",
		StyleMuted.Render("YAML  (enter = nova linha · tab = botões)"),
	}
	lines = append(lines, body...)
	lines = append(lines, "",
		saveBtn+"    "+cancelBtn,
		StyleMuted.Render("tab troca foco  ·  enter no botão confirma  ·  esc sai"),
	)
	return StylePanel.Width(boxW).Background(ColorBgPanel).
		Render(strings.Join(fitExactLines(lines, boxH), "\n"))
}
