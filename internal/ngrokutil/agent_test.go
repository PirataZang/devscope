package ngrokutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeTunnels(t *testing.T) {
	cfg := ProjectConfig{
		Project: "demo",
		Tunnels: []TunnelConfig{
			{Name: "api", Port: 3000, Proto: "http"},
			{Name: "admin", Port: 8081, Proto: "http"},
		},
	}
	live := []Tunnel{{Name: "api", Port: 3000, Proto: "http", PublicURL: "https://x.ngrok-free.app", Status: "online"}}
	got := MergeTunnels(cfg, live)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Status != "online" || got[0].PublicURL == "" {
		t.Fatalf("api should be online: %+v", got[0])
	}
	if got[1].Status != "offline" || got[1].Name != "admin" {
		t.Fatalf("admin should be offline: %+v", got[1])
	}
}

func TestSaveLoadProject(t *testing.T) {
	dir := t.TempDir()
	cfg := ProjectConfig{Project: "demo", Region: "eu", Tunnels: []TunnelConfig{{Name: "api", Port: 3000, Proto: "http"}}}
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".devscope", "ngrok.json")); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir, "demo")
	if len(got.Tunnels) != 1 || got.Tunnels[0].Port != 3000 || got.Region != "eu" {
		t.Fatalf("%+v", got)
	}
}

func TestSuggestPort(t *testing.T) {
	if SuggestPort([]int{4200}, "vue") != 4200 {
		t.Fatal("prefer project ports")
	}
	if SuggestPort(nil, "laravel") != 8000 {
		t.Fatal("laravel default")
	}
}

func TestSanitizeName(t *testing.T) {
	if sanitizeName("My API!") != "my-api" {
		t.Fatalf("%q", sanitizeName("My API!"))
	}
}
