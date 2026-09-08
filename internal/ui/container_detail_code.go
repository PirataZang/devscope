package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Realce por tipo de conteúdo da tela de detalhe. Log, JSON, YAML, env e a
// saída do `top` são coisas diferentes — ler tudo em cinza obriga a decodificar
// cada linha com os olhos. O realce faz a estrutura aparecer sozinha.

// highlightDetailLine pinta uma linha conforme a aba aberta.
func highlightDetailLine(tab containerDetailTab, line string) string {
	switch tab {
	case containerDetailTabLogs:
		return highlightLogLine(line)
	case containerDetailTabConfig:
		return highlightYAMLLine(line)
	case containerDetailTabCompose:
		return highlightYAMLLine(line)
	case containerDetailTabEnv:
		return highlightEnvLine(line)
	case containerDetailTabTop:
		return highlightTopLine(line)
	case containerDetailTabFile:
		return highlightDockerfileLine(line)
	default:
		return StyleNormal.Render(line)
	}
}

// highlightLogLine separa o carimbo de tempo, o nível e a mensagem. O nível é
// o que se procura ao varrer log; o carimbo é ruído até você precisar dele.
func highlightLogLine(line string) string {
	rest := line
	var out strings.Builder

	if ts, tail, ok := splitLogTimestamp(rest); ok {
		out.WriteString(StyleMuted.Render(ts))
		rest = tail
	}
	if lvl, tail, style, ok := splitLogLevel(rest); ok {
		out.WriteString(style.Render(lvl))
		rest = tail
	}
	out.WriteString(StyleNormal.Render(rest))
	return out.String()
}

// splitLogTimestamp reconhece o carimbo no começo da linha (RFC3339 do docker
// ou hh:mm:ss). Devolve o carimbo já com o espaço que o segue.
func splitLogTimestamp(s string) (ts, rest string, ok bool) {
	i := strings.IndexByte(s, ' ')
	if i <= 0 {
		return "", s, false
	}
	head := s[:i]
	if !looksLikeTimestamp(head) {
		return "", s, false
	}
	return s[:i+1], s[i+1:], true
}

func looksLikeTimestamp(s string) bool {
	if len(s) < 5 {
		return false
	}
	digits, seps := 0, 0
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == ':' || r == '-' || r == '.' || r == 'T' || r == 'Z' || r == '+':
			seps++
		default:
			return false
		}
	}
	return digits >= 4 && seps >= 2
}

// splitLogLevel reconhece o nível logo após o carimbo, com ou sem colchetes.
func splitLogLevel(s string) (level, rest string, style lipgloss.Style, ok bool) {
	trimmed := strings.TrimLeft(s, " ")
	pad := len(s) - len(trimmed)
	for _, lv := range []struct {
		word  string
		style lipgloss.Style
	}{
		{"ERROR", StyleUnhealthy}, {"ERRO", StyleUnhealthy}, {"FATAL", StyleUnhealthy},
		{"PANIC", StyleUnhealthy}, {"CRITICAL", StyleUnhealthy},
		{"WARN", StyleWarning}, {"WARNING", StyleWarning},
		{"INFO", StyleHealthy}, {"NOTICE", StyleHealthy},
		{"DEBUG", StyleMuted}, {"TRACE", StyleMuted},
	} {
		for _, form := range []string{lv.word, "[" + lv.word + "]"} {
			if len(trimmed) >= len(form) && strings.EqualFold(trimmed[:len(form)], form) {
				return s[:pad+len(form)], trimmed[len(form):], lv.style.Bold(true), true
			}
		}
	}
	return "", s, StyleNormal, false
}

// highlightYAMLLine: chave em destaque, valor normal, marcador de lista e
// comentário apagados — é a estrutura que se procura num compose.
func highlightYAMLLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return StyleMuted.Render(line)
	}

	prefix := ""
	if strings.HasPrefix(trimmed, "- ") {
		prefix = StyleMuted.Render("- ")
		trimmed = trimmed[2:]
	}

	key := lipgloss.NewStyle().Foreground(ColorAccent)
	if i := strings.Index(trimmed, ":"); i > 0 && !strings.HasPrefix(trimmed, "\"") {
		value := trimmed[i+1:]
		return indent + prefix + key.Render(trimmed[:i]) + StyleMuted.Render(":") +
			StyleNormal.Render(value)
	}
	return indent + prefix + StyleNormal.Render(trimmed)
}

// highlightEnvLine mascara o valor de variáveis que parecem segredo. Esta tela
// costuma ser mostrada em call e print — a chave importa, o valor não.
func highlightEnvLine(line string) string {
	i := strings.Index(line, "=")
	if i <= 0 {
		return StyleMuted.Render(line)
	}
	name, value := line[:i], line[i+1:]
	// O alinhamento entra como espaço antes do valor: mascarar sem preservá-lo
	// jogaria justo a linha do segredo para fora da coluna.
	pad := value[:len(value)-len(strings.TrimLeft(value, " "))]
	shown := StyleNormal.Render(value)
	if envLooksSecret(name) && strings.TrimSpace(value) != "" {
		shown = StyleMuted.Render(pad + maskSecret(value))
	}
	return lipgloss.NewStyle().Foreground(ColorAccent).Render(name) +
		StyleMuted.Render("=") + shown
}

func envLooksSecret(name string) bool {
	up := strings.ToUpper(name)
	for _, needle := range []string{"SECRET", "PASSWORD", "PASSWD", "TOKEN", "APP_KEY",
		"PRIVATE", "CREDENTIAL", "_KEY", "APIKEY", "API_KEY", "SALT", "DSN"} {
		if strings.Contains(up, needle) {
			return true
		}
	}
	return false
}

// maskSecret deixa as duas primeiras letras: dá para conferir se a variável
// está preenchida sem revelar o valor.
func maskSecret(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 2 {
		return strings.Repeat("•", len(v))
	}
	return v[:2] + strings.Repeat("•", minInt(10, len(v)-2))
}

// highlightTopLine destaca o cabeçalho e o PID da saída do `docker top`.
func highlightTopLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) > 1 && strings.EqualFold(fields[0], "UID") {
		return StyleMuted.Bold(true).Render(line)
	}
	if len(fields) >= 2 {
		if i := strings.Index(line, fields[1]); i > 0 {
			return StyleMuted.Render(line[:i]) +
				lipgloss.NewStyle().Foreground(ColorAccent).Render(fields[1]) +
				StyleNormal.Render(line[i+len(fields[1]):])
		}
	}
	return StyleNormal.Render(line)
}

// highlightDockerfileLine: a instrução é o que se procura ao passar o olho num
// Dockerfile — o resto é argumento dela.
func highlightDockerfileLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	if trimmed == "" {
		return line
	}
	if strings.HasPrefix(trimmed, "#") {
		return StyleMuted.Render(line)
	}
	word := trimmed
	if i := strings.IndexAny(trimmed, " \t"); i > 0 {
		word = trimmed[:i]
	}
	if !dockerfileInstructions[strings.ToUpper(word)] {
		return StyleNormal.Render(line)
	}
	rest := trimmed[len(word):]
	return indent + lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(word) +
		StyleNormal.Render(rest)
}

var dockerfileInstructions = map[string]bool{
	"FROM": true, "RUN": true, "CMD": true, "LABEL": true, "EXPOSE": true,
	"ENV": true, "ADD": true, "COPY": true, "ENTRYPOINT": true, "VOLUME": true,
	"USER": true, "WORKDIR": true, "ARG": true, "ONBUILD": true, "SHELL": true,
	"STOPSIGNAL": true, "HEALTHCHECK": true, "MAINTAINER": true,
}

// countLogLevels alimenta o título da aba de logs: "12 erros" resolve antes de
// rolar 400 linhas se vale a pena olhar.
func countLogLevels(lines []string) (errN, warnN int) {
	for _, line := range lines {
		switch up := strings.ToUpper(line); {
		case strings.Contains(up, "ERROR"), strings.Contains(up, "[ERRO"),
			strings.Contains(up, "FATAL"), strings.Contains(up, "PANIC"):
			errN++
		case strings.Contains(up, "WARN"):
			warnN++
		}
	}
	return errN, warnN
}
