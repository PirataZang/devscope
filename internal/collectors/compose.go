package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// composeMapRe casa o formato host:container (específico, vale em qualquer
// linha); composeBareRe casa a porta solta do formato curto `- 5173`, que só é
// confiável dentro de um bloco `ports:` — fora dele, `retries: 5` ou
// `interval: 5s` virariam portas falsas.
var composeMapRe = regexp.MustCompile(`(?:^|\s)(?:-\s*)?["']?(?:\d+\.){0,3}(\d+)\s*:\s*(\d+)["']?`)
var composeBareRe = regexp.MustCompile(`(?:^|\s)(?:-\s*)?["']?(\d+)["']?\s*$`)

// ComposeFile returns the first compose file found in projectPath.
func ComposeFile(projectPath string) string {
	for _, name := range []string{
		"docker-compose.yml", "docker-compose.yaml",
		"compose.yml", "compose.yaml",
	} {
		p := filepath.Join(projectPath, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// ListComposeServiceNames returns service keys from the project's compose file.
func ListComposeServiceNames(projectPath string) []string {
	file := ComposeFile(projectPath)
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	services, err := parseComposeServices(string(data))
	if err != nil || len(services) == 0 {
		return nil
	}
	names := make([]string, 0, len(services))
	for name := range services {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// ParseComposePorts reads host ports from docker-compose files.
func ParseComposePorts(projectPath string) []int {
	file := ComposeFile(projectPath)
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	seen := make(map[int]bool)
	var ports []int
	add := func(s string) {
		if s == "" {
			return
		}
		p, err := strconv.Atoi(s)
		if err != nil || p <= 0 || p > 65535 || seen[p] {
			return
		}
		seen[p] = true
		ports = append(ports, p)
	}

	inPorts, portsIndent := false, 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		// saiu do bloco ports: quando a indentação volta ao nível da chave ou menos
		if inPorts && indent <= portsIndent {
			inPorts = false
		}
		if trimmed == "ports:" {
			inPorts, portsIndent = true, indent
			continue
		}
		// host:container vale em qualquer linha (específico o bastante)
		matchedMap := false
		for _, m := range composeMapRe.FindAllStringSubmatch(line, -1) {
			add(m[1])
			matchedMap = true
		}
		// porta solta (`- 5173`) só dentro de um bloco ports:
		if !matchedMap && inPorts {
			if m := composeBareRe.FindStringSubmatch(line); m != nil {
				add(m[1])
			}
		}
	}
	return ports
}

func runCompose(projectPath, action string, extra ...string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return err
	}
	args := []string{"compose"}
	if f := ComposeFile(projectPath); f != "" {
		args = append(args, "-f", filepath.Base(f))
	}
	args = append(args, action)
	args = append(args, extra...)
	cmd := exec.Command("docker", args...)
	cmd.Dir = projectPath
	// Capture output so compose progress/errors don't smash the alt-screen TUI.
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		if len(msg) > 240 {
			msg = msg[:240] + "…"
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func ComposeUp(projectPath string) error {
	return runCompose(projectPath, "up", "-d")
}

// ComposeBuild reconstrói as imagens e sobe: `docker compose up -d --build`.
func ComposeBuild(projectPath string) error {
	return runCompose(projectPath, "up", "-d", "--build")
}

func ComposeDown(projectPath string) error {
	return runCompose(projectPath, "down")
}

func ComposeRestart(projectPath string) error {
	return runCompose(projectPath, "restart")
}

// ComposeLogs returns recent logs from all compose services.
func ComposeLogs(projectPath string, tail int) (string, error) {
	if tail <= 0 {
		tail = 200
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return "", err
	}
	args := []string{"compose"}
	if f := ComposeFile(projectPath); f != "" {
		args = append(args, "-f", filepath.Base(f))
	}
	args = append(args, "logs", "--tail", strconv.Itoa(tail), "--no-color")
	cmd := exec.Command("docker", args...)
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	return string(out), err
}
