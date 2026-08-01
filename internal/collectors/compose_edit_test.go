package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeComposeYAMLCreatesAndMerges(t *testing.T) {
	dir := t.TempDir()
	snippet := `
services:
  postgres:
    image: postgres:16
    ports:
      - "5432:5432"
`
	path, err := MergeComposeYAML(dir, snippet)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "docker-compose.yml" {
		t.Fatalf("path=%s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "postgres:") || !strings.Contains(got, "postgres:16") {
		t.Fatalf("missing service:\n%s", got)
	}

	_, err = MergeComposeYAML(dir, `
services:
  redis:
    image: redis:7
`)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	got = string(raw)
	if !strings.Contains(got, "postgres:") || !strings.Contains(got, "redis:") {
		t.Fatalf("merge lost service:\n%s", got)
	}
}

func TestComposeServiceTemplate(t *testing.T) {
	got := ComposeServiceTemplate("library/postgres")
	if !strings.Contains(got, "image: library/postgres") || !strings.Contains(got, "postgres:") {
		t.Fatalf("template=%q", got)
	}
	if !strings.Contains(got, "5432:5432") || !strings.Contains(got, "POSTGRES_PASSWORD") {
		t.Fatalf("expected postgres preset:\n%s", got)
	}
}

func TestMergeComposeYAMLMergesVolumes(t *testing.T) {
	dir := t.TempDir()
	path, err := MergeComposeYAML(dir, `
services:
  postgres:
    image: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data
volumes:
  postgres_data:
`)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	if !strings.Contains(got, "postgres_data:") {
		t.Fatalf("missing volume key:\n%s", got)
	}
}
