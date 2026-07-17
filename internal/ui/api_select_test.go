package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/core"
)

func TestApiSelectAllAndDeleteURL(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockURL
	a.apiURL = "https://example.com/path"
	a.beginApiEdit()

	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyCtrlA})
	lo, hi, ok := a.apiSelRange()
	if !ok || lo != 0 || hi != len([]rune(a.apiURL)) {
		t.Fatalf("ctrl+a should select all, got ok=%v lo=%d hi=%d url=%q", ok, lo, hi, a.apiURL)
	}

	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyBackspace})
	if a.apiURL != "" {
		t.Fatalf("backspace on selection should clear, got %q", a.apiURL)
	}
	if _, _, ok := a.apiSelRange(); ok {
		t.Fatal("selection should clear after delete")
	}
}

func TestApiShiftSelectReplaceBody(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})
	a.apiBlock = apiBlockRight
	a.apiRightTab = apiRightBody
	a.apiBody = "abcdef"
	a.beginApiEdit()
	a.apiEditorCursor = 2 // after "ab"

	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyShiftRight})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyShiftRight})
	lo, hi, ok := a.apiSelRange()
	if !ok || lo != 2 || hi != 5 {
		t.Fatalf("expected sel [2,5), got ok=%v lo=%d hi=%d", ok, lo, hi)
	}

	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	// [2,5) removes "cde" → "abf", then "X" → "abXf"
	if a.apiBody != "abXf" {
		t.Fatalf("replace selection: got %q", a.apiBody)
	}
}

func TestApiSelectAllHeadersAuth(t *testing.T) {
	a := &App{width: 100, height: 30}
	a.initApiTab(&core.Project{})

	a.apiBlock = apiBlockHeaders
	a.apiHeaders = "Accept: application/json\nX-Test: 1"
	a.beginApiEdit()
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyCtrlA})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyDelete})
	if a.apiHeaders != "" {
		t.Fatalf("headers select-all delete: %q", a.apiHeaders)
	}

	a.apiBlock = apiBlockAuth
	a.apiAuthType = apiAuthBearer
	a.apiAuthToken = "secret-token-value"
	a.beginApiEdit()
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyCtrlA})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if a.apiAuthToken != "z" {
		t.Fatalf("auth replace all: %q", a.apiAuthToken)
	}
}

func TestApiSelHighlightInWindow(t *testing.T) {
	text := "abcdefghij"
	got := fitApiFieldWindowSel(text, 10, 20, true, 2, 6)
	if !strings.Contains(got, "█") {
		t.Fatalf("missing cursor: %q", got)
	}
	// Selection styling injects ANSI; stripped text should still contain cdef.
	plain := stripANSI(got)
	if !strings.Contains(plain, "cdef") {
		t.Fatalf("selection window missing text: %q", plain)
	}
}
