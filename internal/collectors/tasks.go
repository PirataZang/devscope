package collectors

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectTask é um comando que o próprio projeto declara: alvo do Makefile,
// receita do justfile, tarefa do Taskfile, script do package.json ou do
// composer.json. O DevScope não inventa tarefa — só lê o que já está lá.
type ProjectTask struct {
	Source string // make · just · task · npm/pnpm/yarn/bun · composer
	Name   string
	Desc   string   // comentário do autor, ou o próprio comando do script
	Argv   []string // o que roda, com cwd no projeto
}

// Key identifica a tarefa entre fontes: `build` pode existir no Makefile e no
// package.json ao mesmo tempo.
func (t ProjectTask) Key() string { return t.Source + " " + t.Name }

var taskFiles = []string{"Makefile", "makefile", "GNUmakefile", "justfile", "Justfile", ".justfile",
	"Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "package.json", "composer.json"}

// HasProjectTasks é a sondagem barata da sidebar: só Stat.
func HasProjectTasks(dir string) bool {
	for _, f := range taskFiles {
		if st, err := os.Stat(filepath.Join(dir, f)); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

// ProjectTasks lê todas as fontes do projeto, na ordem em que aparecem em
// taskFiles; dentro de cada fonte, na ordem do arquivo (scripts JSON não têm
// ordem garantida no decode, então vão em ordem alfabética).
func ProjectTasks(dir string) []ProjectTask {
	var out []ProjectTask
	if f := firstFile(dir, "GNUmakefile", "makefile", "Makefile"); f != "" {
		out = append(out, makeTargets(f)...)
	}
	if f := firstFile(dir, "justfile", "Justfile", ".justfile"); f != "" {
		out = append(out, justRecipes(f)...)
	}
	if f := firstFile(dir, "Taskfile.yml", "Taskfile.yaml", "taskfile.yml"); f != "" {
		out = append(out, taskfileTasks(f)...)
	}
	if f := firstFile(dir, "package.json"); f != "" {
		pm := nodePackageManager(dir)
		for _, s := range jsonScripts(f) {
			out = append(out, ProjectTask{Source: pm, Name: s[0], Desc: s[1], Argv: []string{pm, "run", s[0]}})
		}
	}
	if f := firstFile(dir, "composer.json"); f != "" {
		for _, s := range jsonScripts(f) {
			out = append(out, ProjectTask{Source: "composer", Name: s[0], Desc: s[1], Argv: []string{"composer", "run-script", s[0]}})
		}
	}
	return out
}

// TaskCommand roda a tarefa no terminal de verdade (dev server, prompt, cor)
// e segura a tela no fim para a saída não sumir quando o DevScope volta.
// `trap : INT` deixa o ctrl+c matar só a tarefa: o sh sobrevive para mostrar
// o código de saída.
func TaskCommand(dir string, t ProjectTask) *exec.Cmd {
	script := `trap : INT; "$@"; c=$?; printf '\n\033[2m— saiu com %d · enter volta ao DevScope\033[0m' "$c"; read _; exit $c`
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, t.Argv...)...)
	cmd.Dir = dir
	return cmd
}

func firstFile(dir string, names ...string) string {
	for _, n := range names {
		p := filepath.Join(dir, n)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// nodePackageManager segue o lockfile: rodar `npm run` num projeto pnpm
// funciona, mas instala diferente e confunde.
func nodePackageManager(dir string) string {
	for _, c := range []struct{ lock, pm string }{
		{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"bun.lockb", "bun"}, {"bun.lock", "bun"},
	} {
		if firstFile(dir, c.lock) != "" {
			return c.pm
		}
	}
	return "npm"
}

// Alvo de Makefile: `nome:` no começo da linha. Fica de fora o que não se
// chama à mão — `.PHONY` e afins e regra de padrão (`%.o:`). Atribuição
// (`X := y`) quem filtra é o scanRecipes.
var makeTargetRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_./-]*)\s*:`)

func makeTargets(file string) []ProjectTask {
	return scanRecipes(file, makeTargetRe, func(name, desc string) ProjectTask {
		return ProjectTask{Source: "make", Name: name, Desc: desc, Argv: []string{"make", name}}
	})
}

// Receita de justfile: `nome args...:` sem indentação. Começar com `_` é
// receita privada — o próprio `just --list` esconde.
var justRecipeRe = regexp.MustCompile(`^@?([A-Za-z][A-Za-z0-9_-]*)[^:]*:`)

func justRecipes(file string) []ProjectTask {
	return scanRecipes(file, justRecipeRe, func(name, desc string) ProjectTask {
		return ProjectTask{Source: "just", Name: name, Desc: desc, Argv: []string{"just", name}}
	})
}

// scanRecipes lê Makefile e justfile do mesmo jeito: a descrição é o `## x`
// na própria linha (convenção do `make help`) ou o comentário logo acima.
func scanRecipes(file string, re *regexp.Regexp, mk func(name, desc string) ProjectTask) []ProjectTask {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []ProjectTask
	seen := map[string]bool{}
	comment := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if c, ok := strings.CutPrefix(line, "#"); ok {
			comment = strings.TrimSpace(strings.TrimLeft(c, "#"))
			continue
		}
		m := re.FindStringSubmatch(line)
		// `x := y`, `x ::= y`, `set shell := [...]`, `alias b := build`: o
		// primeiro `:` é de atribuição, não de alvo.
		if i := strings.Index(line, ":"); m != nil && (strings.HasPrefix(line[i:], ":=") || strings.HasPrefix(line[i:], "::=")) {
			m = nil
		}
		if m == nil {
			comment = ""
			continue
		}
		desc := comment
		if _, after, ok := strings.Cut(line, "##"); ok {
			desc = strings.TrimSpace(after)
		}
		comment = ""
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, mk(m[1], desc))
	}
	return out
}

func taskfileTasks(file string) []ProjectTask {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	// yaml.Node mantém a ordem do arquivo; um map[string] embaralharia.
	var doc struct {
		Tasks yaml.Node `yaml:"tasks"`
	}
	if yaml.Unmarshal(raw, &doc) != nil || doc.Tasks.Kind != yaml.MappingNode {
		return nil
	}
	var out []ProjectTask
	for i := 0; i+1 < len(doc.Tasks.Content); i += 2 {
		name := doc.Tasks.Content[i].Value
		var body struct {
			Desc     string `yaml:"desc"`
			Internal bool   `yaml:"internal"`
		}
		_ = doc.Tasks.Content[i+1].Decode(&body)
		if body.Internal {
			continue
		}
		out = append(out, ProjectTask{Source: "task", Name: name, Desc: body.Desc, Argv: []string{"task", name}})
	}
	return out
}

// jsonScripts devolve [nome, comando] do bloco "scripts" — no composer o
// valor pode ser lista de comandos.
func jsonScripts(file string) [][2]string {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var doc struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return nil
	}
	out := make([][2]string, 0, len(doc.Scripts))
	for name, v := range doc.Scripts {
		var one string
		var many []string
		switch {
		case json.Unmarshal(v, &one) == nil:
		case json.Unmarshal(v, &many) == nil:
			one = strings.Join(many, " && ")
		}
		out = append(out, [2]string{name, one})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
