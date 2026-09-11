package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// user_config.txt guarda o que a pessoa escolhe no dia a dia: tema, atalhos de
// programas e agente de IA. Fica separado do config.yaml — aquele é do scanner
// (caminhos, intervalos, ignore) e quase nunca muda; este é editado de dentro
// do DevScope, com Shift+C, e por isso precisa ser texto comentado em vez de
// YAML mudo.

// UserPrefs é o que o arquivo declara.
type UserPrefs struct {
	Theme    string
	AI       string
	Commands map[string]AppShortcut
}

func UserConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devscope", "user_config.txt")
}

// DefaultUserConfigPath é a cópia intocada de fábrica. O DevScope nunca lê este
// arquivo — ele existe para a hora em que a edição deu errado e a pessoa quer
// o original de volta, dentro ou fora da ferramenta.
func DefaultUserConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devscope", "user_config.default.txt")
}

// FactoryUserConfig é o conteúdo de fábrica: tema padrão do DevScope e os
// agentes que existem nesta máquina.
func FactoryUserConfig(ais []string) string {
	return DefaultUserConfig(Default().UI.Theme, ais)
}

// WriteDefaultUserConfig (re)escreve a cópia de fábrica. É reescrita a cada
// abertura da tela: assim ela acompanha a versão do binário em vez de guardar
// o padrão de uma versão antiga.
func WriteDefaultUserConfig(ais []string) (string, error) {
	path := DefaultUserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, err
	}
	return path, os.WriteFile(path, []byte(FactoryUserConfig(ais)), 0o644)
}

// EnsureUserConfig devolve o conteúdo do arquivo, criando-o na primeira vez com
// o tema em uso, um comando de exemplo que existe no sistema operacional atual
// e as IAs realmente encontradas na máquina.
func EnsureUserConfig(theme string, ais []string) (path, content string, err error) {
	path = UserConfigPath()
	if b, readErr := os.ReadFile(path); readErr == nil {
		return path, string(b), nil
	}
	content = DefaultUserConfig(theme, ais)
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, content, err
	}
	return path, content, os.WriteFile(path, []byte(content), 0o644)
}

// BackupUserConfigPath guarda o que existia antes de um reset. É a rede embaixo
// da confirmação: mesmo confirmando por engano, o arquivo antigo continua ali.
func BackupUserConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "devscope", "user_config.bak.txt")
}

// ResetUserConfig grava o padrão de fábrica e devolve o conteúdo aplicado,
// guardando o arquivo anterior em user_config.bak.txt.
func ResetUserConfig(ais []string) (content string, err error) {
	if old, readErr := os.ReadFile(UserConfigPath()); readErr == nil {
		if err = os.WriteFile(BackupUserConfigPath(), old, 0o644); err != nil {
			return "", err
		}
	}
	content = FactoryUserConfig(ais)
	return content, SaveUserConfig(content)
}

func SaveUserConfig(content string) error {
	path := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// defaultOpenCommand é o comando de abrir pasta que existe em cada sistema —
// serve de exemplo pronto para o slot 1 sem apostar em programa instalado.
func defaultOpenCommand() (name, command string) {
	switch runtime.GOOS {
	case "windows":
		return "Abrir a pasta", "explorer ."
	case "darwin":
		return "Abrir a pasta", "open ."
	default:
		return "Abrir a pasta", "xdg-open . &"
	}
}

func DefaultUserConfig(theme string, ais []string) string {
	name, cmd := defaultOpenCommand()
	prefs := UserPrefs{
		Theme:    theme,
		Commands: map[string]AppShortcut{"1": {Name: name, Command: cmd}},
	}
	if len(ais) > 0 {
		prefs.AI = ais[0]
	}
	return RenderUserConfig(prefs, ais)
}

// RenderUserConfig escreve o arquivo com os valores dados. Os comentários vêm
// junto sempre: é a mesma explicação que o modal mostra, e é o que faz o
// arquivo continuar editável à mão por quem preferir.
func RenderUserConfig(prefs UserPrefs, ais []string) string {
	theme := strings.TrimSpace(prefs.Theme)
	if theme == "" {
		theme = "devscope"
	}
	detected := "nenhuma encontrada no PATH"
	if len(ais) > 0 {
		detected = strings.Join(ais, ", ")
	}

	var b strings.Builder
	b.WriteString(`# ---------------------------------------------------------------
#  DevScope · preferências do usuário
#  Shift+C abre o painel de preferências dentro do DevScope.
#  Ctrl+R lá volta tudo ao padrão (pede confirmação; o arquivo atual
#  é copiado para user_config.bak.txt antes).
#  A cópia de fábrica fica em user_config.default.txt, ao lado deste.
# ---------------------------------------------------------------

# THEME — as cores do DevScope.
# Shift+T na tela inicial também troca e grava aqui.
# Valores: devscope · dark · light · auto · tokyo-night · catppuccin
#          rose-pine · solarized · dracula · dracula-vivid · nord
#          monokai · gruvbox
Theme: ` + theme + `

# COMMANDS — as teclas 1 a 9 e 0 da tela inicial abrem um programa.
# "name" é o texto que aparece na tela; "command" é o que vai para o
# terminal, igual você digitaria. Termine com "&" para o programa abrir
# solto e o DevScope continuar na frente (app de janela); sem o "&" ele
# assume o terminal até você sair dele.
# O comando roda dentro da pasta do projeto selecionado.
Commands: {
`)
	rows := make([]string, 0, len(AppSlots))
	for _, key := range AppSlots {
		app := prefs.Commands[key]
		rows = append(rows, fmt.Sprintf("  %q: {\"name\": %q, \"command\": %q}",
			key, strings.TrimSpace(app.Name), strings.TrimSpace(app.Command)))
	}
	b.WriteString(strings.Join(rows, ",\n"))
	b.WriteString(`
}

# AI — o agente de IA que o Ctrl+O abre na pasta do projeto.
# Encontrados nesta máquina: ` + detected + `
# Aceita argumentos, por exemplo: claude --continue
# Deixe vazio para o DevScope usar o primeiro que achar no PATH.
AI: ` + strings.TrimSpace(prefs.AI) + `
`)
	return b.String()
}

// ParseUserConfig lê o arquivo. Erro de digitação não pode derrubar a TUI nem
// zerar as outras preferências: cada aviso volta em warnings e o que deu certo
// é aplicado.
func ParseUserConfig(content string) (UserPrefs, []string) {
	prefs := UserPrefs{}
	var warnings []string

	body, jsonPart := splitCommandsBlock(content)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(stripComment(line))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "theme":
			prefs.Theme = value
		case "ai":
			prefs.AI = value
		}
	}

	if jsonPart != "" {
		var cmds map[string]AppShortcut
		if err := json.Unmarshal([]byte(jsonPart), &cmds); err != nil {
			warnings = append(warnings, "Commands: JSON inválido — "+err.Error())
		} else {
			prefs.Commands = map[string]AppShortcut{}
			for key, app := range cmds {
				if !validAppSlot(key) {
					warnings = append(warnings, "Commands: a tecla "+key+" não existe (use 0 a 9)")
					continue
				}
				prefs.Commands[key] = app
			}
		}
	}
	return prefs, warnings
}

// splitCommandsBlock separa o JSON do Commands do resto do arquivo, contando
// chaves e ignorando as que estiverem dentro de string.
func splitCommandsBlock(content string) (rest, jsonPart string) {
	i := indexFold(content, "Commands:")
	if i < 0 {
		return content, ""
	}
	open := strings.Index(content[i:], "{")
	if open < 0 {
		return content, ""
	}
	open += i

	depth, inStr, escaped := 0, false, false
	for j, r := range content[open:] {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inStr:
			escaped = true
		case r == '"':
			inStr = !inStr
		case inStr:
		case r == '{':
			depth++
		case r == '}':
			depth--
			if depth == 0 {
				end := open + j + 1
				return content[:i] + content[end:], content[open:end]
			}
		}
	}
	return content[:i], content[open:]
}

func stripComment(line string) string {
	if i := strings.Index(line, "#"); i >= 0 {
		return line[:i]
	}
	return line
}

func indexFold(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}

func validAppSlot(key string) bool {
	for _, k := range AppSlots {
		if k == key {
			return true
		}
	}
	return false
}

// ApplyUserPrefs joga as preferências do arquivo por cima do que veio do YAML.
func (c *Config) ApplyUserPrefs(p UserPrefs) {
	if c == nil {
		return
	}
	if p.Theme != "" {
		c.UI.Theme = p.Theme
	}
	c.Tools.AI = p.AI
	if p.Commands != nil {
		c.Apps = p.Commands
	}
}

// SetUserConfigTheme troca só a linha do tema, preservando comentários e o
// resto do arquivo — é o que o seletor do Shift+T grava.
func SetUserConfigTheme(content, theme string) string {
	return setUserConfigLine(content, "Theme", theme)
}

// SaveUserTheme grava a escolha do seletor (Shift+T) no user_config.txt,
// preservando comentários e as outras preferências.
func SaveUserTheme(theme string) error {
	_, content, err := EnsureUserConfig(theme, nil)
	if err != nil {
		return err
	}
	return SaveUserConfig(SetUserConfigTheme(content, theme))
}

// LoadUserPrefs lê o arquivo se ele existir. Ausência não é erro: o DevScope
// funciona sem nenhuma preferência declarada.
func LoadUserPrefs() (UserPrefs, []string) {
	b, err := os.ReadFile(UserConfigPath())
	if err != nil {
		return UserPrefs{}, nil
	}
	return ParseUserConfig(string(b))
}

// SaveUserAI grava a escolha do seletor de IA no user_config.txt.
func SaveUserAI(name string) error {
	_, content, err := EnsureUserConfig("", nil)
	if err != nil {
		return err
	}
	return SaveUserConfig(setUserConfigLine(content, "AI", name))
}

// setUserConfigLine troca o valor de uma chave preservando comentários; se a
// chave sumiu do arquivo editado à mão, ela volta no fim.
func setUserConfigLine(content, key, value string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if k, _, ok := strings.Cut(stripComment(line), ":"); ok &&
			strings.EqualFold(strings.TrimSpace(k), key) {
			lines[i] = key + ": " + value
			return strings.Join(lines, "\n")
		}
	}
	return strings.TrimRight(content, "\n") + "\n\n" + key + ": " + value + "\n"
}
