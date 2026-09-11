package collectors

import (
	"path/filepath"
	"testing"
)

func TestProjectShellUsesProjectDir(t *testing.T) {
	path := filepath.Clean("/tmp/my-project")
	cmd := ProjectShell(path)
	if cmd.Dir != path {
		t.Fatalf("expected Dir %q, got %q", path, cmd.Dir)
	}
	if len(cmd.Args) == 0 {
		t.Fatal("expected shell command")
	}
}

func TestProjectAIUsesProjectDirAndArgs(t *testing.T) {
	path := filepath.Clean("/tmp/my-project")
	// "sh" existe em qualquer máquina: o que se testa é o parsing e o cwd,
	// não a presença de um agente de IA.
	cmd, name, err := ProjectAI(path, "sh -c true")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if name != "sh" {
		t.Fatalf("nome resolvido = %q", name)
	}
	if cmd.Dir != path {
		t.Fatalf("Dir = %q, esperado %q", cmd.Dir, path)
	}
	if got := cmd.Args[len(cmd.Args)-2:]; got[0] != "-c" || got[1] != "true" {
		t.Fatalf("argumentos perdidos: %v", cmd.Args)
	}
}

func TestProjectAIReportsMissingBinary(t *testing.T) {
	if _, _, err := ProjectAI("/tmp", "definitivamente-nao-existe-aqui"); err == nil {
		t.Fatal("comando inexistente precisa devolver erro")
	}
}

// O "&" no fim é o que separa app de janela (solta e devolve o terminal) de
// comando que assume a tela — é o idioma que a pessoa já escreveria no shell.
func TestExternalAppDetachedByTrailingAmpersand(t *testing.T) {
	cmd, detached := ExternalApp("/var/www/api", "obsidian &")
	if !detached {
		t.Fatal(`"&" no fim precisa abrir solto`)
	}
	if cmd.Dir != "/var/www/api" {
		t.Fatalf("Dir = %q", cmd.Dir)
	}
	if last := cmd.Args[len(cmd.Args)-1]; last != "obsidian" {
		t.Fatalf("o & devia sair do comando: %q", last)
	}
	if _, detached := ExternalApp("/tmp", "lazygit"); detached {
		t.Fatal("sem & o programa assume o terminal")
	}
}
