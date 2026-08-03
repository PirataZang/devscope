package sshutil

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeMode(t *testing.T) {
	if NormalizeMode("LOCAL") != ModeLocal {
		t.Fatal("local")
	}
	if NormalizeMode("-R") != ModeRemote {
		t.Fatal("remote")
	}
	if NormalizeMode("socks") != ModeDynamic {
		t.Fatal("dynamic")
	}
}

func TestSuggestPort(t *testing.T) {
	if SuggestPort([]int{4200}, "vue") != 4200 {
		t.Fatal("ports first")
	}
	if SuggestPort(nil, "laravel") != 8000 {
		t.Fatal("laravel default")
	}
}

func TestParseBind(t *testing.T) {
	h, p, err := ParseBind("db.internal:5432")
	if err != nil || h != "db.internal" || p != 5432 {
		t.Fatalf("got %q %d %v", h, p, err)
	}
	h, p, err = ParseBind(":3306")
	if err != nil || h != "127.0.0.1" || p != 3306 {
		t.Fatalf("colon: %q %d %v", h, p, err)
	}
	h, p, err = ParseBind("8080")
	if err != nil || h != "127.0.0.1" || p != 8080 {
		t.Fatalf("port-only: %q %d %v", h, p, err)
	}
}

func TestTunnelArgs(t *testing.T) {
	args := tunnelArgs(TunnelConfig{
		Name: "db", Mode: ModeLocal, LocalPort: 5433,
		RemoteHost: "127.0.0.1", RemotePort: 5432, Target: "user@host",
		Identity: "/tmp/id",
	})
	joined := ""
	for _, a := range args {
		joined += a + " "
	}
	if !containsAll(joined, "-N", "-L", "5433:127.0.0.1:5432", "-i", "/tmp/id", "user@host", "accept-new") {
		t.Fatalf("args: %v", args)
	}
	rargs := tunnelArgs(TunnelConfig{
		Mode: ModeRemote, LocalPort: 3000, RemoteHost: "127.0.0.1", RemotePort: 3000, Target: "vps",
	})
	rjoined := strings.Join(rargs, " ")
	if !containsAll(rjoined, "-R", "3000:127.0.0.1:3000", "vps") {
		t.Fatalf("remote args: %v", rargs)
	}
	args = tunnelArgs(TunnelConfig{Mode: ModeDynamic, LocalPort: 1080, Target: "u@h"})
	joined = ""
	for _, a := range args {
		joined += a + " "
	}
	if !containsAll(joined, "-D", "1080", "u@h") {
		t.Fatalf("dynamic: %v", args)
	}
}

func TestMergeTunnels(t *testing.T) {
	cfg := ProjectConfig{
		Project: "demo",
		Tunnels: []TunnelConfig{
			{Name: "db", Mode: ModeLocal, LocalPort: 5433, RemoteHost: "127.0.0.1", RemotePort: 5432, Target: "u@h"},
			{Name: "api", Mode: ModeLocal, LocalPort: 3000, RemoteHost: "127.0.0.1", RemotePort: 3000, Target: "u@h"},
		},
	}
	live := []Tunnel{{Name: "db", LocalPort: 5433, Status: "online", PID: 9}}
	out := MergeTunnels(cfg, live)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Status != "online" || out[0].PID != 9 {
		t.Fatalf("live merge: %+v", out[0])
	}
	if out[1].Status != "offline" {
		t.Fatalf("offline: %+v", out[1])
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := ProjectConfig{Project: "demo"}
	cfg.UpsertTunnel(TunnelConfig{
		Name: "db", Mode: ModeLocal, LocalPort: 5433,
		RemoteHost: "127.0.0.1", RemotePort: 5432, Target: "u@h",
		CreatedAt: time.Now(),
	})
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir, "demo")
	if len(got.Tunnels) != 1 || got.Tunnels[0].Name != "db" || got.Tunnels[0].LocalPort != 5433 {
		t.Fatalf("load: %+v", got)
	}
	got.RemoveTunnel("db")
	if len(got.Tunnels) != 0 {
		t.Fatal("remove")
	}
}

func TestFormatForward(t *testing.T) {
	if FormatForward(ModeDynamic, 1080, "", 0) != "socks5://127.0.0.1:1080" {
		t.Fatal("dynamic")
	}
	if FormatForward(ModeLocal, 5433, "127.0.0.1", 5432) != "L :5433 → 127.0.0.1:5432" {
		t.Fatal("local")
	}
	if FormatForward(ModeRemote, 3000, "127.0.0.1", 3000) != "R :3000 → PC 127.0.0.1:3000" {
		t.Fatal("remote")
	}
}

func TestSuggestSSHTargetAndDefaultRemote(t *testing.T) {
	if SuggestSSHTarget("git@github.com:acme/app.git") != "" {
		t.Fatal("skip github")
	}
	if got := SuggestSSHTarget("git@vps.digiliza.com:app.git"); got != "vps.digiliza.com" {
		t.Fatalf("git remote: %q", got)
	}
	if got := SuggestSSHTarget("ssh://deploy@vps.example:22/srv/app"); got != "deploy@vps.example" {
		t.Fatalf("ssh url: %q", got)
	}
	def := DefaultRemoteTunnel("Tarefas Mobile", []int{4321}, "", "deploy@vps")
	if def.Mode != ModeRemote || def.LocalPort != 4321 || def.RemotePort != 4321 || def.Name != "tarefas-mobile" {
		t.Fatalf("%+v", def)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
