package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesKubernetes(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabKubernetes {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabKubernetes missing from AllTabs")
	}
	if TabKubernetes.String() != "Kubernetes" {
		t.Fatalf("String=%q", TabKubernetes.String())
	}
	if int(TabKubernetes) != 3 || int(TabDatabase) != 8 || int(TabJSON) != 9 {
		t.Fatalf("tab indices shifted unexpectedly: k8s=%d db=%d json=%d", TabKubernetes, TabDatabase, TabJSON)
	}
}

func TestK8sLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterK8sTab(&p)
	if a.tab != TabKubernetes || a.k8sOpen {
		t.Fatalf("4 should open landing, tab=%v open=%v", a.tab, a.k8sOpen)
	}
	landing := stripANSI(a.renderK8sLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "Kubernetes") {
		t.Fatalf("landing missing prompt: %q", landing)
	}

	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.k8sOpen || a.tab != TabKubernetes {
		t.Fatalf("enter should open client, open=%v tab=%v", a.k8sOpen, a.tab)
	}
	_ = cmd

	_, _ = a.handleK8sKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.k8sOpen || a.tab != TabKubernetes || a.view != ViewProject {
		t.Fatalf("esc should return to tab 4 landing, open=%v tab=%v view=%v", a.k8sOpen, a.tab, a.view)
	}
}

func TestSidebarShowsKubernetesInScope(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabKubernetes}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "SCOPE") || !strings.Contains(got, "Kubernetes") {
		t.Fatalf("sidebar missing Kubernetes in SCOPE: %q", got)
	}
	if !strings.Contains(got, "tab · shift+tab") {
		t.Fatalf("footer should mention tab · shift+tab: %q", got)
	}
}

func TestSanitizeK8sName(t *testing.T) {
	if got := sanitizeK8sName("My App!"); got != "my-app" {
		t.Fatalf("got %q", got)
	}
}

func TestK8sApplyKeyAndEnterNewline(t *testing.T) {
	if !isK8sApplyKey(tea.KeyMsg{Type: tea.KeyCtrlS}) {
		t.Fatal("ctrl+s must apply")
	}
	if isK8sApplyKey(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("plain enter must NOT apply (nova linha)")
	}

	a := &App{k8sEditing: true, k8sYAML: "a: 1", k8sEditorCursor: 4, k8sPane: k8sPaneEditor}
	_, _ = a.updateK8sEdit(tea.KeyMsg{Type: tea.KeyEnter}, nil)
	if !strings.Contains(a.k8sYAML, "\n") || !a.k8sEditing {
		t.Fatalf("enter should insert newline and stay editing: yaml=%q editing=%v", a.k8sYAML, a.k8sEditing)
	}

	a = &App{k8sEditing: true, k8sYAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: t\n", k8sPane: k8sPaneEditor}
	_, cmd := a.updateK8sEdit(tea.KeyMsg{Type: tea.KeyCtrlS}, nil)
	if a.k8sEditing || cmd == nil {
		t.Fatalf("ctrl+s should apply: editing=%v cmd=%v", a.k8sEditing, cmd)
	}
}

func TestK8sCreateStartsEditing(t *testing.T) {
	a := &App{k8sNamespace: "default"}
	_ = a.k8sBeginCreate()
	if !a.k8sEditing || a.k8sYAML == "" || a.k8sPane != k8sPaneEditor {
		t.Fatalf("create: editing=%v pane=%v yaml empty=%v", a.k8sEditing, a.k8sPane, a.k8sYAML == "")
	}
}

func TestK8sOverviewLayout(t *testing.T) {
	a := &App{
		width:        120,
		height:       40,
		k8sOpen:      true,
		k8sNamespace: "default",
		k8sContext:   "kind-dev",
		k8sVersion:   "v1.31.0",
		k8sNodeCount: 3,
		k8sKind:      k8sKindPods,
		k8sSubTab:    k8sTabOverview,
		k8sFocus:     k8sFocusTable,
		k8sResources: []collectors.K8sResource{
			{Kind: "Pod", Name: "frontend-1", Status: "Running", Ready: "1/1", Restarts: "0", Node: "node-1", IP: "10.0.0.1", Age: "2d"},
		},
	}
	view := stripANSI(a.renderK8sTab(&core.Project{Name: "demo", Path: "/p"}))
	for _, want := range []string{
		"devscope", "kubernetes", "CLUSTER EXPLORER", "PODS", "POD LOGS", "YAML", "DETAILS", "QUICK STATS", "RELATION",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in:\n%s", want, view)
		}
	}
}

func TestK8sFilterKey(t *testing.T) {
	a := &App{k8sOpen: true, k8sFocus: k8sFocusTable}
	_, _ = a.handleK8sKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}, nil)
	if !a.k8sFilterOn {
		t.Fatal("b should start filter")
	}
}
