package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesSwarm(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabSwarm {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabSwarm missing from AllTabs")
	}
	if TabSwarm.String() != "Swarm" {
		t.Fatalf("String=%q", TabSwarm.String())
	}
	if int(TabKubernetes) != 3 || int(TabSwarm) != 4 {
		t.Fatalf("tab indices: k8s=%d swarm=%d", TabKubernetes, TabSwarm)
	}
}

func TestSidebarShowsSwarmInScope(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabSwarm}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "SCOPE") || !strings.Contains(got, "Swarm") {
		t.Fatalf("sidebar missing Swarm in SCOPE: %q", got)
	}
}

func TestSwarmLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterSwarmTab(&p)
	if a.tab != TabSwarm || a.swarmOpen {
		t.Fatalf("should open landing, tab=%v open=%v", a.tab, a.swarmOpen)
	}
	landing := stripANSI(a.renderSwarmLanding(&p))
	if !strings.Contains(landing, "SWARM") || !strings.Contains(landing, "enter") {
		t.Fatalf("landing missing prompt: %q", landing)
	}

	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.swarmOpen || a.tab != TabSwarm {
		t.Fatalf("enter should open client, open=%v tab=%v", a.swarmOpen, a.tab)
	}
	_ = cmd

	_, _ = a.handleSwarmKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.swarmOpen || a.tab != TabSwarm {
		t.Fatalf("esc should return to landing, open=%v tab=%v", a.swarmOpen, a.tab)
	}
}

func TestSanitizeSwarmStackName(t *testing.T) {
	if got := sanitizeSwarmStackName("My App!"); got != "my-app" {
		t.Fatalf("got %q", got)
	}
}

func TestSwarmKindCycle(t *testing.T) {
	a := &App{swarmOpen: true, swarmKind: swarmKindServices, swarmScreen: swarmScrCluster}
	_, _ = a.handleSwarmKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.swarmKind != swarmKindNodes {
		t.Fatalf("got kind %v want Nodes", a.swarmKind)
	}
	_, _ = a.handleSwarmKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.swarmKind != swarmKindTasks {
		t.Fatalf("got kind %v want Tasks", a.swarmKind)
	}
}

func TestSwarmControlCenterRenders(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/p"}
	a := &App{
		width:       120,
		height:      40,
		swarmOpen:   true,
		swarmKind:   swarmKindServices,
		swarmScreen: swarmScrCluster,
		swarmInfo: collectors.SwarmInfo{
			Active:   true,
			State:    "active",
			Managers: 1,
			Workers:  2,
			Nodes:    3,
		},
		swarmProject: "demo",
		swarmServices: []collectors.SwarmService{
			{Name: "demo-api", Image: "demo/api:latest", Mode: "replicated", Replicas: "2/2", Ports: "8080:8080"},
		},
		swarmNodes: []collectors.SwarmNode{
			{Hostname: "manager-01", Role: "manager", Status: "Ready", Availability: "Active", Manager: "Leader"},
		},
	}
	got := stripANSI(a.renderSwarmTab(&p))
	for _, want := range []string{"DOCKER SWARM", "CLUSTER", "SERVICES", "NODES", "AÇÕES"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestSwarmKindTabsShowAll(t *testing.T) {
	a := &App{swarmKind: swarmKindNodes}
	got := stripANSI(a.renderSwarmKindTabs(120))
	for _, want := range []string{"SERVICES", "NODES", "TASKS", "STACKS", "NETWORKS", "SECRETS", "CONFIGS", "EVENTS"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing tab %q in %q", want, got)
		}
	}
}

func TestSwarmNavPushPop(t *testing.T) {
	a := &App{swarmKind: swarmKindServices, swarmCursor: 2, swarmScroll: 1}
	a.swarmPushNav()
	a.swarmKind = swarmKindNodes
	a.swarmCursor = 0
	if !a.swarmPopNav() {
		t.Fatal("expected pop")
	}
	if a.swarmKind != swarmKindServices || a.swarmCursor != 2 || a.swarmScroll != 1 {
		t.Fatalf("restored kind=%v cursor=%d scroll=%d", a.swarmKind, a.swarmCursor, a.swarmScroll)
	}
}

func TestSwarmScaleForm(t *testing.T) {
	a := &App{
		swarmOpen:   true,
		swarmKind:   swarmKindServices,
		swarmScreen: swarmScrCluster,
		swarmServices: []collectors.SwarmService{
			{Name: "api", Replicas: "3/3"},
		},
	}
	_, _ = a.handleSwarmKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, &core.Project{})
	if a.swarmScreen != swarmScrForm || a.swarmForm != swarmFormScale {
		t.Fatalf("screen=%v form=%v", a.swarmScreen, a.swarmForm)
	}
	if a.swarmFormInput != "3" {
		t.Fatalf("input=%q", a.swarmFormInput)
	}
}
