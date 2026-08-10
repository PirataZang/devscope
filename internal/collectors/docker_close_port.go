package collectors

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type dockerCloseInspect struct {
	Name   string `json:"Name"`
	Config struct {
		Image      string   `json:"Image"`
		Env        []string `json:"Env"`
		Cmd        []string `json:"Cmd"`
		WorkingDir string   `json:"WorkingDir"`
	} `json:"Config"`
	HostConfig struct {
		Binds         []string `json:"Binds"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		PortBindings map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct{} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// DockerCloseHostPort recreates the container without publishing hostPort.
// ponytail: copies image/env/cmd/binds/networks/ports only; expand if exotic HostConfig breaks.
func DockerCloseHostPort(target string, hostPort int) error {
	if target == "" || hostPort <= 0 {
		return fmt.Errorf("alvo/porta inválidos")
	}
	out, err := exec.Command("docker", "inspect", target).Output()
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}
	var rows []dockerCloseInspect
	if err := json.Unmarshal(out, &rows); err != nil || len(rows) == 0 {
		return fmt.Errorf("inspect inválido")
	}
	row := rows[0]
	name := strings.TrimPrefix(row.Name, "/")
	if name == "" {
		name = target
	}
	want := strconv.Itoa(hostPort)
	remaining := make(map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	})
	removed := false
	for cont, binds := range row.HostConfig.PortBindings {
		var keep []struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		}
		for _, b := range binds {
			if b.HostPort == want {
				removed = true
				continue
			}
			keep = append(keep, b)
		}
		if len(keep) > 0 {
			remaining[cont] = keep
		}
	}
	if !removed {
		return fmt.Errorf("porta :%d não está publicada", hostPort)
	}

	args := []string{"create", "--name", name + ".devscope-new"}
	if pol := row.HostConfig.RestartPolicy.Name; pol != "" && pol != "no" {
		args = append(args, "--restart", pol)
	}
	if row.Config.WorkingDir != "" {
		args = append(args, "-w", row.Config.WorkingDir)
	}
	for _, e := range row.Config.Env {
		args = append(args, "-e", e)
	}
	for _, bind := range row.HostConfig.Binds {
		if bind != "" {
			args = append(args, "-v", bind)
		}
	}
	for cont, binds := range remaining {
		contPort, proto, _ := strings.Cut(cont, "/")
		if contPort == "" {
			continue
		}
		for _, b := range binds {
			pub := b.HostPort + ":" + contPort
			if b.HostIP != "" && b.HostIP != "0.0.0.0" && b.HostIP != "::" {
				pub = b.HostIP + ":" + pub
			}
			if proto == "udp" {
				pub += "/udp"
			}
			args = append(args, "-p", pub)
		}
	}
	netName := firstNetworkName(row)
	if netName != "" && row.HostConfig.NetworkMode != "host" {
		args = append(args, "--network", netName)
	}
	if row.Config.Image == "" {
		return fmt.Errorf("imagem vazia no inspect")
	}
	args = append(args, row.Config.Image)
	args = append(args, row.Config.Cmd...)

	newName := name + ".devscope-new"
	oldBak := name + ".devscope-old"
	_ = exec.Command("docker", "rm", "-f", newName).Run()
	_ = exec.Command("docker", "rm", "-f", oldBak).Run()

	if err := exec.Command("docker", "stop", target).Run(); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	if err := exec.Command("docker", "rename", target, oldBak).Run(); err != nil {
		_ = exec.Command("docker", "start", target).Run()
		return fmt.Errorf("rename: %w", err)
	}
	createOut, err := exec.Command("docker", args...).CombinedOutput()
	if err != nil {
		_ = exec.Command("docker", "rename", oldBak, name).Run()
		_ = exec.Command("docker", "start", name).Run()
		return fmt.Errorf("create: %s", strings.TrimSpace(string(createOut)))
	}
	newID := strings.TrimSpace(string(createOut))
	for n := range row.NetworkSettings.Networks {
		if n == "" || n == netName {
			continue
		}
		_ = exec.Command("docker", "network", "connect", n, newID).Run()
	}
	if err := exec.Command("docker", "rename", newName, name).Run(); err != nil {
		_ = exec.Command("docker", "start", newID).Run()
		_ = exec.Command("docker", "rm", "-f", oldBak).Run()
		return fmt.Errorf("rename new: %w", err)
	}
	if err := exec.Command("docker", "start", name).Run(); err != nil {
		_ = exec.Command("docker", "rm", "-f", name).Run()
		_ = exec.Command("docker", "rename", oldBak, name).Run()
		_ = exec.Command("docker", "start", name).Run()
		return fmt.Errorf("start: %w", err)
	}
	_ = exec.Command("docker", "rm", "-f", oldBak).Run()
	return nil
}

func firstNetworkName(row dockerCloseInspect) string {
	for n := range row.NetworkSettings.Networks {
		if n != "" && n != "default" {
			return n
		}
	}
	for n := range row.NetworkSettings.Networks {
		if n != "" {
			return n
		}
	}
	return ""
}
