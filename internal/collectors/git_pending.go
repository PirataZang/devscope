package collectors

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

// CollectGitPending mede o que ficou por fazer num repositório com UM processo:
// `git status --porcelain=v2 --branch`. --no-optional-locks evita disputar o
// index com o git que o usuário está rodando ao mesmo tempo. nil fora de repo.
func CollectGitPending(ctx context.Context, path string) *core.GitPending {
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "status", "--porcelain=v2", "--branch")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseGitPending(string(out))
}

func parseGitPending(out string) *core.GitPending {
	p := &core.GitPending{At: time.Now()}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "# branch.upstream "):
			p.Upstream = true
		case strings.HasPrefix(line, "# branch.ab "):
			f := strings.Fields(line) // # branch.ab +A -B
			if len(f) == 4 {
				p.Ahead, _ = strconv.Atoi(strings.TrimPrefix(f[2], "+"))
				p.Behind, _ = strconv.Atoi(strings.TrimPrefix(f[3], "-"))
			}
		case strings.HasPrefix(line, "#"):
		default:
			p.Changes++ // 1/2 alterado, u conflito, ? não rastreado
		}
	}
	return p
}

// applyGitPending copia as pendências medidas para os projetos. O manager
// regrava a lista de projetos a cada refresh do docker (2s); reaplicar aqui é o
// que impede a medida de 30s de sumir no meio do caminho.
func applyGitPending(projects []core.Project, pending map[string]*core.GitPending) {
	for i := range projects {
		if p, ok := pending[projects[i].Path]; ok {
			projects[i].Pending = p
		}
	}
}
