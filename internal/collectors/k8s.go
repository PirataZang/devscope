package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// K8sAvailable reports whether kubectl is on PATH.
func K8sAvailable() bool {
	_, err := exec.LookPath("kubectl")
	return err == nil
}

func K8sCurrentContext() string {
	out, err := runKubectl(8*time.Second, "config", "current-context")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func K8sContexts() ([]string, error) {
	out, err := runKubectl(8*time.Second, "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}
	return splitNonEmpty(out), nil
}

func K8sNamespaces() ([]string, error) {
	out, err := runKubectl(12*time.Second, "get", "ns", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

type K8sResource struct {
	Kind      string
	Name      string
	Namespace string
	Status    string
	Ready     string
	Restarts  string
	Node      string
	IP        string
	Age       string
	Extra     string
}

type K8sClusterMeta struct {
	Version string
	Nodes   int
}

func K8sClusterMetaInfo() K8sClusterMeta {
	meta := K8sClusterMeta{}
	out, err := runKubectl(8*time.Second, "version", "-o", "json")
	if err == nil {
		// ponytail: tiny JSON scrape beats pulling encoding/json for one field
		if i := strings.Index(out, `"gitVersion"`); i >= 0 {
			rest := out[i:]
			if q1 := strings.Index(rest, `:"`); q1 >= 0 {
				rest = rest[q1+2:]
				if q2 := strings.Index(rest, `"`); q2 >= 0 {
					meta.Version = rest[:q2]
				}
			}
		}
	}
	nodes, err := runKubectl(10*time.Second, "get", "nodes", "--no-headers")
	if err == nil {
		meta.Nodes = len(splitNonEmpty(nodes))
	}
	return meta
}

func K8sListEvents(ns string, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}
	args := []string{"get", "events", "--sort-by=.lastTimestamp", "--no-headers"}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	} else {
		args = append([]string{"-A"}, args...)
	}
	out, err := runKubectl(15*time.Second, args...)
	if err != nil {
		return out, err
	}
	lines := splitNonEmpty(out)
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return strings.Join(lines, "\n"), nil
}

func K8sListPods(ns string) ([]K8sResource, error) {
	// wide: NAME READY STATUS RESTARTS AGE IP NODE ...
	args := []string{"get", "pods", "-o", "wide", "--no-headers"}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	} else {
		args = append([]string{"-A"}, args...)
	}
	out, err := runKubectl(15*time.Second, args...)
	if err != nil {
		return nil, err
	}
	var items []K8sResource
	for _, line := range splitNonEmpty(out) {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		off := 0
		nsName := ns
		if ns == "" && len(f) >= 6 {
			nsName = f[0]
			off = 1
		}
		if len(f) < off+5 {
			continue
		}
		r := K8sResource{
			Kind:      "Pod",
			Name:      f[off],
			Namespace: nsName,
			Ready:     f[off+1],
			Status:    f[off+2],
			Restarts:  f[off+3],
			Age:       f[off+4],
		}
		if len(f) > off+5 {
			r.IP = f[off+5]
		}
		if len(f) > off+6 {
			r.Node = f[off+6]
		}
		items = append(items, r)
	}
	return items, nil
}

func K8sListDeployments(ns string) ([]K8sResource, error) {
	// NAME READY UP-TO-DATE AVAILABLE AGE
	args := k8sNSArgs(ns, "get", "deploy", "--no-headers")
	out, err := runKubectl(15*time.Second, args...)
	if err != nil {
		return nil, err
	}
	var items []K8sResource
	for _, line := range splitNonEmpty(out) {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		items = append(items, K8sResource{
			Kind: "Deployment", Name: f[0], Namespace: ns,
			Ready: f[1], Status: f[1], Age: f[4],
		})
	}
	return items, nil
}

func K8sListServices(ns string) ([]K8sResource, error) {
	// NAME TYPE CLUSTER-IP EXTERNAL-IP PORT(S) AGE
	args := k8sNSArgs(ns, "get", "svc", "--no-headers")
	out, err := runKubectl(15*time.Second, args...)
	if err != nil {
		return nil, err
	}
	var items []K8sResource
	for _, line := range splitNonEmpty(out) {
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		items = append(items, K8sResource{
			Kind: "Service", Name: f[0], Namespace: ns,
			Status: f[1], Extra: f[1], IP: f[2], Age: f[5],
		})
	}
	return items, nil
}

func k8sNSArgs(ns string, args ...string) []string {
	if ns != "" {
		return append([]string{"-n", ns}, args...)
	}
	return append([]string{"-A"}, args...)
}

func K8sDescribe(kind, name, ns string) (string, error) {
	args := []string{"describe", kind, name}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return runKubectl(20*time.Second, args...)
}

func K8sGetYAML(kind, name, ns string) (string, error) {
	args := []string{"get", kind, name, "-o", "yaml"}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return runKubectl(20*time.Second, args...)
}

func K8sDelete(kind, name, ns string) error {
	args := []string{"delete", kind, name, "--wait=false"}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	_, err := runKubectl(30*time.Second, args...)
	return err
}

func K8sApplyFile(path string) (string, error) {
	return runKubectl(45*time.Second, "apply", "-f", path)
}

func K8sApplyYAML(yaml string) (string, error) {
	tmp, err := os.CreateTemp("", "devscope-k8s-*.yaml")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(yaml); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	return K8sApplyFile(tmp.Name())
}

func K8sPodLogs(name, ns string, tail int) (string, error) {
	if tail <= 0 {
		tail = 100
	}
	args := []string{"logs", name, "--tail", fmt.Sprintf("%d", tail)}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return runKubectl(20*time.Second, args...)
}

func K8sScale(name, ns string, replicas int) (string, error) {
	args := []string{"scale", "deploy/" + name, fmt.Sprintf("--replicas=%d", replicas)}
	if ns != "" {
		args = append([]string{"-n", ns}, args...)
	}
	return runKubectl(30*time.Second, args...)
}

// DiscoverProjectManifests finds yaml/yml under common k8s dirs in the project.
func DiscoverProjectManifests(root string) []string {
	if root == "" {
		return nil
	}
	dirs := []string{
		filepath.Join(root, "k8s"),
		filepath.Join(root, "kubernetes"),
		filepath.Join(root, "deploy"),
		filepath.Join(root, "deployments"),
		filepath.Join(root, "manifests"),
		filepath.Join(root, ".k8s"),
	}
	var out []string
	seen := map[string]bool{}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			if seen[path] {
				return nil
			}
			seen[path] = true
			out = append(out, path)
			if len(out) >= 80 {
				return filepath.SkipAll
			}
			return nil
		})
	}
	return out
}

// K8sDeploymentTemplate returns a minimal Deployment YAML.
func K8sDeploymentTemplate(name, ns, image string) string {
	if ns == "" {
		ns = "default"
	}
	if image == "" {
		image = "nginx:alpine"
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: %s
        image: %s
        ports:
        - containerPort: 80
`, name, ns, name, name, name, image)
}

// K8sServiceTemplate returns a minimal ClusterIP Service YAML.
func K8sServiceTemplate(name, ns string, port int) string {
	if ns == "" {
		ns = "default"
	}
	if port <= 0 {
		port = 80
	}
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
  - port: %d
    targetPort: %d
  type: ClusterIP
`, name, ns, name, port, port)
}

func runKubectl(timeout time.Duration, args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
		s := strings.TrimSpace(string(out))
		if err != nil {
			if s != "" {
				return s, fmt.Errorf("%s", s)
			}
			return "", err
		}
		return s, nil
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("kubectl timeout")
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
