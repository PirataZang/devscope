package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func releaseTestApp(t *testing.T, st collectors.ReleaseState) (*App, *core.Project) {
	t.Helper()
	p := &core.Project{Name: "app", Path: "/p/app", Git: &core.GitInfo{IsRepo: true, Branch: "main"}}
	a := &App{width: 140, height: 44, view: ViewProject, tab: TabReleases, selectedProject: p,
		snapshot: core.Snapshot{Projects: []core.Project{*p}}, relPath: p.Path}
	a.handleReleaseLoaded(relLoadedMsg{path: p.Path, state: st})
	return a, p
}

func readyRelease() collectors.ReleaseState {
	return collectors.ReleaseState{
		Last:     collectors.ReleaseTag{Name: "v1.6.9", Date: time.Now().Add(-23 * 24 * time.Hour)},
		Previous: []collectors.ReleaseTag{{Name: "v1.6.8", Date: time.Now().Add(-37 * 24 * time.Hour)}},
		Commits:  []string{"67a16b7 Add Worktrees tab", "54c10dd Remove Routes"},
		Count:    2, Branch: "main", Default: "main",
		Pending:   &core.GitPending{Upstream: true},
		Workflow:  "release.yml",
		Changelog: "/p/app/CHANGELOG.md",
		Origin:    "git@github.com:PirataZang/devscope.git",
		GH:        true,
	}
}

func TestReleaseReadyToLaunch(t *testing.T) {
	a, p := releaseTestApp(t, readyRelease())
	got := stripANSI(a.renderReleasesTab(p))
	for _, want := range []string{
		"⇡ RELEASES", "última   v1.6.9", "2 commits desde então",
		"patch v1.6.10", "minor v1.7.0", "VERIFICAÇÕES", "◆ árvore limpa", "◆ tudo enviado",
		"COMO FUNCIONA", "◆ release.yml", "roda quando a tag chega",
		"◆ origin", "PirataZang/devscope", "COMMITS DESDE v1.6.9 · 2", "Add Worktrees tab", "enter lançar v1.6.10",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if !a.gitConfirmOn || a.gitConfirmAction != "release" || a.gitConfirmBranch != "v1.7.0" {
		t.Fatalf("→ escolhe minor e enter confirma v1.7.0: %v %q", a.gitConfirmOn, a.gitConfirmBranch)
	}
	if box := stripANSI(a.renderGitConfirm()); !strings.Contains(box, "Lançar v1.7.0") || !strings.Contains(box, "2 commits entram") {
		t.Fatalf("confirmação:\n%s", box)
	}
}

// O que bloqueia não deixa nem abrir a confirmação, e diz o porquê.
func TestReleaseBlockedByUnpushedWork(t *testing.T) {
	st := readyRelease()
	st.Pending = &core.GitPending{Upstream: true, Ahead: 2, Changes: 1}
	a, p := releaseTestApp(t, st)
	got := stripANSI(a.renderReleasesTab(p))
	for _, want := range []string{"⚠ 1 alteração sem commit", "⚠ 2 commits não enviados", "enter ver o que falta"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitConfirmOn || !strings.Contains(a.relStatus, "sem commit") {
		t.Fatalf("bloqueado não confirma: %v %q", a.gitConfirmOn, a.relStatus)
	}
}

// A versão sugerida é a que o CHANGELOG já tem — o autor decidiu ali.
func TestReleaseSuggestsBumpFromChangelog(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(f, []byte("## [Unreleased]\n\n## [1.7.0] - 2026-10-18\n- novo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := readyRelease()
	st.Changelog = f
	a, p := releaseTestApp(t, st)
	if a.relNext() != "v1.7.0" {
		t.Fatalf("sugeriu %s", a.relNext())
	}
	if got := stripANSI(a.renderReleasesTab(p)); !strings.Contains(got, "◆ CHANGELOG tem a seção 1.7.0") {
		t.Fatalf("faltou a verificação do CHANGELOG:\n%s", got)
	}
}

// Depois do push a tela acompanha o workflow da tag até ele terminar.
func TestReleaseWatchesWorkflow(t *testing.T) {
	a, p := releaseTestApp(t, readyRelease())
	a.relWatch = "v1.7.0"
	if cmd := a.handleReleaseRuns(relRunsMsg{path: p.Path, runs: map[string]collectors.GHARun{
		"v1.7.0": {Status: "in_progress", Workflow: "Release"},
		"v1.6.9": {Status: "completed", Conclusion: "success", Workflow: "Release"},
	}}); cmd == nil {
		t.Fatal("rodando: continua acompanhando")
	}
	got := stripANSI(a.renderReleasesTab(p))
	for _, want := range []string{"v1.7.0", "Release rodando…", "◆ v1.6.9", "publicada · Release"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
	if cmd := a.handleReleaseRuns(relRunsMsg{path: p.Path, runs: map[string]collectors.GHARun{
		"v1.7.0": {Status: "completed", Conclusion: "failure", Workflow: "Release"},
	}}); cmd != nil || a.relWatch != "" || !strings.Contains(a.relStatus, "failure") {
		t.Fatalf("terminou: para e avisa — %q", a.relStatus)
	}
}

// Repositório sem nada configurado: a tela ensina o que cada peça faz e como
// criar, e só o remoto bloqueia — o resto é opcional.
func TestReleaseGuideTeachesWhatIsMissing(t *testing.T) {
	a, p := releaseTestApp(t, collectors.ReleaseState{
		Commits: []string{"abc primeiro"}, Count: 1, Branch: "main", Default: "main",
		Pending: &core.GitPending{},
	})
	got := stripANSI(a.renderReleasesTab(p))
	for _, want := range []string{
		"nenhuma release ainda",
		"◌ tags vX.Y.Z", "a primeira release começa a numeração",
		"◌ CHANGELOG.md", "w cria, com instruções para a IA",
		"◌ release.yml", "w cria .github/workflows/release.yml", "a tag só marca o commit",
		"◌ gh", "gh auth login",
		"⚠ origin", "obrigatório: git remote add origin",
		"⚠ sem remoto origin",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
}

// Pedir o release sem workflow oferece criar antes; recusar uma vez e o próximo
// enter lança assim mesmo.
func TestReleaseOffersWorkflowBeforeLaunch(t *testing.T) {
	st := readyRelease()
	st.Workflow = ""
	a, p := releaseTestApp(t, st)
	if got := stripANSI(a.renderReleasesTab(p)); !strings.Contains(got, "w criar release.yml") || strings.Contains(got, "e CHANGELOG.md") {
		t.Fatalf("a barra anuncia o w:\n%s", got)
	}

	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitConfirmAction != "release-setup" {
		t.Fatalf("sem workflow, enter oferece criar antes: %q", a.gitConfirmAction)
	}
	box := stripANSI(a.renderGitConfirm())
	for _, want := range []string{"Preparar o release", "release.yml (GitHub Release)", "commit só desses arquivos"} {
		if !strings.Contains(box, want) {
			t.Fatalf("faltou %q:\n%s", want, box)
		}
	}

	a.updateGitConfirm(tea.KeyMsg{Type: tea.KeyEsc})
	if !a.relSkipSetup || !strings.Contains(a.relStatus, "enter de novo lança") {
		t.Fatalf("recusar explica o que acontece: %q", a.relStatus)
	}
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if a.gitConfirmAction != "release" || a.gitConfirmBranch != "v1.6.10" {
		t.Fatalf("depois de recusar, enter lança: %q %q", a.gitConfirmAction, a.gitConfirmBranch)
	}
}

func TestReleaseWKeyWithExistingWorkflow(t *testing.T) {
	a, p := releaseTestApp(t, readyRelease())
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}, p)
	if a.gitConfirmOn || !strings.Contains(a.relStatus, "já existem") {
		t.Fatalf("com workflow, w só avisa: %v %q", a.gitConfirmOn, a.relStatus)
	}
}

// Faltando os dois, uma confirmação só cria os dois — e diz que o CHANGELOG
// vem com as instruções para a IA.
func TestReleaseSetupCreatesBoth(t *testing.T) {
	st := readyRelease()
	st.Workflow, st.Changelog = "", ""
	a, p := releaseTestApp(t, st)
	if got := stripANSI(a.renderReleasesTab(p)); !strings.Contains(got, "w criar release.yml e CHANGELOG.md") {
		t.Fatalf("a barra anuncia os dois:\n%s", got)
	}
	a.handleReleaseKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")}, p)
	box := stripANSI(a.renderGitConfirm())
	for _, want := range []string{"Preparar o release", "release.yml (GitHub Release)", "release.yml (GitHub Release) · CHANGELOG.md", "CHANGELOG com instruções para IA"} {
		if !strings.Contains(box, want) {
			t.Fatalf("faltou %q:\n%s", want, box)
		}
	}
}
