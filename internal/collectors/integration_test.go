package collectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectDeployScript(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "deploy.sh")
	if err := os.WriteFile(script, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := DetectDeployScript(dir)
	if got != script {
		t.Fatalf("got %q", got)
	}
}

func TestParseNginxConfig(t *testing.T) {
	content := `
server {
    listen 443 ssl;
    server_name api.example.com;
    proxy_pass http://127.0.0.1:3000;
}
`
	domains := parseNginxConfig(content)
	if len(domains) == 0 || domains[0].Host != "api.example.com" {
		t.Fatalf("domains %v", domains)
	}
}
