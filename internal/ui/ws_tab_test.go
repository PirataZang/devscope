package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/wsutil"
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
	if !strings.Contains(landing, "WEBSOCKET") || !strings.Contains(landing, "enter") || !strings.Contains(landing, "AÇÕES") {
		t.Fatalf("landing=%q", landing)
	}
	_ = a.openWsClient(&p)
	if !a.wsOpen || !strings.Contains(a.wsURL, "3000") || a.wsFocus != wsFocusConnections {
		t.Fatalf("open url=%q open=%v focus=%v", a.wsURL, a.wsOpen, a.wsFocus)
	}
	view := stripANSI(a.renderWsTab(&p))
	for _, want := range []string{"Conversa", "MENSAGENS", "SERVIDORES"} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in %q", want, truncate(view, 200))
		}
	}
	if strings.Contains(view, "ENVIAR") {
		t.Fatalf("overview should not show send box: %q", truncate(view, 200))
	}
	for _, hide := range []string{"RESUMO", "MOSTRAR", "AÇÕES", "Visão"} {
		if strings.Contains(view, hide) {
			t.Fatalf("overview still shows %q in %q", hide, truncate(view, 200))
		}
	}
	if strings.Contains(view, "2:Send") || strings.Contains(view, "1:Send") {
		t.Fatal("Send subtab should be removed")
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

func TestWsNewURLKey(t *testing.T) {
	p := &core.Project{Path: "/p", Name: "demo", Ports: []int{3000}}
	a := &App{
		wsOpen:   true,
		wsSubTab: wsTabOverview,
		wsURL:    "wss://echo.websocket.events/",
		wsRecent: []string{"wss://echo.websocket.events/", "ws://localhost:5"},
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, p)
	if a.wsSubTab != wsTabOverview {
		t.Fatalf("subtab=%v", a.wsSubTab)
	}
	if a.wsFocus != wsFocusConnections {
		t.Fatalf("focus=%v", a.wsFocus)
	}
	if !a.wsEditing {
		t.Fatal("expected editing")
	}
	if a.wsURL != "ws://localhost:3000/ws" {
		t.Fatalf("url=%q", a.wsURL)
	}
}

func TestWsComposeMOpensAndTabChangesType(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width: 100, height: 30,
		wsOpen: true, wsSubTab: wsTabOverview, wsFocus: wsFocusMessages,
		wsSendMode:      wsSendText,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}, &p)
	if !a.wsComposeOn {
		t.Fatal("m should open compose")
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("oi")})
	if a.wsSend != "oi" {
		t.Fatalf("typed %q", a.wsSend)
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyEnter})
	if a.wsSend != "oi\n" {
		t.Fatalf("enter in editor should be newline: %q", a.wsSend)
	}
	if !a.wsComposeOn {
		t.Fatal("enter in editor must not send")
	}
	got := stripANSI(a.renderWsCompose())
	for _, want := range []string{"Nova mensagem", "Texto", "JSON", "Enviar", "Cancelar", "oi"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, truncate(got, 300))
		}
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyTab})
	if a.wsComposeFocus != wsComposeFocusType {
		t.Fatalf("tab→tipo focus=%v", a.wsComposeFocus)
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyTab})
	if a.wsSendMode != wsSendJSON {
		t.Fatalf("tab should select JSON, mode=%v", a.wsSendMode)
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyTab})
	if a.wsSendMode != wsSendBinary {
		t.Fatalf("tab should select Binary, mode=%v", a.wsSendMode)
	}
	_, _ = a.updateWsCompose(tea.KeyMsg{Type: tea.KeyTab})
	if a.wsComposeFocus != wsComposeFocusSend {
		t.Fatalf("tab→Enviar focus=%v", a.wsComposeFocus)
	}
	_, cmd := a.updateWsCompose(tea.KeyMsg{Type: tea.KeyEnter})
	if a.wsComposeFocus != wsComposeFocusSend {
		t.Fatalf("Enviar must stay focused, got %v", a.wsComposeFocus)
	}
	if !a.wsComposeOn || !a.wsComposePending {
		t.Fatal("Enviar should keep compose and connect")
	}
	if cmd == nil {
		t.Fatal("expected connect cmd")
	}
}

func TestWsMessagesTabTogglesAddress(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width: 100, height: 30,
		wsOpen: true, wsSubTab: wsTabOverview, wsFocus: wsFocusMessages,
		wsFrames:        []wsFrame{{ID: 1, Dir: "in", Kind: "text", Payload: "hello-from-server", Size: 17}},
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	view := stripANSI(a.renderWsTab(&p))
	if !strings.Contains(view, "MENSAGENS") || !strings.Contains(view, "SERVIDORES") || !strings.Contains(view, "0:Conversa") {
		t.Fatalf("conversa=%q", truncate(view, 300))
	}
	if strings.Contains(view, "ENVIAR") || strings.Contains(view, ":Send") {
		t.Fatalf("send box should be gone: %q", truncate(view, 200))
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyTab}, &p)
	if a.wsFocus != wsFocusConnections {
		t.Fatalf("tab→endereço focus=%v", a.wsFocus)
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyTab}, &p)
	if a.wsFocus != wsFocusMessages {
		t.Fatalf("tab→conversa focus=%v", a.wsFocus)
	}
	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}, &p)
	if a.wsMsgHScroll != 4 {
		t.Fatalf("hscroll=%d", a.wsMsgHScroll)
	}
}

func TestWsAllConnectionsPopup(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	pA := core.Project{Path: dirA, Name: "alpha"}
	pB := core.Project{Path: dirB, Name: "beta"}
	a := &App{
		width:           100,
		height:          30,
		wsOpen:          true,
		wsSubTab:        wsTabOverview,
		wsFocus:         wsFocusConnections,
		selectedProject: &pA,
		snapshot:        core.Snapshot{Projects: []core.Project{pA, pB}},
		wsRecent:        []string{"ws://localhost:3000/ws"},
		wsURL:           "ws://localhost:3000/ws",
	}
	a.persistWsProjectConns()
	a.selectedProject = &pB
	a.wsRecent = []string{"wss://echo.test/"}
	a.persistWsProjectConns()
	a.selectedProject = &pA
	a.loadWsProjectConns(&pA)

	_, _ = a.handleWsKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}, &pA)
	if !a.wsShowAll {
		t.Fatal("expected show all")
	}
	entries := a.wsAllConnEntries()
	if len(entries) != 2 {
		t.Fatalf("entries=%+v", entries)
	}
	view := stripANSI(a.renderWsTab(&pA))
	if !strings.Contains(view, "SERVIDORES · TODOS") || !strings.Contains(view, "STATE") || !strings.Contains(view, "PROJECT") {
		t.Fatalf("sidebar all=%q", truncate(view, 400))
	}
	if !strings.Contains(view, "alpha") || !strings.Contains(view, "beta") {
		t.Fatalf("projects missing=%q", truncate(view, 400))
	}
	if strings.Contains(view, "TODOS OS ENDEREÇOS") {
		t.Fatal("A should fill the sidebar, not a popup")
	}
	a.wsAllCursor = 1
	cmd := a.pickWsAllEntry()
	if !a.wsShowAll {
		t.Fatal("all-projects list should stay in the sidebar")
	}
	if a.wsURL != "wss://echo.test/" {
		t.Fatalf("url=%q", a.wsURL)
	}
	if cmd == nil {
		t.Fatal("expected connect cmd")
	}
}

func TestWsConnectionManage(t *testing.T) {
	a := &App{
		wsOpen:          true,
		wsFocus:         wsFocusConnections,
		wsURL:           "wss://ws.ifelse.io",
		wsConnected:     true,
		wsRecent:        []string{"wss://ws.ifelse.io", "wss://echo.websocket.events/", "ws://localhost:5"},
		wsRecentCursor:  1,
		wsEditSourceIdx: -1,
	}

	a.disconnectSelectedWsURL()
	if a.wsStatus != "não está conectada" {
		t.Fatalf("disconnect other: status=%q", a.wsStatus)
	}

	a.wsRecentCursor = 0
	a.disconnectSelectedWsURL()
	if a.wsConnected {
		t.Fatal("expected disconnect of active")
	}

	a.wsRecentCursor = 1
	a.deleteSelectedWsURL()
	if len(a.wsRecent) != 2 {
		t.Fatalf("len=%d", len(a.wsRecent))
	}
	for _, u := range a.wsRecent {
		if u == "wss://echo.websocket.events/" {
			t.Fatal("deleted url still present")
		}
	}

	p := &core.Project{Ports: []int{3000}}
	a.startNewWsURL(p)
	if !a.wsEditing || a.wsEditSourceIdx != -1 {
		t.Fatalf("new url edit=%v idx=%d", a.wsEditing, a.wsEditSourceIdx)
	}
	a.wsURL = "wss://example.test/ws"
	a.saveWsURLFromEditor()
	if a.wsRecent[0] != "wss://example.test/ws" {
		t.Fatalf("recent=%v", a.wsRecent)
	}
	if a.wsSubTab != wsTabOverview || a.wsFocus != wsFocusConnections {
		t.Fatalf("after save sub=%v focus=%v", a.wsSubTab, a.wsFocus)
	}

	a.wsRecentCursor = 0
	a.startEditSelectedWsURL()
	if a.wsEditSourceIdx != 0 || !a.wsEditing {
		t.Fatalf("edit idx=%d editing=%v", a.wsEditSourceIdx, a.wsEditing)
	}
	a.wsURL = "wss://edited.test/ws"
	a.saveWsURLFromEditor()
	if a.wsRecent[0] != "wss://edited.test/ws" {
		t.Fatalf("edited=%v", a.wsRecent)
	}
}

func TestWsConnectSelectedSwitches(t *testing.T) {
	a := &App{
		wsOpen:          true,
		wsFocus:         wsFocusConnections,
		wsURL:           "wss://a.test",
		wsConnected:     true,
		wsInfo:          wsutil.Info{URL: "wss://a.test"},
		wsRecent:        []string{"wss://a.test", "wss://b.test"},
		wsRecentCursor:  1,
		wsEditSourceIdx: -1,
	}
	cmd := a.connectSelectedWsURL()
	if a.wsConnected {
		t.Fatal("should close before switching")
	}
	if a.wsURL != "wss://b.test" {
		t.Fatalf("url=%q", a.wsURL)
	}
	if cmd == nil {
		t.Fatal("expected connect cmd")
	}
}

func TestWsSwitchIgnoresStaleURLDraft(t *testing.T) {
	// Bug: draft wsURL already set to target while socket still on ifelse →
	// connectSelected used to return "já conectado" and keep sending on ifelse.
	a := &App{
		wsOpen:         true,
		wsFocus:        wsFocusConnections,
		wsConnected:    true,
		wsURL:          "ws://localhost:80/ws",
		wsInfo:         wsutil.Info{URL: "wss://ws.ifelse.io"},
		wsRecent:       []string{"ws://localhost:80/ws", "wss://ws.ifelse.io"},
		wsRecentCursor: 0,
	}
	if a.liveWsURL() != "wss://ws.ifelse.io" {
		t.Fatalf("live=%q", a.liveWsURL())
	}
	cmd := a.connectSelectedWsURL()
	if a.wsConnected {
		t.Fatal("must close live socket before dialing localhost")
	}
	if a.wsStatus == "já conectado" {
		t.Fatal("stale draft must not fake already-connected")
	}
	if a.wsURL != "ws://localhost:80/ws" {
		t.Fatalf("url=%q", a.wsURL)
	}
	if cmd == nil {
		t.Fatal("expected dial cmd")
	}
}
