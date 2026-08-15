package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

// ~10 fps (8–12 range). Separate from tickMsg so store/git sync stay at 300ms.
const animInterval = 100 * time.Millisecond

type animTickMsg struct{}

// Braille spinner — bolinhas circulando (8 frames).
var animSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// Pico de energia de baixo pra cima (Braille 2×4), sobe e desce.
var animPulseFrames = []string{"⣀", "⣤", "⣶", "⣿", "⣿", "⣶", "⣤", "⣀"}

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

func (a *App) wantsPulseAnim() bool {
	if a.view == ViewDashboard {
		for _, p := range a.snapshot.Projects {
			if p.Health == core.HealthHealthy || p.Status == core.StatusRunning || p.Status == core.StatusDegraded {
				return true
			}
		}
		return false
	}
	if p := a.currentProject(); p != nil && (p.Health == core.HealthHealthy || p.Status == core.StatusRunning || p.Status == core.StatusDegraded) {
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
