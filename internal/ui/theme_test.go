package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/config"
)

func TestThemesCatalog(t *testing.T) {
	if len(Themes) < 12 {
		t.Fatalf("want >=12 themes, got %d", len(Themes))
	}
	ApplyTheme("devscope")
	if CurrentTheme() != "devscope" || string(ColorPrimary) != "#7C3AED" {
		t.Fatalf("devscope primary=%q theme=%q", ColorPrimary, CurrentTheme())
	}
	ApplyTheme("dracula-vivid")
	if CurrentTheme() != "dracula-vivid" || string(ColorPrimary) != "#D4A5FF" {
		t.Fatalf("dracula-vivid primary=%q theme=%q", ColorPrimary, CurrentTheme())
	}
	ApplyTheme("dracula")
	if string(ColorPrimary) != "#BD93F9" {
		t.Fatalf("classic dracula must stay #BD93F9, got %q", ColorPrimary)
	}
	ApplyTheme("dracula")
	if CurrentTheme() != "dracula" || string(ColorBg) != "#282A36" {
		t.Fatalf("dracula bg=%q theme=%q", ColorBg, CurrentTheme())
	}
	ApplyTheme("tokyo-night")
	if CurrentTheme() != "tokyo-night" || string(ColorBg) != "#1A1B26" {
		t.Fatalf("tokyo-night bg=%q theme=%q", ColorBg, CurrentTheme())
	}
	ApplyTheme("rose-pine")
	if CurrentTheme() != "rose-pine" || string(ColorPrimary) != "#C4A7E7" {
		t.Fatalf("rose-pine primary=%q theme=%q", ColorPrimary, CurrentTheme())
	}
	ApplyTheme("github")
	if CurrentTheme() != "dark" {
		t.Fatalf("github alias → dark, got %q", CurrentTheme())
	}
	ApplyTheme("nord")
	if CurrentTheme() != "nord" {
		t.Fatal(CurrentTheme())
	}
	ApplyTheme("nope")
	if CurrentTheme() != "dark" {
		t.Fatal(CurrentTheme())
	}
}

func TestThemePickerSaves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgPath := filepath.Join(dir, ".config", "devscope", "config.yaml")

	a := &App{width: 80, height: 24, cfg: &config.Config{UI: config.UIConfig{Theme: "dark"}}}
	ApplyTheme("dark")
	a.openThemePicker()
	if !a.themeOn {
		t.Fatal("picker closed")
	}
	// openThemePicker starts on current theme (dark = index 1); move to DevScope (index 0)
	_, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyUp})
	_, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if a.themeOn {
		t.Fatal("should close on save")
	}
	if a.cfg.UI.Theme != "devscope" {
		t.Fatalf("cfg=%q", a.cfg.UI.Theme)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "devscope") {
		t.Fatalf("config missing theme: %s", b)
	}
	loaded, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.UI.Theme != "devscope" {
		t.Fatalf("reload theme=%q", loaded.UI.Theme)
	}
}

func TestThemePickerEscRestores(t *testing.T) {
	a := &App{width: 80, height: 24}
	ApplyTheme("light")
	a.openThemePicker()
	_, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = a.updateThemePicker(tea.KeyMsg{Type: tea.KeyEsc})
	if a.themeOn || CurrentTheme() != "light" {
		t.Fatalf("on=%v theme=%q", a.themeOn, CurrentTheme())
	}
}

func TestHexRGB(t *testing.T) {
	got := hexRGB("#282A36")
	if got == nil || got[0] != 0x28 || got[1] != 0x2a || got[2] != 0x36 {
		t.Fatalf("%v", got)
	}
}

func TestRenderThemePopup(t *testing.T) {
	a := &App{width: 100, height: 30, themeOn: true, themeCursor: 0, themePrevious: "dark"}
	ApplyTheme("dark")
	got := stripANSI(a.renderThemePopup("bg"))
	for _, want := range []string{"Themes", "DevScope", "GitHub Dark", "Tokyo Night", "Dracula", "Dracula Vivid"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, truncate(got, 400))
		}
	}
}
