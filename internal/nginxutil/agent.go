// Package nginxutil detecta e edita configs de nginx dentro de um projeto
// devscope: um main.conf/nginx.conf na raiz que inclui uma pasta com um
// arquivo .conf por entrada (single ou hub). A pasta pode ter qualquer nome —
// não tem lista fixa ("sites", "conf.d" etc): a gente lê o include de
// verdade no main.conf pra achar ela, e só cai pra uma varredura por
// conteúdo se não achar include nenhum.
//
// Cada .conf de nível 1 é ou "single" (um server{} completo, com proxy_pass
// ou root — uma rota que fecha sozinha) ou "hub" (um server{} de verdade,
// com seu próprio listen/server_name, que só inclui uma pasta de .inc). Cada
// .inc dentro de um hub não é outro server{} — é um ou mais location{} que
// entram no server{} do hub, o jeito certo de hospedar vários caminhos
// (/portfolio/, /blog/, ...) sob o mesmo domínio/porta. Não existe um JSON
// de config próprio: os .conf/.inc já são a fonte da verdade, igual o tab de
// git lê o .git direto.
package nginxutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// mainConfNames são os nomes mais comuns do arquivo raiz que agrega os .conf.
var mainConfNames = []string{"main.conf", "nginx.conf"}

// skipDirs não entram na varredura por conteúdo — nunca é onde ficam rotas.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, ".devscope": true}

// Kind identifica o que um .conf de nível 1 é.
type Kind string

const (
	KindSingle Kind = "single"
	KindHub    Kind = "hub"
)

// Layout é onde a config de nginx deste projeto mora no disco.
type Layout struct {
	MainConf string // caminho absoluto, "" se não achou
	SitesDir string // caminho absoluto da pasta de .conf de nível 1, "" se não achou
	Ext      string // extensão dos arquivos de nível 1 (normalmente .conf)
}

// Site é um arquivo já parseado (best-effort, via regex — não é um parser
// completo da gramática do nginx). Pode ser uma entrada de nível 1 (Kind
// preenchido: single ou hub) ou uma .inc dentro de um hub (Kind vazio — um
// location{}, não um server{}).
type Site struct {
	File        string // relativo à raiz do projeto
	Name        string // nome do arquivo sem extensão
	Kind        Kind   // "single" | "hub" — só em entradas de nível 1
	HubDir      string // caminho absoluto da pasta de .inc — só quando Kind == KindHub
	ServerNames []string
	Listen      string
	SSL         bool
	Location    string // path do location{} — só em .inc de nível 2
	ProxyPass   string
	Root        string // de "root" ou "alias"
	Raw         string
	Project     string // preenchido pela UI quando o item vem de outro projeto
}

// NewSite são os campos de um .conf single de nível 1 — um server{} completo.
type NewSite struct {
	Name       string // nome do arquivo (sem extensão)
	ServerName string // domínio(s), separados por espaço
	Target     string // proxy_pass — se vazio, usa Root (site estático)
	Root       string
	Port       int
	SSL        bool
}

// NewConf são os campos do modal de criação de nível 1: nome + kind, e só os
// campos do kind escolhido. single usa os mesmos campos de NewSite. hub usa
// ServerName/Port/SSL (é um server{} de verdade) mais o nome da pasta que
// vai guardar as .inc — Target/Root de NewSite não se aplicam a um hub.
type NewConf struct {
	Name string
	Kind Kind
	NewSite
	HubDirName string
}

// NewLocation são os campos de uma .inc dentro de um hub: um location{} (ou
// par redirect+location, se for pasta estática), não um server{} novo — o
// domínio/porta já vêm do hub.
type NewLocation struct {
	Path   string // ex: /portfolio — normalizado com barra no início e no fim
	Label  string // comentário opcional no topo do arquivo, ex: "Astro Portfolio"
	Target string // proxy_pass — porta ("3000") ou URL; se vazio, usa Dist
	Dist   string // pasta estática (alias) — ex: /var/www/html/portfolio/dist
}

// Detect é uma checagem barata pra landing screen: o projeto tem cara de
// nginx? (main.conf/nginx.conf na raiz e/ou pasta de .conf de nível 1)
func Detect(projectPath string) bool {
	l, _ := findLayout(projectPath)
	return l.MainConf != "" || l.SitesDir != ""
}

func findLayout(projectPath string) (Layout, error) {
	var l Layout
	for _, name := range mainConfNames {
		p := filepath.Join(projectPath, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			l.MainConf = p
			break
		}
	}
	if l.MainConf == "" {
		l.MainConf = findAnyRootConf(projectPath)
	}
	if l.MainConf != "" {
		l.SitesDir, l.Ext = includedDir(projectPath, l.MainConf)
	}
	if l.SitesDir == "" {
		l.SitesDir, l.Ext = scanForSitesDir(projectPath)
	}
	if l.MainConf == "" && l.SitesDir == "" {
		return l, fmt.Errorf("nenhuma config de nginx encontrada em %s", projectPath)
	}
	return l, nil
}

// findAnyRootConf pega um .conf na raiz do projeto quando não existe
// main.conf/nginx.conf — qualquer um com cara de config de nginx.
func findAnyRootConf(projectPath string) string {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".conf" {
			continue
		}
		full := filepath.Join(projectPath, e.Name())
		b, err := os.ReadFile(full)
		if err == nil && looksLikeNginxSite(b) {
			return full
		}
	}
	return ""
}

var reInclude = regexp.MustCompile(`(?m)^\s*include\s+([^;]+);`)

// includedDir lê o(s) include de um arquivo pra achar a pasta de nível 1 de
// verdade, seja qual for o nome dela.
func includedDir(projectPath, confFile string) (dir, ext string) {
	b, err := os.ReadFile(confFile)
	if err != nil {
		return "", ""
	}
	for _, m := range reInclude.FindAllSubmatch(b, -1) {
		if dir, ext := resolveIncludeDir([]string{projectPath}, string(m[1]), ".conf"); dir != "" {
			return dir, ext
		}
	}
	return "", ""
}

// resolveIncludeDir tenta achar a pasta de um include em cada uma das bases,
// nessa ordem — a de um hub, por exemplo, pode estar dentro da pasta de
// nível 1 (convenção de quem o devscope cria) ou ao lado dela, direto na
// raiz do projeto (setup feito à mão, main.conf e sites/ irmãos). Se o
// include usa o caminho de dentro do container (ex: /etc/nginx/sites/*.inc),
// tenta também só o nome final da pasta — é o que o volume do docker
// costuma espelhar.
func resolveIncludeDir(bases []string, rawInclude, defaultExt string) (dir, ext string) {
	raw := strings.Trim(strings.TrimSpace(rawInclude), `"'`)
	globDir := filepath.Dir(raw)
	if globDir == "." || globDir == "/" || globDir == "" {
		return "", "" // include de um arquivo específico, não de uma pasta
	}
	globExt := filepath.Ext(filepath.Base(raw))
	if globExt == ".*" {
		globExt = ""
	}
	var candidates []string
	for _, base := range bases {
		candidates = append(candidates, filepath.Join(base, globDir), filepath.Join(base, filepath.Base(globDir)))
	}
	for _, full := range candidates {
		st, err := os.Stat(full)
		if err != nil || !st.IsDir() {
			continue
		}
		e := globExt
		if e == "" {
			entries, _ := os.ReadDir(full)
			e = firstNonEmpty(dominantExt(entries), defaultExt)
		}
		return full, e
	}
	return "", ""
}

// scanForSitesDir é o último recurso quando não há main.conf ou nenhum
// include aponta pra uma pasta que exista: olha as subpastas de primeiro
// nível e escolhe a que tem mais arquivos .inc/.conf com cara de server{}.
func scanForSitesDir(projectPath string) (dir, ext string) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", ""
	}
	bestDir, bestExt, bestCount := "", "", 0
	for _, e := range entries {
		if !e.IsDir() || skipDirs[e.Name()] {
			continue
		}
		full := filepath.Join(projectPath, e.Name())
		sub, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		count, ext := 0, ""
		for _, f := range sub {
			fx := filepath.Ext(f.Name())
			if f.IsDir() || (fx != ".inc" && fx != ".conf") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(full, f.Name()))
			if err != nil || !looksLikeNginxSite(b) {
				continue
			}
			count++
			if ext == "" {
				ext = fx
			}
		}
		if count > bestCount {
			bestDir, bestExt, bestCount = full, ext, count
		}
	}
	return bestDir, bestExt
}

func looksLikeNginxSite(b []byte) bool {
	return bytes.Contains(b, []byte("server_name")) || bytes.Contains(b, []byte("proxy_pass")) || bytes.Contains(b, []byte("listen"))
}

func dominantExt(entries []os.DirEntry) string {
	counts := map[string]int{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".inc" || ext == ".conf" {
			counts[ext]++
		}
	}
	best := ""
	for ext, n := range counts {
		if best == "" || n > counts[best] {
			best = ext
		}
	}
	return best
}

// Discover acha o layout e parseia cada .conf de nível 1 (single ou hub).
func Discover(projectPath string) (Layout, []Site, error) {
	layout, err := findLayout(projectPath)
	if err != nil {
		return layout, nil, err
	}
	var entries []Site
	if layout.SitesDir != "" {
		des, err := os.ReadDir(layout.SitesDir)
		if err == nil {
			for _, e := range des {
				if e.IsDir() || filepath.Ext(e.Name()) != layout.Ext {
					continue
				}
				full := filepath.Join(layout.SitesDir, e.Name())
				b, err := os.ReadFile(full)
				if err != nil {
					continue
				}
				rel, _ := filepath.Rel(projectPath, full)
				entries = append(entries, parseTopEntry(rel, string(b), projectPath, layout.SitesDir))
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return layout, entries, nil
}

// DiscoverHubIncs lista as .inc (location{}) de dentro da pasta de um hub.
func DiscoverHubIncs(projectPath string, hub Site) ([]Site, error) {
	if hub.Kind != KindHub || hub.HubDir == "" {
		return nil, fmt.Errorf("%s não é um hub", hub.Name)
	}
	des, err := os.ReadDir(hub.HubDir)
	if err != nil {
		return nil, err
	}
	var out []Site
	for _, e := range des {
		if e.IsDir() || filepath.Ext(e.Name()) != ".inc" {
			continue
		}
		full := filepath.Join(hub.HubDir, e.Name())
		b, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(projectPath, full)
		out = append(out, parseInc(rel, string(b)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

var (
	reServerName = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	reListen     = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	reProxyPass  = regexp.MustCompile(`(?m)^\s*proxy_pass\s+([^;]+);`)
	reRoot       = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)
	reAlias      = regexp.MustCompile(`(?m)^\s*alias\s+([^;]+);`)
	reLocation   = regexp.MustCompile(`(?m)^\s*location\s+(?:=|~\*|~|\^~)?\s*([^\s{]+)\s*\{`)
)

// parseTopEntry decide se um .conf de nível 1 é hub (tem um include que
// resolve pra uma pasta de verdade) ou single, e parseia de acordo. Um hub
// hoje em dia é um server{} de verdade — então tenta o parse de single
// primeiro (pega server_name/listen/ssl) e só depois checa o include.
func parseTopEntry(rel, raw, projectPath, sitesDir string) Site {
	s := parseSingle(rel, raw)
	if m := reInclude.FindStringSubmatch(raw); m != nil {
		if dir, _ := resolveIncludeDir([]string{sitesDir, projectPath}, m[1], ".inc"); dir != "" {
			s.Kind = KindHub
			s.HubDir = dir
			return s
		}
	}
	s.Kind = KindSingle
	return s
}

func parseSingle(rel, raw string) Site {
	s := Site{File: rel, Name: strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)), Raw: raw}
	if m := reServerName.FindStringSubmatch(raw); m != nil {
		s.ServerNames = strings.Fields(m[1])
	}
	if ms := reListen.FindAllStringSubmatch(raw, -1); ms != nil {
		parts := make([]string, 0, len(ms))
		for _, m := range ms {
			v := strings.TrimSpace(m[1])
			parts = append(parts, v)
			if strings.Contains(v, "ssl") {
				s.SSL = true
			}
		}
		s.Listen = strings.Join(parts, ", ")
	}
	if m := reProxyPass.FindStringSubmatch(raw); m != nil {
		s.ProxyPass = strings.TrimSpace(m[1])
	}
	if m := reRoot.FindStringSubmatch(raw); m != nil {
		s.Root = strings.TrimSpace(m[1])
	}
	return s
}

// parseInc parseia uma .inc de dentro de um hub: um ou mais location{}, não
// um server{}. Pega o path do location mais específico (o mais longo — o
// redirect "location = /x" perde pro "location ^~ /x/" de verdade).
func parseInc(rel, raw string) Site {
	s := Site{File: rel, Name: strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)), Raw: raw}
	for _, m := range reLocation.FindAllStringSubmatch(raw, -1) {
		if len(m[1]) > len(s.Location) {
			s.Location = m[1]
		}
	}
	if m := reProxyPass.FindStringSubmatch(raw); m != nil {
		s.ProxyPass = strings.TrimSpace(m[1])
	}
	if m := reRoot.FindStringSubmatch(raw); m != nil {
		s.Root = strings.TrimSpace(m[1])
	} else if m := reAlias.FindStringSubmatch(raw); m != nil {
		s.Root = strings.TrimSpace(m[1])
	}
	return s
}

// CreateConf escreve um novo .conf de nível 1 — single (rota completa) ou hub
// (server{} + pasta nova de .inc) — e garante que o main.conf inclui a pasta
// de nível 1.
func CreateConf(projectPath string, n NewConf) (Site, error) {
	layout, err := findLayout(projectPath)
	if err != nil {
		return Site{}, err
	}
	if layout.SitesDir == "" {
		return Site{}, fmt.Errorf("pasta de confs não encontrada — crie um .conf de exemplo e um include no main.conf apontando pra ela")
	}
	name := sanitizeName(n.Name)
	if name == "" {
		return Site{}, fmt.Errorf("nome vazio")
	}
	ext := firstNonEmpty(layout.Ext, ".conf")
	dest := filepath.Join(layout.SitesDir, name+ext)
	if _, err := os.Stat(dest); err == nil {
		return Site{}, fmt.Errorf("%s já existe", name+ext)
	}
	if strings.TrimSpace(n.ServerName) == "" {
		return Site{}, fmt.Errorf("server_name vazio")
	}
	if n.Port <= 0 {
		n.Port = 80
		if n.SSL {
			n.Port = 443
		}
	}

	var content string
	var site Site
	if n.Kind == KindHub {
		hubName := sanitizeName(n.HubDirName)
		if hubName == "" {
			return Site{}, fmt.Errorf("nome da pasta vazio")
		}
		hubDir := filepath.Join(layout.SitesDir, hubName)
		if _, err := os.Stat(hubDir); err == nil {
			return Site{}, fmt.Errorf("pasta %s já existe", hubName)
		}
		if err := os.MkdirAll(hubDir, 0o755); err != nil {
			return Site{}, err
		}
		content = renderHub(n, hubName)
		site = parseSingle("", content)
		site.Kind = KindHub
		site.HubDir = hubDir
	} else {
		if strings.TrimSpace(n.Target) == "" && strings.TrimSpace(n.Root) == "" {
			return Site{}, fmt.Errorf("informe um destino: proxy_pass ou root")
		}
		content = renderSite(n.NewSite)
		site = parseSingle("", content)
		site.Kind = KindSingle
	}
	site.Name = name

	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return Site{}, err
	}
	if layout.MainConf != "" {
		// ponytail: best-effort — se o main.conf já faz include glob da pasta
		// (o caso comum), não mexe em nada; senão, só acrescenta um include
		// solto no fim do arquivo, fora de qualquer bloco. Funciona pro
		// layout descrito (main.conf com include glob), mas em layouts
		// incomuns pode cair fora do bloco http/server certo — revisar
		// main.conf à mão nesse caso.
		_ = ensureInclude(layout.MainConf, layout.SitesDir, ext)
	}
	rel, _ := filepath.Rel(projectPath, dest)
	site.File = rel
	return site, nil
}

// CreateInc escreve uma nova .inc (location{}) dentro da pasta de um hub. O
// hub já nasce com o include glob pra pasta, então não precisa mexer em nada
// além de escrever o arquivo.
func CreateInc(projectPath string, hub Site, n NewLocation) (Site, error) {
	if hub.Kind != KindHub || hub.HubDir == "" {
		return Site{}, fmt.Errorf("%s não é um hub", hub.Name)
	}
	if strings.TrimSpace(n.Path) == "" {
		return Site{}, fmt.Errorf("path vazio")
	}
	if strings.TrimSpace(n.Target) == "" && strings.TrimSpace(n.Dist) == "" {
		return Site{}, fmt.Errorf("informe um destino: porta/proxy_pass ou dist")
	}
	name := sanitizeName(firstNonEmpty(n.Label, n.Path))
	if name == "" {
		return Site{}, fmt.Errorf("nome vazio")
	}
	dest := filepath.Join(hub.HubDir, name+".inc")
	if _, err := os.Stat(dest); err == nil {
		return Site{}, fmt.Errorf("%s já existe", name+".inc")
	}
	content := renderLocation(n)
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return Site{}, err
	}
	rel, _ := filepath.Rel(projectPath, dest)
	return parseInc(rel, content), nil
}

// DeleteSite remove um arquivo (.conf de nível 1 ou .inc de um hub). file é
// relativo à raiz do projeto. Deletar um hub não apaga a pasta das .inc dele.
func DeleteSite(projectPath, file string) error {
	full := filepath.Clean(filepath.Join(projectPath, file))
	root := filepath.Clean(projectPath)
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return fmt.Errorf("caminho inválido")
	}
	return os.Remove(full)
}

func ensureInclude(mainConf, sitesDir, ext string) error {
	b, err := os.ReadFile(mainConf)
	if err != nil {
		return err
	}
	content := string(b)
	dirName := filepath.Base(sitesDir)
	glob := dirName + "/*" + ext
	if strings.Contains(content, glob) {
		return nil
	}
	line := fmt.Sprintf("\ninclude %s;\n", glob)
	return os.WriteFile(mainConf, append(b, []byte(line)...), 0o644)
}

func renderSite(n NewSite) string {
	listen := fmt.Sprintf("%d", n.Port)
	if n.SSL {
		listen += " ssl"
	}
	var body string
	if strings.TrimSpace(n.Target) != "" {
		body = fmt.Sprintf(`    location / {
        proxy_pass %s;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }`, normalizeTarget(n.Target))
	} else {
		body = fmt.Sprintf(`    root %s;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }`, strings.TrimSpace(n.Root))
	}
	return fmt.Sprintf(`server {
    listen %s;
    server_name %s;
%s
%s
}
`, listen, strings.TrimSpace(n.ServerName), sslCertLines(n.SSL, n.ServerName), body)
}

// renderHub monta o server{} do hub: só o listen/server_name (+ ssl) mais o
// include pra pasta das .inc — o corpo de verdade vem de lá.
func renderHub(n NewConf, hubDirName string) string {
	listen := fmt.Sprintf("%d", n.Port)
	if n.SSL {
		listen += " ssl"
	}
	return fmt.Sprintf(`server {
    listen %s;
    server_name %s;
%s
    include %s/*.inc;
}
`, listen, strings.TrimSpace(n.ServerName), sslCertLines(n.SSL, n.ServerName), hubDirName)
}

// renderLocation monta uma .inc: um location{} de proxy, ou o par
// redirect+location de site estático (canonicaliza a barra final, do jeito
// que o Astro/Vite/etc esperam no build).
func renderLocation(n NewLocation) string {
	path := normalizeLocationPath(n.Path)
	bare := strings.TrimSuffix(path, "/")
	header := ""
	if strings.TrimSpace(n.Label) != "" {
		header = fmt.Sprintf("# %s\n#\n", strings.TrimSpace(n.Label))
	}
	if strings.TrimSpace(n.Target) != "" {
		return fmt.Sprintf(`%slocation %s {
    proxy_pass %s;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
`, header, path, normalizeTarget(n.Target))
	}
	dist := strings.TrimSpace(n.Dist)
	return fmt.Sprintf(`%s# %s -> %s
location = %s {
    return 301 %s;
}

location ^~ %s {
    alias %s;
    index index.html;
    try_files $uri $uri/ %sindex.html;
}
`, header, bare, path, bare, path, path, dist, path)
}

func sslCertLines(ssl bool, serverName string) string {
	if !ssl {
		return ""
	}
	host := "example.com"
	if domain := strings.Fields(serverName); len(domain) > 0 {
		host = domain[0]
	}
	return fmt.Sprintf(`
    ssl_certificate     /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
`, host, host)
}

// normalizeTarget aceita uma porta solta ("3000") ou uma URL/host e devolve
// o valor pronto pro proxy_pass.
func normalizeTarget(s string) string {
	s = strings.TrimSpace(s)
	if p, err := strconv.Atoi(s); err == nil && p > 0 && p < 65536 {
		return fmt.Sprintf("http://127.0.0.1:%d", p)
	}
	if !strings.Contains(s, "://") {
		return "http://" + s
	}
	return s
}

// normalizeLocationPath garante barra no início e no fim (ex: "portfolio" ->
// "/portfolio/") — é o formato que o par redirect+alias espera.
func normalizeLocationPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "/"
	}
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	if !strings.HasSuffix(s, "/") {
		s += "/"
	}
	for strings.Contains(s, "//") {
		s = strings.ReplaceAll(s, "//", "/")
	}
	return s
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
