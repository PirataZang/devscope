package ngrokutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type TunnelConfig struct {
	Name      string    `json:"name"`
	Port      int       `json:"port"`
	Proto     string    `json:"proto"`
	Domain    string    `json:"domain,omitempty"`
	Region    string    `json:"region,omitempty"`
	AutoStart bool      `json:"auto_start,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ProjectConfig struct {
	Project   string         `json:"project"`
	Region    string         `json:"region,omitempty"`
	Tunnels   []TunnelConfig `json:"tunnels"`
	History   []HistoryEntry `json:"history,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

type HistoryEntry struct {
	Name     string    `json:"name"`
	Port     int       `json:"port"`
	Proto    string    `json:"proto"`
	Started  time.Time `json:"started"`
	Stopped  time.Time `json:"stopped,omitempty"`
	Requests int64     `json:"requests,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "ngrok.json")
}

func LoadProject(projectPath, projectName string) ProjectConfig {
	cfg := ProjectConfig{Project: projectName, Region: "us"}
	b, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.Project == "" {
		cfg.Project = projectName
	}
	if cfg.Region == "" {
		cfg.Region = "us"
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
	if t.Proto == "" {
		t.Proto = "http"
	}
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

func MergeTunnels(cfg ProjectConfig, live []Tunnel) []Tunnel {
	byName := map[string]Tunnel{}
	for _, t := range live {
		t.Project = cfg.Project
		byName[t.Name] = t
	}
	var out []Tunnel
	seen := map[string]bool{}
	for _, c := range cfg.Tunnels {
		if liveT, ok := byName[c.Name]; ok {
			liveT.Project = cfg.Project
			if liveT.Port == 0 {
				liveT.Port = c.Port
			}
			if liveT.Proto == "" {
				liveT.Proto = c.Proto
			}
			out = append(out, liveT)
			seen[c.Name] = true
			continue
		}
		out = append(out, Tunnel{
			Name:    c.Name,
			Project: cfg.Project,
			Port:    c.Port,
			Proto:   c.Proto,
			Domain:  c.Domain,
			Region:  c.Region,
			Status:  "offline",
		})
		seen[c.Name] = true
	}
	for _, t := range live {
		if seen[t.Name] {
			continue
		}
		t.Project = cfg.Project
		out = append(out, t)
	}
	return out
}
