package wsutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ProjectConfig struct {
	URLs      []string  `json:"urls"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "ws.json")
}

func LoadProject(projectPath string) ProjectConfig {
	var cfg ProjectConfig
	if projectPath == "" {
		return cfg
	}
	b, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	cfg.URLs = cleanURLs(cfg.URLs)
	return cfg
}

func SaveProject(projectPath string, cfg ProjectConfig) error {
	if projectPath == "" {
		return nil
	}
	cfg.URLs = cleanURLs(cfg.URLs)
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

func cleanURLs(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, u := range in {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
		if len(out) >= 24 {
			break
		}
	}
	return out
}
