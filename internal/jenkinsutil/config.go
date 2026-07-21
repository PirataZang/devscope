package jenkinsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ProjectConfig struct {
	URL        string    `json:"url"`
	User       string    `json:"user"`
	Token      string    `json:"token"`
	Folder     string    `json:"folder,omitempty"`
	RefreshSec int       `json:"refresh_sec,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "jenkins.json")
}

func LoadProject(projectPath string) ProjectConfig {
	cfg := ProjectConfig{RefreshSec: 5}
	b, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.RefreshSec <= 0 {
		cfg.RefreshSec = 5
	}
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	return cfg
}

func SaveProject(projectPath string, cfg ProjectConfig) error {
	cfg.URL = strings.TrimRight(strings.TrimSpace(cfg.URL), "/")
	if cfg.RefreshSec <= 0 {
		cfg.RefreshSec = 5
	}
	cfg.UpdatedAt = time.Now()
	dir := filepath.Dir(ConfigPath(projectPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(projectPath), b, 0o600)
}

func (c ProjectConfig) Configured() bool {
	return c.URL != "" && c.User != "" && c.Token != ""
}

func (c ProjectConfig) Host() string {
	u := strings.TrimPrefix(c.URL, "https://")
	u = strings.TrimPrefix(u, "http://")
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	return u
}

func MaskToken(token string) string {
	if token == "" {
		return "(vazio)"
	}
	if len(token) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(token)-4) + token[len(token)-4:]
}
