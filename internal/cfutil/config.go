package cfutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type TunnelConfig struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`            // destino local: http://localhost:4321
	Port      int       `json:"port,omitempty"` // derivado da URL (compat)
	Hostname  string    `json:"hostname,omitempty"`
	Mode      string    `json:"mode,omitempty"` // quick | named | http2
	TunnelID  string    `json:"tunnel_id,omitempty"`
	AutoStart bool      `json:"auto_start,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// normalize keeps URL as the source of truth and Port derived from it, so
// configs written before the URL field still load.
func (t *TunnelConfig) normalize() {
	t.URL = NormalizeURL(t.URL)
	if t.URL == "" {
		t.URL = localURL(t.Port)
	}
	t.Port = parseAddrPort(t.URL)
}

type ProjectConfig struct {
	Project   string         `json:"project"`
	Tunnels   []TunnelConfig `json:"tunnels"`
	History   []HistoryEntry `json:"history,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
}

type HistoryEntry struct {
	Name     string    `json:"name"`
	Target   string    `json:"target,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	Mode     string    `json:"mode,omitempty"`
	Started  time.Time `json:"started"`
	Stopped  time.Time `json:"stopped,omitempty"`
	URL      string    `json:"url,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "cloudflare.json")
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
	t.Mode = NormalizeMode(t.Mode, t.Hostname)
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
			if liveT.LocalURL == "" {
				liveT.LocalURL = c.URL
			}
			if liveT.Hostname == "" {
				liveT.Hostname = c.Hostname
			}
			if liveT.Mode == "" {
				liveT.Mode = c.Mode
			}
			if liveT.TunnelID == "" {
				liveT.TunnelID = c.TunnelID
			}
			out = append(out, liveT)
			continue
		}
		mode := c.Mode
		if mode == "" {
			mode = "quick"
		}
		out = append(out, Tunnel{
			Name:     c.Name,
			Project:  cfg.Project,
			Port:     c.Port,
			Hostname: c.Hostname,
			Mode:     mode,
			TunnelID: c.TunnelID,
			Status:   "offline",
			LocalURL: c.URL,
		})
	}
	return out
}

// CountForeignLive returns live tunnels that are not in this project config.
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

// MergeTunnelsAll lists project tunnels first, then other live system tunnels.
func MergeTunnelsAll(cfg ProjectConfig, live []Tunnel) []Tunnel {
	out := MergeTunnels(cfg, live)
	ownedName := map[string]bool{}
	ownedPort := map[int]bool{}
	ownedPID := map[int]bool{}
	for _, t := range out {
		ownedName[t.Name] = true
		if t.Port > 0 {
			ownedPort[t.Port] = true
		}
		if t.PID > 0 {
			ownedPID[t.PID] = true
		}
	}
	for _, t := range live {
		if ownedName[t.Name] || (t.Port > 0 && ownedPort[t.Port]) || (t.PID > 0 && ownedPID[t.PID]) {
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
