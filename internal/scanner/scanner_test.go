package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFastMarkers(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("KEY=1"), 0644)

	markers, ok := readFastMarkers(dir)
	if !ok || !markers.Env {
		t.Fatalf("expected .env marker, got %+v ok=%v", markers, ok)
	}
}

func TestReadFastMarkersGitOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	_, ok := readFastMarkers(dir)
	if !ok {
		t.Fatal("expected .git directory to mark project")
	}
}

func TestFastScanSkipsSubdirs(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "myapp")
	nested := filepath.Join(project, "packages", "api")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "package.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	s := New([]string{root}, 5, []string{"node_modules", "vendor", ".git"})
	projects, err := s.FastScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %+v", len(projects), projects)
	}
	if projects[0].Path != project {
		t.Fatalf("expected project at %s, got %s", project, projects[0].Path)
	}
}

func TestFastScanFindsMultipleRoots(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "alpha")
	b := filepath.Join(root, "beta")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "go.mod"), []byte("module x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	s := New([]string{root}, 5, nil)
	projects, err := s.FastScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

func TestIsServiceSubfolder(t *testing.T) {
	root := t.TempDir()
	compose := filepath.Join(root, "compose-stack")
	nginx := filepath.Join(compose, "nginx")
	for _, p := range []string{compose, nginx} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(compose, "docker-compose.yml"), []byte("services: {}"), 0644)
	os.WriteFile(filepath.Join(nginx, "Dockerfile"), []byte("FROM nginx"), 0644)

	markers, _ := readFastMarkers(nginx)
	if !isServiceSubfolder(nginx, markers) {
		t.Fatal("expected nginx service folder to be skipped")
	}
}

func TestFocusedProjectPath(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "myapp")
	sub := filepath.Join(proj, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(sub)
	if got := FocusedProjectPath(); got != proj {
		t.Fatalf("expected %s, got %s", proj, got)
	}
}

func TestProjectStubAt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(nil, 5, nil)
	stub := s.ProjectStubAt(dir)
	if stub.Path != dir || stub.Framework.Name != "Go" {
		t.Fatalf("unexpected stub: %+v", stub)
	}
}
