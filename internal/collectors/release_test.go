package collectors

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNextVersions(t *testing.T) {
	cases := map[string][3]string{
		"v1.6.9": {"v1.6.10", "v1.7.0", "v2.0.0"},
		"0.3.1":  {"0.3.2", "0.4.0", "1.0.0"},
		"":       {"v0.0.1", "v0.1.0", "v1.0.0"},
	}
	for last, want := range cases {
		if got := NextVersions(last); got != want {
			t.Errorf("%q: %v, queria %v", last, got, want)
		}
	}
}

func TestParseReleaseTagsKeepsOnlySemver(t *testing.T) {
	out := "v1.10.0\t2026-10-01T10:00:00-03:00\nnightly\t2026-10-02T10:00:00-03:00\nv1.9.2\t2026-09-01T10:00:00-03:00\n"
	tags := parseReleaseTags(out)
	if len(tags) != 2 || tags[0].Name != "v1.10.0" || tags[1].Name != "v1.9.2" || tags[0].Date.IsZero() {
		t.Fatalf("%+v", tags)
	}
}

func TestChangelogSection(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "CHANGELOG.md")
	writeFile(t, f, "# Changelog\n\n## [Unreleased]\n\n## [1.7.0] - 2026-10-18\n\n### Added\n- Worktrees\n\n## [1.6.9] - 2026-09-25\n- velho\n")
	if got := ChangelogSection(f, "v1.7.0"); got != "### Added\n- Worktrees" {
		t.Fatalf("%q", got)
	}
	if ChangelogSection(f, "v2.0.0") != "" {
		t.Fatal("versão sem seção devolve vazio")
	}
}

// Ciclo inteiro contra um remoto de verdade (bare local): mede, cria a tag
// anotada com a seção do CHANGELOG e confere que ela chegou no remoto.
func TestReleaseLifecycle(t *testing.T) {
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "app")
	runGit(t, root, "init", "--bare", remote)
	runGit(t, root, "clone", remote, repo)
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	writeFile(t, filepath.Join(repo, "CHANGELOG.md"), "## [0.2.0]\n\n- coisa nova\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "primeiro")
	runGit(t, repo, "tag", "v0.1.0")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "segundo")
	runGit(t, repo, "push", "origin", "HEAD")

	st := CollectRelease(context.Background(), repo)
	if st.Last.Name != "v0.1.0" || st.Count != 1 || len(st.Commits) != 1 || !strings.HasSuffix(st.Commits[0], "segundo") {
		t.Fatalf("estado: %+v", st)
	}
	if st.Pending == nil || st.Pending.Changes != 0 || st.Changelog == "" {
		t.Fatalf("pendências/changelog: %+v", st)
	}

	msg := ChangelogSection(st.Changelog, "v0.2.0")
	if out, err := CreateReleaseTag(repo, "v0.2.0", msg); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if got := strings.TrimSpace(gitOutput(remote, "tag", "--list")); !strings.Contains(got, "v0.2.0") {
		t.Fatalf("a tag não chegou no remoto: %q", got)
	}
	if body := gitOutput(repo, "tag", "-l", "--format=%(contents)", "v0.2.0"); !strings.Contains(body, "coisa nova") {
		t.Fatalf("a mensagem da tag é a seção do CHANGELOG: %q", body)
	}
	if st = CollectRelease(context.Background(), repo); st.Last.Name != "v0.2.0" || st.Count != 0 {
		t.Fatalf("depois da release: %+v", st)
	}
}

// Push que falha não deixa tag local órfã para atrapalhar a próxima tentativa.
func TestCreateReleaseTagUndoesOnPushFailure(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "x")
	if _, err := CreateReleaseTag(repo, "v1.0.0", ""); err == nil {
		t.Fatal("sem origin o push tem de falhar")
	}
	if strings.TrimSpace(gitOutput(repo, "tag", "--list")) != "" {
		t.Fatal("a tag local devia ter sido desfeita")
	}
}

// Cria, commita SÓ os arquivos de release e envia: a alteração que o usuário já tinha
// continua pendente, e o remoto passa a ter o workflow.
func TestCreateReleaseWorkflow(t *testing.T) {
	root := t.TempDir()
	remote, repo := filepath.Join(root, "remote.git"), filepath.Join(root, "app")
	runGit(t, root, "init", "--bare", remote)
	runGit(t, root, "clone", remote, repo)
	runGit(t, repo, "config", "user.email", "t@t.com")
	runGit(t, repo, "config", "user.name", "T")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "primeiro")
	runGit(t, repo, "push", "-u", "origin", "HEAD")
	writeFile(t, filepath.Join(repo, "a.txt"), "mexido e não commitado")

	created, kind, err := CreateReleaseFiles(repo, true, true)
	if err != nil || kind != "github" || len(created) != 2 {
		t.Fatalf("%v %q %v", created, kind, err)
	}
	if tagWorkflow(repo) != "release.yml" {
		t.Fatal("o workflow criado tem de rodar em tag")
	}
	if p := CollectGitPending(context.Background(), repo); p.Changes != 1 || p.Ahead != 0 {
		t.Fatalf("só o workflow entra no commit, e ele foi enviado: %+v", p)
	}
	if got := gitOutput(remote, "log", "-1", "--format=%s"); strings.TrimSpace(got) != "chore: arquivos de release (workflow e CHANGELOG)" {
		t.Fatalf("o remoto tem de receber o commit: %q", got)
	}
	if st := CollectRelease(context.Background(), repo); st.Changelog == "" || st.Workflow == "" {
		t.Fatalf("o Releases passa a enxergar os dois: %+v", st)
	}
	if _, _, err := CreateReleaseFiles(repo, true, false); err == nil {
		t.Fatal("arquivo existente não é sobrescrito")
	}
}

// Erro de indentação no modelo só apareceria no GitHub, depois da tag enviada.
func TestReleaseWorkflowTemplatesAreValidYAML(t *testing.T) {
	for name, tpl := range map[string]string{"goreleaser": goreleaserWorkflow, "github": githubReleaseWorkflow} {
		var doc struct {
			On struct {
				Push struct {
					Tags []string `yaml:"tags"`
				} `yaml:"push"`
			} `yaml:"on"`
			Jobs map[string]any `yaml:"jobs"`
		}
		if err := yaml.Unmarshal([]byte(tpl), &doc); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(doc.On.Push.Tags) != 1 || doc.On.Push.Tags[0] != "v*" || doc.Jobs["release"] == nil {
			t.Fatalf("%s: %+v", name, doc)
		}
	}
}

func TestReleaseWorkflowTemplatePicksGoreleaser(t *testing.T) {
	dir := t.TempDir()
	if _, kind := ReleaseWorkflowTemplate(dir); kind != "github" {
		t.Fatalf("sem goreleaser: %q", kind)
	}
	writeFile(t, filepath.Join(dir, ".goreleaser.yaml"), "version: 2\n")
	content, kind := ReleaseWorkflowTemplate(dir)
	if kind != "goreleaser" || !strings.Contains(content, "goreleaser-action") || !strings.Contains(content, "go-version-file: go.mod") {
		t.Fatalf("%q", kind)
	}
}

func TestTagWorkflow(t *testing.T) {
	dir := t.TempDir()
	wf := filepath.Join(dir, ".github", "workflows")
	writeTree(t, filepath.Join(wf, "ci.yml"), "on:\n  push:\n    branches: [main]\n")
	writeTree(t, filepath.Join(wf, "release.yml"), "on:\n  push:\n    tags:\n      - \"v*\"\n")
	if got := tagWorkflow(dir); got != "release.yml" {
		t.Fatalf("%q", got)
	}
}

// O CHANGELOG criado traz as instruções para IA num comentário, e elas não
// podem confundir a leitura das seções: a seção da versão continua sendo só
// o que está embaixo do título dela.
func TestChangelogTemplateKeepsSectionsReadable(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "CHANGELOG.md")
	body := strings.Replace(changelogTemplate, "## [Unreleased]\n",
		"## [Unreleased]\n\n## [0.2.0] - 2026-10-18\n\n### Added\n- **Login** lembra o e-mail\n", 1)
	writeFile(t, f, body)
	if got := ChangelogSection(f, "v0.2.0"); got != "### Added\n- **Login** lembra o e-mail" {
		t.Fatalf("%q", got)
	}
	for _, want := range []string{"<!--", "INSTRUÇÕES PARA IA", "git describe --tags", "-->"} {
		if !strings.Contains(changelogTemplate, want) {
			t.Fatalf("faltou %q no modelo", want)
		}
	}
	for _, line := range strings.Split(changelogTemplate, "\n") {
		if strings.HasPrefix(line, "## ") && line != "## [Unreleased]" {
			t.Fatalf("linha começando com ## dentro das instruções vira seção: %q", line)
		}
	}
}
