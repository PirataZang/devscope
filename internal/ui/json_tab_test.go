package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestJSONNotInNav(t *testing.T) {
	for _, tab := range AllTabs {
		if tab == TabJSON {
			t.Fatal("TabJSON should be removed from AllTabs")
		}
	}
	if TabJSON.String() != "JSON" {
		t.Fatalf("String=%q", TabJSON.String())
	}
}

func TestJsonLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterJsonTab(&p)
	if a.tab != TabJSON || a.jsonOpen {
		t.Fatalf("0 should open landing, tab=%v open=%v", a.tab, a.jsonOpen)
	}
	landing := stripANSI(a.renderJsonLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "JSON") {
		t.Fatalf("landing missing prompt: %q", landing)
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.jsonOpen || a.tab != TabJSON {
		t.Fatalf("enter should open client, open=%v tab=%v", a.jsonOpen, a.tab)
	}

	_, _ = a.handleJsonKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.jsonOpen || a.tab != TabJSON || a.view != ViewProject {
		t.Fatalf("esc should return to landing, open=%v tab=%v view=%v", a.jsonOpen, a.tab, a.view)
	}
}

func TestJsonPrettyAction(t *testing.T) {
	a := &App{jsonInput: `{"b":1,"a":2}`, jsonOpen: true, tab: TabJSON}
	a.runJsonAction("p")
	if a.jsonErr != "" || !strings.Contains(a.jsonOutput, "\n") {
		t.Fatalf("pretty failed: err=%q out=%q", a.jsonErr, a.jsonOutput)
	}
}

func TestJsonShiftSelectAndCtrlWord(t *testing.T) {
	a := &App{
		jsonInput:        `{"hello":"world"}`,
		jsonOpen:         true,
		jsonEditing:      true,
		jsonEditorCursor: 2, // after {"
		jsonEditorAnchor: -1,
		tab:              TabJSON,
	}
	_, _ = a.updateJsonEdit(tea.KeyMsg{Type: tea.KeyShiftRight})
	lo, hi, ok := a.jsonSelRange()
	if !ok || hi <= lo {
		t.Fatalf("shift+right should select, lo=%d hi=%d ok=%v", lo, hi, ok)
	}
	_, _ = a.updateJsonEdit(tea.KeyMsg{Type: tea.KeyCtrlRight})
	if _, _, still := a.jsonSelRange(); still {
		t.Fatal("ctrl+right without shift should clear selection")
	}
	start := a.jsonEditorCursor
	_, _ = a.updateJsonEdit(tea.KeyMsg{Type: tea.KeyCtrlShiftLeft})
	lo, hi, ok = a.jsonSelRange()
	if !ok || a.jsonEditorCursor >= start {
		t.Fatalf("ctrl+shift+left should extend left, cursor=%d start=%d lo=%d hi=%d", a.jsonEditorCursor, start, lo, hi)
	}
}

func TestSidebarHidesJSON(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabRoutes}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "UTILS") || !strings.Contains(got, "Rotas") {
		t.Fatalf("sidebar missing UTILS/Rotas: %q", got)
	}
	if strings.Contains(got, "JSON") || strings.Contains(got, "JWT") {
		t.Fatalf("sidebar should not list JSON/JWT: %q", got)
	}
}

func TestJsonClientDashboard(t *testing.T) {
	a := &App{
		width: 120, height: 36, jsonOpen: true, tab: TabJSON,
		jsonInput:  "{\n  \"hello\": \"devscope\",\n  \"n\": 1\n}\n",
		jsonOutput: "{\n  \"hello\": \"devscope\",\n  \"n\": 1\n}\n",
		jsonStatus: "pretty",
	}
	got := stripANSI(a.renderJsonTab(nil))
	for _, want := range []string{"devscope", "json", "INPUT", "OUTPUT", "AÇÕES", "STATUS", "pretty"} {
		if !strings.Contains(got, want) {
			t.Fatalf("json client missing %q in:\n%s", want, got)
		}
	}
}
