package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Scan    ScanConfig    `mapstructure:"scan" yaml:"scan"`
	Refresh RefreshConfig `mapstructure:"refresh" yaml:"refresh"`
	UI      UIConfig      `mapstructure:"ui" yaml:"ui"`
	Health  HealthConfig  `mapstructure:"health" yaml:"health"`
	Pinned  []string      `mapstructure:"pinned" yaml:"pinned"`
}

type ScanConfig struct {
	Paths    []string `mapstructure:"paths" yaml:"paths"`
	MaxDepth int      `mapstructure:"max_depth" yaml:"max_depth"`
	Ignore   []string `mapstructure:"ignore" yaml:"ignore"`
}

type RefreshConfig struct {
	ScanInterval    time.Duration `mapstructure:"scan_interval" yaml:"scan_interval"`
	MetricsInterval time.Duration `mapstructure:"metrics_interval" yaml:"metrics_interval"`
	HealthInterval  time.Duration `mapstructure:"health_interval" yaml:"health_interval"`
	GitInterval     time.Duration `mapstructure:"git_interval" yaml:"git_interval"`
}

type UIConfig struct {
	Theme string `mapstructure:"theme" yaml:"theme"`
}

type HealthConfig struct {
	Timeout    time.Duration `mapstructure:"timeout" yaml:"timeout"`
	Concurrent int           `mapstructure:"concurrent" yaml:"concurrent"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	paths := []string{
		"/var/www",
		filepath.Join(home, "projects"),
		filepath.Join(home, "Projects"),
		filepath.Join(home, "projetos"),
		filepath.Join(home, "Projetos"),
		filepath.Join(home, "workspace"),
		filepath.Join(home, "dev"),
		filepath.Join(home, "code"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "src"),
		filepath.Join(home, "Documentos"),
		filepath.Join(home, "Documentos", "Projeto Pessoal"),
		filepath.Join(home, "Documentos", "Projeto Pessoial"),
		filepath.Join(home, "Área de trabalho"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Área de Trabalho"),
		"/opt",
		"/srv",
		"/workspace",
		"/projetos",
		"/etc/projects",
	}

	var existing []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		existing = []string{home}
	}

	return &Config{
		Scan: ScanConfig{
			Paths:    existing,
			MaxDepth: 6,
			Ignore: []string{
				"node_modules", "vendor", ".git", ".cache", ".cursor", ".npm",
				".local", ".config", ".nvm", ".continue", "dist", "build",
				".next", "target", "Trash",
			},
		},
		Refresh: RefreshConfig{
			ScanInterval:    60 * time.Second,
			MetricsInterval: 2 * time.Second,
			HealthInterval:  10 * time.Second,
			GitInterval:     30 * time.Second,
		},
		UI: UIConfig{Theme: "dark"},
		Health: HealthConfig{
			Timeout:    5 * time.Second,
			Concurrent: 10,
		},
	}
}

func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devscope", "config.yaml")
}

func Load(cfgFile string) (*Config, error) {
	cfg := Default()

	v := viper.New()
	v.SetConfigType("yaml")

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", "devscope"))
			v.AddConfigPath("/etc/devscope")
		}
		v.SetConfigName("config")
	}

	v.SetEnvPrefix("DEVSCOPE")
	v.AutomaticEnv()
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && cfgFile != "" {
			return nil, err
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("scan.max_depth", 6)
	v.SetDefault("refresh.scan_interval", "60s")
	v.SetDefault("refresh.metrics_interval", "2s")
	v.SetDefault("refresh.health_interval", "10s")
	v.SetDefault("refresh.git_interval", "30s")
	v.SetDefault("ui.theme", "dark")
	v.SetDefault("health.timeout", "5s")
	v.SetDefault("health.concurrent", 10)
}

// SaveTheme merges ui.theme into the user config so it survives restarts.
func SaveTheme(theme string) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	raw := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(b, &raw)
	}
	ui, _ := raw["ui"].(map[string]any)
	if ui == nil {
		ui = map[string]any{}
	}
	ui["theme"] = theme
	raw["ui"] = ui

	b, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
