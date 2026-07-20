package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesWebSocket(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabWebSocket {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabWebSocket missing")
	}
	if TabWebSocket.String() != "WS" {
		t.Fatalf("String=%q", TabWebSocket.String())
	}
}

func TestWsLandingEnterEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo", Ports: []int{3000}}
	a := &App{
		width:           120,
		height:          40,
		view:            ViewProject,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterWsTab(&p)
	landing := stripANSI(a.renderWsLanding(&p))
	if !strings.Contains(landing, "WebSocket") || !strings.Contains(landing, "enter") {
		t.Fatalf("landing=%q", landing)
	}
	_ = a.openWsClient(&p)
	if !a.wsOpen || !strings.Contains(a.wsURL, "3000") {
		t.Fatalf("open url=%q open=%v", a.wsURL, a.wsOpen)
	}
	view := stripANSI(a.renderWsTab(&p))
	for _, want := range []string{"Overview", "MESSAGES", "SEND", "CONNECTIONS", "STATS", "FILTERS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in %q", want, truncate(view, 200))
		}
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.wsOpen {
		t.Fatal("esc should close client")
	}
}

func TestWsFilterAndSearch(t *testing.T) {
	a := &App{
		wsFrames: []wsFrame{
			{ID: 1, Dir: "in", Kind: "json", Payload: `{"user":1}`},
			{ID: 2, Dir: "out", Kind: "text", Payload: "hello"},
			{ID: 3, Dir: "err", Kind: "error", Payload: "boom"},
		},
		wsFilter: wsFilterJSON,
	}
	if got := a.filteredWsFrames(); len(got) != 1 || got[0].Kind != "json" {
		t.Fatalf("json filter: %+v", got)
	}
	a.wsFilter = wsFilterAll
	a.wsSearch = "user"
	if got := a.filteredWsFrames(); len(got) != 1 {
		t.Fatalf("search: %+v", got)
	}
}

func TestSidebarShowsWebSocket(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabWebSocket}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "WS") || !strings.Contains(got, "tab · shift+tab") {
		t.Fatalf("sidebar: %q", got)
	}
}

func TestWsCyclePort(t *testing.T) {
	p := &core.Project{Ports: []int{3000, 8080}}
	a := &App{wsURL: "ws://localhost:3000/chat"}
	a.cycleWsPort(p)
	if a.wsURL != "ws://localhost:8080/chat" {
		t.Fatalf("url=%q", a.wsURL)
	}
}
