package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Scan    ScanConfig             `mapstructure:"scan" yaml:"scan"`
	Refresh RefreshConfig          `mapstructure:"refresh" yaml:"refresh"`
	UI      UIConfig               `mapstructure:"ui" yaml:"ui"`
	Health  HealthConfig           `mapstructure:"health" yaml:"health"`
	Tools   ToolsConfig            `mapstructure:"tools" yaml:"tools"`
	Apps    map[string]AppShortcut `mapstructure:"apps" yaml:"apps"`
	Pinned  []string               `mapstructure:"pinned" yaml:"pinned"`
}

// ToolsConfig aponta para os programas externos que o DevScope abre no lugar
// dele mesmo. Vazio significa "descubra": qual agente de IA está no PATH, qual
// editor o $EDITOR indica.
type ToolsConfig struct {
	AI     string `mapstructure:"ai" yaml:"ai"`
	Editor string `mapstructure:"editor" yaml:"editor"`
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

// AppShortcut é um programa externo aberto por uma tecla da tela inicial.
// Command vai para o shell, então aceita argumentos, ~, variáveis e o "&" do
// final, que é o que diferencia app de janela (solta e volta pro DevScope) de
// comando de terminal (assume a tela até sair).
type AppShortcut struct {
	Name    string `mapstructure:"name" yaml:"name"`
	Command string `mapstructure:"command" yaml:"command"`
}

// AppSlots é a ordem das teclas na tela inicial: 1 a 9 e o 0 por último, como
// na fileira do teclado. Dez atalhos é o limite — são dez teclas.
var AppSlots = []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"}

type AppEntry struct {
	Key     string
	Name    string
	Command string
}

// AppShortcuts devolve os atalhos configurados na ordem das teclas, ignorando
// slot fora de 1-0 e entrada sem comando. Sem nome, a tecla mostra o comando.
func (c *Config) AppShortcuts() []AppEntry {
	if c == nil || len(c.Apps) == 0 {
		return nil
	}
	var out []AppEntry
	for _, key := range AppSlots {
		app, ok := c.Apps[key]
		if !ok {
			continue
		}
		cmd := strings.TrimSpace(app.Command)
		if cmd == "" {
			continue
		}
		name := strings.TrimSpace(app.Name)
		if name == "" {
			name = strings.Fields(cmd)[0]
		}
		out = append(out, AppEntry{Key: key, Name: name, Command: cmd})
	}
	return out
}

// AppShortcut acha o atalho de uma tecla.
func (c *Config) AppShortcutFor(key string) (AppEntry, bool) {
	for _, e := range c.AppShortcuts() {
		if e.Key == key {
			return e, true
		}
	}
	return AppEntry{}, false
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

// LogPath returns where background collector errors are logged. These must
// never go to stdout/stderr while the TUI owns the terminal (alt screen) —
// raw writes there desync Bubble Tea's renderer and cause stacked/duplicated
// frames, since it loses track of what it actually painted.
func LogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devscope", "devscope.log")
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
	// user_config.txt é a palavra final sobre tema, atalhos e IA — é ele que a
	// pessoa edita com Shift+C.
	prefs, _ := LoadUserPrefs()
	cfg.ApplyUserPrefs(prefs)
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

// SaveValue faz merge de uma chave no config do usuário sem tocar no resto do
// arquivo: quem editou o YAML à mão não perde comentário de outra seção.

// EnsureConfigFile cria o arquivo com os defaults comentados quando ele ainda
// não existe. Abrir um arquivo vazio no editor não diz o que pode ser escrito.
