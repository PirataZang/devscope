package collectors

import (
	"strings"
	"testing"
)

func TestComposeServiceYAMLFromPresetPostgres(t *testing.T) {
	got, source, ok := ComposeServiceYAMLFromPreset("library/postgres:16")
	if !ok {
		t.Fatal("expected preset match")
	}
	if source != ComposeSourcePreset {
		t.Fatalf("source=%q", source)
	}
	for _, want := range []string{
		"image: library/postgres:16",
		"5432:5432",
		"POSTGRES_PASSWORD",
		"postgres_data:/var/lib/postgresql/data",
		"healthcheck:",
		"volumes:\n  postgres_data:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "EXAMPLE") || strings.Contains(got, "8080:80") {
		t.Fatalf("generic template leaked:\n%s", got)
	}
}

func TestComposeServiceYAMLFromPresetRedis(t *testing.T) {
	got, _, ok := ComposeServiceYAMLFromPreset("redis")
	if !ok {
		t.Fatal("expected redis preset")
	}
	if !strings.Contains(got, "6379:6379") {
		t.Fatalf("got:\n%s", got)
	}
}

func TestComposeServiceYAMLFromPresetNode(t *testing.T) {
	got, source, ok := ComposeServiceYAMLFromPreset("node:22")
	if !ok {
		t.Fatal("expected node preset")
	}
	if source != ComposeSourcePreset {
		t.Fatalf("source=%q", source)
	}
	for _, want := range []string{
		"image: node:22",
		"working_dir: /app",
		"3000:3000",
		"NODE_ENV",
		"./:/app",
		"node_modules:/app/node_modules",
		`command: ["npm", "run", "dev"]`,
		"volumes:\n  node_modules:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "NODE_VERSION") {
		t.Fatalf("should not use manifest-only NODE_VERSION:\n%s", got)
	}
}

func TestComposeServiceYAMLFromPresetUnknown(t *testing.T) {
	_, _, ok := ComposeServiceYAMLFromPreset("itzg/minecraft-server")
	if ok {
		t.Fatal("unknown image should not match preset")
	}
}

func TestComposeServiceTemplateUsesPreset(t *testing.T) {
	got := ComposeServiceTemplate("mysql:8")
	if !strings.Contains(got, "3306:3306") || !strings.Contains(got, "MYSQL_ROOT_PASSWORD") {
		t.Fatalf("template=\n%s", got)
	}
}

func TestComposeImageBaseName(t *testing.T) {
	cases := map[string]string{
		"postgres:16":           "postgres",
		"library/redis":         "redis",
		"itzg/minecraft-server": "minecraft-server",
		"MySQL":                 "mysql",
	}
	for in, want := range cases {
		if got := composeImageBaseName(in); got != want {
			t.Fatalf("%q => %q want %q", in, got, want)
		}
	}
}

func TestPortsAndEnvFromConfig(t *testing.T) {
	ports := portsFromExposed(map[string]struct{}{"25565/tcp": {}, "8080/tcp": {}})
	if len(ports) != 2 || ports[0] != "25565:25565" {
		t.Fatalf("ports=%v", ports)
	}
	env := envMapFromSlice([]string{"PATH=/usr/bin", "EULA=TRUE", "TYPE=VANILLA", "NODE_VERSION=22.0.0"})
	if env["PATH"] != "" || env["NODE_VERSION"] != "" || env["EULA"] != "TRUE" || env["TYPE"] != "VANILLA" {
		t.Fatalf("env=%v", env)
	}
}
