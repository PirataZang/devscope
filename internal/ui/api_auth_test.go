package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestApiAuthCycleWithA(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockAuth
	a.apiAuthType = apiAuthNone

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, &core.Project{})
	if a.apiAuthType != apiAuthBearer {
		t.Fatalf("a should cycle none→bearer, got %v", a.apiAuthType)
	}
	if a.apiEditing {
		t.Fatal("a must cycle type, not start editing")
	}

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, &core.Project{})
	if a.apiAuthType != apiAuthBasic {
		t.Fatalf("a should cycle bearer→basic, got %v", a.apiAuthType)
	}
}

func TestApiAuthEditBearerToken(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockAuth
	a.apiAuthType = apiAuthBearer

	// Typing without e must NOT edit.
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, &core.Project{})
	if a.apiEditing || a.apiAuthToken != "" {
		t.Fatal("bearer must require e to edit")
	}

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, &core.Project{})
	if !a.apiEditing {
		t.Fatal("e should start editing bearer token")
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tok")}, &core.Project{})
	if a.apiAuthToken != "tok" {
		t.Fatalf("token=%q", a.apiAuthToken)
	}
}

func TestApiAuthTokenWindowFollowsCursor(t *testing.T) {
	token := "abcdefghijklmnopqrstuvwxyz0123456789TOKENEND"
	got := fitApiFieldWindow(token, len([]rune(token)), 16, true)
	if !strings.Contains(got, "█") {
		t.Fatalf("missing cursor: %q", got)
	}
	if !strings.Contains(got, "END") {
		t.Fatalf("should follow typing to the end: %q", got)
	}
	mid := fitApiFieldWindow(token, 10, 12, true)
	if !strings.Contains(mid, "█") {
		t.Fatalf("missing cursor mid-edit: %q", mid)
	}
}

func TestApiAuthEditAcceptsKQ(t *testing.T) {
	p := core.Project{Name: "demo", Path: "/tmp/demo"}
	a := &App{width: 100, height: 30, view: ViewProject, tab: TabAPI, apiOpen: true, selectedProject: &p}
	a.initApiTab(&p)
	a.apiBlock = apiBlockAuth
	a.apiAuthType = apiAuthBearer
	a.beginApiEdit()

	// Simulate global key path (q used to quit the app).
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.quitting {
		t.Fatal("q while editing auth must not quit")
	}
	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if a.apiAuthToken != "qk" {
		t.Fatalf("expected qk typed into token, got %q", a.apiAuthToken)
	}
	if !a.apiEditing {
		t.Fatal("should still be editing")
	}

	_, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.apiEditing {
		t.Fatal("esc should leave edit mode back to Auth view")
	}
	if a.apiBlock != apiBlockAuth {
		t.Fatalf("esc should stay on Auth block, got %v", a.apiBlock)
	}
}

func TestApiAuthBasicTabSwitchesFields(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockAuth
	a.apiAuthType = apiAuthBasic
	a.beginApiEdit()
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("alice")}, &core.Project{})
	if a.apiAuthUser != "alice" {
		t.Fatalf("user=%q", a.apiAuthUser)
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyTab}, &core.Project{})
	if !a.apiAuthEditPass {
		t.Fatal("tab should switch to password field")
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s3cret")}, &core.Project{})
	if a.apiAuthPass != "s3cret" {
		t.Fatalf("pass=%q", a.apiAuthPass)
	}
}
