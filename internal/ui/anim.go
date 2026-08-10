package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ~10 fps (8–12 range). Separate from tickMsg so store/git sync stay at 300ms.
const animInterval = 100 * time.Millisecond

type animTickMsg struct{}

// Braille spinner — 10 frames.
var animSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Soft pulse for online / healthy dots.
var animPulseFrames = []string{"●", "◉", "○", "◉"}

// Arc spinner for "starting" / queued (4 frames, still reads at 10fps).
var animArcFrames = []string{"◐", "◓", "◑", "◒"}

func scheduleAnimTick() tea.Cmd {
	return tea.Tick(animInterval, func(t time.Time) tea.Msg {
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

func (a *App) arc() string {
	return animArc(a.animFrame)
}

// loadingText renders "⠋ carregando…" with the current spinner frame.
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
	if a.gitBranchLoading || a.gitCommitFilesLoading || a.gitActionLoading || a.gitCommitDiffLoading ||
		a.dockerAddLoading || a.dockerAddDetailsLoading || a.dockerAddTagsLoading ||
		a.containerDetailLoading || a.apiLoading || a.dbLoading || a.dbSchemaLoading ||
		a.k8sLoading || a.swarmLoading || a.routesLoading ||
		a.ngrokLoading || a.cfLoading || a.sshLoading || a.jenkinsLoading || a.ghaLoading ||
		a.projectLogsLoading || a.projectGitLoading || a.projectDockerLoading {
		return true
	}
	return false
}

func (a *App) kickAnim() tea.Cmd {
	if !a.needsAnim() {
		return nil
	}
	return scheduleAnimTick()
}
