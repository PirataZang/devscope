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

// MergeTunnels keeps only tunnels from this project's config.
// Live agent tunnels from other projects are ignored (agent is global).
func MergeTunnels(cfg ProjectConfig, live []Tunnel) []Tunnel {
	byName := map[string]Tunnel{}
	byPort := map[int][]Tunnel{}
	for _, t := range live {
		byName[t.Name] = t
		if t.Port > 0 {
			byPort[t.Port] = append(byPort[t.Port], t)
		}
	}
	var out []Tunnel
	for _, c := range cfg.Tunnels {
		liveT, ok := byName[c.Name]
		if !ok && c.Port > 0 {
			if matches := byPort[c.Port]; len(matches) == 1 {
				liveT, ok = matches[0], true
			}
		}
		if ok {
			liveT.Name = c.Name
			liveT.Project = cfg.Project
			if liveT.Port == 0 {
				liveT.Port = c.Port
			}
			if liveT.Proto == "" {
				liveT.Proto = c.Proto
			}
			if liveT.Domain == "" {
				liveT.Domain = c.Domain
			}
			out = append(out, liveT)
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
	}
	return out
}

// CountForeignLive returns how many live tunnels are not in this project config.
func CountForeignLive(cfg ProjectConfig, live []Tunnel) int {
	owned := map[string]bool{}
	ports := map[int]bool{}
	for _, c := range cfg.Tunnels {
		owned[c.Name] = true
		if c.Port > 0 {
			ports[c.Port] = true
		}
	}
	n := 0
	for _, t := range live {
		if owned[t.Name] || (t.Port > 0 && ports[t.Port]) {
			continue
		}
		n++
	}
	return n
}

// MergeTunnelsAll lists this project's tunnels first, then other live agent tunnels.
func MergeTunnelsAll(cfg ProjectConfig, live []Tunnel) []Tunnel {
	out := MergeTunnels(cfg, live)
	ownedName := map[string]bool{}
	ownedPort := map[int]bool{}
	for _, t := range out {
		ownedName[t.Name] = true
		if t.Port > 0 {
			ownedPort[t.Port] = true
		}
	}
	for _, t := range live {
		if ownedName[t.Name] || (t.Port > 0 && ownedPort[t.Port]) {
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
