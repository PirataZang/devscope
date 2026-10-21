package nginxutil

// "Publicar projeto": um projeto de ~/projetos vira UMA .inc no hub do
// nginx-main (sites/<projeto>.inc), no formato que já roda em produção —
// redirect sem barra → back-end em /<projeto>/api/ → rotas extras → front.
// O hub, o conf.d/ e as .inc dos outros projetos nunca são tocados.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/devscope/devscope/internal/config"
	"gopkg.in/yaml.v3"
)

// Hub é onde e como o nginx-main roda. Vem do user_config.txt (seção Nginx);
// o que estiver vazio lá é lido do docker-compose.yml e do conf.d/ dele.
type Hub struct {
	Dir            string // pasta do nginx-main no host
	SitesDir       string // pasta das .inc no host (montada em /etc/nginx/sites)
	Container      string
	PublicURL      string // http://178.104.78.64 — base do smoke test
	Gateway        string // 172.17.0.1 — o host visto de dentro do container
	HostRoot       string // ~/projetos no host...
	ContainerRoot  string // ...é /var/www/html dentro do container
	BodySize       string
	ReadTimeout    string
	DistCandidates []string
}

var defaultDistCandidates = []string{"frontend/dist", "dist", "web/dist", "client/dist", "frontend/build", "build"}

// LoadHub resolve o hub: o que a pessoa declarou vale; o resto é detectado.
func LoadHub(p config.NginxPrefs) Hub {
	home, _ := os.UserHomeDir()
	h := Hub{
		Dir:            expandHome(firstNonEmpty(p.Dir, filepath.Join(home, "projetos", "nginx-main")), home),
		Container:      p.Container,
		Gateway:        firstNonEmpty(p.Gateway, "172.17.0.1"),
		HostRoot:       expandHome(p.HostRoot, home),
		ContainerRoot:  p.ContainerRoot,
		BodySize:       firstNonEmpty(p.BodySize, "20m"),
		ReadTimeout:    firstNonEmpty(p.ReadTimeout, "120s"),
		DistCandidates: p.DistCandidates,
	}
	if len(h.DistCandidates) == 0 {
		h.DistCandidates = defaultDistCandidates
	}

	container, mounts := readHubCompose(h.Dir, home)
	include := hubIncludeDir(h.Dir)
	for _, m := range mounts {
		switch {
		case m.target == firstNonEmpty(include, "/etc/nginx/sites"):
			if h.SitesDir == "" {
				h.SitesDir = m.source
			}
		case !strings.HasPrefix(m.target, "/etc/nginx") && h.HostRoot == "" && isDir(m.source):
			h.HostRoot, h.ContainerRoot = m.source, firstNonEmpty(h.ContainerRoot, m.target)
		}
	}
	h.Container = firstNonEmpty(h.Container, container, "nginx-main")
	h.SitesDir = firstNonEmpty(h.SitesDir, filepath.Join(h.Dir, "sites"))
	h.HostRoot = firstNonEmpty(h.HostRoot, filepath.Join(home, "projetos"))
	h.ContainerRoot = firstNonEmpty(h.ContainerRoot, "/var/www/html")

	host := firstNonEmpty(p.PublicHost, hubServerName(h.Dir), "localhost")
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	h.PublicURL = strings.TrimRight(host, "/")
	return h
}

// Available: o hub existe no disco — sem ele não há onde publicar.
func (h Hub) Available() bool { return isDir(h.SitesDir) }

// IncPath é o único arquivo que a publicação de um projeto escreve.
func (h Hub) IncPath(slug string) string { return filepath.Join(h.SitesDir, slug+".inc") }

// ContainerPath traduz um caminho do host para o que o nginx enxerga.
func (h Hub) ContainerPath(hostPath string) (string, bool) {
	rel, err := filepath.Rel(h.HostRoot, hostPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(filepath.Join(h.ContainerRoot, rel)), true
}

type hubMount struct{ source, target string }

// readHubCompose acha o serviço do nginx no compose do hub: o container_name e
// os volumes (host → container).
func readHubCompose(dir, home string) (container string, mounts []hubMount) {
	doc, ok := readCompose(dir)
	if !ok {
		return "", nil
	}
	for _, svc := range doc.Services {
		if !strings.Contains(svc.Image, "nginx") && !strings.Contains(strings.Join(svc.volumeTargets(), " "), "/etc/nginx") {
			continue
		}
		for _, v := range svc.Volumes {
			src, tgt := v.split()
			if src == "" || tgt == "" {
				continue
			}
			src = expandHome(os.ExpandEnv(src), home)
			if !filepath.IsAbs(src) {
				src = filepath.Join(dir, src)
			}
			mounts = append(mounts, hubMount{filepath.Clean(src), strings.TrimRight(tgt, "/")})
		}
		return svc.ContainerName, mounts
	}
	return "", nil
}

var reConfInclude = regexp.MustCompile(`include\s+([^;]+)/\*\.inc\s*;`)

// hubIncludeDir é a pasta que o conf.d/ inclui (/etc/nginx/sites) — é ela que
// diz qual volume do compose é o das .inc.
func hubIncludeDir(dir string) string {
	files, _ := filepath.Glob(filepath.Join(dir, "conf.d", "*.conf"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if m := reConfInclude.FindStringSubmatch(string(b)); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

// hubServerName é o server_name do server{} que inclui as .inc: o endereço
// público onde /<projeto>/ vai responder.
func hubServerName(dir string) string {
	files, _ := filepath.Glob(filepath.Join(dir, "conf.d", "*.conf"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		if !reConfInclude.Match(b) {
			continue
		}
		if m := reServerName.FindStringSubmatch(string(b)); m != nil {
			if names := strings.Fields(m[1]); len(names) > 0 && names[0] != "_" {
				return names[0]
			}
		}
	}
	return ""
}

// ─── compose ────────────────────────────────────────────────────────────────

type composeDoc struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Image         string          `yaml:"image"`
	ContainerName string          `yaml:"container_name"`
	Ports         []composeEntry  `yaml:"ports"`
	Volumes       []composeEntry  `yaml:"volumes"`
	Environment   composeEnv      `yaml:"environment"`
	Build         composeBuildArg `yaml:"build"`
}

// composeEntry aceita a sintaxe curta ("8001:8000") e a longa (mapa).
type composeEntry struct {
	Short string
	Long  map[string]any
}

func (e *composeEntry) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		e.Short = n.Value
		return nil
	}
	return n.Decode(&e.Long)
}

func (e composeEntry) split() (source, target string) {
	if e.Short != "" {
		parts := strings.Split(e.Short, ":")
		if len(parts) < 2 {
			return "", parts[0]
		}
		return parts[0], parts[1]
	}
	return fmt.Sprint(e.Long["source"]), fmt.Sprint(e.Long["target"])
}

func (s composeService) volumeTargets() []string {
	var out []string
	for _, v := range s.Volumes {
		_, t := v.split()
		out = append(out, t)
	}
	return out
}

// composeEnv aceita environment como mapa ou lista "K=V".
type composeEnv map[string]string

func (e *composeEnv) UnmarshalYAML(n *yaml.Node) error {
	*e = composeEnv{}
	if n.Kind == yaml.SequenceNode {
		var list []string
		if err := n.Decode(&list); err != nil {
			return err
		}
		for _, kv := range list {
			k, v, _ := strings.Cut(kv, "=")
			(*e)[k] = v
		}
		return nil
	}
	var m map[string]any
	if err := n.Decode(&m); err != nil {
		return err
	}
	for k, v := range m {
		if v != nil {
			(*e)[k] = fmt.Sprint(v)
		}
	}
	return nil
}

// composeBuildArg: build pode ser só o contexto ("./frontend") ou um mapa com args.
type composeBuildArg struct{ Args composeEnv }

func (b *composeBuildArg) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	var raw struct {
		Args composeEnv `yaml:"args"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	b.Args = raw.Args
	return nil
}

func readCompose(dir string) (composeDoc, bool) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var doc composeDoc
		if yaml.Unmarshal(b, &doc) == nil {
			return doc, true
		}
	}
	return composeDoc{}, false
}

var reComposeVar = regexp.MustCompile(`\$\{(\w+)(?::?-([^}]*))?\}|\$(\w+)`)

// resolveVars troca ${VAR:-padrão}/${VAR}/$VAR pelo .env, como o compose faz.
func resolveVars(s string, env map[string]string) string {
	return reComposeVar.ReplaceAllStringFunc(s, func(m string) string {
		g := reComposeVar.FindStringSubmatch(m)
		name := firstNonEmpty(g[1], g[3])
		if v, ok := env[name]; ok && v != "" {
			return v
		}
		if v := os.Getenv(name); v != "" {
			return v
		}
		return g[2]
	})
}

// readEnvFile lê KEY=valor, ignorando comentário e aspas.
func readEnvFile(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		} else if i := strings.Index(v, " #"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		out[strings.TrimSpace(k)] = v
	}
	return out
}

// ─── o plano e a .inc ───────────────────────────────────────────────────────

// Profile é o que a .inc publica.
type Profile string

const (
	ProfileSaaS  Profile = "saas"  // redirect + api + front
	ProfileFront Profile = "front" // redirect + front
	ProfileAPI   Profile = "api"   // redirect + api
)

// Profiles na ordem do seletor.
var Profiles = []Profile{ProfileSaaS, ProfileFront, ProfileAPI}

func (p Profile) HasAPI() bool   { return p == ProfileSaaS || p == ProfileAPI }
func (p Profile) HasFront() bool { return p == ProfileSaaS || p == ProfileFront }

// Choice é um valor candidato e de onde ele veio ("8001 · .env BACKEND_PORT").
type Choice struct {
	Value string
	From  string
}

// Plan é tudo que a .inc precisa. Sai da detecção e pode ser editado na tela.
type Plan struct {
	Slug         string
	Dir          string // pasta do projeto no host
	Profile      Profile
	Port         int
	PortFrom     string
	Dist         string // relativo a Dir: "frontend/dist"
	DistFrom     string
	BuildPending bool
	BodySize     string
	ReadTimeout  string
	Extras       []string // caminhos fora do prefixo que redirecionam para dentro dele: "/portal/"
	PortChoices  []Choice // mais de uma = ambíguo, a tela pede para escolher
	DistChoices  []Choice
}

// URL pública do projeto.
func (p Plan) URL(h Hub) string { return h.PublicURL + "/" + p.Slug + "/" }

// Render gera a .inc no formato canônico, byte a byte.
func Render(p Plan, h Hub) string {
	s := p.Slug
	blocks := []string{fmt.Sprintf(`# /%[1]s -> /%[1]s/
location = /%[1]s {
    return 301 /%[1]s/;
}
`, s)}
	if p.Profile.HasAPI() {
		blocks = append(blocks, fmt.Sprintf(`# Back-end (container %[1]s_backend publicado na porta %[2]d do host)
location ^~ /%[1]s/api/ {
    rewrite ^/%[1]s/(.*)$ /$1 break;
    proxy_pass http://%[3]s:%[2]d;

    proxy_http_version 1.1;
    proxy_set_header Host              $host;
    proxy_set_header X-Real-IP         $remote_addr;
    proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header X-Forwarded-Prefix /%[1]s;

    client_max_body_size %[4]s;
    proxy_read_timeout %[5]s;
}
`, s, p.Port, h.Gateway, firstNonEmpty(p.BodySize, h.BodySize), firstNonEmpty(p.ReadTimeout, h.ReadTimeout)))
	}
	for _, x := range p.Extras {
		x = normalizeLocationPath(x)
		blocks = append(blocks, fmt.Sprintf(`# %[2]s -> /%[1]s%[2]s
location ^~ %[2]s {
    return 302 /%[1]s$request_uri;
}
`, s, x))
	}
	if p.Profile.HasFront() {
		alias, _ := h.ContainerPath(filepath.Join(p.Dir, p.Dist))
		blocks = append(blocks, fmt.Sprintf(`# Front-end (build do Vite)
location ^~ /%[1]s/ {
    alias %[2]s/;
    index index.html;
    try_files $uri $uri/ /%[1]s/index.html;
}
`, s, alias))
	}
	return strings.Join(blocks, "\n")
}

var (
	reRedirect301 = regexp.MustCompile(`(?m)^location = /([^\s{]+) \{`)
	reGatewayPort = regexp.MustCompile(`(?m)^\s*proxy_pass\s+https?://[^:/]+:(\d+)\s*;`)
	reBodySize    = regexp.MustCompile(`(?m)^\s*client_max_body_size\s+([^;]+);`)
	reReadTimeout = regexp.MustCompile(`(?m)^\s*proxy_read_timeout\s+([^;]+);`)
	reExtra       = regexp.MustCompile(`(?m)^location \^~ (\S+) \{\n\s*return 302 /`)
)

// ReadPlan lê de volta uma .inc publicada: é o que mantém, numa republicação,
// a rota extra e os limites que alguém ajustou à mão.
func ReadPlan(raw, dir string, h Hub) Plan {
	inc := parseInc("", raw)
	p := Plan{Dir: dir}
	if m := reRedirect301.FindStringSubmatch(raw); m != nil {
		p.Slug = m[1]
	}
	if m := reGatewayPort.FindStringSubmatch(raw); m != nil {
		p.Port, _ = strconv.Atoi(m[1])
	}
	if m := reBodySize.FindStringSubmatch(raw); m != nil {
		p.BodySize = strings.TrimSpace(m[1])
	}
	if m := reReadTimeout.FindStringSubmatch(raw); m != nil {
		p.ReadTimeout = strings.TrimSpace(m[1])
	}
	for _, m := range reExtra.FindAllStringSubmatch(raw, -1) {
		p.Extras = append(p.Extras, m[1])
	}
	if m := reAlias.FindStringSubmatch(raw); m != nil {
		alias := strings.TrimSuffix(strings.TrimSpace(m[1]), "/")
		if base, ok := h.ContainerPath(dir); ok {
			p.Dist = strings.TrimPrefix(strings.TrimPrefix(alias, base), "/")
		}
	}
	hasAPI, hasFront := inc.ProxyPass != "", inc.Root != ""
	switch {
	case hasAPI && hasFront:
		p.Profile = ProfileSaaS
	case hasFront:
		p.Profile = ProfileFront
	default:
		p.Profile = ProfileAPI
	}
	return p
}

// ─── detecção ───────────────────────────────────────────────────────────────

// Slug é o prefixo público que a pasta ganha: /<slug>/.
func Slug(dir string) string { return sanitizeName(filepath.Base(filepath.Clean(dir))) }

// DetectProject monta o plano a partir da pasta do projeto e roda as checagens.
func DetectProject(dir string, h Hub) (Plan, []Check) {
	p := DetectPlan(dir, h)
	return p, Checks(p, h)
}

// DetectPlan descobre slug, porta, dist e perfil sem perguntar nada. Quando há
// mais de um candidato, deixa o campo vazio e guarda as opções.
func DetectPlan(dir string, h Hub) Plan {
	dir = filepath.Clean(dir)
	p := Plan{Slug: Slug(dir), Dir: dir, BodySize: h.BodySize, ReadTimeout: h.ReadTimeout}

	p.PortChoices = detectPorts(dir, p.Slug)
	if len(p.PortChoices) == 1 {
		p.Port, _ = strconv.Atoi(p.PortChoices[0].Value)
		p.PortFrom = p.PortChoices[0].From
	}

	for _, c := range h.DistCandidates {
		if fileExistsAt(filepath.Join(dir, c, "index.html")) {
			p.DistChoices = append(p.DistChoices, Choice{c, "index.html"})
		}
	}
	switch {
	case len(p.DistChoices) == 1:
		p.Dist, p.DistFrom = p.DistChoices[0].Value, "index.html"
	case len(p.DistChoices) == 0:
		if front := viteDir(dir); front != "" {
			p.Dist, p.DistFrom, p.BuildPending = filepath.ToSlash(filepath.Join(front, "dist")), "build pendente", true
		}
	}

	hasAPI := len(p.PortChoices) > 0
	hasFront := p.Dist != "" || len(p.DistChoices) > 1
	switch {
	case hasAPI && hasFront:
		p.Profile = ProfileSaaS
	case hasFront:
		p.Profile = ProfileFront
	default:
		p.Profile = ProfileAPI
	}

	// Republicar não pode perder o que foi ajustado na .inc em produção.
	if raw, err := os.ReadFile(h.IncPath(p.Slug)); err == nil {
		old := ReadPlan(string(raw), dir, h)
		p.Extras = old.Extras
		p.BodySize = firstNonEmpty(old.BodySize, p.BodySize)
		p.ReadTimeout = firstNonEmpty(old.ReadTimeout, p.ReadTimeout)
	}
	return p
}

// detectPorts: BACKEND_PORT no .env → ports do serviço backend no compose →
// container <slug>_backend no ar. A primeira fonte que responder vale.
func detectPorts(dir, slug string) []Choice {
	env := readEnvFile(filepath.Join(dir, ".env"))
	if v := env["BACKEND_PORT"]; v != "" {
		if _, err := strconv.Atoi(v); err == nil {
			return []Choice{{v, ".env BACKEND_PORT"}}
		}
	}
	if doc, ok := readCompose(dir); ok {
		if svc, ok := doc.Services["backend"]; ok {
			var out []Choice
			for _, e := range svc.Ports {
				if port := hostPort(e, env); port != "" {
					out = appendChoice(out, Choice{port, "compose backend"})
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return dockerBackendPorts(slug)
}

// hostPort tira a porta do host de "8001:8000", "127.0.0.1:8001:8000",
// "${BACKEND_PORT:-8000}:8000" ou da sintaxe longa. Só a porta do container
// ("8000") não publica nada no host.
func hostPort(e composeEntry, env map[string]string) string {
	if e.Short == "" {
		if v, ok := e.Long["published"]; ok {
			return resolveVars(fmt.Sprint(v), env)
		}
		return ""
	}
	parts := strings.Split(resolveVars(e.Short, env), ":")
	if len(parts) < 2 {
		return ""
	}
	port := parts[len(parts)-2]
	if _, err := strconv.Atoi(port); err != nil {
		return ""
	}
	return port
}

var reDockerPublished = regexp.MustCompile(`:(\d+)->`)

// dockerPS é trocado nos testes.
var dockerPS = func() string {
	out, _ := runCmd("docker", "ps", "--format", "{{.Names}}\t{{.Ports}}")
	return out
}

func dockerBackendPorts(slug string) []Choice {
	var out []Choice
	for _, line := range strings.Split(dockerPS(), "\n") {
		name, ports, ok := strings.Cut(line, "\t")
		if !ok || !(strings.HasPrefix(name, slug+"_backend") || strings.HasPrefix(name, slug+"-backend")) {
			continue
		}
		for _, m := range reDockerPublished.FindAllStringSubmatch(ports, -1) {
			out = appendChoice(out, Choice{m[1], "docker " + name})
		}
	}
	return out
}

func appendChoice(list []Choice, c Choice) []Choice {
	for _, x := range list {
		if x.Value == c.Value {
			return list
		}
	}
	return append(list, c)
}

// viteDir é a pasta (relativa) do front com Vite: a raiz ou uma subpasta comum.
func viteDir(dir string) string {
	for _, sub := range []string{"frontend", ".", "web", "client"} {
		b, err := os.ReadFile(filepath.Join(dir, sub, "package.json"))
		if err == nil && strings.Contains(string(b), `"vite"`) {
			return sub
		}
	}
	return ""
}

func expandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func fileExistsAt(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ─── edição ─────────────────────────────────────────────────────────────────

// PlanFields são os campos editáveis, na ordem da tela. A mesma Set serve a
// edição inline da TUI e as flags do devscope nginx publish.
var PlanFields = []string{"slug", "perfil", "porta", "dist", "body", "timeout", "extras"}

// Get devolve o campo como texto editável.
func (p Plan) Get(field string) string {
	switch field {
	case "slug":
		return p.Slug
	case "perfil":
		return string(p.Profile)
	case "porta":
		if p.Port == 0 {
			return ""
		}
		return strconv.Itoa(p.Port)
	case "dist":
		return p.Dist
	case "body":
		return p.BodySize
	case "timeout":
		return p.ReadTimeout
	case "extras":
		return strings.Join(p.Extras, " ")
	}
	return ""
}

// Set aplica um campo editado. Valor inválido não mexe no plano.
func (p *Plan) Set(field, value string) error {
	value = strings.TrimSpace(value)
	switch field {
	case "slug":
		s := sanitizeName(value)
		if s == "" {
			return fmt.Errorf("slug vazio")
		}
		p.Slug = s
	case "perfil":
		for _, pr := range Profiles {
			if string(pr) == value {
				p.Profile = pr
				return nil
			}
		}
		return fmt.Errorf("perfil %q: use saas, front ou api", value)
	case "porta":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 || n > 65535 {
			return fmt.Errorf("porta inválida: %q", value)
		}
		p.Port, p.PortFrom = n, "editada"
	case "dist":
		p.Dist, p.DistFrom = strings.Trim(filepath.ToSlash(value), "/"), "editado"
		p.BuildPending = p.Dist != "" && !fileExistsAt(filepath.Join(p.Dir, p.Dist, "index.html"))
	case "body":
		p.BodySize = value
	case "timeout":
		p.ReadTimeout = value
	case "extras":
		p.Extras = nil
		for _, x := range strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == ',' }) {
			p.Extras = append(p.Extras, normalizeLocationPath(x))
		}
	default:
		return fmt.Errorf("campo desconhecido: %s", field)
	}
	return nil
}
