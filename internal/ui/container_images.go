package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// imageScope controls how wide the images list casts its net: the exact
// repository of one container, everything the current project references
// (live + dangling rebuilds, matched via compose build labels), or every
// image docker knows about.
type imageScope int

const (
	imageScopeContainer imageScope = iota
	imageScopeProject
	imageScopeAll
)

type dockerImagesLoadedMsg struct {
	gen    int
	images []core.Image
	err    error
}

type dockerImagesRemovedMsg struct {
	gen    int
	ids    []string
	output string
	err    error
}

var imageRemoveOptions = []string{
	"Remover",
	"Remover somente sem tags vinculadas",
	"Remover (Forçado)",
	"Remover (Forçado) sem tags vinculadas",
	"Cancelar",
}

func (a *App) openContainerImages(c core.Container) tea.Cmd {
	a.containerSubview = containerSubviewImages
	a.imageScope = imageScopeContainer
	a.imageContainerRepo = repoOf(c.Image)
	a.imageCursor = 0
	a.imageScroll = 0
	a.imageStatusMsg = ""
	a.imageConfirmRemove = false
	a.imageConfirmCursor = 0
	return a.loadDockerImages()
}

func (a *App) loadDockerImages() tea.Cmd {
	a.imageLoading = true
	a.imageGen++
	gen := a.imageGen
	return func() tea.Msg {
		images, err := collectors.CollectDockerImages(context.Background())
		return dockerImagesLoadedMsg{gen: gen, images: images, err: err}
	}
}

func (a *App) handleDockerImagesLoaded(msg dockerImagesLoadedMsg) {
	if msg.gen != a.imageGen {
		return
	}
	a.imageLoading = false
	if msg.err != nil {
		a.imageStatusMsg = "erro: " + msg.err.Error()
		return
	}
	a.imageAll = msg.images
	a.imageCursor = clampCursor(a.imageCursor, len(a.currentImageList(a.currentProject())))
}

func (a *App) handleDockerImagesRemoved(msg dockerImagesRemovedMsg) tea.Cmd {
	a.imageLoading = false
	if msg.err != nil {
		detail := firstLine(msg.output)
		if detail == "" {
			detail = msg.err.Error()
		}
		a.imageStatusMsg = "erro: " + detail
	} else {
		a.imageStatusMsg = fmt.Sprintf("✓ %d imagem(ns) removida(s)", len(msg.ids))
	}
	return a.loadDockerImages()
}

func (a *App) currentImageList(p *core.Project) []core.Image {
	switch a.imageScope {
	case imageScopeContainer:
		return filterImagesByRepo(a.imageAll, a.imageContainerRepo)
	case imageScopeProject:
		return filterImagesByProject(a.imageAll, p)
	default:
		return a.imageAll
	}
}

func (a *App) cycleImageScope() {
	switch a.imageScope {
	case imageScopeContainer:
		a.imageScope = imageScopeProject
	case imageScopeProject:
		a.imageScope = imageScopeAll
	default:
		a.imageScope = imageScopeContainer
	}
	a.imageCursor = 0
	a.imageScroll = 0
}

func (a *App) syncImageScroll(count int) {
	viewport := maxInt(4, a.projectPanelHeight()*45/100-6)
	a.imageScroll = ensureVisible(a.imageCursor, a.imageScroll, viewport, count)
}

func repoOf(image string) string {
	if image == "" {
		return ""
	}
	if i := strings.LastIndex(image, "@"); i >= 0 {
		image = image[:i]
	}
	if i := strings.LastIndex(image, ":"); i >= 0 {
		return image[:i]
	}
	return image
}

func imageDangling(img core.Image) bool {
	return img.Repository == "" || img.Repository == "<none>" || img.Tag == "<none>"
}

func imageRef(img core.Image) string {
	if imageDangling(img) {
		return "<none>  " + img.ID
	}
	return img.Repository + ":" + img.Tag
}

func filterImagesByRepo(images []core.Image, repo string) []core.Image {
	if repo == "" {
		return nil
	}
	var out []core.Image
	for _, img := range images {
		if img.Repository == repo {
			out = append(out, img)
		}
	}
	return out
}

// imageBelongsToProject matches by the exact live reference any of the
// project's containers run, or by the compose build labels baked into the
// image itself — the latter is what still identifies an old, now-dangling
// rebuild that `docker images` no longer names.
func imageBelongsToProject(img core.Image, p *core.Project) bool {
	if p == nil {
		return false
	}
	if img.ComposeProject != "" {
		cp := strings.ToLower(img.ComposeProject)
		if cp == strings.ToLower(filepath.Base(p.Path)) || cp == strings.ToLower(p.Name) {
			return true
		}
	}
	if !imageDangling(img) {
		ref := img.Repository + ":" + img.Tag
		for _, c := range p.Containers {
			if c.Image == ref {
				return true
			}
		}
	}
	return false
}

func filterImagesByProject(images []core.Image, p *core.Project) []core.Image {
	var out []core.Image
	for _, img := range images {
		if imageBelongsToProject(img, p) {
			out = append(out, img)
		}
	}
	return out
}

func (a *App) handleContainerImagesKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	if a.imageConfirmRemove {
		return a.handleImageRemoveModalKeys(msg, p)
	}
	list := a.currentImageList(p)
	switch msg.String() {
	case "esc", "q":
		a.containerSubview = containerSubviewList
		return a, a.requestContainerPreview()
	case "up", "k":
		if a.imageCursor > 0 {
			a.imageCursor--
			a.syncImageScroll(len(list))
		}
	case "down", "j":
		if a.imageCursor < len(list)-1 {
			a.imageCursor++
			a.syncImageScroll(len(list))
		}
	case "A", "shift+a", "shift+A":
		a.cycleImageScope()
	case "D", "shift+d", "shift+D":
		if len(list) == 0 {
			a.imageStatusMsg = "nenhuma imagem neste escopo"
			return a, nil
		}
		a.imageConfirmRemove = true
		a.imageConfirmCursor = 0
	case "r":
		return a, a.loadDockerImages()
	}
	return a, nil
}

func (a *App) handleImageRemoveModalKeys(msg tea.KeyMsg, p *core.Project) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.imageConfirmRemove = false
	case "up", "k":
		if a.imageConfirmCursor > 0 {
			a.imageConfirmCursor--
		}
	case "down", "j":
		if a.imageConfirmCursor < len(imageRemoveOptions)-1 {
			a.imageConfirmCursor++
		}
	case "enter":
		a.imageConfirmRemove = false
		return a, a.runImageRemoveOption(a.imageConfirmCursor, p)
	}
	return a, nil
}

func (a *App) runImageRemoveOption(option int, p *core.Project) tea.Cmd {
	list := a.currentImageList(p)
	var ids []string
	switch option {
	case 0, 2: // Remover / Remover (Forçado) — só a imagem selecionada
		if a.imageCursor >= len(list) {
			a.imageStatusMsg = "nenhuma imagem selecionada"
			return nil
		}
		ids = []string{list[a.imageCursor].ID}
	case 1, 3: // …sem tags vinculadas — todas as imagens dangling do escopo atual
		for _, img := range list {
			if imageDangling(img) {
				ids = append(ids, img.ID)
			}
		}
		if len(ids) == 0 {
			a.imageStatusMsg = "nenhuma imagem sem tag neste escopo"
			return nil
		}
	default: // Cancelar
		return nil
	}
	force := option == 2 || option == 3
	a.imageLoading = true
	a.imageStatusMsg = "removendo…"
	gen := a.imageGen
	return func() tea.Msg {
		out, err := collectors.RemoveDockerImages(context.Background(), ids, force)
		return dockerImagesRemovedMsg{gen: gen, ids: ids, output: out, err: err}
	}
}

func (a *App) imageScopeLabel() string {
	switch a.imageScope {
	case imageScopeProject:
		return "PROJETO"
	case imageScopeAll:
		return "TODAS"
	default:
		return "CONTAINER"
	}
}

func (a *App) renderContainerImages(p *core.Project) string {
	w := maxInt(40, a.width)
	h := maxInt(8, a.projectPanelHeight())

	images := a.currentImageList(p)
	if a.imageLoading && len(a.imageAll) == 0 {
		box := panelBox("IMAGENS", fitExactLines([]string{a.loadingText("Carregando imagens…")}, h-2), w, h, true)
		return box
	}

	header := a.renderImagesHeader(p, len(images), w)
	notif := a.renderImagesNotif()
	chromeH := lipgloss.Height(header) + lipgloss.Height(notif) + 1
	bodyH := maxInt(10, h-chromeH-1)
	actionsNeed := len(a.imageActionItems()) + 3
	bottomH := maxInt(actionsNeed, 6)
	if bottomH > bodyH-6 {
		bottomH = maxInt(actionsNeed, bodyH-6)
	}
	if bottomH > bodyH {
		bottomH = bodyH
	}
	tableH := maxInt(6, bodyH-bottomH)

	table := a.renderImagesTable(images, p, w, tableH)
	bottom := a.renderImagesActionsBox(w, bottomH)

	view := lipgloss.JoinVertical(lipgloss.Left, header, notif, table, bottom)
	if a.imageConfirmRemove {
		box := a.renderImageRemoveModal(p, w, h)
		view = overlayCentered(view, box, w, h)
	}
	return view
}

func (a *App) renderImagesHeader(p *core.Project, count int, width int) string {
	left := StyleSection.Render("IMAGENS") + StyleAccent.Render("  "+a.imageScopeLabel())
	if p != nil && a.imageScope != imageScopeAll {
		left += StyleMuted.Render("  " + p.Name)
	}
	right := StyleAccent.Render("● projeto") + StyleMuted.Render("  ") +
		StyleWarning.Render("· outros") + StyleMuted.Render("  ") +
		StyleMuted.Render(fmt.Sprintf("%d imagem(ns)", count))
	pad := width - lipgloss.Width(stripANSI(left)) - lipgloss.Width(stripANSI(right)) - 1
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (a *App) renderImagesNotif() string {
	if a.imageStatusMsg == "" {
		return StyleMuted.Render(" ")
	}
	style := StyleWarning
	if strings.Contains(a.imageStatusMsg, "✓") {
		style = StyleHealthy
	}
	return style.Render(truncate(a.imageStatusMsg, maxInt(40, a.width-4)))
}

type imageCols struct {
	indicator, repo, tag, id, created, size int
}

func (a *App) imageColumns() imageCols {
	tableWidth := maxInt(38, a.width-8)
	cols := imageCols{indicator: 1, id: 12, size: 9}
	flexible := tableWidth - cols.indicator - cols.id - cols.size - 4
	if flexible < 20 {
		flexible = 20
	}
	cols.repo = flexible * 42 / 100
	cols.tag = flexible * 20 / 100
	cols.created = flexible - cols.repo - cols.tag
	return cols
}

func (a *App) renderImagesTable(images []core.Image, p *core.Project, width, height int) string {
	inner := maxInt(3, height-2)
	viewport := maxInt(1, inner-2)
	if len(images) == 0 {
		return panelBox("LISTA", fitExactLines([]string{StyleMuted.Render("nenhuma imagem neste escopo")}, inner), width, height, true)
	}
	a.imageScroll = ensureVisible(a.imageCursor, a.imageScroll, viewport, len(images))
	start := a.imageScroll
	end := minInt(start+viewport, len(images))

	cols := a.imageColumns()
	lines := []string{a.renderImagesHeaderRow(cols), rule(maxInt(20, width-6))}
	for i := start; i < end; i++ {
		lines = append(lines, a.renderImageRow(images[i], cols, i == a.imageCursor, p))
	}
	for i := end - start; i < viewport; i++ {
		lines = append(lines, "")
	}
	if rem := len(images) - end; rem > 0 {
		lines = append(lines, StyleMuted.Render(fmt.Sprintf("↓ %d abaixo", rem)))
	}
	return panelBox(panelTitle("LISTA", fmt.Sprint(len(images))), fitExactLines(lines, inner), width, height, true)
}

func (a *App) renderImagesHeaderRow(cols imageCols) string {
	style := StyleTableHeader
	gap := lipgloss.NewStyle().Width(1).Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(1).Render(""),
		style.Width(cols.indicator).Render(" "),
		gap,
		style.Width(cols.repo).Render("REPOSITORY"),
		gap,
		style.Width(cols.tag).Render("TAG"),
		gap,
		style.Width(cols.id).Render("IMAGE ID"),
		gap,
		style.Width(cols.created).Render("CREATED"),
		gap,
		style.Width(cols.size).Render("SIZE"),
	)
}

// imageOwnerStyle is the same colour language the containers list uses:
// accent = belongs to the current project, warning = someone else's,
// muted = an untagged leftover nobody claims.
func imageOwnerStyle(img core.Image, p *core.Project) (lipgloss.Style, string) {
	if imageBelongsToProject(img, p) {
		return StyleAccent, "●"
	}
	if imageDangling(img) {
		return StyleMuted, "·"
	}
	return StyleWarning, "·"
}

func (a *App) renderImageRow(img core.Image, cols imageCols, selected bool, p *core.Project) string {
	style := StyleNormal
	if selected {
		style = StyleSelected
	}
	gap := lipgloss.NewStyle().Width(1).Render("")
	cell := func(width int, text string) string {
		return style.Width(width).MaxWidth(width).Render(truncate(text, width))
	}

	refStyle, indicator := imageOwnerStyle(img, p)
	indicatorStyle := refStyle
	if selected {
		indicatorStyle, refStyle = style, style
	}
	refCell := func(width int, text string) string {
		return refStyle.Width(width).MaxWidth(width).Render(truncate(text, width))
	}

	repo, tag := img.Repository, img.Tag
	if imageDangling(img) {
		repo, tag = "<none>", "<none>"
	}

	parts := []string{
		lipgloss.NewStyle().Width(1).Render(""),
		indicatorStyle.Width(cols.indicator).Render(indicator),
		gap,
		refCell(cols.repo, repo),
		gap,
		refCell(cols.tag, tag),
		gap,
		cell(cols.id, img.ID),
		gap,
		cell(cols.created, img.Created),
		gap,
		cell(cols.size, img.Size),
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (a *App) imageActionItems() [][2]string {
	next := "escopo: projeto"
	switch a.imageScope {
	case imageScopeProject:
		next = "escopo: todas"
	case imageScopeAll:
		next = "escopo: container"
	}
	return [][2]string{
		{"↑↓", "navegar"},
		{"A", next},
		{"D", "remover"},
		{"r", "atualizar"},
		{"esc", "voltar"},
	}
}

func (a *App) renderImagesActionsBox(width, height int) string {
	innerW := maxInt(4, width-2)
	lines := moduleActionLinesWidth(innerW, a.imageActionItems()...)
	if height < len(lines)+2 {
		height = len(lines) + 2
	}
	return panelBox("AÇÕES", fitExactLines(lines, height-2), width, height, false)
}

func (a *App) renderImageRemoveModal(p *core.Project, width, height int) string {
	boxW := minInt(width-4, maxInt(48, width*55/100))
	boxH := minInt(height-2, 16)
	innerW := maxInt(28, boxW-6)

	target := "—"
	if list := a.currentImageList(p); a.imageCursor < len(list) {
		target = imageRef(list[a.imageCursor])
	}

	lines := tunnelModalChrome("DOCKER", tabAccentColor(TabContainers), "Remover imagem", "docker rmi — escolha uma opção", "", innerW)
	lines = append(lines, "")
	nameBox := panelBox("imagem selecionada",
		[]string{StyleWarning.Bold(true).Render(truncate(target, innerW-2))},
		innerW, 3, true,
	)
	lines = append(lines, strings.Split(nameBox, "\n")...)
	lines = append(lines, "")
	for i, opt := range imageRemoveOptions {
		style := StyleNormal
		prefix := "  "
		if i == 2 || i == 3 {
			style = style.Foreground(ColorDanger)
		}
		if i == a.imageConfirmCursor {
			style = StyleSelected
			if i == 2 || i == 3 {
				style = style.Foreground(ColorDanger)
			}
			prefix = "▸ "
		}
		lines = append(lines, style.Render(prefix+opt))
	}
	lines = append(lines, "", StyleMuted.Render("↑↓ escolhe  ·  enter confirma  ·  esc cancela"))
	return tunnelModalBox(lines, boxW, boxH, tabAccentColor(TabContainers))
}
