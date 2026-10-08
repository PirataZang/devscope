package collectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseComposePorts(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(compose, []byte(`
services:
  web:
    ports:
      - "3000:3000"
      - 8080:80
  api:
    ports:
      - '5173'
`), 0644); err != nil {
		t.Fatal(err)
	}

	ports := ParseComposePorts(dir)
	want := map[int]bool{3000: true, 8080: true, 5173: true}
	if len(ports) != len(want) {
		t.Fatalf("got %v, want 3 ports", ports)
	}
	for _, p := range ports {
		if !want[p] {
			t.Fatalf("unexpected port %d in %v", p, ports)
		}
	}
}

func TestParseComposePortsIgnoresHealthcheck(t *testing.T) {
	dir := t.TempDir()
	compose := filepath.Join(dir, "docker-compose.yml")
	// retries/interval fora de um bloco ports: não podem virar porta 5.
	if err := os.WriteFile(compose, []byte(`
services:
  web:
    ports:
      - "3000:3000"
    healthcheck:
      interval: 5s
      timeout: 5s
      retries: 5
`), 0644); err != nil {
		t.Fatal(err)
	}
	ports := ParseComposePorts(dir)
	for _, p := range ports {
		if p == 5 {
			t.Fatalf("retries/interval vazaram como porta: %v", ports)
		}
	}
	// a porta real do mapeamento ainda é capturada
	found := false
	for _, p := range ports {
		if p == 3000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("porta 3000 do mapeamento não foi capturada: %v", ports)
	}
}

func TestComposeFile(t *testing.T) {
	dir := t.TempDir()
	if ComposeFile(dir) != "" {
		t.Fatal("expected empty for missing compose")
	}
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := ComposeFile(dir); got != path {
		t.Fatalf("got %q want %q", got, path)
	}
}
