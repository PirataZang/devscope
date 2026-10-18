package ui

import (
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Worktrees: várias branches abertas ao mesmo tempo, cada uma na sua pasta.
//
// Do `git worktree` o DevScope usa o que se faz todo dia — listar, criar,
// abrir, remover e podar. lock/unlock, move e repair ficam no terminal: são
// manutenção rara, e cada um seria uma tecla a mais numa tela que tem de
// caber de cabeça.
//
// Duas decisões que tiram perguntas do caminho:
//
//  1. Criar pede só o nome da branch. A pasta é sempre irmã da principal,
//     `<repo>-<branch>` (collectors.GitWorktreePath), e branch que já existe
//     faz checkout em vez de dar erro.
//  2. Remover pergunta uma vez. Com alteração não commitada a confirmação diz
//     quantas se perdem e já usa --force — não há segundo "tem certeza?".
//
// Não tem faixa de abertura (§1.5): a lista é o conteúdo, como em Logs.

type wtLoadedMsg struct {
	path string
	list []collectors.GitWorktree
	err  error
}

type wtActionMsg struct {
	path   string
	note   string
	err    error
	follow string // caminho que o cursor deve seguir depois de recarregar
}

func (a *App) loadWorktrees(p *core.Project) tea.Cmd {
	if p == nil {
		return nil
	}
	path := p.Path
	a.wtLoading = true
	a.wtErr = ""
	return func() tea.Msg {
		list, err := collectors.GitWorktrees(path)
		return wtLoadedMsg{path: path, list: list, err: err}
	}
}

func (a *App) handleWorktreesLoaded(msg wtLoadedMsg) {
	if p := a.currentProject(); p == nil || !pathsMatch(p.Path, msg.path) {
		return
	}
	a.wtLoading = false
	a.wtList = msg.list
	a.wtErr = ""
	if msg.err != nil {
		a.wtErr = lastLine(msg.err.Error())
	}
	if a.wtFollow != "" {
		for i, w := range a.wtList {
			if pathsMatch(w.Path, a.wtFollow) {
				a.wtCursor = i
			}
		}
		a.wtFollow = ""
	}
	a.wtCursor = clampCursor(a.wtCursor, len(a.wtList))
}

func (a *App) handleWorktreeAction(msg wtActionMsg) tea.Cmd {
	a.wtStatus = msg.note
	if msg.err != nil {
		a.wtStatus = "erro: " + lastLine(msg.err.Error())
	}
	a.wtFollow = msg.follow
	p := a.currentProject()
	if p == nil || !pathsMatch(p.Path, msg.path) {
		return nil
	}
	return a.loadWorktrees(p)
}

// wtRun roda a ação do git fora do View e atualiza as branches do projeto —
// criar worktree com nome novo cria branch, e o Git tem de enxergar.
func (a *App) wtRun(p *core.Project, note, follow string, fn func(repo string) (string, error)) tea.Cmd {
	path, store := p.Path, a.store
	a.wtStatus = note + "…"
	a.wtLoading = true
	return func() tea.Msg {
		_, err := fn(path)
		if store != nil {
			collectors.RefreshProjectGit(store, path)
		}
		return wtActionMsg{path: path, note: note, err: err, follow: follow}
	}
}

func (a *App) selectedWorktree() (collectors.GitWorktree, bool) {
	if a.wtCursor < 0 || a.wtCursor >= len(a.wtList) {
		return collectors.GitWorktree{}, false
	}
	return a.wtList[a.wtCursor], true
}

// wtMainPath é onde a worktree nova nasce ao lado: a principal, não o
// projeto aberto — aberto de dentro de uma worktree, a irmã dele seria
// `app-feat-fix`.
func (a *App) wtMainPath(p *core.Project) string {
	for _, w := range a.wtList {
		if w.Main {
			return w.Path
		}
	}
	if p != nil {
		return p.Path
	}
	return ""
}

func (a *App) handleWorktreeKeys(msg tea.KeyMsg, p *core.Project) (tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		a.wtCursor = clampCursor(a.wtCursor-1, len(a.wtList))
	case "down", "j":
		a.wtCursor = clampCursor(a.wtCursor+1, len(a.wtList))
	case "r":
		a.wtStatus = ""
		return a.loadWorktrees(p), true
	case "n":
		a.gitPromptOn = true
		a.gitPromptKind = gitPromptNewWorktree
		a.gitPromptInput = ""
		a.gitPromptCursor = 0
		a.gitPromptBranch = ""
		if g := p.Git; g != nil {
			a.gitPromptBranch = g.Branch
		}
	case "d":
		w, ok := a.selectedWorktree()
		switch {
		case !ok:
		case w.Main:
			a.wtStatus = "a worktree principal não pode ser removida"
		case w.Locked:
			a.wtStatus = "worktree travada — git worktree unlock antes de remover"
		default:
			a.gitConfirmOn = true
			a.gitConfirmAction = "wt-remove"
			a.gitConfirmBranch = w.Path
		}
	case "p":
		return a.wtRun(p, "worktrees podadas", "", collectors.GitWorktreePrune), true
	case "enter":
		if w, ok := a.selectedWorktree(); ok {
			return a.openWorktreeProject(p, w), true
		}
	case "E", "shift+e":
		if w, ok := a.selectedWorktree(); ok && !w.Prunable {
			return a.projectExecShell(w.Path), true
		}
	case "ctrl+o":
		if w, ok := a.selectedWorktree(); ok && !w.Prunable {
			return a.openProjectAI(w.Path), true
		}
	default:
		return nil, false
	}
	return nil, true
}

func (a *App) wtAdd(p *core.Project, branch string) tea.Cmd {
	dest := collectors.GitWorktreePath(a.wtMainPath(p), branch)
	return a.wtRun(p, "worktree criada em "+shortenPath(dest), dest, func(repo string) (string, error) {
		return collectors.GitWorktreeAdd(repo, dest, branch)
	})
}

func (a *App) wtRemove(p *core.Project, path string) tea.Cmd {
	force := false
	for _, w := range a.wtList {
		if pathsMatch(w.Path, path) {
			force = w.Changes > 0
		}
	}
	return a.wtRun(p, "worktree removida", "", func(repo string) (string, error) {
		return collectors.GitWorktreeRemove(repo, path, force)
	})
}

// openWorktreeProject troca o projeto aberto pela worktree: Git, Containers e
// o resto passam a olhar para a pasta dela.
func (a *App) openWorktreeProject(p *core.Project, w collectors.GitWorktree) tea.Cmd {
	if w.Prunable {
		a.wtStatus = "a pasta não existe mais — p poda o registro"
		return nil
	}
	if p != nil && pathsMatch(p.Path, w.Path) {
		a.wtStatus = "esta é a worktree aberta"
		return nil
	}
	proj := core.Project{Name: filepath.Base(w.Path), Path: w.Path}
	if known := projectByPath(a.store.Get().Projects, w.Path); known != nil {
		proj = *known
	} else {
		// ponytail: a worktree fora das pastas escaneadas entra no snapshot só
		// até a próxima varredura; incluir nas raízes do scanner se incomodar.
		a.store.Update(func(s *core.Snapshot) { s.Projects = append(s.Projects, proj) })
	}
	a.closeToolClients()
	return a.openProject(proj, TabGit)
}

// ─── tela ────────────────────────────────────────────────────────────────────

func (a *App) renderWorktreesTab(p *core.Project) string {
	w, h := a.moduleSize()
	head := []string{a.renderModuleContext(p, w, "Worktrees", a.wtStatus)}
	cmdBar := StyleStatusBar.Width(w).Render(fitKeybindsWrap(maxInt(10, w-2), 2,
		[2]string{"enter", "abrir no DevScope"},
		[2]string{"n", "nova"},
		[2]string{"d", "remover"},
		[2]string{"shift+E", "terminal"},
		[2]string{"ctrl+O", "agente de IA"},
		[2]string{"p", "podar"},
		[2]string{"r", "atualizar"},
	))
	room := maxInt(4, h-len(head)-lipgloss.Height(cmdBar))

	detail := a.worktreeDetail(p, w)
	if a.projectTiny() || len(detail)+5 > room {
		detail = nil
	}
	// A lista pede o que precisa (§1.3); o detalhe vai logo abaixo dela e a
	// sobra fica inteira no fim, acima da barra.
	listH := minInt(room-len(detail), len(a.wtList)+2)
	listH = maxInt(3, listH)
	parts := append(head, a.renderWorktreeList(p, w, listH))
	parts = append(parts, detail...)
	stack := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if fill := h - lipgloss.Height(stack) - lipgloss.Height(cmdBar); fill > 0 {
		stack += strings.Repeat("\n", fill)
	}
	return lipgloss.JoinVertical(lipgloss.Left, stack, cmdBar)
}

func (a *App) renderWorktreeList(p *core.Project, width, height int) string {
	viewport := maxInt(1, height-2)
	title := panelTitle("WORKTREES", strconv.Itoa(len(a.wtList)))
	switch {
	case a.wtLoading && len(a.wtList) == 0:
		return panelBox(title, fitExactLines([]string{a.loadingText("carregando worktrees…")}, viewport), width, height, true)
	case a.wtErr != "":
		return panelBox(title, fitExactLines([]string{StyleWarning.Render(truncate(a.wtErr, width-4))}, viewport), width, height, true)
	}

	a.wtScroll = ensureVisible(a.wtCursor, a.wtScroll, viewport, len(a.wtList))
	if below := len(a.wtList) - a.wtScroll - viewport; below > 0 {
		title = panelTitle("WORKTREES", strconv.Itoa(len(a.wtList)), "↓"+strconv.Itoa(below))
	}

	inner := maxInt(20, width-4)
	mainPath := a.wtMainPath(p)
	branchW, stateW := 6, 4
	for _, wt := range a.wtList {
		branchW = maxInt(branchW, lipgloss.Width(worktreeBranchLabel(wt)))
		stateW = maxInt(stateW, lipgloss.Width(worktreeState(wt).text))
	}
	branchW = minInt(branchW, 28)
	headW := 7
	// Prefixo (2) + glifo (1) + separadores. A pasta leva o resto; o hash é o
	// primeiro a sair quando aperta — ele está no detalhe logo abaixo.
	pathW := inner - 2 - 1 - branchW - stateW - headW - 4
	if pathW < 12 {
		headW = 0
		pathW = inner - 2 - 1 - branchW - stateW - 3
	}
	pathW = maxInt(8, pathW)

	lines := make([]string, 0, viewport)
	end := minInt(len(a.wtList), a.wtScroll+viewport)
	for i := a.wtScroll; i < end; i++ {
		wt := a.wtList[i]
		glyph, glyphSt := worktreeGlyph(wt, p)
		st := worktreeState(wt)
		head := ""
		if headW > 0 && len(wt.Head) >= 7 {
			head = wt.Head[:7]
		}
		selected := i == a.wtCursor
		row := renderCells(selected, []dashCell{
			{text: glyph, width: 1, style: glyphSt},
			{text: worktreeBranchLabel(wt), width: branchW, style: lipgloss.NewStyle().Foreground(ColorAccent)},
			{text: elideLeft(worktreeFolder(mainPath, wt.Path), pathW), width: pathW, style: StyleMuted},
			{text: head, width: headW, style: StyleMuted},
			{text: st.text, width: stateW, style: st.style},
		})
		if selected {
			row = StyleKey.Render("▌") + lipgloss.NewStyle().Background(ColorSelBg).Render(" ") + row
		} else {
			row = "  " + row
		}
		lines = append(lines, row)
	}
	return panelBox(title, fitExactLines(lines, viewport), width, height, true)
}

// worktreeDetail é o que a linha não mostra: assunto do commit e o motivo da
// trava ou da poda. Sem worktree além da principal, vira o estado vazio.
func (a *App) worktreeDetail(p *core.Project, width int) []string {
	if a.wtErr != "" || len(a.wtList) == 0 {
		return nil
	}
	if len(a.wtList) == 1 {
		dest := collectors.GitWorktreePath(a.wtMainPath(p), "<branch>")
		return []string{
			rule(width),
			StyleMuted.Render(truncate("  só a worktree principal — n abre outra branch em "+shortenPath(dest), width)),
		}
	}
	wt, ok := a.selectedWorktree()
	if !ok {
		return nil
	}
	head := emDash
	if len(wt.Head) >= 7 {
		head = wt.Head[:7]
	}
	commit := []string{StyleMuted.Render(head)}
	if wt.Subject != "" {
		commit = append(commit, StyleNormal.Render(wt.Subject))
	}
	var state []string
	if wt.Main {
		state = append(state, StyleMuted.Render("principal"))
	}
	st := worktreeState(wt)
	state = append(state, st.style.Render(st.text))
	// A palavra da tabela mostra um estado só; trava com alteração ainda é
	// trava, e o motivo só cabe aqui.
	if wt.Locked && (wt.LockReason != "" || st.text != "travada") {
		state = append(state, StyleWarning.Render(strings.TrimSuffix("travada: "+wt.LockReason, ": ")))
	}
	if wt.PruneReason != "" {
		state = append(state, StyleMuted.Render(wt.PruneReason))
	}
	return []string{
		rule(width),
		factLine("pasta", []string{StyleNormal.Render(elideLeft(shortenPath(wt.Path), maxInt(10, width-14)))}, width),
		factLine("commit", commit, width),
		factLine("estado", state, width),
	}
}

type stateLabel struct {
	text  string
	style lipgloss.Style
}

// worktreeState: uma palavra por linha da tabela. Pasta sumida ganha de tudo
// (não há o que abrir); alteração ganha de trava (é o que se perde).
func worktreeState(w collectors.GitWorktree) stateLabel {
	switch {
	case w.Prunable:
		return stateLabel{"pasta sumiu", StyleUnhealthy}
	case w.Bare:
		return stateLabel{"bare", StyleMuted}
	case w.Changes > 0:
		return stateLabel{countOf(w.Changes, "alteração", "alterações"), StyleWarning}
	case w.Locked:
		return stateLabel{"travada", StyleWarning}
	}
	return stateLabel{"limpa", StyleHealthy}
}

// worktreeFolder: as irmãs da principal (onde o DevScope cria) aparecem só
// pelo nome da pasta — o prefixo é o mesmo em toda linha, e o caminho inteiro
// está no detalhe logo abaixo.
func worktreeFolder(mainPath, path string) string {
	if rel, err := filepath.Rel(filepath.Dir(mainPath), path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return shortenPath(path)
}

func worktreeBranchLabel(w collectors.GitWorktree) string {
	switch {
	case w.Branch != "":
		return w.Branch
	case w.Bare:
		return "(bare)"
	}
	return "(detached)"
}

// worktreeGlyph: ◆ é onde você está — o projeto aberto. Os glifos de estado
// são distintos para a tela ler sem cor (§1).
func worktreeGlyph(w collectors.GitWorktree, p *core.Project) (string, lipgloss.Style) {
	switch {
	case p != nil && pathsMatch(w.Path, p.Path):
		return "◆", StyleKey
	case w.Prunable:
		return "⚠", StyleUnhealthy
	case w.Locked:
		return "⚿", StyleWarning
	}
	return "·", StyleMuted
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		// O git escreve "Preparing worktree…" antes do "fatal:" — a última
		// linha é a que explica.
		return strings.TrimSpace(s[i+1:])
	}
	return s
}
