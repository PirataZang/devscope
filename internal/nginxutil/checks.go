package nginxutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Level diz o peso de uma checagem: Fail bloqueia a publicação, Warn só avisa.
type Level int

const (
	CheckOK Level = iota
	CheckWarn
	CheckFail
)

// Check é uma linha da revisão. File aponta a .inc em conflito, para a tela
// poder abri-la.
type Check struct {
	Level Level
	Text  string
	File  string
}

// Blocked devolve a primeira checagem que impede publicar.
func Blocked(checks []Check) (Check, bool) {
	for _, c := range checks {
		if c.Level == CheckFail {
			return c, true
		}
	}
	return Check{}, false
}

// Checks confere o plano contra o hub e contra o próprio projeto: o que
// impede publicar e o que vai quebrar debaixo de /<projeto>/.
func Checks(p Plan, h Hub) []Check {
	var out []Check
	add := func(l Level, format string, args ...any) {
		out = append(out, Check{Level: l, Text: fmt.Sprintf(format, args...)})
	}
	prefix := "/" + p.Slug + "/"

	if !h.Available() {
		add(CheckFail, "hub não encontrado em %s — ajuste Nginx.dir no user_config.txt", h.SitesDir)
		return out
	}
	if p.Slug == "" {
		add(CheckFail, "slug vazio — e define um")
		return out
	}

	switch {
	case p.Profile.HasAPI() && p.Port == 0 && len(p.PortChoices) > 1:
		add(CheckFail, "%d portas possíveis (%s) — e escolhe", len(p.PortChoices), choiceList(p.PortChoices))
	case p.Profile.HasAPI() && p.Port == 0:
		add(CheckFail, "back-end sem porta: nem BACKEND_PORT no .env, nem ports no serviço backend, nem container %s_backend no ar", p.Slug)
	case p.Profile.HasAPI():
		add(CheckOK, "back-end na porta %d (%s)", p.Port, firstNonEmpty(p.PortFrom, "editada"))
	}

	if p.Profile.HasFront() {
		switch {
		case p.Dist == "" && len(p.DistChoices) > 1:
			add(CheckFail, "%d builds possíveis (%s) — e escolhe", len(p.DistChoices), choiceList(p.DistChoices))
		case p.Dist == "":
			add(CheckFail, "front sem build: nenhum %s com index.html", strings.Join(h.DistCandidates, ", "))
		case p.BuildPending:
			add(CheckWarn, "build pendente: %s/index.html ainda não existe — rode o build do Vite antes do smoke test", p.Dist)
		default:
			add(CheckOK, "front em %s", p.Dist)
		}
		if _, ok := h.ContainerPath(p.Dir); !ok {
			add(CheckFail, "%s está fora de %s — o nginx não enxerga a pasta", p.Dir, h.HostRoot)
		}
	}

	conflicts := hubConflicts(h, p)
	for _, c := range conflicts {
		out = append(out, c)
	}
	if len(conflicts) == 0 {
		add(CheckOK, "nenhuma outra .inc usa %s", prefix)
	}

	if p.Profile.HasFront() {
		out = append(out, viteChecks(p)...)
	}
	if p.Profile.HasAPI() {
		out = append(out, laravelChecks(p, h.Gateway)...)
	}
	return out
}

func choiceList(cs []Choice) string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Value + " " + c.From
	}
	return strings.Join(parts, " · ")
}

// hubConflicts: outra .inc do hub que já declara /<slug>, /<slug>/… ou uma
// das rotas extras. Duas .inc com o mesmo location derrubam o nginx -t; uma
// com o prefixo mais longo rouba as rotas em silêncio.
func hubConflicts(h Hub, p Plan) []Check {
	own := filepath.Base(h.IncPath(p.Slug))
	files, _ := filepath.Glob(filepath.Join(h.SitesDir, "*.inc"))
	var out []Check
	for _, f := range files {
		if filepath.Base(f) == own {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range reLocation.FindAllStringSubmatch(string(b), -1) {
			loc := m[1]
			hit := loc == "/"+p.Slug || strings.HasPrefix(loc, "/"+p.Slug+"/")
			for _, x := range p.Extras {
				hit = hit || normalizeLocationPath(loc) == normalizeLocationPath(x)
			}
			if hit {
				out = append(out, Check{Level: CheckFail, File: f,
					Text: fmt.Sprintf("conflito: sites/%s já declara %s — o abre a existente", filepath.Base(f), loc)})
				break
			}
		}
	}
	return out
}

var (
	reViteBaseLiteral = regexp.MustCompile("base\\s*:\\s*['\"`]([^'\"`]*)['\"`]")
	reViteBaseVar     = regexp.MustCompile(`base\s*:[^,\n]*?(VITE_\w+)`)
	reAssetsAtRoot    = regexp.MustCompile(`(?:src|href)="/assets/`)
)

// viteChecks: o Vite tem de gerar os assets sob /<slug>/ e o front tem de
// chamar a API pelo prefixo — senão a página abre em branco.
func viteChecks(p Plan) []Check {
	var out []Check
	prefix := "/" + p.Slug + "/"
	frontDir := filepath.Join(p.Dir, filepath.Dir(p.Dist))
	env := projectEnv(p.Dir, frontDir)

	if cfg := viteConfig(frontDir); cfg != "" {
		name := filepath.Base(cfg)
		b, _ := os.ReadFile(cfg)
		src := string(b)
		base, from := "", ""
		if m := reViteBaseLiteral.FindStringSubmatch(src); m != nil {
			base, from = m[1], name
		} else if m := reViteBaseVar.FindStringSubmatch(src); m != nil {
			base, from = env[m[1]], m[1]
		}
		switch {
		case from == "":
			out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("%s sem base — assets vão quebrar sob %s", name, prefix)})
		case base == "":
			out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("base do %s vem de %s, que não achei no .env nem no compose", name, from)})
		case strings.TrimSuffix(base, "/")+"/" != prefix:
			out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("base %q (%s) — assets vão quebrar sob %s", base, from, prefix)})
		default:
			out = append(out, Check{Level: CheckOK, Text: fmt.Sprintf("vite base %s (%s)", prefix, from)})
		}
	}

	if v, ok := env["VITE_API_URL"]; ok {
		want := "/" + p.Slug + "/api"
		if strings.HasSuffix(strings.TrimSuffix(v, "/"), want) {
			out = append(out, Check{Level: CheckOK, Text: "VITE_API_URL termina em " + want})
		} else {
			out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("VITE_API_URL=%s não termina em %s", v, want)})
		}
	}

	if b, err := os.ReadFile(filepath.Join(p.Dir, p.Dist, "index.html")); err == nil && reAssetsAtRoot.Match(b) {
		out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("%s/index.html aponta para /assets/ — build feito com base errada, refaça", p.Dist)})
	}
	return out
}

func viteConfig(dir string) string {
	for _, ext := range []string{"ts", "js", "mts", "mjs", "cjs"} {
		if f := filepath.Join(dir, "vite.config."+ext); fileExistsAt(f) {
			return f
		}
	}
	return ""
}

// projectEnv junta o que o build do front enxerga: .env da raiz e do front e o
// environment/args do compose. O mais específico (o do front) ganha.
func projectEnv(dir, frontDir string) map[string]string {
	env := map[string]string{}
	root := readEnvFile(filepath.Join(dir, ".env"))
	if doc, ok := readCompose(dir); ok {
		for _, svc := range doc.Services {
			for k, v := range svc.Environment {
				env[k] = resolveVars(v, root)
			}
			for k, v := range svc.Build.Args {
				env[k] = resolveVars(v, root)
			}
		}
	}
	for _, f := range []string{filepath.Join(dir, ".env"), filepath.Join(frontDir, ".env"), filepath.Join(frontDir, ".env.production")} {
		for k, v := range readEnvFile(f) {
			env[k] = v
		}
	}
	return env
}

var reTrustCall = regexp.MustCompile(`trustProxies\s*\(|\$proxies\s*=`)

// trustsGateway: o Laravel confia no proxy quando a lista do TrustProxies tem
// "*", o IP do gateway ou a faixa privada 172.16.0.0/12 que o contém.
func trustsGateway(src, gateway string) bool {
	for _, loc := range reTrustCall.FindAllStringIndex(src, -1) {
		args := src[loc[1]:min(len(src), loc[1]+400)]
		if strings.Contains(args, "*") || strings.Contains(args, gateway) ||
			(strings.HasPrefix(gateway, "172.") && strings.Contains(args, "172.16.0.0/12")) {
			return true
		}
	}
	return false
}

// laravelChecks: atrás de /<slug>/api/ o Laravel precisa saber do prefixo
// para gerar URL (APP_URL) e confiar no proxy que manda o X-Forwarded-Prefix.
func laravelChecks(p Plan, gateway string) []Check {
	backend := filepath.Join(p.Dir, "backend")
	if !fileExistsAt(filepath.Join(backend, "artisan")) {
		return nil
	}
	var out []Check
	env := readEnvFile(filepath.Join(backend, ".env"))
	appURL := env["APP_URL"]
	if appURL == "" {
		if doc, ok := readCompose(p.Dir); ok {
			appURL = resolveVars(doc.Services["backend"].Environment["APP_URL"], readEnvFile(filepath.Join(p.Dir, ".env")))
		}
	}
	switch {
	case appURL == "":
		out = append(out, Check{Level: CheckWarn, Text: "Laravel sem APP_URL em backend/.env nem no compose"})
	case !strings.Contains(appURL, "/"+p.Slug):
		out = append(out, Check{Level: CheckWarn, Text: fmt.Sprintf("APP_URL=%s não inclui /%s — links e redirects do Laravel saem sem o prefixo", appURL, p.Slug)})
	default:
		out = append(out, Check{Level: CheckOK, Text: "APP_URL inclui /" + p.Slug})
	}

	trusted := false
	for _, f := range []string{"bootstrap/app.php", "app/Http/Middleware/TrustProxies.php"} {
		if b, err := os.ReadFile(filepath.Join(backend, f)); err == nil && trustsGateway(string(b), gateway) {
			trusted = true
		}
	}
	if trusted {
		out = append(out, Check{Level: CheckOK, Text: "TrustProxies confia no gateway " + gateway})
	} else {
		out = append(out, Check{Level: CheckWarn, Text: "TrustProxies não confia em " + gateway + " — o Laravel ignora o X-Forwarded-Prefix"})
	}
	return out
}
