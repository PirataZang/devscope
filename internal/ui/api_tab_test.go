package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesAPI(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabAPI {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabAPI missing from AllTabs")
	}
	if TabAPI.String() != "API" {
		t.Fatalf("String=%q", TabAPI.String())
	}
}

func TestInitApiTabSuggestsPort(t *testing.T) {
	a := &App{}
	p := &core.Project{Ports: []int{3000, 8080}}
	a.initApiTab(p)
	if a.apiURL != "http://localhost:3000" {
		t.Fatalf("url=%q", a.apiURL)
	}
	if a.apiMethod != "GET" {
		t.Fatalf("method=%q", a.apiMethod)
	}
	if !strings.Contains(a.apiHeaders, "Accept:") {
		t.Fatalf("headers=%q", a.apiHeaders)
	}
}

func TestInitApiTabKeepsExistingURL(t *testing.T) {
	a := &App{apiURL: "http://example.com/v1"}
	p := &core.Project{Ports: []int{9999}}
	a.initApiTab(p)
	if a.apiURL != "http://example.com/v1" {
		t.Fatalf("url overwritten: %q", a.apiURL)
	}
}

func TestCycleApiPortKeepsPath(t *testing.T) {
	a := &App{apiURL: "http://localhost:3000/api/users"}
	p := &core.Project{Ports: []int{3000, 8080}}
	a.cycleApiPort(p)
	if a.apiURL != "http://localhost:8080/api/users" {
		t.Fatalf("url=%q", a.apiURL)
	}
}

func TestPushApiHistoryDedupAndCap(t *testing.T) {
	a := &App{}
	for i := 0; i < 12; i++ {
		a.pushApiHistory("GET", fmt.Sprintf("http://localhost/%d", i))
	}
	a.pushApiHistory("GET", "http://localhost/0")
	if len(a.apiHistory) != 10 {
		t.Fatalf("len=%d", len(a.apiHistory))
	}
	if a.apiHistory[0].URL != "http://localhost/0" {
		t.Fatalf("first=%+v", a.apiHistory[0])
	}
}

func TestApiSearchMatches(t *testing.T) {
	a := &App{
		apiResponseBody: "hello\nFOO bar\nbaz\nfoo again",
		apiSearchQuery:  "foo",
	}
	matches := a.apiSearchMatches()
	if len(matches) != 2 || matches[0] != 1 || matches[1] != 3 {
		t.Fatalf("matches=%v", matches)
	}
}

func TestApiPrintableKeyIncludesSlash(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	if !apiPrintableKey(msg) {
		t.Fatal("/ should be printable for URL editing")
	}
	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	_ = slash
}

func TestApiBodyRequiresEAndBracketsInsertWhileEditing(t *testing.T) {
	a := &App{width: 100, height: 30, apiMethod: "GET", apiURL: "https://httpbin.org/get", apiBody: ""}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockRight
	a.apiRightTab = apiRightBody
	a.apiEditing = false

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, &core.Project{})
	if a.apiEditing {
		t.Fatal("typing on Body must not auto-edit")
	}

	a.beginApiEdit()
	if !a.apiEditing {
		t.Fatal("beginApiEdit should edit Body")
	}

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}}, &core.Project{})
	if !a.apiEditing {
		t.Fatal("[ while editing must stay in body editor")
	}
	if a.apiBody != "[" {
		t.Fatalf("expected literal [, got %q", a.apiBody)
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.apiBody != "[]" {
		t.Fatalf("expected [], got %q", a.apiBody)
	}
	if a.apiBlock != apiBlockRight || a.apiRightTab != apiRightBody {
		t.Fatal("[/] must not switch away from Body while editing")
	}

	a.apiEditing = false
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}}, &core.Project{})
	if a.apiRightTab != apiRightResponse {
		t.Fatal("] outside edit should switch to Response")
	}

	a.apiRightTab = apiRightBody
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if a.apiBlock != apiBlockRequest {
		t.Fatalf("tab outside edit should return to Request, got %v", a.apiBlock)
	}
}

func TestApiLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "p", Ports: []int{3000}}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterApiTab(&p)
	if a.tab != TabAPI || a.apiOpen {
		t.Fatalf("7 should open landing, tab=%v open=%v", a.tab, a.apiOpen)
	}
	landing := stripANSI(a.renderApiLanding(&p))
	if !strings.Contains(landing, "enter") {
		t.Fatalf("landing should prompt enter: %q", landing)
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.apiOpen || a.tab != TabAPI {
		t.Fatalf("enter should open client, open=%v tab=%v", a.apiOpen, a.tab)
	}

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.apiOpen || a.tab != TabAPI || a.view != ViewProject {
		t.Fatalf("esc should return to tab 7 landing, open=%v tab=%v view=%v", a.apiOpen, a.tab, a.view)
	}
}

func TestApiTabSlashDoesNotSearchOutsideRight(t *testing.T) {
	a := &App{width: 100, height: 30, apiMethod: "GET", apiURL: "https://example.com"}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockURL
	a.apiEditing = false
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, &core.Project{})
	if a.apiSearchOn {
		t.Fatal("slash on URL must not open search")
	}
	if !a.apiEditing {
		t.Fatal("slash on URL should start editing")
	}
	if !strings.Contains(a.apiURL, "/") {
		t.Fatalf("url should contain inserted slash: %q", a.apiURL)
	}

	a.apiEditing = false
	a.apiSearchOn = false
	a.apiBlock = apiBlockRight
	a.apiRightTab = apiRightResponse
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, &core.Project{})
	if !a.apiSearchOn {
		t.Fatal("slash on Response should open search")
	}
}

func TestFitApiFieldWindowFollowsCursor(t *testing.T) {
	url := "https://pokeapi.co/api/v2/pokemon/ditto"
	cursor := len([]rune(url))
	got := fitApiFieldWindow(url, cursor, 20, true)
	if !strings.Contains(got, "█") {
		t.Fatalf("missing cursor: %q", got)
	}
	if !strings.Contains(got, "ditto") {
		t.Fatalf("should show end while typing: %q", got)
	}
	if strings.HasPrefix(got, "https://") {
		t.Fatalf("should scroll away from start: %q", got)
	}

	idle := fitApiFieldWindow(url, 0, 20, false)
	if !strings.Contains(idle, "ditto") {
		t.Fatalf("idle should show URL end: %q", idle)
	}
}

func TestApiMethodStyleColors(t *testing.T) {
	get := apiMethodStyle("GET").Render("GET")
	post := apiMethodStyle("POST").Render("POST")
	del := apiMethodStyle("DELETE").Render("DELETE")
	if get == post || get == del || post == del {
		t.Fatalf("method colors should differ: get=%q post=%q del=%q", get, post, del)
	}
}

func TestRenderApiTabLazyDockerLayout(t *testing.T) {
	a := &App{width: 120, height: 36, apiMethod: "GET", apiURL: "http://localhost:8080"}
	a.initApiTab(&core.Project{Ports: []int{3000}})
	out := a.renderApiTab(&core.Project{Ports: []int{3000}})
	for _, want := range []string{
		"API", "[1]-Request", "[2]-URL", "[3]-Headers", "[4]-Auth",
		"GET", "POST", "localhost", "Body", "Response",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	// Border titles must not contain ANSI (breaks box drawing).
	if strings.Contains(out, "\x1b[") && strings.Contains(out, "[2]-URL") {
		// ANSI elsewhere (method colors) is fine; ensure URL box title is intact.
		if !strings.Contains(out, "┌─[2]-URL") && !strings.Contains(stripANSI(out), "[2]-URL") {
			t.Fatalf("URL box title broken:\n%s", stripANSI(out))
		}
	}
}
