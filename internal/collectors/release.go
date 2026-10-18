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
