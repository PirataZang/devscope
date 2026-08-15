package cfutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeTunnels(t *testing.T) {
	cfg := ProjectConfig{
		Project: "demo",
		Tunnels: []TunnelConfig{
			{Name: "api", URL: "http://localhost:3000", Port: 3000, Mode: "quick"},
			{Name: "admin", URL: "http://127.0.0.1:8081", Port: 8081, Mode: "named", Hostname: "admin.example.com"},
		},
	}
	live := []Tunnel{
		{Name: "api", Port: 3000, PublicURL: "https://x.trycloudflare.com", Status: "online", Mode: "quick"},
		{Name: "quick-4321", Port: 4321, PublicURL: "https://y.trycloudflare.com", Status: "online", Mode: "quick", Project: "(sistema)"},
	}
	got := MergeTunnels(cfg, live)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Status != "online" || got[0].PublicURL == "" {
		t.Fatalf("api should be online: %+v", got[0])
	}
	if got[1].Status != "offline" || got[1].Hostname != "admin.example.com" {
		t.Fatalf("admin should be offline named: %+v", got[1])
	}
	if n := CountForeignLive(cfg, live); n != 1 {
		t.Fatalf("foreign=%d want 1", n)
	}
	all := MergeTunnelsAll(cfg, live)
	if len(all) != 3 {
		t.Fatalf("all len=%d want 3", len(all))
	}
	if all[2].Name != "quick-4321" || all[2].Project != "(sistema)" && all[2].Project != "(outro)" {
		t.Fatalf("foreign in all: %+v", all[2])
	}
}

func TestSaveLoadProject(t *testing.T) {
	dir := t.TempDir()
	cfg := ProjectConfig{Project: "demo", Tunnels: []TunnelConfig{{Name: "api", URL: "http://127.0.0.1:4321", Mode: "quick"}}}
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".devscope", "cloudflare.json")); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir, "demo")
	if len(got.Tunnels) != 1 || got.Tunnels[0].URL != "http://127.0.0.1:4321" || got.Tunnels[0].Port != 4321 {
		t.Fatalf("%+v", got)
	}
}

func TestLegacyPortOnlyConfigGetsURL(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".devscope"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"project":"demo","tunnels":[{"name":"api","port":3000,"mode":"quick"}]}`
	if err := os.WriteFile(filepath.Join(dir, ".devscope", "cloudflare.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir, "demo")
	if got.Tunnels[0].URL != "http://127.0.0.1:3000" {
		t.Fatalf("%+v", got.Tunnels[0])
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"4321":                      "http://127.0.0.1:4321",
		"localhost:3000":            "http://127.0.0.1:3000",
		"http://localhost:4321":     "http://127.0.0.1:4321",
		"http://localhost:8000/api": "http://127.0.0.1:8000/api",
		"http://localhost.dev:8080": "http://localhost.dev:8080",
		"http://127.0.0.1:8080/":    "http://127.0.0.1:8080",
		"https://app.local:8443":    "https://app.local:8443",
		"":                          "",
		"0":                         "",
	}
	for in, want := range cases {
		if got := NormalizeURL(in); got != want {
			t.Fatalf("NormalizeURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestTunnelArgs(t *testing.T) {
	quick := strings.Join(tunnelArgs("api", "http://localhost:3000", "quick"), " ")
	if quick != "tunnel --url http://localhost:3000" {
		t.Fatalf("quick: %q", quick)
	}
	named := strings.Join(tunnelArgs("api", "http://localhost:3000", "named"), " ")
	if named != "tunnel run --url http://localhost:3000 api" {
		t.Fatalf("named: %q", named)
	}
	h2 := strings.Join(tunnelArgs("api", "http://localhost:3000", "http2"), " ")
	if h2 != "tunnel --protocol http2 --url http://localhost:3000" {
		t.Fatalf("http2: %q", h2)
	}
}

func TestNormalizeMode(t *testing.T) {
	cases := []struct{ mode, hostname, want string }{
		{"", "", "quick"},
		{"", "api.example.com", "named"},
		{"quick", "", "quick"},
		{"named", "api.example.com", "named"},
		{"http2", "", "http2"},
		{" HTTP2 ", "", "http2"},
		// Modo desconhecido não pode virar named nem escapar pro tunnelArgs:
		// vira quick, que é o único que roda sem login e sem hostname.
		{"grpc", "", "quick"},
		{"grpc", "api.example.com", "quick"},
	}
	for _, c := range cases {
		if got := NormalizeMode(c.mode, c.hostname); got != c.want {
			t.Fatalf("NormalizeMode(%q, %q) = %q, queria %q", c.mode, c.hostname, got, c.want)
		}
	}
	// Todo modo da lista tem de sobreviver ao normalize, senão o wizard oferece
	// um que o agente descarta.
	for _, m := range Modes {
		if got := NormalizeMode(m, ""); got != m {
			t.Fatalf("modo %q do menu vira %q no agente", m, got)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	if sanitizeName("My API!") != "my-api" {
		t.Fatalf("%q", sanitizeName("My API!"))
	}
}

func TestExtractPublicURL(t *testing.T) {
	line := `INF |  https://random-words-here.trycloudflare.com`
	got := extractPublicURL(line)
	if got != "https://random-words-here.trycloudflare.com" {
		t.Fatalf("got %q", got)
	}
	if extractPublicURL("no url here") != "" {
		t.Fatal("expected empty")
	}
}
