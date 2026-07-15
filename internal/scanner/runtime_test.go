package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProjectRoot(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	sub := filepath.Join(dir, "src", "api")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	if got := ResolveProjectRoot(sub); got != dir {
		t.Fatalf("ResolveProjectRoot(%q) = %q, want %q", sub, got, dir)
	}
}

func TestResolveProjectRootNoMarker(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "random")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	if got := ResolveProjectRoot(sub); got != "" {
		t.Fatalf("expected empty root, got %q", got)
	}
}

func TestExistingPaths(t *testing.T) {
	dir := t.TempDir()
	paths := existingPaths([]string{dir, "/definitely/not/a/path", dir})
	if len(paths) != 1 || paths[0] != dir {
		t.Fatalf("unexpected paths: %v", paths)
	}
}
