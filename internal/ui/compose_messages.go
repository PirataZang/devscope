package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
)

type composeDoneMsg struct {
	action string
	err    error
}

// beginCompose marca um compose em andamento — a barra de progresso e o tick de
// animação leem daqui. Chamado no goroutine principal (dentro do Update), antes
// de devolver o tea.Cmd que roda o docker de fato.
func (a *App) beginCompose(verb string) {
	a.composeRunning = true
	a.composeVerb = verb
	a.composeStart = time.Now()
}

func (a *App) composeUp(path string) tea.Cmd {
	a.beginCompose("up")
	return a.runCompose(path, "up", collectors.ComposeUp)
}

func (a *App) composeBuild(path string) tea.Cmd {
	a.beginCompose("build")
	return a.runCompose(path, "up --build", collectors.ComposeBuild)
}

func (a *App) composeDown(path string) tea.Cmd {
	a.beginCompose("down")
	return a.runCompose(path, "down", collectors.ComposeDown)
}

func (a *App) composeRestart(path string) tea.Cmd {
	a.beginCompose("restart")
	return a.runCompose(path, "restart", collectors.ComposeRestart)
}

// composeVerbLabel é o gerúndio mostrado na barra ("buildando…").
func composeVerbLabel(v string) string {
	switch v {
	case "build":
		return "buildando"
	case "up":
		return "subindo"
	case "down":
		return "parando"
	case "restart":
		return "reiniciando"
	}
	return "compose"
}

// composeProgressBar desenha a barra ASCII animada enquanto o compose roda. Como
// o docker não dá % confiável em bloco, o preenchimento é assintótico pelo tempo
// (cresce rápido e desacelera perto do fim) e um pulso Braille garante que ela
// lê como "trabalhando", não travada. O 100% + ✓ vem no composeDoneMsg.
func composeProgressBar(verb string, elapsed time.Duration, frame, width int) string {
	inner := width - 26
	if inner < 10 {
		inner = 10
	}
	if inner > 40 {
		inner = 40
	}
	pct := 1 - math.Exp(-elapsed.Seconds()/6.0)
	if pct > 0.92 {
		pct = 0.92
	}
	filled := int(pct*float64(inner) + 0.5)
	if filled > inner {
		filled = inner
	}
	// Âmbar enquanto roda: build/up é TRANSIÇÃO (DESIGN §6, amarelo). O verde só
	// aparece no ✓ do composeDoneMsg — verde é "pronto", não "trabalhando".
	bar := StyleWarning.Render(strings.Repeat("█", filled)) +
		StyleMuted.Render(strings.Repeat("░", inner-filled))
	pulse := StyleWarning.Render(pulseGlyph(pulseWarn, frame))
	label := StyleWarning.Render(composeVerbLabel(verb) + "…")
	return pulse + " " + label + "  " +
		StyleMuted.Render("[") + bar + StyleMuted.Render("]") +
		StyleMuted.Render(fmt.Sprintf(" %2d%%", int(pct*100)))
}

func (a *App) runCompose(path, action string, fn func(string) error) tea.Cmd {
	store := a.store
	healthCfg := a.cfg.Health
	return func() tea.Msg {
		err := fn(path)
		collectors.RefreshProjectsDocker(store, path, healthCfg)
		return composeDoneMsg{action: action, err: err}
	}
}

func (a *App) noteCompose(msg string) {
	a.statusMsg = msg
	if a.tab == TabContainers {
		a.containerStatusMsg = msg
	}
}

func (a *App) handleComposeDone(msg composeDoneMsg) {
	a.composeRunning = false
	a.snapshot = a.store.Get()
	text := "compose " + msg.action + " ✓"
	if msg.err != nil {
		text = "compose " + msg.action + ": " + msg.err.Error()
	}
	a.noteCompose(text)
}
