package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/config"
)

type projectShellDoneMsg struct {
	err error
}

func (a *App) projectExecShell(path string) tea.Cmd {
	cmd := collectors.ProjectShell(path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return projectShellDoneMsg{err: err}
	})
}

// projectExecAI abre o agente de IA configurado no diretório do projeto.
func (a *App) projectExecAI(path string) tea.Cmd {
	cmd, name, err := collectors.ProjectAI(path, a.aiCommand())
	if err != nil {
		a.statusMsg = err.Error() + " — Shift+C configura tools.ai"
		return nil
	}
	a.statusMsg = "abrindo " + name + "…"
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return projectShellDoneMsg{err: err}
	})
}

// projectExecConfig abre o config.yaml no editor e recarrega o que mudou.

// aiCommand lê a config sem explodir quando ela não existe (os testes montam
// o App na mão).
func (a *App) aiCommand() string {
	if a.cfg == nil {
		return ""
	}
	return a.cfg.Tools.AI
}

func (a *App) handleProjectShellDone(msg projectShellDoneMsg) {
	a.dashboardSubview = dashboardSubviewShellReturn
	if msg.err != nil {
		a.projectShellExitErr = "terminal: " + msg.err.Error()
	} else {
		a.projectShellExitErr = ""
	}
}

func (a *App) dismissProjectShellReturn() {
	a.dashboardSubview = dashboardSubviewList
	a.projectShellExitErr = ""
	a.snapshot = a.store.Get()
	projects := filterNestedProjects(sortProjects(a.filteredProjects()))
	a.syncDashboardScroll(len(projects))
}

// openProjectAI: com tools.ai configurado, abre direto. Sem configuração e com
// mais de um agente instalado, pergunta uma vez e guarda a resposta — assim a
// escolha mais comum não precisa de tela de configuração.
func (a *App) openProjectAI(path string) tea.Cmd {
	if a.aiCommand() != "" {
		return a.projectExecAI(path)
	}
	found := collectors.DetectAITools()
	if len(found) > 1 {
		a.aiPickerOn = true
		a.aiPickerOpts = found
		a.aiPickerIdx = 0
		a.aiPickerPath = path
		return nil
	}
	return a.projectExecAI(path)
}

func (a *App) updateAIPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.aiPickerOn = false
		a.statusMsg = ""
	case "up", "k":
		if a.aiPickerIdx > 0 {
			a.aiPickerIdx--
		}
	case "down", "j":
		if a.aiPickerIdx < len(a.aiPickerOpts)-1 {
			a.aiPickerIdx++
		}
	case "enter", " ":
		name := a.aiPickerOpts[a.aiPickerIdx]
		a.aiPickerOn = false
		if a.cfg != nil {
			a.cfg.Tools.AI = name
		}
		if err := config.SaveUserAI(name); err != nil {
			a.statusMsg = "não consegui salvar tools.ai: " + err.Error()
		}
		return a, a.projectExecAI(a.aiPickerPath)
	}
	return a, nil
}

// aiToolLabel: a barra de comandos mostra o que vai rodar de verdade, não um
// nome fixo. Sem nenhum agente instalado, o atalho continua existindo — ele é
// que ensina a configurar.
func (a *App) aiToolLabel() string {
	if cmd := a.aiCommand(); cmd != "" {
		return strings.Fields(cmd)[0]
	}
	if found := collectors.DetectAITools(); len(found) > 0 {
		return found[0]
	}
	return "agente ia"
}

func (a *App) renderAIPickerPopup(background string) string {
	lines := []string{
		StyleSection.Render("Agente de IA"),
		StyleMuted.Render("↑↓ escolhe  ·  enter abre e salva  ·  esc cancela"),
		"",
	}
	for i, name := range a.aiPickerOpts {
		mark, label := "  ", StyleNormal.Render(name)
		if i == a.aiPickerIdx {
			mark, label = StyleSelected.Render("▸ "), StyleSelected.Render(name)
		}
		lines = append(lines, mark+label)
	}
	lines = append(lines, "",
		StyleMuted.Render("salvo em tools.ai  ·  Shift+C edita o config"))
	boxWidth := minInt(60, maxInt(40, a.width-6))
	box := StylePanel.Width(boxWidth).Background(ColorBgPanel).Render(strings.Join(lines, "\n"))
	return overlayCentered(background, box, a.width, a.height)
}

type appLaunchedMsg struct {
	name string
	err  error
}

// runAppShortcut abre o programa da tecla. App de janela sai solto e devolve o
// foco na hora; comando de terminal assume a tela e volta quando termina.
func (a *App) runAppShortcut(key, dir string) tea.Cmd {
	if a.cfg == nil {
		return nil
	}
	app, ok := a.cfg.AppShortcutFor(key)
	if !ok {
		return nil
	}
	cmd, detached := collectors.ExternalApp(dir, app.Command)
	if !detached {
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return appLaunchedMsg{name: app.Name, err: err}
		})
	}
	return func() tea.Msg {
		if err := cmd.Start(); err != nil {
			return appLaunchedMsg{name: app.Name, err: err}
		}
		// O shell sai assim que solta o processo; esperar por ele só evita o
		// zumbi — quem foi solto continua vivo, reparentado.
		go func() { _ = cmd.Wait() }()
		return appLaunchedMsg{name: app.Name}
	}
}

func (a *App) handleAppLaunched(msg appLaunchedMsg) {
	if msg.err != nil {
		a.statusMsg = msg.name + ": " + msg.err.Error()
		return
	}
	a.statusMsg = "abriu " + msg.name
}
