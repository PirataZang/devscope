package collectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseWorktreePorcelain(t *testing.T) {
	out := "worktree /r/app\x00HEAD aaa\x00branch refs/heads/main\x00\x00" +
		"worktree /r/app-feat/x y\x00HEAD bbb\x00branch refs/heads/feat/x\x00locked usb\x00\x00" +
		"worktree /r/gone\x00HEAD ccc\x00detached\x00prunable gitdir file points to non-existent location\x00\x00"
	got := parseWorktreePorcelain(out)
	if len(got) != 3 {
		t.Fatalf("esperava 3 worktrees, veio %d: %+v", len(got), got)
	}
	if !got[0].Main || got[0].Branch != "main" || got[1].Main {
		t.Fatalf("a principal é a primeira: %+v", got)
	}
	if got[1].Path != "/r/app-feat/x y" || got[1].Branch != "feat/x" || !got[1].Locked || got[1].LockReason != "usb" {
		t.Fatalf("caminho com espaço / branch / trava: %+v", got[1])
	}
	if !got[2].Detached || !got[2].Prunable || got[2].Branch != "" {
		t.Fatalf("detached e podável: %+v", got[2])
	}
}

// Ciclo inteiro num repositório de verdade: criar com branch nova, criar com
// branch existente, medir alteração, remover com e sem force, podar.
func TestGitWorktreeLifecycle(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "app")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@test.com")
	runGit(t, repo, "config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-m", "primeiro")
	runGit(t, repo, "branch", "feat")

	newPath := GitWorktreePath(repo, "nova/coisa")
	if want := filepath.Join(root, "app-nova-coisa"); newPath != want {
		t.Fatalf("pasta irmã: %q, queria %q", newPath, want)
	}
	if out, err := GitWorktreeAdd(repo, newPath, "nova/coisa"); err != nil {
		t.Fatalf("branch nova: %v %s", err, out)
	}
	featPath := GitWorktreePath(repo, "feat")
	if out, err := GitWorktreeAdd(repo, featPath, "feat"); err != nil {
		t.Fatalf("branch existente: %v %s", err, out)
	}

	writeFile(t, filepath.Join(featPath, "b.txt"), "b")
	list, err := GitWorktrees(repo)
	if err != nil || len(list) != 3 {
		t.Fatalf("esperava 3 worktrees: %v %+v", err, list)
	}
	byBranch := map[string]GitWorktree{}
	for _, w := range list {
		byBranch[w.Branch] = w
	}
	if byBranch["feat"].Changes != 1 || byBranch["nova/coisa"].Changes != 0 || byBranch["feat"].Subject != "primeiro" {
		t.Fatalf("alterações/assunto: %+v", byBranch)
	}

	if _, err := GitWorktreeRemove(repo, featPath, false); err == nil {
		t.Fatal("sem force o git devia recusar worktree com alteração")
	}
	if out, err := GitWorktreeRemove(repo, featPath, true); err != nil {
		t.Fatalf("force: %v %s", err, out)
	}

	if err := os.RemoveAll(newPath); err != nil {
		t.Fatal(err)
	}
	list, _ = GitWorktrees(repo)
	prunable := 0
	for _, w := range list {
		if w.Prunable {
			prunable++
		}
	}
	if prunable != 1 {
		t.Fatalf("pasta apagada à mão vira podável: %+v", list)
	}
	if out, err := GitWorktreePrune(repo); err != nil {
		t.Fatalf("prune: %v %s", err, out)
	}
	if list, _ = GitWorktrees(repo); len(list) != 1 {
		t.Fatalf("depois do prune sobra a principal: %+v", list)
	}
}
