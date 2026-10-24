package devscopeutil

import (
	"regexp"
	"strings"
)

// EnvLooksSecret diz se o nome de uma variável sugere segredo. Erra para o
// lado de mascarar: esconder um valor inofensivo custa menos que vazar um.
func EnvLooksSecret(name string) bool {
	up := strings.ToUpper(name)
	for _, needle := range []string{"SECRET", "PASSWORD", "PASSWD", "TOKEN", "APP_KEY",
		"PRIVATE", "CREDENTIAL", "_KEY", "APIKEY", "API_KEY", "SALT", "DSN"} {
		if strings.Contains(up, needle) {
			return true
		}
	}
	return false
}

// MaskSecret deixa as duas primeiras letras: dá para conferir se a variável
// está preenchida sem revelar o valor.
func MaskSecret(v string) string {
	r := []rune(strings.TrimSpace(v))
	if len(r) <= 2 {
		return strings.Repeat("•", len(r))
	}
	return string(r[:2]) + strings.Repeat("•", min(10, len(r)-2))
}

var (
	// O valor para em espaço, ; , & e aspas: assim um DSN com várias chaves
	// na mesma linha tem cada uma tratada por conta própria.
	maskAssignRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_.-]*)(\s*=\s*)("[^"]*"|'[^']*'|[^\s;&,"']+)`)
	maskJSONRe   = regexp.MustCompile(`"([^"]+)"(\s*:\s*)"((?:[^"\\]|\\.)*)"`)
	maskDSNRe    = regexp.MustCompile(`(?i)\b(password|pwd)=([^;&\s"']+)`)
	maskURLRe    = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://[^:/@\s]+):([^@\s/]+)@`)
	maskBearerRe = regexp.MustCompile(`(?i)\b(bearer\s+)([^\s"',]+)`)
	maskTokenRe  = regexp.MustCompile(`\b(ghp_|gho_|ghu_|ghs_|ghr_|github_pat_|glpat-|sk-|xox[abprs]-)[A-Za-z0-9_-]{8,}|\bAKIA[0-9A-Z]{16}\b`)
)

// MaskLine mascara o que parece segredo numa linha de log, diff ou .env. É o
// filtro de tudo que sai da máquina para o hub: o hub não mascara de novo.
func MaskLine(line string) string {
	line = maskAssignRe.ReplaceAllStringFunc(line, func(m string) string {
		p := maskAssignRe.FindStringSubmatch(m)
		if !EnvLooksSecret(p[1]) || strings.Trim(p[3], `"'`) == "" {
			return m
		}
		return p[1] + p[2] + MaskSecret(strings.Trim(p[3], `"'`))
	})
	line = maskJSONRe.ReplaceAllStringFunc(line, func(m string) string {
		p := maskJSONRe.FindStringSubmatch(m)
		if !EnvLooksSecret(p[1]) || p[3] == "" {
			return m
		}
		return `"` + p[1] + `"` + p[2] + `"` + MaskSecret(p[3]) + `"`
	})
	line = maskDSNRe.ReplaceAllStringFunc(line, func(m string) string {
		p := maskDSNRe.FindStringSubmatch(m)
		if strings.Contains(p[2], "•") { // já mascarado pela regra de CHAVE=valor
			return m
		}
		return p[1] + "=" + MaskSecret(p[2])
	})
	line = maskURLRe.ReplaceAllString(line, "$1:••••@")
	line = maskBearerRe.ReplaceAllStringFunc(line, func(m string) string {
		p := maskBearerRe.FindStringSubmatch(m)
		return p[1] + MaskSecret(p[2])
	})
	return maskTokenRe.ReplaceAllStringFunc(line, func(m string) string {
		return m[:4] + strings.Repeat("•", 8)
	})
}
