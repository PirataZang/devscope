package ui

import (
	"fmt"
	"os/exec"
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

type dockerAddResultsFocus int

const (
	dockerAddResultsList dockerAddResultsFocus = iota
	dockerAddResultsImage
	dockerAddResultsDetails
)

const dockerHubPageSize = 15

type dockerHubSearchDoneMsg struct {
	query   string
	page    int
	results []collectors.DockerHubRepo
	hasMore bool
	append  bool
	err     error
}

type dockerHubDetailsDoneMsg struct {
	seq     int
	name    string
	details collectors.DockerHubDetails
	err     error
}

type dockerHubTagsDoneMsg struct {
	seq  int
	name string
	tags []collectors.DockerHubTag
	err  error
}

type dockerAddComposeReadyMsg struct {
	image  string
	yaml   string
	source string
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
	a.dockerAddPage = 0
	a.dockerAddHasMore = false
	a.dockerAddImage = ""
	a.dockerAddResultsFocus = dockerAddResultsList
	a.dockerAddEdit = ""
	a.dockerAddEditState = editorState{Cursor: 0, Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.dockerAddSearchFocus = dockerAddSearchQuery
	a.dockerAddLoading = false
	a.dockerAddComposeSource = ""
	a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{}
	a.dockerAddDetailsName = ""
	a.dockerAddDetailsLoading = false
	a.dockerAddDetailsScroll = 0
	a.dockerAddTags = nil
	a.dockerAddTagsRepo = ""
	a.dockerAddTagsLoading = false
	a.dockerAddTagCursor = 0
	a.dockerAddTagsScroll = 0
	a.containerStatusMsg = "Docker Hub · buscar imagem ou YAML manual"
}

func (a *App) closeDockerAdd() {
	a.dockerAddOn = false
	a.dockerAddStep = dockerAddStepSearch
	a.dockerAddQuery = ""
	a.dockerAddCursor = 0
	a.dockerAddResults = nil
	a.dockerAddPage = 0
	a.dockerAddHasMore = false
	a.dockerAddImage = ""
	a.dockerAddResultsFocus = dockerAddResultsList
	a.dockerAddEdit = ""
	a.dockerAddEditState = editorState{Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.dockerAddSearchFocus = dockerAddSearchQuery
	a.dockerAddLoading = false
	a.dockerAddComposeSource = ""
	a.dockerAddDetailsCache = nil
	a.dockerAddDetailsName = ""
	a.dockerAddDetailsLoading = false
	a.dockerAddDetailsScroll = 0
	a.dockerAddTags = nil
	a.dockerAddTagsRepo = ""
	a.dockerAddTagsLoading = false
	a.dockerAddTagCursor = 0
	a.dockerAddTagsScroll = 0
}

func (a *App) applyDockerAddCompose(yamlText, source string) {
	a.dockerAddLoading = false
	a.dockerAddComposeSource = source
	a.dockerAddStep = dockerAddStepEdit
	a.dockerAddEdit = yamlText
	a.dockerAddEditState = editorState{Cursor: len([]rune(a.dockerAddEdit)), Anchor: -1}
	a.dockerAddFocus = dockerAddFocusEditor
	a.containerStatusMsg = "compose · " + source + " · tab Salvar"
}

func (a *App) openDockerAddManualEdit() {
	yamlText, source := collectors.BuildComposeServiceYAML("")
	a.applyDockerAddCompose(yamlText, source)
}

func (a *App) openDockerAddEditFromImage(image string) tea.Cmd {
	image = strings.TrimSpace(image)
	if image == "" {
		a.containerStatusMsg = "imagem vazia"
		return nil
	}
	if yamlText, source, ok := collectors.ComposeServiceYAMLFromPreset(image); ok {
		a.applyDockerAddCompose(yamlText, source)
		return nil
	}
	a.dockerAddLoading = true
	a.containerStatusMsg = "detectando config da imagem…"
	return func() tea.Msg {
		yamlText, source := collectors.BuildComposeServiceYAML(image)
		return dockerAddComposeReadyMsg{image: image, yaml: yamlText, source: source}
	}
}

func (a *App) handleDockerAddComposeReady(msg dockerAddComposeReadyMsg) {
	if !a.dockerAddOn {
		return
	}
	a.applyDockerAddCompose(msg.yaml, msg.source)
}

func (a *App) syncDockerAddImageFromCursor() {
	if a.dockerAddCursor < 0 || a.dockerAddCursor >= len(a.dockerAddResults) {
		return
	}
	name := a.dockerAddResults[a.dockerAddCursor].Name
	a.dockerAddImage = name
	a.dockerAddDetailsScroll = 0
	if a.dockerAddTagsRepo != name {
		a.dockerAddTags = nil
		a.dockerAddTagsRepo = ""
		a.dockerAddTagCursor = 0
		a.dockerAddTagsScroll = 0
	}
}

func (a *App) selectedDockerHubRepo() (collectors.DockerHubRepo, bool) {
	if a.dockerAddCursor < 0 || a.dockerAddCursor >= len(a.dockerAddResults) {
		return collectors.DockerHubRepo{}, false
	}
	return a.dockerAddResults[a.dockerAddCursor], true
}

func (a *App) applyDockerAddTagAtCursor() {
	if a.dockerAddTagCursor < 0 || a.dockerAddTagCursor >= len(a.dockerAddTags) {
		return
	}
	repo := collectors.ImageRepoName(a.dockerAddImage)
	if repo == "" {
		if r, ok := a.selectedDockerHubRepo(); ok {
			repo = r.Name
		}
	}
	a.dockerAddImage = collectors.WithImageTag(repo, a.dockerAddTags[a.dockerAddTagCursor].Name)
}

func (a *App) ensureDockerHubDetails() tea.Cmd {
	repo, ok := a.selectedDockerHubRepo()
	if !ok {
		return nil
	}
	if a.dockerAddDetailsCache == nil {
		a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{}
	}
	name := repo.Name

	needDetails := false
	if _, hit := a.dockerAddDetailsCache[name]; hit {
		a.dockerAddDetailsName = name
		a.dockerAddDetailsLoading = false
	} else if a.dockerAddDetailsLoading && a.dockerAddDetailsName == name {
		// já pedindo
	} else {
		needDetails = true
	}

	needTags := false
	if a.dockerAddTagsRepo == name && len(a.dockerAddTags) > 0 {
		// ok
	} else if a.dockerAddTagsLoading && a.dockerAddTagsRepo == name {
		// já pedindo
	} else {
		needTags = true
	}

	if !needDetails && !needTags {
		return nil
	}

	a.dockerAddDetailsSeq++
	seq := a.dockerAddDetailsSeq
	var cmds []tea.Cmd
	if needDetails {
		a.dockerAddDetailsName = name
		a.dockerAddDetailsLoading = true
		cmds = append(cmds, func() tea.Msg {
			d, err := collectors.FetchDockerHubDetails(name)
			return dockerHubDetailsDoneMsg{seq: seq, name: name, details: d, err: err}
		})
	}
	if needTags {
		a.dockerAddTagsRepo = name
		a.dockerAddTagsLoading = true
		a.dockerAddTags = nil
		a.dockerAddTagCursor = 0
		a.dockerAddTagsScroll = 0
		cmds = append(cmds, func() tea.Msg {
			tags, err := collectors.ListDockerHubTags(name, 30)
			return dockerHubTagsDoneMsg{seq: seq, name: name, tags: tags, err: err}
		})
	}
	return tea.Batch(cmds...)
}

func (a *App) handleDockerHubDetailsDone(msg dockerHubDetailsDoneMsg) {
	if !a.dockerAddOn || msg.seq != a.dockerAddDetailsSeq {
		return
	}
	if msg.name != a.dockerAddDetailsName {
		return
	}
	a.dockerAddDetailsLoading = false
	if msg.err != nil {
		a.containerStatusMsg = "detalhes: " + msg.err.Error()
		return
	}
	if a.dockerAddDetailsCache == nil {
		a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{}
	}
	a.dockerAddDetailsCache[msg.name] = msg.details
}

func (a *App) handleDockerHubTagsDone(msg dockerHubTagsDoneMsg) {
	if !a.dockerAddOn || msg.seq != a.dockerAddDetailsSeq {
		return
	}
	if msg.name != a.dockerAddTagsRepo {
		return
	}
	a.dockerAddTagsLoading = false
	if msg.err != nil {
		a.containerStatusMsg = "tags: " + msg.err.Error()
		a.dockerAddTags = nil
		return
	}
	a.dockerAddTags = msg.tags
	a.dockerAddTagCursor = 0
	a.dockerAddTagsScroll = 0
	// Prefere "latest" se existir; senão a mais recente.
	for i, t := range a.dockerAddTags {
		if t.Name == "latest" {
			a.dockerAddTagCursor = i
			break
		}
	}
}

func (a *App) requestDockerHubPage(query string, page int, appendResults bool) tea.Cmd {
	a.dockerAddLoading = true
	if appendResults {
		a.containerStatusMsg = "carregando mais imagens…"
	} else {
		a.containerStatusMsg = "buscando no Docker Hub…"
	}
	return func() tea.Msg {
		pageData, err := collectors.SearchDockerHubPage(query, page, dockerHubPageSize)
		if err != nil {
			return dockerHubSearchDoneMsg{query: query, page: page, append: appendResults, err: err}
		}
		return dockerHubSearchDoneMsg{
			query:   query,
			page:    pageData.Page,
			results: pageData.Results,
			hasMore: pageData.HasMore,
			append:  appendResults,
		}
	}
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
			a.containerStatusMsg = "digite um termo ou tab → YAML manual"
			return a, nil
		}
		a.dockerAddResults = nil
		a.dockerAddPage = 0
		a.dockerAddHasMore = false
		a.dockerAddCursor = 0
		return a, a.requestDockerHubPage(q, 1, false)
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
		a.dockerAddPage = 0
		a.dockerAddHasMore = false
		a.dockerAddImage = ""
		a.dockerAddResultsFocus = dockerAddResultsList
		a.dockerAddSearchFocus = dockerAddSearchQuery
		a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{}
		a.dockerAddDetailsName = ""
		a.dockerAddDetailsLoading = false
		a.dockerAddDetailsScroll = 0
		a.dockerAddTags = nil
		a.dockerAddTagsRepo = ""
		a.dockerAddTagsLoading = false
		a.dockerAddTagCursor = 0
		a.dockerAddTagsScroll = 0
		a.containerStatusMsg = "Docker Hub · buscar imagem ou YAML manual"
		return a, nil
	case "tab":
		switch a.dockerAddResultsFocus {
		case dockerAddResultsList:
			a.dockerAddResultsFocus = dockerAddResultsImage
		case dockerAddResultsImage:
			a.dockerAddResultsFocus = dockerAddResultsDetails
		default:
			a.dockerAddResultsFocus = dockerAddResultsList
		}
		return a, nil
	case "shift+tab":
		switch a.dockerAddResultsFocus {
		case dockerAddResultsList:
			a.dockerAddResultsFocus = dockerAddResultsDetails
		case dockerAddResultsImage:
			a.dockerAddResultsFocus = dockerAddResultsList
		default:
			a.dockerAddResultsFocus = dockerAddResultsImage
		}
		return a, nil
	case "o", "O":
		a.openSelectedDockerHubLink()
		return a, nil
	case "up", "k":
		switch a.dockerAddResultsFocus {
		case dockerAddResultsImage:
			// ↑↓ só navega tags; troca de painel é só via tab.
			if len(a.dockerAddTags) > 0 && a.dockerAddTagCursor > 0 {
				a.dockerAddTagCursor--
				a.applyDockerAddTagAtCursor()
			}
			return a, nil
		case dockerAddResultsDetails:
			if a.dockerAddDetailsScroll > 0 {
				a.dockerAddDetailsScroll--
			}
			return a, nil
		}
		if a.dockerAddCursor > 0 {
			a.dockerAddCursor--
			a.syncDockerAddImageFromCursor()
			return a, a.ensureDockerHubDetails()
		}
		return a, nil
	case "down", "j":
		switch a.dockerAddResultsFocus {
		case dockerAddResultsImage:
			if len(a.dockerAddTags) > 0 && a.dockerAddTagCursor < len(a.dockerAddTags)-1 {
				a.dockerAddTagCursor++
				a.applyDockerAddTagAtCursor()
			}
			return a, nil
		case dockerAddResultsDetails:
			a.dockerAddDetailsScroll++
			return a, nil
		}
		if a.dockerAddCursor < len(a.dockerAddResults)-1 {
			a.dockerAddCursor++
			a.syncDockerAddImageFromCursor()
			return a, a.ensureDockerHubDetails()
		}
		if a.dockerAddHasMore && !a.dockerAddLoading {
			q := strings.TrimSpace(a.dockerAddQuery)
			return a, a.requestDockerHubPage(q, a.dockerAddPage+1, true)
		}
		return a, nil
	case "pgup":
		if a.dockerAddResultsFocus == dockerAddResultsDetails {
			a.dockerAddDetailsScroll = maxInt(0, a.dockerAddDetailsScroll-5)
			return a, nil
		}
		return a, nil
	case "pgdown":
		if a.dockerAddResultsFocus == dockerAddResultsDetails {
			a.dockerAddDetailsScroll += 5
			return a, nil
		}
		if a.dockerAddHasMore && !a.dockerAddLoading {
			q := strings.TrimSpace(a.dockerAddQuery)
			return a, a.requestDockerHubPage(q, a.dockerAddPage+1, true)
		}
		return a, nil
	case "enter":
		img := strings.TrimSpace(a.dockerAddImage)
		if img == "" {
			if r, ok := a.selectedDockerHubRepo(); ok {
				img = r.Name
			}
		}
		return a, a.openDockerAddEditFromImage(img)
	case "backspace":
		if a.dockerAddResultsFocus != dockerAddResultsImage {
			return a, nil
		}
		runes := []rune(a.dockerAddImage)
		if len(runes) > 0 {
			a.dockerAddImage = string(runes[:len(runes)-1])
		}
		return a, nil
	case "ctrl+u":
		if a.dockerAddResultsFocus == dockerAddResultsImage {
			a.dockerAddImage = ""
		}
		return a, nil
	}
	if a.dockerAddResultsFocus == dockerAddResultsImage && msg.Type == tea.KeyRunes {
		a.dockerAddImage += string(msg.Runes)
	}
	return a, nil
}

func (a *App) openSelectedDockerHubLink() {
	name := strings.TrimSpace(a.dockerAddImage)
	if name == "" {
		if r, ok := a.selectedDockerHubRepo(); ok {
			name = r.Name
		}
	}
	url := collectors.DockerHubURL(name)
	if url == "" {
		a.containerStatusMsg = "link Docker Hub indisponível"
		return
	}
	_ = exec.Command("xdg-open", url).Start()
	a.containerStatusMsg = "abrindo " + truncate(url, 48)
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

func (a *App) handleDockerHubSearchDone(msg dockerHubSearchDoneMsg) tea.Cmd {
	a.dockerAddLoading = false
	if !a.dockerAddOn {
		return nil
	}
	if msg.err != nil {
		a.containerStatusMsg = "docker hub: " + msg.err.Error()
		return nil
	}
	if msg.append {
		if len(msg.results) == 0 {
			a.dockerAddHasMore = false
			a.containerStatusMsg = "não há mais resultados"
			return nil
		}
		a.dockerAddResults = append(a.dockerAddResults, msg.results...)
		a.dockerAddPage = msg.page
		a.dockerAddHasMore = msg.hasMore
		a.dockerAddCursor = len(a.dockerAddResults) - len(msg.results)
		a.syncDockerAddImageFromCursor()
		a.containerStatusMsg = fmt.Sprintf("%d imagens · ↓ carrega mais", len(a.dockerAddResults))
		return a.ensureDockerHubDetails()
	}
	if len(msg.results) == 0 {
		a.containerStatusMsg = "nenhum resultado para " + msg.query
		return nil
	}
	a.dockerAddResults = msg.results
	a.dockerAddPage = msg.page
	a.dockerAddHasMore = msg.hasMore
	a.dockerAddCursor = 0
	a.dockerAddResultsFocus = dockerAddResultsList
	a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{}
	a.syncDockerAddImageFromCursor()
	a.dockerAddStep = dockerAddStepResults
	more := ""
	if a.dockerAddHasMore {
		more = " · ↓ mais"
	}
	a.containerStatusMsg = fmt.Sprintf("%d resultados%s · tab ajusta imagem", len(msg.results), more)
	return a.ensureDockerHubDetails()
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
