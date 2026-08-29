// Package nginxutil detecta e edita configs de nginx dentro de um projeto
// devscope: um main.conf/nginx.conf na raiz que inclui uma pasta com um
// arquivo por rota (.inc ou .conf). A pasta pode ter qualquer nome — não tem
// lista fixa ("sites", "conf.d" etc): a gente lê o include de verdade no
// main.conf pra achar ela, e só cai pra uma varredura por conteúdo se não
// achar include nenhum. Não existe um JSON de config próprio: os .conf/.inc
// já são a fonte da verdade, igual o tab de git lê o .git direto.
package nginxutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// mainConfNames são os nomes mais comuns do arquivo raiz que agrega os sites.
var mainConfNames = []string{"main.conf", "nginx.conf"}

// skipDirs não entram na varredura por conteúdo — nunca é onde ficam rotas.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, ".devscope": true}

// Layout é onde a config de nginx deste projeto mora no disco.
type Layout struct {
	MainConf string // caminho absoluto, "" se não achou
	SitesDir string // caminho absoluto, "" se não achou
	Ext      string // extensão dominante nos arquivos de site (.inc ou .conf)
}

// Site é um arquivo de rota já parseado (best-effort, via regex — não é um
// parser completo da gramática do nginx, só o suficiente pra listar/editar
// um server{} por arquivo, que é o padrão real desse tipo de setup).
type Site struct {
	File        string   // relativo à raiz do projeto, ex: sites/api.inc
	Name        string   // nome do arquivo sem extensão
	ServerNames []string
	Listen      string
	SSL         bool
	ProxyPass   string
	Root        string
	Raw         string
	Project     string // preenchido pela UI quando o site vem de outro projeto
}

// NewSite são os campos preenchidos no modal de criação de rota.
type NewSite struct {
	Name       string // nome do arquivo (sem extensão)
	ServerName string // domínio(s), separados por espaço
	Target     string // proxy_pass — se vazio, usa Root (site estático)
	Root       string
	Port       int
	SSL        bool
}

// Detect é uma checagem barata pra landing screen: o projeto tem cara de
// nginx? (main.conf/nginx.conf na raiz e/ou pasta de sites com .conf/.inc)
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
		l.SitesDir, l.Ext = includedSitesDir(projectPath, l.MainConf)
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

// includedSitesDir lê o(s) include do main.conf pra achar a pasta de rotas de
// verdade, seja qual for o nome dela. Se o include usa o caminho de dentro do
// container (ex: /etc/nginx/sites/*.inc), tenta também só o nome final da
// pasta na raiz do projeto — é o que o volume do docker costuma espelhar.
func includedSitesDir(projectPath, mainConf string) (dir, ext string) {
	b, err := os.ReadFile(mainConf)
	if err != nil {
		return "", ""
	}
	for _, m := range reInclude.FindAllSubmatch(b, -1) {
		raw := strings.Trim(strings.TrimSpace(string(m[1])), `"'`)
		globDir := filepath.Dir(raw)
		if globDir == "." || globDir == "/" || globDir == "" {
			continue // include de um arquivo específico, não de uma pasta
		}
		globExt := filepath.Ext(filepath.Base(raw))
		if globExt == ".*" {
			globExt = ""
		}
		for _, full := range []string{filepath.Join(projectPath, globDir), filepath.Join(projectPath, filepath.Base(globDir))} {
			st, err := os.Stat(full)
			if err != nil || !st.IsDir() {
				continue
			}
			e := globExt
			if e == "" {
				entries, _ := os.ReadDir(full)
				e = firstNonEmpty(dominantExt(entries), ".conf")
			}
			return full, e
		}
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

// Discover acha o layout e parseia cada arquivo de site.
func Discover(projectPath string) (Layout, []Site, error) {
	layout, err := findLayout(projectPath)
	if err != nil {
		return layout, nil, err
	}
	var sites []Site
	if layout.SitesDir != "" {
		entries, err := os.ReadDir(layout.SitesDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || filepath.Ext(e.Name()) != layout.Ext {
					continue
				}
				full := filepath.Join(layout.SitesDir, e.Name())
				b, err := os.ReadFile(full)
				if err != nil {
					continue
				}
				rel, _ := filepath.Rel(projectPath, full)
				sites = append(sites, parseSite(rel, string(b)))
			}
		}
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].Name < sites[j].Name })
	return layout, sites, nil
}

var (
	reServerName = regexp.MustCompile(`(?m)^\s*server_name\s+([^;]+);`)
	reListen     = regexp.MustCompile(`(?m)^\s*listen\s+([^;]+);`)
	reProxyPass  = regexp.MustCompile(`(?m)^\s*proxy_pass\s+([^;]+);`)
	reRoot       = regexp.MustCompile(`(?m)^\s*root\s+([^;]+);`)
)

func parseSite(rel, raw string) Site {
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

// CreateSite escreve um novo arquivo de rota na pasta de sites e garante que
// o main.conf inclui a pasta.
func CreateSite(projectPath string, n NewSite) (Site, error) {
	layout, err := findLayout(projectPath)
	if err != nil {
		return Site{}, err
	}
	if layout.SitesDir == "" {
		return Site{}, fmt.Errorf("pasta de rotas não encontrada — crie uma pasta com um .inc/.conf de exemplo e um include apontando pra ela no main.conf")
	}
	name := sanitizeName(n.Name)
	if name == "" {
		return Site{}, fmt.Errorf("nome vazio")
	}
	if strings.TrimSpace(n.ServerName) == "" {
		return Site{}, fmt.Errorf("server_name vazio")
	}
	if strings.TrimSpace(n.Target) == "" && strings.TrimSpace(n.Root) == "" {
		return Site{}, fmt.Errorf("informe um destino: proxy_pass ou root")
	}
	if n.Port <= 0 {
		n.Port = 80
		if n.SSL {
			n.Port = 443
		}
	}
	ext := firstNonEmpty(layout.Ext, ".inc")
	dest := filepath.Join(layout.SitesDir, name+ext)
	if _, err := os.Stat(dest); err == nil {
		return Site{}, fmt.Errorf("%s já existe", name+ext)
	}
	content := renderSite(n)
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		return Site{}, err
	}
	if layout.MainConf != "" {
		// ponytail: best-effort — se o main.conf já faz include glob da pasta
		// (o caso comum), não mexe em nada; senão, só acrescenta um include
		// solto no fim do arquivo, fora de qualquer bloco. Funciona pro
		// layout descrito (main.conf com include glob), mas em layouts
		// incomuns pode cair fora do bloco http/server certo — revisar main.conf
		// à mão nesse caso.
		_ = ensureInclude(layout.MainConf, layout.SitesDir, ext)
	}
	rel, _ := filepath.Rel(projectPath, dest)
	return parseSite(rel, content), nil
}

// DeleteSite remove um arquivo de rota. file é relativo à raiz do projeto.
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
    }`, strings.TrimSpace(n.Target))
	} else {
		body = fmt.Sprintf(`    root %s;
    index index.html;

    location / {
        try_files $uri $uri/ =404;
    }`, strings.TrimSpace(n.Root))
	}
	ssl := ""
	if n.SSL {
		domain := strings.Fields(n.ServerName)
		host := "example.com"
		if len(domain) > 0 {
			host = domain[0]
		}
		ssl = fmt.Sprintf(`
    ssl_certificate     /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
`, host, host)
	}
	return fmt.Sprintf(`server {
    listen %s;
    server_name %s;
%s
%s
}
`, listen, strings.TrimSpace(n.ServerName), ssl, body)
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
