package devscopeutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDirAddsAndDedupesGitignore(t *testing.T) {
	dir := t.TempDir()

	if _, err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".devscope")); err != nil {
		t.Fatalf(".devscope not created: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if strings.Count(string(b), ".devscope/") != 1 {
		t.Fatalf("expected exactly one .devscope/ entry, got: %q", string(b))
	}

	// Existing .gitignore content is preserved, and a second call doesn't duplicate.
	if _, err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir (2nd): %v", err)
	}
	b2, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Count(string(b2), ".devscope/") != 1 {
		t.Fatalf("expected still exactly one .devscope/ entry after second call, got: %q", string(b2))
	}
}

func TestEnsureDirPreservesExistingGitignore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(b), "node_modules/") || !strings.Contains(string(b), ".devscope/") {
		t.Fatalf("expected both entries preserved, got: %q", string(b))
	}
}
