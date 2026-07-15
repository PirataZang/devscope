package collectors

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type dockerInspectRow struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Image  string            `json:"Image"`
		Cmd    []string          `json:"Cmd"`
		Env    []string          `json:"Env"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	HostConfig struct {
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
	} `json:"HostConfig"`
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIP"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
	} `json:"NetworkSettings"`
}

func DockerContainerStats(target string) (string, error) {
	out, err := exec.Command("docker", "stats", "--no-stream",
		"--format", "CPU (%): {{.CPUPerc}}\nMemory: {{.MemUsage}} ({{.MemPerc}})\nNet I/O: {{.NetIO}}\nBlock I/O: {{.BlockIO}}\nPIDs: {{.PIDs}}",
		target).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return string(out), err
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func DockerContainerEnv(target string) (string, error) {
	out, err := exec.Command("docker", "inspect", "-f", "{{range .Config.Env}}{{println .}}{{end}}", target).CombinedOutput()
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "(sem variáveis de ambiente)", nil
	}
	return s, nil
}

func DockerContainerConfig(target string) (string, error) {
	row, err := dockerInspect(target)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ID: %s\n", row.ID)
	fmt.Fprintf(&b, "Name: %s\n", strings.TrimPrefix(row.Name, "/"))
	fmt.Fprintf(&b, "Image: %s\n", row.Config.Image)
	if len(row.Config.Cmd) > 0 {
		fmt.Fprintf(&b, "Command: %s\n", strings.Join(row.Config.Cmd, " "))
	}
	if row.HostConfig.RestartPolicy.Name != "" {
		fmt.Fprintf(&b, "Restart: %s\n", row.HostConfig.RestartPolicy.Name)
	}
	if len(row.Config.Labels) > 0 {
		b.WriteString("Labels:\n")
		for k, v := range row.Config.Labels {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	if len(row.Mounts) == 0 {
		b.WriteString("Mounts: none\n")
	} else {
		b.WriteString("Mounts:\n")
		for _, m := range row.Mounts {
			fmt.Fprintf(&b, "  %s → %s (%s)\n", m.Source, m.Destination, m.Type)
		}
	}
	if len(row.NetworkSettings.Ports) == 0 {
		b.WriteString("Ports: none\n")
	} else {
		b.WriteString("Ports:\n")
		for containerPort, bindings := range row.NetworkSettings.Ports {
			if len(bindings) == 0 {
				fmt.Fprintf(&b, "  %s (not published)\n", containerPort)
				continue
			}
			for _, bind := range bindings {
				host := bind.HostPort
				if bind.HostIP != "" && bind.HostIP != "0.0.0.0" {
					host = bind.HostIP + ":" + host
				}
				fmt.Fprintf(&b, "  %s → %s\n", host, containerPort)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func DockerContainerTop(target string) (string, error) {
	out, err := exec.Command("docker", "top", target).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return string(out), err
		}
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "(container não está running)", nil
	}
	return s, nil
}

func DockerComposeServiceName(target string) string {
	out, err := exec.Command("docker", "inspect", "-f",
		`{{index .Config.Labels "com.docker.compose.service"}}`, target).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ComposeFileForContainer(target, fallbackProjectPath string) string {
	out, err := exec.Command("docker", "inspect", "-f",
		`{{index .Config.Labels "com.docker.compose.project.config_files"}}`, target).Output()
	if err == nil {
		for _, f := range strings.Split(string(out), ",") {
			f = strings.TrimSpace(f)
			if f != "" {
				if _, statErr := os.Stat(f); statErr == nil {
					return f
				}
			}
		}
	}
	return ComposeFile(fallbackProjectPath)
}

func ReadFileContent(path string, maxBytes int) (string, error) {
	if path == "" {
		return "", fmt.Errorf("arquivo não encontrado")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if maxBytes > 0 && len(data) > maxBytes {
		data = data[:maxBytes]
		return string(data) + "\n\n... (arquivo truncado)", nil
	}
	if len(data) == 0 {
		return "(arquivo vazio)", nil
	}
	return string(data), nil
}

func ReadComposeForContainer(target, projectPath string) (path, content string, err error) {
	path = ComposeFileForContainer(target, projectPath)
	if path == "" {
		return "", "", fmt.Errorf("docker-compose não encontrado")
	}
	content, err = ReadFileContent(path, 120000)
	return path, content, err
}

func DockerfileForContainer(target, projectPath string) (path, content string, err error) {
	service := DockerComposeServiceName(target)
	composePath := ComposeFileForContainer(target, projectPath)
	if composePath != "" && service != "" {
		if df := dockerfileFromCompose(composePath, service); df != "" {
			if content, err := ReadFileContent(df, 120000); err == nil {
				return df, content, nil
			}
		}
	}
	root := projectPath
	if root == "" {
		if composePath != "" {
			root = filepath.Dir(composePath)
		}
	}
	for _, name := range []string{"Dockerfile", "dockerfile"} {
		p := filepath.Join(root, name)
		if _, statErr := os.Stat(p); statErr == nil {
			content, err := ReadFileContent(p, 120000)
			return p, content, err
		}
	}
	return "", "", fmt.Errorf("Dockerfile não encontrado")
}

func dockerInspect(target string) (*dockerInspectRow, error) {
	out, err := exec.Command("docker", "inspect", target).Output()
	if err != nil {
		return nil, err
	}
	var rows []dockerInspectRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("container não encontrado")
	}
	return &rows[0], nil
}

func dockerfileFromCompose(composePath, service string) string {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	inService := false
	serviceIndent := 0
	composeDir := filepath.Dir(composePath)
	var buildContext string
	var dockerfileName string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if trimmed == service+":" {
			inService = true
			serviceIndent = indent
			buildContext = ""
			dockerfileName = ""
			continue
		}
		if !inService {
			continue
		}
		if indent <= serviceIndent {
			break
		}
		if strings.HasPrefix(trimmed, "dockerfile:") {
			dockerfileName = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "dockerfile:")), `"'`)
		}
		if strings.HasPrefix(trimmed, "context:") {
			buildContext = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "context:")), `"'`)
		}
		if strings.HasPrefix(trimmed, "build:") {
			ctx := strings.TrimSpace(strings.TrimPrefix(trimmed, "build:"))
			ctx = strings.Trim(ctx, `"'`)
			if ctx == "." || ctx == "" {
				buildContext = "."
			}
		}
	}
	if dockerfileName != "" {
		base := composeDir
		if buildContext != "" && buildContext != "." {
			base = filepath.Join(composeDir, buildContext)
		}
		return resolveDockerfilePath(base, dockerfileName)
	}
	if buildContext != "" {
		p := filepath.Join(composeDir, buildContext, "Dockerfile")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func resolveDockerfilePath(base, df string) string {
	if df == "" {
		df = "Dockerfile"
	}
	if filepath.IsAbs(df) {
		return df
	}
	return filepath.Join(base, df)
}
