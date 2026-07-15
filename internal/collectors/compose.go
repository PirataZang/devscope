package collectors

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

var composePortRe = regexp.MustCompile(`(?m)(?:^|\s)(?:-\s*)?(?:["']?(?:\d+\.){0,3}(\d+)\s*:\s*(\d+)["']?|["']?(\d+)["']?\s*$)`)

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
	for _, m := range composePortRe.FindAllStringSubmatch(string(data), -1) {
		for _, g := range []string{m[1], m[3]} {
			if g == "" {
				continue
			}
			p, err := strconv.Atoi(g)
			if err != nil || p <= 0 || seen[p] {
				continue
			}
			seen[p] = true
			ports = append(ports, p)
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
	return cmd.Run()
}

func ComposeUp(projectPath string) error {
	return runCompose(projectPath, "up", "-d")
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
