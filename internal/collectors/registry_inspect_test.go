package collectors

import (
	"strings"
	"testing"
)

func TestParseImageConfigJSON(t *testing.T) {
	raw := []byte(`{
  "architecture": "amd64",
  "config": {
    "ExposedPorts": {"25565/tcp": {}, "8080/tcp": {}},
    "Env": ["PATH=/usr/local/sbin:/usr/local/bin", "EULA=FALSE", "TYPE=VANILLA"],
    "Volumes": {"/data": {}}
  }
}`)
	cfg, err := ParseImageConfigJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.ExposedPorts["25565/tcp"]; !ok {
		t.Fatalf("ports=%v", cfg.ExposedPorts)
	}
	if len(cfg.Env) != 3 {
		t.Fatalf("env=%v", cfg.Env)
	}
	if _, ok := cfg.Volumes["/data"]; !ok {
		t.Fatalf("vols=%v", cfg.Volumes)
	}

	yamlText := renderComposeFromParts("itzg/minecraft-server",
		portsFromExposed(cfg.ExposedPorts),
		envMapFromSlice(cfg.Env),
		volumesFromSet(cfg.Volumes, "minecraft-server"),
		"",
	)
	for _, want := range []string{"25565:25565", "EULA", "minecraft-server_data:/data"} {
		if !strings.Contains(yamlText, want) {
			t.Fatalf("missing %q in:\n%s", want, yamlText)
		}
	}
	if strings.Contains(yamlText, "PATH:") {
		t.Fatalf("PATH should be skipped:\n%s", yamlText)
	}
}

func TestSplitRegistryImage(t *testing.T) {
	repo, tag := splitRegistryImage("postgres:16")
	if repo != "library/postgres" || tag != "16" {
		t.Fatalf("%s %s", repo, tag)
	}
	repo, tag = splitRegistryImage("itzg/minecraft-server")
	if repo != "itzg/minecraft-server" || tag != "latest" {
		t.Fatalf("%s %s", repo, tag)
	}
}
