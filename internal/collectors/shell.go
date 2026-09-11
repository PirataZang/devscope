package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ProjectShell(path string) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Dir = path
	return cmd
}

// AITools são os agentes de IA de terminal que o DevScope sabe abrir, na ordem
// em que procura quando tools.ai não está configurado.
var AITools = []string{"claude", "opencode", "aider", "codex", "gemini"}

// DetectAITools devolve os agentes instalados no PATH, na ordem de AITools.
func DetectAITools() []string {
	var found []string
	for _, name := range AITools {
		if _, err := exec.LookPath(name); err == nil {
			found = append(found, name)
		}
	}
	return found
}

// ProjectAI monta o comando do agente de IA com cwd no projeto. command vazio
// cai no primeiro do PATH; com argumentos ("claude --continue") também vale.
// Devolve o nome resolvido para a tela poder dizer o que rodou.
func ProjectAI(path, command string) (*exec.Cmd, string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		found := DetectAITools()
		if len(found) == 0 {
			return nil, "", fmt.Errorf("nenhum agente de IA no PATH (%s)", strings.Join(AITools, ", "))
		}
		parts = []string{found[0]}
	}
	bin, err := exec.LookPath(parts[0])
	if err != nil {
		return nil, parts[0], fmt.Errorf("%s não está no PATH", parts[0])
	}
	cmd := exec.Command(bin, parts[1:]...)
	cmd.Dir = path
	return cmd, parts[0], nil
}

// EditorCommand abre um arquivo no editor configurado, no $EDITOR, ou no vi.

// ExternalApp monta o comando de um atalho de app externo. Vai pelo shell para
// aceitar o que a pessoa já escreveria no terminal: argumentos, ~, variáveis e
// pipes. O "&" no fim decide o modo — solto (a TUI continua na frente) ou de
// terminal (o programa assume a tela até sair).
func ExternalApp(dir, command string) (cmd *exec.Cmd, detached bool) {
	command = strings.TrimSpace(command)
	detached = strings.HasSuffix(command, "&")
	if detached {
		command = strings.TrimSpace(strings.TrimSuffix(command, "&"))
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd = exec.Command(shell, "-c", command)
	cmd.Dir = dir
	return cmd, detached
}
