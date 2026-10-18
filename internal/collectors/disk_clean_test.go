package collectors

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree é o writeFile que cria as pastas no caminho.
func writeTree(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, content)
}

func cleanNames(ts []CleanTarget) string {
	var names []string
	for _, t := range ts {
		names = append(names, t.Name)
	}
	return strings.Join(names, " ")
}

// Num repositório git só entra o que o git ignora: o vendor commitado (projeto
// Go) e o build/ versionado ficam de fora.
func TestProjectCleanTargetsRespectsGit(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	writeFile(t, filepath.Join(dir, ".gitignore"), "node_modules/\n.next/\n")
	writeTree(t, filepath.Join(dir, "node_modules", "react", "index.js"), strings.Repeat("x", 3000))
	writeTree(t, filepath.Join(dir, ".next", "cache.bin"), strings.Repeat("x", 100))
	writeTree(t, filepath.Join(dir, "vendor", "lib.go"), "package lib")
	writeTree(t, filepath.Join(dir, "build", "Dockerfile"), "FROM scratch")

	got := ProjectCleanTargets(context.Background(), dir)
	if cleanNames(got) != "node_modules .next" {
		t.Fatalf("só as ignoradas, maior primeiro: %q", cleanNames(got))
	}
	if got[0].Size != 3000 || got[0].Hint != "npm install" {
		t.Fatalf("%+v", got[0])
	}

	if err := RemoveCleanTargets(dir, got); err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"vendor/lib.go", "build/Dockerfile", ".gitignore"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Fatalf("%s não podia sumir: %v", keep, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); !os.IsNotExist(err) {
		t.Fatal("node_modules devia ter sido apagada")
	}
}

// Fora de git só as pastas sem outro significado possível.
func TestProjectCleanTargetsWithoutGit(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, filepath.Join(dir, "node_modules", "a.js"), "x")
	writeTree(t, filepath.Join(dir, "dist", "app.js"), "x")
	if got := cleanNames(ProjectCleanTargets(context.Background(), dir)); got != "node_modules" {
		t.Fatalf("sem git, dist é ambíguo: %q", got)
	}
}

// Caminho fora do projeto nunca é apagado, mesmo que chegue na lista.
func TestRemoveCleanTargetsStaysInsideProject(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(other, "keep.txt"), "x")
	if err := RemoveCleanTargets(dir, []CleanTarget{{Name: "x", Path: other}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, "keep.txt")); err != nil {
		t.Fatal("apagou fora do projeto")
	}
}
