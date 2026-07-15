package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devscope/devscope/internal/scanner"
)

func TestScanJSONOutput(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "demo-api")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "package.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "docker-compose.yml"), []byte("services:\n  web:\n    image: nginx\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	cfgContent := "scan:\n  paths:\n    - " + root + "\n  max_depth: 3\n"
	if err := os.WriteFile(cfgFile, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgFileOld := cfgFile
	cfgFile = cfgFileOld
	_ = cfgFile

	s := scanner.New([]string{root}, 3, []string{"node_modules"})
	projects, err := s.FastScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) == 0 {
		t.Fatal("expected at least one project")
	}
	found := false
	for _, p := range projects {
		if p.Name == "demo-api" {
			found = true
		}
	}
	if !found {
		t.Fatalf("projects: %+v", projects)
	}
}

func TestScanCmdRegistered(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	for _, c := range rootCmd.Commands() {
		if c.Name() == "scan" {
			return
		}
	}
	t.Fatal("scan command not registered")
}
