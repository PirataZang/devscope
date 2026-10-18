package collectors

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestParseGitPending(t *testing.T) {
	out := "# branch.oid abc\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -1\n" +
		"1 .M N... 100644 100644 100644 a b x.go\n? novo.txt\n"
	p := parseGitPending(out)
	if !p.Upstream || p.Ahead != 2 || p.Behind != 1 || p.Changes != 2 || !p.Any() {
		t.Fatalf("%+v", p)
	}
	if parseGitPending("# branch.head main\n").Any() {
		t.Fatal("repo limpo sem upstream não tem pendência")
	}
}

func TestCollectGitPending(t *testing.T) {
	ctx := context.Background()
	if CollectGitPending(ctx, t.TempDir()) != nil {
		t.Fatal("fora de repo é nil")
	}
	dir := t.TempDir()
	runGit(t, dir, "init")
	writeFile(t, filepath.Join(dir, "a.txt"), "a")
	if p := CollectGitPending(ctx, dir); p == nil || p.Changes != 1 || p.Upstream {
		t.Fatalf("1 arquivo novo, sem upstream: %+v", p)
	}
}

// O refresh do docker regrava os projetos a cada 2s: a medida tem de voltar.
func TestApplyGitPendingSurvivesRefresh(t *testing.T) {
	projects := []core.Project{{Path: "/a"}, {Path: "/b"}}
	applyGitPending(projects, map[string]*core.GitPending{"/a": {Changes: 3}})
	if projects[0].Pending == nil || projects[0].Pending.Changes != 3 || projects[1].Pending != nil {
		t.Fatalf("%+v", projects)
	}
}
