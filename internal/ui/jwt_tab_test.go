package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestJWTNotInNav(t *testing.T) {
	for _, tab := range AllTabs {
		if tab == TabJWT {
			t.Fatal("TabJWT should be removed from AllTabs")
		}
	}
	if TabJWT.String() != "JWT" {
		t.Fatalf("String=%q", TabJWT.String())
	}
}

func TestJwtLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterJwtTab(&p)
	if a.tab != TabJWT || a.jwtOpen {
		t.Fatalf("- should open landing, tab=%v open=%v", a.tab, a.jwtOpen)
	}
	landing := stripANSI(a.renderJwtLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "JWT") {
		t.Fatalf("landing missing prompt: %q", landing)
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.jwtOpen || a.tab != TabJWT {
		t.Fatalf("enter should open client, open=%v tab=%v", a.jwtOpen, a.tab)
	}

	_, _ = a.handleJwtKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.jwtOpen || a.tab != TabJWT || a.view != ViewProject {
		t.Fatalf("esc should return to landing, open=%v tab=%v view=%v", a.jwtOpen, a.tab, a.view)
	}
}

func TestJwtDecodeAndSign(t *testing.T) {
	a := &App{jwtOpen: true, tab: TabJWT, jwtAlg: "HS256", jwtSecret: "secret", jwtEdit: editorState{Anchor: -1}}
	a.runJwtGenerate()
	a.jwtEditing = false
	a.runJwtSign()
	if a.jwtErr != "" || !strings.Contains(a.jwtInput, ".") {
		t.Fatalf("sign failed: err=%q input=%q", a.jwtErr, a.jwtInput)
	}
	if a.jwtLastToken == "" || a.jwtLastToken != strings.TrimSpace(a.jwtInput) {
		t.Fatalf("sign should cache token: last=%q input=%q", a.jwtLastToken, a.jwtInput)
	}
	if !strings.Contains(a.jwtOutput, "PAYLOAD") {
		t.Fatalf("sign should decode into output: %q", a.jwtOutput)
	}
	a.runJwtVerify()
	if a.jwtErr != "" || !strings.Contains(a.jwtOutput, "VALID") {
		t.Fatalf("verify failed: err=%q out=%q", a.jwtErr, a.jwtOutput)
	}
}

func TestJwtSourceTokenUsesCache(t *testing.T) {
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIn0.sig"
	a := &App{jwtLastToken: tok, jwtInput: `{"sub":"1"}`}
	if got := a.jwtSourceToken(); got != tok {
		t.Fatalf("jwtSourceToken=%q want cached", got)
	}
	if !lookLikeJWT(tok) || lookLikeJWT(`{"a":1}`) {
		t.Fatal("lookLikeJWT mismatch")
	}
}

func TestJwtHScrollAndEditorSelect(t *testing.T) {
	a := &App{
		width:    80,
		jwtOpen:  true,
		tab:      TabJWT,
		jwtPane:  jwtPaneInput,
		jwtInput: strings.Repeat("abcdefghi.", 20),
		jwtEdit:  editorState{Anchor: -1},
	}
	a.jwtHScrollDelta(8)
	if a.jwtHScrollIn != 8 {
		t.Fatalf("hscroll=%d", a.jwtHScrollIn)
	}
	a.jwtEditing = true
	a.jwtEdit.Cursor = 2
	_, _ = a.updateJwtEdit(tea.KeyMsg{Type: tea.KeyShiftRight})
	if _, _, ok := a.jwtEdit.selRange(true); !ok {
		t.Fatal("expected selection after shift+right")
	}
}

func TestSidebarHidesJWT(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabRoutes}
	got := stripANSI(a.renderProjectSidebar())
	if strings.Contains(got, "JWT") {
		t.Fatalf("sidebar should not list JWT: %q", got)
	}
}

func TestJwtClientDashboard(t *testing.T) {
	a := &App{
		width: 120, height: 40, jwtOpen: true, tab: TabJWT,
		jwtAlg: "HS256", jwtSecret: "your-256-bit-secret",
		jwtEdit: editorState{Anchor: -1},
	}
	a.openJwtClient(nil)
	got := stripANSI(a.renderJwtTab(nil))
	for _, want := range []string{"devscope", "jwt", "TOKEN", "RESULT", "AÇÕES", "SECRET", "HS256", "ALG"} {
		if !strings.Contains(got, want) {
			t.Fatalf("jwt client missing %q in:\n%s", want, got)
		}
	}
}
