package scanner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/core"
	"github.com/devscope/devscope/internal/scanner"
)

func TestMergeDiscoveredKeepsExistingProjects(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "projeto")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New([]string{root}, 5, []string{"node_modules", "vendor", ".git"})
	ctx := context.Background()

	existing := []core.Project{{Path: proj, Name: "projeto"}}
	merged := s.MergeDiscovered(ctx, existing)

	if len(merged) < 1 {
		t.Fatal("expected at least one project")
	}
	found := false
	for _, p := range merged {
		if p.Path == proj && p.Name == "projeto" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected existing project preserved, got %+v", merged)
	}
}

func TestFastScanFindsProjetoInTempDir(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "projeto")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "go.mod"), []byte("module projeto"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := scanner.New([]string{root}, 5, nil)
	projects, err := s.FastScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "projeto" {
		t.Fatalf("expected projeto, got %+v", projects)
	}
}
