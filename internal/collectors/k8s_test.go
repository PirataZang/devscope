package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestK8sDeploymentTemplate(t *testing.T) {
	yaml := K8sDeploymentTemplate("demo", "default", "nginx:alpine")
	for _, want := range []string{"kind: Deployment", "name: demo", "namespace: default", "nginx:alpine"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("template missing %q:\n%s", want, yaml)
		}
	}
}

func TestK8sServiceTemplate(t *testing.T) {
	yaml := K8sServiceTemplate("demo", "default", 8080)
	for _, want := range []string{"kind: Service", "name: demo", "port: 8080"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("template missing %q:\n%s", want, yaml)
		}
	}
}

func TestDiscoverProjectManifests(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "k8s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "deploy.yaml")
	if err := os.WriteFile(path, []byte("apiVersion: v1\nkind: ConfigMap\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := DiscoverProjectManifests(root)
	if len(got) != 1 || got[0] != path {
		t.Fatalf("got %#v", got)
	}
}
