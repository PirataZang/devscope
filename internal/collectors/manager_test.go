package collectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

// O que acontece no disco com o DevScope aberto chega na lista sem reabrir:
// pasta apagada sai, pasta nova entra, o conhecido mantém o que já foi coletado.
func TestReconcileProjectsFollowsTheDisk(t *testing.T) {
	root := t.TempDir()
	kept, gone, added := filepath.Join(root, "kept"), filepath.Join(root, "gone"), filepath.Join(root, "added")
	for _, d := range []string{kept, gone, added} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	known := []core.Project{
		{Path: kept, Health: core.HealthHealthy},
		{Path: gone},
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	got := reconcileProjects(known, []core.Project{{Path: kept}, {Path: added}})
	if len(got) != 2 || got[0].Path != kept || got[1].Path != added {
		t.Fatalf("%+v", got)
	}
	if got[0].Health != core.HealthHealthy {
		t.Fatal("o projeto conhecido entra como estava, não como stub da varredura")
	}
}

func TestPopulateGitSummariesFollowsBranchSwitch(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "x")

	projects := []core.Project{{Path: repo}}
	populateGitSummaries(projects)
	projects[0].Git.Ahead = 4 // coletado por outro ciclo; não pode se perder
	runGit(t, repo, "switch", "-c", "feat/x")
	populateGitSummaries(projects)
	if g := projects[0].Git; g.Branch != "feat/x" || g.Ahead != 4 {
		t.Fatalf("%+v", g)
	}
}
