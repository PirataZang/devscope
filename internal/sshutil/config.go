package sshutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Modes: local (-L), remote (-R), dynamic (-D).
const (
	ModeLocal   = "local"
	ModeRemote  = "remote"
	ModeDynamic = "dynamic"
)

type TunnelConfig struct {
	Name       string    `json:"name"`
	Mode       string    `json:"mode"` // local | remote | dynamic
	LocalPort  int       `json:"local_port"`
	RemoteHost string    `json:"remote_host,omitempty"`
	RemotePort int       `json:"remote_port,omitempty"`
	Target     string    `json:"target"` // user@host
	Identity   string    `json:"identity,omitempty"`
	AutoStart  bool      `json:"auto_start,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
}

func (t *TunnelConfig) normalize() {
	t.Name = sanitizeName(t.Name)
	t.Mode = NormalizeMode(t.Mode)
	t.Target = strings.TrimSpace(t.Target)
	t.Identity = strings.TrimSpace(t.Identity)
	t.RemoteHost = strings.TrimSpace(t.RemoteHost)
	if t.Mode != ModeDynamic {
		if t.RemoteHost == "" {
			t.RemoteHost = "127.0.0.1"
		}
	}
}

type ProjectConfig struct {
	Project   string         `json:"project"`
	Tunnels   []TunnelConfig `json:"tunnels"`
	History   []HistoryEntry `json:"history,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

type HistoryEntry struct {
	Name      string    `json:"name"`
	Mode      string    `json:"mode"`
	LocalPort int       `json:"local_port"`
	Target    string    `json:"target,omitempty"`
	Started   time.Time `json:"started"`
	Stopped   time.Time `json:"stopped,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "ssh.json")
}

func LoadProject(projectPath, projectName string) ProjectConfig {
	cfg := ProjectConfig{Project: projectName}
	b, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.Project == "" {
		cfg.Project = projectName
	}
	for i := range cfg.Tunnels {
		cfg.Tunnels[i].normalize()
	}
	return cfg
}

func SaveProject(projectPath string, cfg ProjectConfig) error {
	cfg.UpdatedAt = time.Now()
	dir := filepath.Dir(ConfigPath(projectPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(projectPath), b, 0o644)
}

func (c *ProjectConfig) UpsertTunnel(t TunnelConfig) {
	t.normalize()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	for i := range c.Tunnels {
		if c.Tunnels[i].Name == t.Name {
			c.Tunnels[i] = t
			return
		}
	}
	c.Tunnels = append(c.Tunnels, t)
}

func (c *ProjectConfig) RemoveTunnel(name string) {
	out := c.Tunnels[:0]
	for _, t := range c.Tunnels {
		if t.Name != name {
			out = append(out, t)
		}
	}
	c.Tunnels = out
}

func NormalizeMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case ModeRemote, "r", "-r":
		return ModeRemote
	case ModeDynamic, "d", "-d", "socks":
		return ModeDynamic
	default:
		return ModeLocal
	}
}

func MergeTunnels(cfg ProjectConfig, live []Tunnel) []Tunnel {
	byName := map[string]Tunnel{}
	byPort := map[int][]Tunnel{}
	for _, t := range live {
		byName[t.Name] = t
		if t.LocalPort > 0 {
			byPort[t.LocalPort] = append(byPort[t.LocalPort], t)
		}
	}
	var out []Tunnel
	for _, c := range cfg.Tunnels {
		liveT, ok := byName[c.Name]
		if !ok && c.LocalPort > 0 {
			if matches := byPort[c.LocalPort]; len(matches) == 1 {
				liveT, ok = matches[0], true
			}
		}
		if ok {
			liveT.Name = c.Name
			liveT.Project = cfg.Project
			if liveT.LocalPort == 0 {
				liveT.LocalPort = c.LocalPort
			}
			if liveT.Mode == "" {
				liveT.Mode = c.Mode
			}
			if liveT.RemoteHost == "" {
				liveT.RemoteHost = c.RemoteHost
			}
			if liveT.RemotePort == 0 {
				liveT.RemotePort = c.RemotePort
			}
			if liveT.Target == "" {
				liveT.Target = c.Target
			}
			if liveT.Identity == "" {
				liveT.Identity = c.Identity
			}
			out = append(out, liveT)
			continue
		}
		out = append(out, Tunnel{
			Name:       c.Name,
			Project:    cfg.Project,
			Mode:       c.Mode,
			LocalPort:  c.LocalPort,
			RemoteHost: c.RemoteHost,
			RemotePort: c.RemotePort,
			Target:     c.Target,
			Identity:   c.Identity,
			Status:     "offline",
		})
	}
	return out
}

func CountForeignLive(cfg ProjectConfig, live []Tunnel) int {
	owned := map[string]bool{}
	ports := map[int]bool{}
	for _, c := range cfg.Tunnels {
		owned[c.Name] = true
		if c.LocalPort > 0 {
			ports[c.LocalPort] = true
		}
	}
	n := 0
	for _, t := range live {
		if owned[t.Name] || (t.LocalPort > 0 && ports[t.LocalPort]) {
			continue
		}
		n++
	}
	return n
}

func MergeTunnelsAll(cfg ProjectConfig, live []Tunnel) []Tunnel {
	out := MergeTunnels(cfg, live)
	ownedName := map[string]bool{}
	ownedPort := map[int]bool{}
	for _, t := range out {
		ownedName[t.Name] = true
		if t.LocalPort > 0 {
			ownedPort[t.LocalPort] = true
		}
	}
	for _, t := range live {
		if ownedName[t.Name] || (t.LocalPort > 0 && ownedPort[t.LocalPort]) {
			continue
		}
		if t.Project == "" || t.Project == cfg.Project {
			t.Project = "(outro)"
		}
		if t.Status == "" {
			t.Status = "online"
		}
		out = append(out, t)
	}
	return out
}
