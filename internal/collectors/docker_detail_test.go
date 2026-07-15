package collectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDockerfileFromCompose(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	content := `services:
  web:
    build:
      context: ./app
      dockerfile: Dockerfile.dev
  db:
    image: postgres
`
	if err := os.WriteFile(compose, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	appDir := filepath.Join(dir, "app")
	if err := os.Mkdir(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	df := filepath.Join(appDir, "Dockerfile.dev")
	if err := os.WriteFile(df, []byte("FROM alpine"), 0644); err != nil {
		t.Fatal(err)
	}

	got := dockerfileFromCompose(compose, "web")
	if got != df {
		t.Fatalf("expected %q, got %q", df, got)
	}
	if dockerfileFromCompose(compose, "db") != "" {
		t.Fatal("db service should not resolve dockerfile")
	}
}

func TestResolveDockerfilePath(t *testing.T) {
	base := "/project"
	if got := resolveDockerfilePath(base, "docker/Dockerfile"); got != "/project/docker/Dockerfile" {
		t.Fatalf("unexpected relative path: %s", got)
	}
	if got := resolveDockerfilePath(base, "/abs/Dockerfile"); got != "/abs/Dockerfile" {
		t.Fatalf("unexpected absolute path: %s", got)
	}
}
