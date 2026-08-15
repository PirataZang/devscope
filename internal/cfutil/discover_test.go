package cfutil

import "testing"

func TestTunnelFromCmdlineQuick(t *testing.T) {
	args := []string{"cloudflared", "tunnel", "--url", "http://localhost:4321"}
	got := tunnelFromCmdline(99, args)
	if got.Port != 4321 || got.Mode != "quick" || got.Name != "quick-4321" {
		t.Fatalf("%+v", got)
	}
}

func TestTunnelFromCmdlineToken(t *testing.T) {
	args := []string{"/usr/bin/cloudflared", "--no-autoupdate", "tunnel", "run", "--token-file", "/etc/cloudflared/token"}
	got := tunnelFromCmdline(42, args)
	if got.Mode != "token" || got.Name != "token" {
		t.Fatalf("%+v", got)
	}
}

func TestTunnelFromCmdlineNamed(t *testing.T) {
	args := []string{"cloudflared", "tunnel", "run", "--url", "http://localhost:8080", "my-api"}
	got := tunnelFromCmdline(7, args)
	if h2 := tunnelFromCmdline(9, []string{"cloudflared", "tunnel", "--protocol", "http2", "--url", "http://localhost:4321"}); h2.Mode != "http2" || h2.Port != 4321 {
		t.Fatalf("http2 solto: mode=%q port=%d", h2.Mode, h2.Port)
	}
	if h2 := tunnelFromCmdline(10, []string{"cloudflared", "tunnel", "--protocol=http2", "--url=http://localhost:4321"}); h2.Mode != "http2" {
		t.Fatalf("http2 com =: mode=%q", h2.Mode)
	}
	if got.Mode != "named" || got.Name != "my-api" || got.Port != 8080 {
		t.Fatalf("%+v", got)
	}
}

func TestIsCloudflaredCmd(t *testing.T) {
	if !isCloudflaredCmd([]string{"/usr/bin/cloudflared", "tunnel", "--url", "http://localhost:1"}) {
		t.Fatal("expected true")
	}
	if isCloudflaredCmd([]string{"cloudflared", "version"}) {
		t.Fatal("version-only should be false")
	}
}
