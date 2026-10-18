package collectors

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// CleanTarget é uma pasta que o projeto GERA e que pode ser apagada sem perder
// nada: o comando do Hint traz ela de volta.
type CleanTarget struct {
	Name string
	Path string
	Size int64
	Hint string
}

// cleanCandidates são as pastas de dependência e de build mais comuns.
// safeWithoutGit marca as que não têm outro significado em projeto nenhum —
// fora de um repositório git são só elas que entram.
var cleanCandidates = []struct {
	name, hint     string
	safeWithoutGit bool
}{
	{"node_modules", "npm install", true},
	{"vendor", "composer install", false},
	{"target", "cargo build / mvn package", false},
	{".next", "next build", true},
	{".nuxt", "nuxt build", true},
	{".svelte-kit", "vite build", true},
	{".turbo", "turbo run build", true},
	{".parcel-cache", "parcel build", true},
	{".gradle", "gradle build", true},
	{".venv", "pip install -r requirements.txt", true},
	{"venv", "pip install -r requirements.txt", false},
	{"dist", "o build do projeto", false},
	{"build", "o build do projeto", false},
	{"coverage", "rodar os testes", false},
}

// ProjectCleanTargets mede as pastas regeneráveis do projeto, maiores primeiro.
//
// A regra que manda: num repositório git, só entra pasta que o git IGNORA. É o
// que separa o `vendor/` do composer do `vendor/` commitado de um projeto Go,
// e o `build/` de saída do `build/` cheio de Dockerfile.
func ProjectCleanTargets(ctx context.Context, dir string) []CleanTarget {
	var present []string
	hints := map[string]string{}
	safe := map[string]bool{}
	for _, c := range cleanCandidates {
		if st, err := os.Lstat(filepath.Join(dir, c.name)); err == nil && st.IsDir() {
			present = append(present, c.name)
			hints[c.name], safe[c.name] = c.hint, c.safeWithoutGit
		}
	}
	if len(present) == 0 {
		return nil
	}

	allowed := map[string]bool{}
	if gitRepoRoot(dir) != "" {
		// Uma chamada para todas; check-ignore sai 1 quando nenhuma é ignorada.
		cmd := exec.CommandContext(ctx, "git", append([]string{"check-ignore", "--"}, present...)...)
		cmd.Dir = dir
		out, _ := cmd.Output()
		for _, name := range strings.Fields(string(out)) {
			allowed[strings.TrimSuffix(name, "/")] = true
		}
	} else {
		for _, name := range present {
			allowed[name] = safe[name]
		}
	}

	var out []CleanTarget
	for _, name := range present {
		if !allowed[name] || ctx.Err() != nil {
			continue
		}
		path := filepath.Join(dir, name)
		out = append(out, CleanTarget{Name: name, Path: path, Size: dirSize(ctx, path), Hint: hints[name]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// dirSize soma o tamanho aparente dos arquivos. WalkDir não segue symlink, então
// um node_modules com pacotes linkados (pnpm, workspaces) não conta duas vezes.
func dirSize(ctx context.Context, path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// RemoveCleanTargets apaga as pastas. Confere de novo que cada uma está dentro
// do projeto: um caminho errado aqui seria um `rm -rf` no lugar errado.
func RemoveCleanTargets(dir string, targets []CleanTarget) error {
	root := filepath.Clean(dir) + string(filepath.Separator)
	for _, t := range targets {
		p := filepath.Clean(t.Path)
		if !strings.HasPrefix(p, root) || filepath.Dir(p) != filepath.Clean(dir) {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
