package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseServiceReplicas(t *testing.T) {
	r, d := ParseServiceReplicas("2/3")
	if r != 2 || d != 3 {
		t.Fatalf("got %d/%d", r, d)
	}
	r, d = ParseServiceReplicas("5")
	if r != 5 || d != 5 {
		t.Fatalf("single: %d/%d", r, d)
	}
}

func TestSwarmServiceStatus(t *testing.T) {
	if SwarmServiceStatus("3/3") != "running" {
		t.Fatal("running")
	}
	if SwarmServiceStatus("1/3") != "updating" {
		t.Fatal("updating")
	}
	if SwarmServiceStatus("0/2") != "pending" {
		t.Fatal("pending")
	}
}

func TestSwarmBelongsToProject(t *testing.T) {
	if !SwarmBelongsToProject("devscope-api", "devscope") {
		t.Fatal("expected match")
	}
	if SwarmBelongsToProject("other", "devscope") {
		t.Fatal("expected miss")
	}
}

func TestDiscoverSwarmComposePrefersSwarmFile(t *testing.T) {
	dir := t.TempDir()
	body := []byte("services:\n  web:\n    image: nginx\n")
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.swarm.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	got := DiscoverSwarmCompose(dir)
	if !strings.HasSuffix(got, "docker-compose.swarm.yml") {
		t.Fatalf("got %q", got)
	}
}
