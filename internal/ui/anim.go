package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ~10 fps (8–12 range). Separate from tickMsg so store/git sync stay at 300ms.
const animInterval = 100 * time.Millisecond

type animTickMsg struct{}

// Braille spinner — bolinhas circulando (8 frames).
var animSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// Pico de energia de baixo pra cima (Braille 2×4), sobe e desce.
var animPulseFrames = []string{"⣀", "⣤", "⣶", "⣿", "⣿", "⣶", "⣤", "⣀"}

// ─── vocabulário de status ──────────────────────────────────────────────────
//
// Todo status e todo loading do app usam Braille animado: a coluna sobe e
// desce. A ALTURA que a onda alcança diz o estado — cheia = saudável, meia =
// degradado, rasteira = parado — e a cor confirma. Assim o glifo continua
// distinguível num screenshot parado, onde só o movimento não ajudaria.

type pulseLevel int

const (
	pulseOK pulseLevel = iota
	pulseWarn
	pulseBad
	pulseIdle
)

var (
	// As quatro famílias são disjuntas de propósito: em QUALQUER quadro dá para
	// dizer o estado só pelo glifo, sem depender da cor. Isso importa em
	// screenshot parado e em terminal sem cor.
	//
	// Saudável: coluna dupla, respira entre a metade e o topo — nunca encosta
	// na base, que é território de "parado". 8 quadros ≈ 0,8 s por respiração.
	pulseFramesOK = []string{"⣤", "⣦", "⣶", "⣷", "⣿", "⣷", "⣶", "⣦"}
	// Degradado: só a coluna da esquerda — metade do sinal, e mais rápido.
	pulseFramesWarn = []string{"⡀", "⡄", "⡆", "⡇", "⡆", "⡄"}
	// Parado: reta na base com um tremor. Sem energia, mas não congelado.
	pulseFramesBad = []string{"⣀", "⣀", "⣀", "⣄", "⣀", "⣀"}
	// Sem status: fraquinha lá embaixo.
	pulseFramesIdle = []string{"⠄", "⠆", "⠄", "⠀"}
)

func pulseFrames(level pulseLevel) []string {
	switch level {
	case pulseWarn:
		return pulseFramesWarn
	case pulseBad:
		return pulseFramesBad
	case pulseIdle:
		return pulseFramesIdle
	default:
		return pulseFramesOK
	}
}

// pulseGlyph é o glifo de status do app inteiro. Use sempre este — nada de
// ●/○/◐ soltos, para o vocabulário não divergir entre telas.
func pulseGlyph(level pulseLevel, frame int) string {
	frames := pulseFrames(level)
	if len(frames) == 0 {
		return "⣀"
	}
	if frame < 0 {
		frame = -frame
	}
	return frames[frame%len(frames)]
}

// Bolinha na borda da célula — "starting" / queued.
var animArcFrames = []string{"⠁", "⠂", "⠄", "⡀", "⢀", "⠠", "⠐", "⠈"}

func scheduleAnimTick() tea.Cmd {
	return tea.Tick(animInterval, func(t time.Time) tea.Msg {
		return animTickMsg{}
	})
}

// scheduleRelaxTick é o frame de desenho do Relax (~30fps). A simulação segue
// em passos fixos dentro do relaxEngine; aqui só se pede o próximo quadro.
func scheduleRelaxTick() tea.Cmd {
	return tea.Tick(relaxRenderInterval, func(t time.Time) tea.Msg {
		return animTickMsg{}
	})
}

func animSpinner(frame int) string {
	if len(animSpinnerFrames) == 0 {
		return "…"
	}
	if frame < 0 {
		frame = -frame
	}
	return animSpinnerFrames[frame%len(animSpinnerFrames)]
}

func animPulse(frame int) string {
	if len(animPulseFrames) == 0 {
		return "●"
	}
	if frame < 0 {
		frame = -frame
	}
	return animPulseFrames[frame%len(animPulseFrames)]
}

func animPulseSlow(frame int) string {
	return animPulse(frame / 3)
}

const animStoppedGlyph = "⣀"

func animArc(frame int) string {
	if len(animArcFrames) == 0 {
		return "●"
	}
	if frame < 0 {
		frame = -frame
	}
	return animArcFrames[frame%len(animArcFrames)]
}

func (a *App) spinner() string {
	return animSpinner(a.animFrame)
}

func (a *App) pulse() string {
	return animPulse(a.animFrame)
}

// okPulse é o pulso de "saudável". a.pulse() usa animPulseFrames, que começa
// em ⣀ — o MESMO glifo de animStoppedGlyph, ou seja "parado". Num screenshot
// parado ou em terminal sem cor os dois viravam a mesma coisa, que é o que o
// §6 proíbe. pulseOK nunca encosta na base.
func (a *App) okPulse() string { return pulseGlyph(pulseOK, a.animFrame) }

func (a *App) livePulse(label string) string {
	g := a.pulse()
	if label == "" {
		return StyleHealthy.Render(g)
	}
	return StyleHealthy.Render(g + " " + label)
}

func (a *App) arc() string {
	return animArc(a.animFrame)
}

// loadingText renders "⣾ carregando…" with the current spinner frame.
func (a *App) loadingText(label string) string {
	if label == "" {
		label = "carregando…"
	}
	return StyleAccent.Render(a.spinner()) + " " + StyleMuted.Render(label)
}

func (a *App) loadingMuted(label string) string {
	if label == "" {
		label = "carregando…"
	}
	return StyleMuted.Render(a.spinner() + " " + label)
}

// needsAnim is true only while something on screen actually animates.
// Idle landings must not keep a 10fps tick (that re-renders View constantly).
func (a *App) needsAnim() bool {
	if a == nil {
		return false
	}
	if a.view == ViewRelax {
		return true
	}
	if a.gitBranchLoading || a.gitCommitFilesLoading || a.gitActionLoading || a.gitCommitDiffLoading ||
		a.dockerAddLoading || a.dockerAddDetailsLoading || a.dockerAddTagsLoading ||
		a.containerDetailLoading || a.apiLoading || a.dbLoading || a.dbSchemaLoading ||
		a.k8sLoading || a.swarmLoading || a.routesLoading ||
		a.ngrokLoading || a.cfLoading || a.sshLoading || a.jenkinsLoading || a.ghaLoading ||
		(a.ghaOpen && a.ghaHasActiveWork()) ||
		a.projectLogsLoading || a.projectGitLoading || a.projectDockerLoading {
		return true
	}
	return a.wantsPulseAnim()
}

// wantsPulseAnim: agora TODO status pulsa — parado treme na base, sem status
// pisca fraquinho —, então basta existir algo com estado na tela. Custa um
// redraw a 10 fps, que é o modo normal do app sempre que há algo no ar.
func (a *App) wantsPulseAnim() bool {
	if a.view == ViewDashboard {
		return len(a.snapshot.Projects) > 0
	}
	if a.currentProject() != nil {
		return true
	}
	if a.ghaOpen || (a.jenkinsOpen && a.jenkinsInfo.Connected) || a.wsConnected {
		return true
	}
	return false
}

// kickAnim starts the 10fps loop. animOn keeps a single chain alive — sem isso
// cada tick de 300ms criaria um loop novo em paralelo.
func (a *App) kickAnim() tea.Cmd {
	if a.animOn || !a.needsAnim() {
		return nil
	}
	a.animOn = true
	if a.view == ViewRelax {
		return scheduleRelaxTick()
	}
	return scheduleAnimTick()
}
