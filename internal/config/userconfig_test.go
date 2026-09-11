package config

import (
	"strings"
	"testing"
)

// O arquivo que o DevScope cria tem que voltar inteiro quando lido de novo —
// é ele que a pessoa edita na tela, então o formato é a interface.
func TestUserConfigRoundTrip(t *testing.T) {
	txt := DefaultUserConfig("gruvbox", []string{"claude", "opencode"})
	prefs, warnings := ParseUserConfig(txt)
	if len(warnings) != 0 {
		t.Fatalf("o padrão não pode nascer com aviso: %v", warnings)
	}
	if prefs.Theme != "gruvbox" || prefs.AI != "claude" {
		t.Fatalf("%+v", prefs)
	}
	if len(prefs.Commands) != len(AppSlots) {
		t.Fatalf("esperava %d slots, veio %d", len(AppSlots), len(prefs.Commands))
	}
	if prefs.Commands["1"].Command == "" || prefs.Commands["1"].Name == "" {
		t.Fatalf("o slot 1 nasce com o exemplo pronto: %+v", prefs.Commands["1"])
	}
	if !strings.Contains(txt, "claude, opencode") {
		t.Fatal("o comentário da IA deve listar o que foi detectado")
	}
}

// Erro de digitação no JSON não pode zerar tema e IA nem derrubar a TUI.
func TestUserConfigSurvivesBrokenJSON(t *testing.T) {
	prefs, warnings := ParseUserConfig(`
# comentário
Theme: nord
Commands: {
  "1": {"name": "Obsidian" "command": "obsidian &"}
}
AI: claude
`)
	if prefs.Theme != "nord" || prefs.AI != "claude" {
		t.Fatalf("o resto do arquivo tinha que valer: %+v", prefs)
	}
	if len(warnings) == 0 {
		t.Fatal("JSON quebrado precisa avisar")
	}
}

func TestUserConfigRejectsUnknownSlot(t *testing.T) {
	prefs, warnings := ParseUserConfig(`Commands: {"1": {"name":"ok","command":"ls"}, "12": {"name":"x","command":"y"}}`)
	if _, ok := prefs.Commands["12"]; ok {
		t.Fatal("tecla fora de 0-9 não pode virar atalho")
	}
	if len(prefs.Commands) != 1 || len(warnings) != 1 {
		t.Fatalf("commands=%v warnings=%v", prefs.Commands, warnings)
	}
}

// O seletor de tema (Shift+T) reescreve uma linha e preserva o resto.
func TestSetUserConfigThemeKeepsComments(t *testing.T) {
	txt := DefaultUserConfig("dark", nil)
	out := SetUserConfigTheme(txt, "nord")
	if !strings.Contains(out, "Theme: nord") {
		t.Fatal("tema não foi trocado")
	}
	if !strings.Contains(out, "# THEME — as cores do DevScope.") {
		t.Fatal("comentários foram perdidos")
	}
	prefs, _ := ParseUserConfig(out)
	if prefs.Commands == nil {
		t.Fatal("os atalhos foram perdidos ao gravar o tema")
	}
}

// Comentário depois do valor não pode entrar no valor.
func TestUserConfigIgnoresTrailingComment(t *testing.T) {
	prefs, _ := ParseUserConfig("Theme: nord # o meu preferido\nAI: claude # com login\n")
	if prefs.Theme != "nord" || prefs.AI != "claude" {
		t.Fatalf("%+v", prefs)
	}
}
