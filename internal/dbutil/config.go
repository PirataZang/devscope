// Package dbutil stores the database credentials a user explicitly grants
// devscope access to, in .devscope/database.json — the same pattern used for
// ssh.json, ngrok.json and cloudflare.json.
package dbutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/devscopeutil"
)

// Credential is one manually granted database connection. Either Container
// (docker exec target) or Host (direct TCP via the local psql/mysql client)
// must be set.
type Credential struct {
	Name      string    `json:"name"`
	Engine    string    `json:"engine"` // postgres | mysql
	Container string    `json:"container,omitempty"`
	Host      string    `json:"host,omitempty"`
	Port      int       `json:"port,omitempty"`
	User      string    `json:"user"`
	Password  string    `json:"password,omitempty"`
	Database  string    `json:"database"`
	SSLMode   string    `json:"ssl_mode,omitempty"` // postgres only
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ProjectConfig struct {
	Credentials []Credential `json:"credentials"`
	UpdatedAt   time.Time    `json:"updated_at,omitempty"`
}

func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".devscope", "database.json")
}

func LoadProject(projectPath string) ProjectConfig {
	var cfg ProjectConfig
	b, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	return cfg
}

func SaveProject(projectPath string, cfg ProjectConfig) error {
	cfg.UpdatedAt = time.Now()
	if _, err := devscopeutil.EnsureDir(projectPath); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(projectPath), b, 0o600)
}

func (c *ProjectConfig) Upsert(cred Credential) {
	cred.Name = strings.TrimSpace(cred.Name)
	if cred.CreatedAt.IsZero() {
		cred.CreatedAt = time.Now()
	}
	for i := range c.Credentials {
		if c.Credentials[i].Name == cred.Name {
			c.Credentials[i] = cred
			return
		}
	}
	c.Credentials = append(c.Credentials, cred)
}

func (c *ProjectConfig) Remove(name string) {
	out := c.Credentials[:0]
	for _, cred := range c.Credentials {
		if cred.Name != name {
			out = append(out, cred)
		}
	}
	c.Credentials = out
}
