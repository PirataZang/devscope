package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/jenkinsutil"
)

func TestAllTabsIncludesJenkins(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabJenkins {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabJenkins missing")
	}
	if TabJenkins.String() != "Jenkins" {
		t.Fatalf("String=%q", TabJenkins.String())
	}
}

func TestJenkinsSidebar(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabJenkins}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "Jenkins") {
		t.Fatalf("sidebar missing Jenkins: %q", got)
	}
}

func TestJenkinsLandingAndOpen(t *testing.T) {
	dir := t.TempDir()
	p := core.Project{Path: dir, Name: "demo"}
	a := &App{
		width: 120, height: 40, view: ViewProject, tab: TabOverview,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterJenkinsTab(&p)
	landing := stripANSI(a.renderJenkinsLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "JENKINS") {
		t.Fatalf("landing: %q", landing)
	}
	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.jenkinsOpen {
		t.Fatal("enter should open client")
	}
}

func TestJenkinsSettingsNoPanic(t *testing.T) {
	dir := t.TempDir()
	p := core.Project{Path: dir, Name: "demo"}
	a := &App{
		width: 100, height: 30, jenkinsOpen: true, jenkinsSubTab: jenkinsTabSettings,
	}
	a.loadJenkinsSettingsDraft(&p)
	view := stripANSI(a.renderJenkinsTab(&p))
	if !strings.Contains(view, "SETTINGS") {
		t.Fatalf("missing settings: %s", view)
	}
	a.jenkinsEditing = true
	a.jenkinsSetField = jenkinsSetURL
	a.jenkinsEditURL = "https://ci.example.com"
	a.jenkinsSetCursor = len([]rune(a.jenkinsEditURL))
	view = stripANSI(a.renderJenkinsSettings(&p, 80, 16))
	if !strings.Contains(view, "ci.example.com") {
		t.Fatalf("edit url missing: %s", view)
	}
}

func TestJenkinsPipelinesRender(t *testing.T) {
	a := &App{
		width: 120, height: 40, jenkinsOpen: true, jenkinsSubTab: jenkinsTabPipelines,
		jenkinsJobs: []jenkinsutil.Job{
			{Name: "app", FullName: "app", Status: "success", LastBuild: 12},
			{Name: "api", FullName: "api", Status: "failure", LastBuild: 3},
		},
		jenkinsBuilds: []jenkinsutil.Build{
			{Number: 12, Result: "SUCCESS", Duration: 40000, Timestamp: 1},
			{Number: 11, Result: "FAILURE", Duration: 12000, Timestamp: 1},
			{Number: 10, Result: "SUCCESS", Duration: 8000, Timestamp: 1},
		},
		jenkinsInfo: jenkinsutil.ServerInfo{Connected: true, Version: "2.452"},
		jenkinsCfg:  jenkinsutil.ProjectConfig{URL: "https://ci.example.com", User: "u", Token: "t"},
	}
	view := stripANSI(a.renderJenkinsTab(&core.Project{Name: "demo"}))
	for _, want := range []string{"devscope", "jenkins", "PIPELINES", "DETAILS", "AÇÕES", "app", "api", "ACTIVITY"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestJenkinsActivityChart(t *testing.T) {
	a := &App{
		jenkinsBuilds: []jenkinsutil.Build{
			{Number: 3, Result: "SUCCESS", Duration: 30000},
			{Number: 2, Result: "FAILURE", Duration: 10000},
			{Number: 1, Building: true, Duration: 5000},
		},
	}
	view := stripANSI(a.renderJenkinsActivityPane(40, 10, false))
	if !strings.Contains(view, "ACTIVITY") || !strings.Contains(view, "DUR") || !strings.Contains(view, "ST") {
		t.Fatalf("chart missing: %s", view)
	}
}

func TestJenkinsTokenMaskedInSettings(t *testing.T) {
	a := &App{
		width: 100, height: 28, jenkinsOpen: true, jenkinsSubTab: jenkinsTabSettings,
		jenkinsEditToken: "supersecrettoken",
		jenkinsCfg:       jenkinsutil.ProjectConfig{URL: "https://ci.example.com", User: "admin", Token: "supersecrettoken"},
	}
	view := stripANSI(a.renderJenkinsSettings(&core.Project{Name: "demo"}, 80, 14))
	if strings.Contains(view, "supersecrettoken") {
		t.Fatal("token must be masked")
	}
	if !strings.Contains(view, "oken") && !strings.Contains(view, "****") {
		t.Fatalf("expected masked token: %s", view)
	}
}
