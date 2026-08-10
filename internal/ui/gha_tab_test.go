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

func TestAllTabsIncludesActions(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabActions {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabActions missing from AllTabs")
	}
	if TabActions.String() != "GH Actions" {
		t.Fatalf("String=%q", TabActions.String())
	}
}

func TestSidebarShowsActionsInAutomation(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabActions}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "AUTOMATION") || !strings.Contains(got, "GH Actions") {
		t.Fatalf("sidebar missing GH Actions in AUTOMATION: %q", got)
	}
}

func TestGHALandingEnterAndEsc(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yml"), []byte("name: CI\n"), 0o644)
	p := core.Project{Path: dir, Name: "demo", Git: &core.GitInfo{IsRepo: true, Remote: "git@github.com:acme/demo.git"}}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterGHATab(&p)
	if a.tab != TabActions || a.ghaOpen {
		t.Fatalf("landing tab=%v open=%v", a.tab, a.ghaOpen)
	}
	landing := stripANSI(a.renderGHALanding(&p))
	if !strings.Contains(landing, "ACTIONS") || !strings.Contains(landing, "enter") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.ghaOpen {
		t.Fatal("enter should open client")
	}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.ghaOpen || a.tab != TabActions {
		t.Fatalf("esc landing open=%v tab=%v", a.ghaOpen, a.tab)
	}
}

func TestGHAControlCenterRenders(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/p"}
	a := &App{
		width:     120,
		height:    40,
		ghaOpen:   true,
		ghaKind:   ghaKindProcesses,
		ghaScreen: ghaScrCluster,
		ghaInfo: collectors.GHAInfo{
			Available: true,
			Authed:    true,
			Owner:     "acme",
			Repo:      "demo",
		},
		ghaProcesses: []collectors.GHAProcess{
			{Name: "ci", File: ".github/workflows/ci.yml", Description: "CI"},
		},
		ghaRuns: []collectors.GHARun{
			{ID: "1", Workflow: "ci", Status: "completed", Conclusion: "success", Branch: "main"},
		},
	}
	got := stripANSI(a.renderGHATab(&p))
	for _, want := range []string{"GITHUB ACTIONS", "PROCESSES", "RUNS", "USO", "AÇÕES"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestGHAUsagePanelAndCard(t *testing.T) {
	a := &App{
		width: 120, height: 40, ghaOpen: true, ghaScreen: ghaScrCluster,
		ghaBilling: collectors.GHAActionsBilling{
			OK: true, Included: 2000, Used: 500, Remaining: 1500, Source: "org", DaysLeft: 12,
		},
		ghaProcesses: []collectors.GHAProcess{
			{Name: "android-release", File: ".github/workflows/android-release.yml"},
			{Name: "ci", File: ".github/workflows/ci.yml"},
		},
		ghaRuns: []collectors.GHARun{
			{Workflow: "android-release", StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:20:00Z"},
			{Workflow: "ci", StartedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:05:00Z"},
		},
	}
	panel := stripANSI(a.renderGHAUsagePanel(36, 16))
	for _, want := range []string{"Restante", "1500m", "Projeto", "Por processo", "android-release", "ci"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("usage panel missing %q:\n%s", want, panel)
		}
	}
	title, val, _ := a.ghaUsageCardBits(20)
	if title != "MIN LEFT" || !strings.Contains(val, "1500m") {
		t.Fatalf("card title=%q val=%q", title, val)
	}

	a.ghaProcName = "android-release"
	a.ghaProcFile = ".github/workflows/android-release.yml"
	ov := stripANSI(a.renderGHAProcOverview(80, 20, ghaProcLive{Label: "failure"}))
	for _, want := range []string{"uso Actions", "Restante", "Processo", "Projeto"} {
		if !strings.Contains(ov, want) {
			t.Fatalf("overview missing %q:\n%s", want, ov)
		}
	}
}

func TestGHAKindCycle(t *testing.T) {
	a := &App{ghaOpen: true, ghaKind: ghaKindProcesses, ghaScreen: ghaScrCluster}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.ghaKind != ghaKindRuns {
		t.Fatalf("got %v", a.ghaKind)
	}
}

func TestGHAEnterOpensProcessDetail(t *testing.T) {
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	_ = os.MkdirAll(wf, 0o755)
	_ = os.WriteFile(filepath.Join(wf, "ci.yml"), []byte("name: CI\non: push\n"), 0o644)
	a := &App{
		ghaOpen:   true,
		ghaKind:   ghaKindProcesses,
		ghaScreen: ghaScrCluster,
		ghaFocus:  ghaFocusTable,
		ghaPath:   dir,
		ghaProcesses: []collectors.GHAProcess{
			{Name: "ci", File: ".github/workflows/ci.yml"},
		},
		ghaTriggered: map[string]time.Time{},
	}
	_, cmd := a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyEnter}, &core.Project{})
	if a.ghaScreen != ghaScrProcess {
		t.Fatalf("screen=%v want process detail", a.ghaScreen)
	}
	if a.ghaProcName != "ci" {
		t.Fatalf("proc=%q", a.ghaProcName)
	}
	_ = cmd
	got := stripANSI(a.renderGHAProcessDetail(&core.Project{Name: "demo"}))
	if !strings.Contains(got, "ACTION") || !strings.Contains(got, "Overview") {
		t.Fatalf("%q", got)
	}
}

func TestGHAProcessStatusFromRuns(t *testing.T) {
	a := &App{
		ghaRuns: []collectors.GHARun{
			{ID: "9", Workflow: "Android develop", Status: "in_progress", Event: "workflow_dispatch"},
		},
		ghaTriggered: map[string]time.Time{},
	}
	live := a.ghaStatusForProcess("android-develop", ".github/workflows/android-develop.yml")
	if live.Label != "running" {
		t.Fatalf("label=%q", live.Label)
	}
}

func TestGHATabCyclesPanels(t *testing.T) {
	a := &App{ghaOpen: true, ghaScreen: ghaScrCluster, ghaFocus: ghaFocusTable}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.ghaFocus != ghaFocusResumo {
		t.Fatalf("got %d", a.ghaFocus)
	}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.ghaFocus != ghaFocusActions {
		t.Fatalf("got %d", a.ghaFocus)
	}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.ghaFocus != ghaFocusTable {
		t.Fatalf("got %d", a.ghaFocus)
	}
}

func TestGHAKindSwitchOnlyOnTable(t *testing.T) {
	a := &App{ghaOpen: true, ghaScreen: ghaScrCluster, ghaFocus: ghaFocusResumo, ghaKind: ghaKindProcesses}
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.ghaKind != ghaKindProcesses {
		t.Fatal("] should not change kind outside table focus")
	}
	a.ghaFocus = ghaFocusTable
	_, _ = a.handleGHAKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.ghaKind != ghaKindRuns {
		t.Fatalf("got %v", a.ghaKind)
	}
}

func TestGHASetupModalWhenNoGH(t *testing.T) {
	a := &App{
		ghaOpen:   true,
		ghaScreen: ghaScrCluster,
		ghaInfo:   collectors.GHAInfo{Available: false, Owner: "acme", Repo: "demo"},
	}
	a.ghaShowSetup()
	if a.ghaForm != ghaFormSetup {
		t.Fatalf("form=%v", a.ghaForm)
	}
	box := stripANSI(a.renderGHASetupBox())
	if !strings.Contains(box, "NÃO INSTALADO") || !strings.Contains(box, "apt install gh") {
		t.Fatalf("%q", box)
	}
	_, _ = a.handleGHAFormKeys(tea.KeyMsg{Type: tea.KeyEnter})
	if a.ghaScreen != ghaScrCluster || a.ghaForm != ghaFormNone {
		t.Fatalf("should dismiss setup")
	}
}

func TestGHASetupModalAuth(t *testing.T) {
	a := &App{ghaInfo: collectors.GHAInfo{Available: true, Authed: false}}
	a.ghaShowSetup()
	box := stripANSI(a.renderGHASetupBox())
	if !strings.Contains(box, "AUTENTICAÇÃO") || !strings.Contains(box, "Login") {
		t.Fatalf("%q", box)
	}
}

func TestGHAOpenBrowserUsesRemote(t *testing.T) {
	a := &App{
		ghaOpen:   true,
		ghaKind:   ghaKindProcesses,
		ghaRemote: "git@github.com:acme/demo.git",
		ghaProcesses: []collectors.GHAProcess{
			{Name: "ci", File: ".github/workflows/ci.yml"},
		},
	}
	owner, repo := a.ghaResolveOwnerRepo()
	if owner != "acme" || repo != "demo" {
		t.Fatalf("%s/%s", owner, repo)
	}
	url := collectors.GHAActionsURL(owner, repo, a.ghaProcesses[0].File)
	if !strings.Contains(url, "github.com/acme/demo/actions/workflows/ci.yml") {
		t.Fatalf("%q", url)
	}
}

func TestGHAAskStopJobConfirm(t *testing.T) {
	a := &App{
		ghaOpen:   true,
		ghaScreen: ghaScrCluster,
		ghaKind:   ghaKindProcesses,
		ghaInfo:   collectors.GHAInfo{Available: true, Authed: true},
		ghaProcesses: []collectors.GHAProcess{
			{Name: "ci", File: ".github/workflows/ci.yml"},
		},
		ghaRuns: []collectors.GHARun{
			{ID: "99", Workflow: "ci", Status: "in_progress", Name: "ci"},
		},
	}
	cmd := a.ghaAskStopJob()
	if cmd != nil {
		t.Fatal("ask stop should only set confirm")
	}
	if !a.ghaConfirm || !strings.HasPrefix(a.ghaConfirmAction, "stop-run:99") {
		t.Fatalf("confirm=%v action=%q", a.ghaConfirm, a.ghaConfirmAction)
	}
	hint := a.ghaConfirmHint()
	if !strings.Contains(hint, "y confirma") {
		t.Fatalf("hint=%q", hint)
	}
	modal := stripANSI(renderDeleteConfirmBox(a.ghaConfirmOpts(), 80, 24))
	if !strings.Contains(modal, "Parar job") || !strings.Contains(modal, "#99") {
		t.Fatalf("modal=%q", modal)
	}
	// idle process → nothing to stop
	a.ghaConfirm = false
	a.ghaConfirmAction = ""
	a.ghaRuns[0].Status = "completed"
	a.ghaRuns[0].Conclusion = "success"
	_ = a.ghaAskStopJob()
	if a.ghaConfirm {
		t.Fatal("should not confirm when no active job")
	}
}

func TestGHAAskStopAllJobs(t *testing.T) {
	a := &App{
		ghaOpen:   true,
		ghaScreen: ghaScrCluster,
		ghaKind:   ghaKindProcesses,
		ghaInfo:   collectors.GHAInfo{Available: true, Authed: true},
		ghaProcesses: []collectors.GHAProcess{
			{Name: "deploy", File: ".github/workflows/deploy.yml"},
		},
		ghaRuns: []collectors.GHARun{
			{ID: "1", Workflow: "deploy", Status: "queued"},
			{ID: "2", Workflow: "deploy", Status: "in_progress"},
			{ID: "3", Workflow: "deploy", Status: "completed", Conclusion: "success"},
		},
	}
	_ = a.ghaAskStopAllJobs()
	if !a.ghaConfirm || a.ghaConfirmAction != "stop-all:deploy:2" {
		t.Fatalf("action=%q", a.ghaConfirmAction)
	}
}

func TestGHARunIsActive(t *testing.T) {
	if !ghaRunIsActive(collectors.GHARun{Status: "in_progress"}) {
		t.Fatal("in_progress")
	}
	if ghaRunIsActive(collectors.GHARun{Status: "completed"}) {
		t.Fatal("completed")
	}
}

func TestGHAOpenBrowserUsesSelectedRun(t *testing.T) {
	a := &App{
		ghaOpen:          true,
		ghaScreen:        ghaScrProcess,
		ghaProcTab:       ghaProcTabRuns,
		ghaProcName:      "android-develop",
		ghaProcFile:      ".github/workflows/android-develop.yml",
		ghaRemote:        "git@github.com:acme/demo.git",
		ghaProcRunCursor: 0,
		ghaProcRunID:     "old-completed", // stale — must sync from cursor
		ghaRuns: []collectors.GHARun{
			{ID: "111", Workflow: "android-develop", Status: "in_progress", URL: "https://github.com/acme/demo/actions/runs/111"},
			{ID: "222", Workflow: "android-develop", Status: "completed", Conclusion: "success", URL: "https://github.com/acme/demo/actions/runs/222"},
		},
	}
	// Don't actually open browser — just verify sync + URL resolution path.
	a.ghaSyncSelectedProcRun()
	if a.ghaProcRunID != "111" {
		t.Fatalf("run id=%q want 111", a.ghaProcRunID)
	}
	owner, repo := a.ghaResolveOwnerRepo()
	url := ""
	for _, r := range a.ghaRuns {
		if r.ID == a.ghaProcRunID {
			url = r.URL
			break
		}
	}
	if url != "https://github.com/acme/demo/actions/runs/111" {
		t.Fatalf("url=%q owner=%s/%s", url, owner, repo)
	}
}

func TestGHATickIntervalFasterWhenActive(t *testing.T) {
	a := &App{ghaOpen: true, ghaScreen: ghaScrCluster}
	if a.ghaTickInterval() != 6*time.Second {
		t.Fatalf("idle=%v", a.ghaTickInterval())
	}
	a.ghaRuns = []collectors.GHARun{{ID: "1", Status: "in_progress"}}
	if a.ghaTickInterval() != 3*time.Second {
		t.Fatalf("active=%v", a.ghaTickInterval())
	}
	a.ghaRuns = nil
	a.ghaScreen = ghaScrProcess
	if a.ghaTickInterval() != 3*time.Second {
		t.Fatalf("process=%v", a.ghaTickInterval())
	}
}

func TestGHATickKeepsChainOnConfirm(t *testing.T) {
	a := &App{ghaOpen: true, ghaGen: 1, ghaConfirm: true, ghaScreen: ghaScrCluster}
	_, cmd := a.handleGHAMsg(ghaTickMsg{gen: 1})
	if cmd == nil {
		t.Fatal("should reschedule tick while confirming")
	}
}

func TestGHABeginTriggerSelectsBranch(t *testing.T) {
	a := &App{
		ghaOpen: true,
		ghaPath: t.TempDir(),
		ghaKind: ghaKindProcesses,
		ghaInfo: collectors.GHAInfo{Available: true, Authed: true},
		ghaProcesses: []collectors.GHAProcess{
			{Name: "ci", File: ".github/workflows/ci.yml"},
		},
	}
	_ = a.ghaBeginTrigger("ci", ".github/workflows/ci.yml", "ci.yml", ghaScrCluster)
	if a.ghaForm != ghaFormTrigger || a.ghaScreen != ghaScrForm {
		t.Fatalf("form=%v screen=%v", a.ghaForm, a.ghaScreen)
	}
	// inject pushed branches (collector covered in collectors tests)
	a.ghaTriggerBranches = []string{"main", "develop", "feature/x"}
	a.ghaTriggerCursor = 1
	box := stripANSI(a.renderGHATriggerBox())
	if !strings.Contains(box, "TRIGGER") || !strings.Contains(box, "develop") {
		t.Fatalf("%q", box)
	}
	cmd := a.ghaConfirmTrigger()
	if cmd == nil {
		t.Fatal("expected trigger cmd")
	}
	if a.ghaForm != ghaFormNone {
		t.Fatal("form should close")
	}
	if !strings.Contains(a.ghaStatus, "develop") {
		t.Fatalf("status=%q", a.ghaStatus)
	}
}

func TestGHAFilteredRunsScopeAndProcess(t *testing.T) {
	a := &App{
		ghaKind: ghaKindRuns,
		ghaRuns: []collectors.GHARun{
			{ID: "1", Workflow: "Android develop", Status: "in_progress"},
			{ID: "2", Workflow: "Android develop", Status: "completed", Conclusion: "success"},
			{ID: "3", Workflow: "iOS release", Status: "queued"},
			{ID: "4", Workflow: "iOS release", Status: "completed", Conclusion: "failure"},
		},
	}
	if n := len(a.ghaFilteredRuns()); n != 4 {
		t.Fatalf("all=%d", n)
	}
	a.ghaCycleRunScope()
	if a.ghaRunScope != ghaRunScopeRunning {
		t.Fatal("scope")
	}
	got := a.ghaFilteredRuns()
	if len(got) != 2 {
		t.Fatalf("running=%d", len(got))
	}
	a.ghaRunProcFilter = "android-develop"
	got = a.ghaFilteredRuns()
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("%+v", got)
	}
	a.ghaCycleRunProcFilter(1) // next process option
	if a.ghaRunProcFilter == "android-develop" {
		// may cycle to another; just ensure cursor reset path works
	}
	a.ghaRunScope = ghaRunScopeAll
	a.ghaRunProcFilter = ""
	a.ghaKind = ghaKindProcesses
	a.ghaProcesses = []collectors.GHAProcess{{Name: "ios-release"}}
	a.ghaCursor = 0
	a.ghaSeedRunFilterFromProcess()
	a.ghaKind = ghaKindRuns
	if a.ghaRunProcFilter != "ios-release" {
		t.Fatalf("seed=%q", a.ghaRunProcFilter)
	}
}
