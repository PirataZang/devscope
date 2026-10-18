package collectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/core"
)

// ReleaseTag é uma versão já lançada.
type ReleaseTag struct {
	Name string
	Date time.Time
}

// ReleaseState é tudo que a tela de Releases precisa para responder "posso
// lançar agora, e o quê?" — medido de uma vez, fora do View.
type ReleaseState struct {
	Last     ReleaseTag   // última tag semver; Name "" = nunca houve release
	Previous []ReleaseTag // as anteriores, mais nova primeiro (sem a Last)
	Commits  []string     // "hash assunto" desde a Last, mais novo primeiro
	Count    int          // total de commits desde a Last (Commits corta em 50)
	Branch   string
	Default  string
	Pending  *core.GitPending
	// Changelog é o CHANGELOG.md do projeto, "" quando não existe.
	Changelog string
	// Workflow é o arquivo de workflow que dispara em push de tag — é ele que
	// transforma a tag em release de verdade (build, binários, página).
	Workflow string
	// Origin é a URL do remoto para onde a tag vai; "" = sem remoto.
	Origin string
	// GH diz se o GitHub CLI está instalado — sem ele não dá para acompanhar o
	// workflow, mas a release sai igual.
	GH bool
}

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// CollectRelease mede o repositório. Não faz fetch: a divergência com o remoto
// é a da última vez que você buscou — a tela diz isso.
func CollectRelease(ctx context.Context, path string) ReleaseState {
	st := ReleaseState{
		Branch:  strings.TrimSpace(gitOutput(path, "rev-parse", "--abbrev-ref", "HEAD")),
		Default: gitDefaultBranch(path),
		Pending: CollectGitPending(ctx, path),
	}
	tags := parseReleaseTags(gitOutput(path, "tag", "--list", "--sort=-v:refname",
		"--format=%(refname:short)\t%(creatordate:iso-strict)"))
	if len(tags) > 0 {
		st.Last = tags[0]
		st.Previous = tags[1:minInt(len(tags), 7)]
	}

	rangeArg := "HEAD"
	if st.Last.Name != "" {
		rangeArg = st.Last.Name + "..HEAD"
	}
	st.Count, _ = strconv.Atoi(strings.TrimSpace(gitOutput(path, "rev-list", "--count", rangeArg)))
	if log := strings.TrimSpace(gitOutput(path, "log", "--max-count=50", "--format=%h %s", rangeArg)); log != "" {
		st.Commits = strings.Split(log, "\n")
	}

	if f := firstFile(path, "CHANGELOG.md", "changelog.md", "CHANGELOG"); f != "" {
		st.Changelog = f
	}
	st.Workflow = tagWorkflow(path)
	st.Origin = strings.TrimSpace(gitOutput(path, "remote", "get-url", "origin"))
	st.GH = GHAAvailable()
	return st
}

// parseReleaseTags fica só com as tags semver — `deploy-prod` e `nightly` não
// são versão e não entram na conta da próxima.
func parseReleaseTags(out string) []ReleaseTag {
	var tags []ReleaseTag
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, date, _ := strings.Cut(line, "\t")
		if !semverRe.MatchString(name) {
			continue
		}
		t, _ := time.Parse(time.RFC3339, date)
		tags = append(tags, ReleaseTag{Name: name, Date: t})
	}
	return tags
}

// NextVersions devolve patch, minor e major a partir da última tag, mantendo
// o "v" se a tag usa. Sem release nenhuma, começa em 0.1.0.
func NextVersions(last string) [3]string {
	m := semverRe.FindStringSubmatch(last)
	if m == nil {
		return [3]string{"v0.0.1", "v0.1.0", "v1.0.0"}
	}
	prefix := ""
	if strings.HasPrefix(last, "v") {
		prefix = "v"
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat, _ := strconv.Atoi(m[3])
	return [3]string{
		fmt.Sprintf("%s%d.%d.%d", prefix, maj, min, pat+1),
		fmt.Sprintf("%s%d.%d.0", prefix, maj, min+1),
		fmt.Sprintf("%s%d.0.0", prefix, maj+1),
	}
}

// ChangelogSection devolve o texto da seção `## [1.7.0]` (Keep a Changelog),
// sem o título. "" quando a versão não tem seção.
func ChangelogSection(file, version string) string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	version = strings.TrimPrefix(version, "v")
	var b strings.Builder
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			head := strings.TrimPrefix(line, "## ")
			in = strings.HasPrefix(head, "["+version+"]") || strings.HasPrefix(head, version+" ") || head == version
			continue
		}
		if in {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

// CreateReleaseTag cria a tag anotada no HEAD e envia só ela para o origin.
// A mensagem vira o corpo da tag — o GitHub e o goreleaser leem dali.
func CreateReleaseTag(path, tag, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		message = "Release " + tag
	}
	if out, err := gitRunOutput(path, "tag", "-a", tag, "-m", message); err != nil {
		return out, err
	}
	out, err := gitRunOutput(path, "push", "origin", "refs/tags/"+tag)
	if err != nil {
		// Tag local sem push confunde a próxima tentativa ("já existe"):
		// desfaz para o usuário poder tentar de novo depois de resolver.
		_, _ = gitRunOutput(path, "tag", "-d", tag)
	}
	return out, err
}

// ReleaseRuns devolve o run mais recente do GitHub Actions para cada tag —
// run disparado por tag vem com o nome dela no campo da branch.
func ReleaseRuns(path string, tags []string) map[string]GHARun {
	if !GHAAvailable() || len(tags) == 0 {
		return nil
	}
	runs, err := GHAListRuns(path, 40)
	if err != nil {
		return nil
	}
	want := map[string]bool{}
	for _, t := range tags {
		want[t] = true
	}
	out := map[string]GHARun{}
	for _, r := range runs { // gh lista do mais novo para o mais velho
		if _, seen := out[r.Branch]; want[r.Branch] && !seen {
			out[r.Branch] = r
		}
	}
	return out
}

// ReleaseWorkflowPath é onde o DevScope cria o workflow de release.
const ReleaseWorkflowPath = ".github/workflows/release.yml"

// ReleaseWorkflowTemplate escolhe o modelo pelo que o projeto já tem: com
// .goreleaser.yaml, o goreleaser (binários por sistema, como o próprio
// DevScope); sem, uma Release do GitHub com as notas da tag — serve para
// qualquer stack, e as notas são a seção do CHANGELOG que vira a mensagem da
// tag.
func ReleaseWorkflowTemplate(path string) (content, kind string) {
	if firstFile(path, ".goreleaser.yaml", ".goreleaser.yml") != "" {
		return goreleaserWorkflow, "goreleaser"
	}
	return githubReleaseWorkflow, "github"
}

const goreleaserWorkflow = `name: Release

# Criado pelo DevScope: roda quando uma tag vX.Y.Z chega ao GitHub.
on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`

const githubReleaseWorkflow = `name: Release

# Criado pelo DevScope: roda quando uma tag vX.Y.Z chega ao GitHub e publica a
# Release com a mensagem da tag como notas (o DevScope põe nela a seção do
# CHANGELOG.md da versão).
on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      # O checkout pode trazer a tag sem a mensagem; busca de novo a anotada.
      - run: git fetch --force --tags

      - name: Notas da tag
        run: git tag -l --format='%(contents)' "$GITHUB_REF_NAME" > notes.md

      - name: Publicar a release
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release create "$GITHUB_REF_NAME" --title "$GITHUB_REF_NAME" --notes-file notes.md --verify-tag
`

// ChangelogPath é onde o DevScope cria o CHANGELOG.
const ChangelogPath = "CHANGELOG.md"

// changelogTemplate é o CHANGELOG inicial. O bloco de instruções vai num
// comentário HTML: o GitHub não mostra, mas qualquer IA que abrir o arquivo
// para escrever as notas lê antes — e escreve no formato que o Releases lê.
const changelogTemplate = `# Changelog

Todas as mudanças notáveis deste projeto ficam neste arquivo.

O formato segue o [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/)
e o projeto usa [Versionamento Semântico](https://semver.org/lang/pt-BR/).

<!--
INSTRUÇÕES PARA IA (Claude, Copilot, Cursor, ChatGPT…) — leia antes de editar.

Este arquivo é lido pelo DevScope (módulo Releases) na hora de lançar:
  - a seção "## [X.Y.Z] - AAAA-MM-DD" da versão vira a MENSAGEM DA TAG e as
    notas da Release no GitHub — escreva para quem vai ler a página da release;
  - se existe seção para a próxima patch, minor ou major, essa é a versão que
    o DevScope sugere. Então a versão escolhida aqui é a que vai ser lançada.

Como escrever:
  1. Levante o que mudou desde a última release, sem inventar nada:
       git log $(git describe --tags --abbrev=0)..HEAD --oneline
     e o diff quando a mensagem do commit não disser o efeito.
  2. Novidades entram em "## [Unreleased]". Na hora de lançar, renomeie para
     "## [X.Y.Z] - AAAA-MM-DD" e deixe um "## [Unreleased]" vazio acima dela.
  3. Agrupe em ### Added, ### Changed, ### Deprecated, ### Removed, ### Fixed e
     ### Security — nesta ordem, só os que tiverem item.
  4. Um item por mudança que o USUÁRIO percebe, não um por commit. Comece pelo
     efeito ("Login lembra o último e-mail"), não pela implementação
     ("refatora AuthService"). Nome da funcionalidade em negrito.
  5. Escolha a versão pelo maior impacto: algo removido ou incompatível →
     major; funcionalidade nova → minor; só correção ou ajuste pequeno → patch.
  6. Escreva no idioma deste arquivo. Não apague nem reescreva versões já
     lançadas.
-->

## [Unreleased]
`

// ReleaseSetup diz o que falta para o release publicar algo: o workflow que
// roda em tag e o CHANGELOG que vira as notas.
func ReleaseSetup(st ReleaseState) (workflow, changelog bool) {
	return st.Workflow == "", st.Changelog == ""
}

// CreateReleaseFiles cria o que falta (workflow de tag e/ou CHANGELOG.md),
// commita SÓ esses arquivos — o que mais estiver alterado fica como estava — e
// envia a branch. Tem de ir antes da tag: o GitHub roda o workflow que está no
// commit da tag, e a mensagem da tag sai do CHANGELOG.
func CreateReleaseFiles(path string, workflow, changelog bool) (created []string, kind string, err error) {
	write := func(rel, content string) error {
		file := filepath.Join(path, filepath.FromSlash(rel))
		if _, err := os.Stat(file); err == nil {
			return fmt.Errorf("%s já existe — o DevScope não sobrescreve", rel)
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			return err
		}
		created = append(created, rel)
		return nil
	}
	if workflow {
		var content string
		content, kind = ReleaseWorkflowTemplate(path)
		if err := write(ReleaseWorkflowPath, content); err != nil {
			return created, kind, err
		}
	}
	if changelog {
		if err := write(ChangelogPath, changelogTemplate); err != nil {
			return created, kind, err
		}
	}
	if len(created) == 0 {
		return nil, kind, fmt.Errorf("nada a criar")
	}

	msg := "chore: arquivos de release (workflow e CHANGELOG)"
	switch {
	case !changelog:
		msg = "ci: workflow de release"
	case !workflow:
		msg = "docs: CHANGELOG"
	}
	if out, err := gitRunOutput(path, append([]string{"add", "--"}, created...)...); err != nil {
		return created, kind, fmt.Errorf("git add: %s", firstNonEmptyStr(out, err.Error()))
	}
	if out, err := gitRunOutput(path, append([]string{"commit", "-m", msg, "--"}, created...)...); err != nil {
		return created, kind, fmt.Errorf("commit: %s", firstNonEmptyStr(out, err.Error()))
	}
	if out, err := gitRunOutput(path, "push"); err != nil {
		return created, kind, fmt.Errorf("commit feito, mas o push falhou: %s", firstNonEmptyStr(out, err.Error()))
	}
	return created, kind, nil
}

// gitDefaultBranch é a branch para onde o origin aponta; sem isso, main ou
// master, o que existir.
func gitDefaultBranch(path string) string {
	if ref := strings.TrimSpace(gitOutput(path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")); ref != "" {
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, b := range []string{"main", "master"} {
		if strings.TrimSpace(gitOutput(path, "rev-parse", "--verify", "--quiet", "refs/heads/"+b)) != "" {
			return b
		}
	}
	return ""
}

// tagWorkflow acha o workflow que roda em push de tag. Leitura de texto, não
// de YAML: basta saber se o arquivo fala de `tags:` dentro de um `push:`.
func tagWorkflow(path string) string {
	files, _ := filepath.Glob(filepath.Join(path, ".github", "workflows", "*.y*ml"))
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := string(raw)
		if i := strings.Index(s, "push:"); i >= 0 && strings.Contains(s[i:], "tags:") {
			return filepath.Base(f)
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
