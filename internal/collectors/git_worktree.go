package collectors

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GitWorktree é uma entrada de `git worktree list --porcelain`, mais o que a
// tela precisa saber dela sem abrir: quantos arquivos mudaram e o assunto do
// último commit.
type GitWorktree struct {
	Path     string
	Head     string
	Branch   string // vazio quando detached ou bare
	Bare     bool
	Detached bool
	Main     bool // a primeira da lista: não pode ser removida
	Locked   bool
	// LockReason / PruneReason vêm do porcelain e podem ser vazios mesmo com
	// a flag ligada.
	LockReason  string
	Prunable    bool
	PruneReason string
	Changes     int
	Subject     string
}

// GitWorktrees lista as worktrees do repositório em `repo` — vale chamar de
// dentro de qualquer uma delas, a lista é a mesma.
func GitWorktrees(repo string) ([]GitWorktree, error) {
	out, err := gitRunOutput(repo, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	list := parseWorktreePorcelain(out)
	for i := range list {
		w := &list[i]
		// Pasta sumida ou repositório bare: não há working tree para medir.
		if w.Bare || w.Prunable {
			continue
		}
		if st := strings.TrimRight(gitOutput(w.Path, "status", "--porcelain"), "\n"); st != "" {
			w.Changes = len(strings.Split(st, "\n"))
		}
		w.Subject = strings.TrimSpace(gitOutput(w.Path, "log", "-1", "--format=%s"))
	}
	return list, nil
}

// parseWorktreePorcelain lê o formato `--porcelain -z`: um atributo por campo
// terminado em NUL, e um NUL extra separando as worktrees. Com -z o caminho
// pode ter qualquer caractere.
func parseWorktreePorcelain(out string) []GitWorktree {
	var list []GitWorktree
	var cur *GitWorktree
	for _, field := range strings.Split(out, "\x00") {
		if field == "" {
			cur = nil
			continue
		}
		key, val, _ := strings.Cut(field, " ")
		if key == "worktree" {
			list = append(list, GitWorktree{Path: val, Main: len(list) == 0})
			cur = &list[len(list)-1]
			continue
		}
		if cur == nil {
			continue
		}
		switch key {
		case "HEAD":
			cur.Head = val
		case "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "bare":
			cur.Bare = true
		case "detached":
			cur.Detached = true
		case "locked":
			cur.Locked, cur.LockReason = true, val
		case "prunable":
			cur.Prunable, cur.PruneReason = true, val
		}
	}
	return list
}

// GitWorktreePath é onde o DevScope cria a worktree da branch: pasta irmã da
// principal, `<repo>-<branch>` — a convenção dos exemplos do `git worktree`
// (../hotfix). Dentro do repositório ela apareceria como não rastreada.
func GitWorktreePath(mainPath, branch string) string {
	name := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(strings.TrimSpace(branch))
	return filepath.Join(filepath.Dir(mainPath), filepath.Base(mainPath)+"-"+name)
}

// GitWorktreeAdd cria a worktree de `branch` em `path`. Branch local ou remota
// existente faz checkout (o git cria a branch rastreando o remoto sozinho);
// nome novo cria a branch a partir do HEAD de `repo`.
func GitWorktreeAdd(repo, path, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", fmt.Errorf("branch vazia")
	}
	// for-each-ref casa por prefixo ("feat" pegaria "feat/x"): compara exato.
	// lstrip=2 em refs/heads/x dá x; lstrip=3 em refs/remotes/origin/x dá x.
	refs := gitOutput(repo, "for-each-ref", "--format=%(refname:lstrip=2)", "refs/heads") +
		gitOutput(repo, "for-each-ref", "--format=%(refname:lstrip=3)", "refs/remotes")
	exists := false
	for _, r := range strings.Split(refs, "\n") {
		if r == branch {
			exists = true
			break
		}
	}
	if exists {
		return gitRunOutput(repo, "worktree", "add", path, branch)
	}
	return gitRunOutput(repo, "worktree", "add", "-b", branch, path)
}

// GitWorktreeRemove apaga a worktree. Sem force o git recusa a que tem
// alteração — quem pergunta antes é a tela.
func GitWorktreeRemove(repo, path string, force bool) (string, error) {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	return gitRunOutput(repo, append(args, path)...)
}

// GitWorktreePrune limpa o registro das worktrees cuja pasta foi apagada à mão.
func GitWorktreePrune(repo string) (string, error) {
	return gitRunOutput(repo, "worktree", "prune", "-v")
}
