package collectors

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/devscope/devscope/internal/config"
)

// Cloudflare: os domínios da conta, qual deles é deste projeto e quem está
// acessando. Tudo é leitura pela API v4 com um API Token — o DevScope nunca
// muda nada na conta.
//
// O token vem de CLOUDFLARE_API_TOKEN (o mesmo nome que wrangler e terraform
// usam) ou do arquivo cloudflare.token, que `devscope cloudflare login` grava.

// cfAPI é variável para os testes apontarem para um servidor falso.
var cfAPI = "https://api.cloudflare.com/client/v4"

func CloudflareTokenPath() string {
	return filepath.Join(filepath.Dir(config.ConfigPath()), "cloudflare.token")
}

func CloudflareToken() string {
	if t := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN")); t != "" {
		return t
	}
	b, _ := os.ReadFile(CloudflareTokenPath())
	return strings.TrimSpace(string(b))
}

// SaveCloudflareToken grava o token só para o dono do arquivo. Vazio apaga.
func SaveCloudflareToken(token string) error {
	p := CloudflareTokenPath()
	if token == "" {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

type CFZone struct {
	ID, Name, Status, Plan, AccountID string
	NameServers                       []string
}

// CFPagesProject é um site do Cloudflare Pages. Repo é owner/repo do GitHub
// ligado a ele — é o vínculo mais forte com um projeto do DevScope.
type CFPagesProject struct {
	Name, Subdomain, Repo string
	Domains               []string
}

type CFAccount struct {
	Zones []CFZone
	Pages []CFPagesProject
}

// CloudflareDashURL é a página do domínio no painel da Cloudflare.
func CloudflareDashURL(z CFZone) string {
	return "https://dash.cloudflare.com/" + z.AccountID + "/" + z.Name
}

// ─── API ─────────────────────────────────────────────────────────────────────

type cfEnvelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo struct {
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

func cfRequest(ctx context.Context, token, method, path string, body any) ([]byte, error) {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfAPI+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("sem conexão com a API da Cloudflare")
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return buf.Bytes(), nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("a Cloudflare recusou o token (HTTP %d): %s", resp.StatusCode, cfMessage(buf.Bytes()))
	}
	return nil, fmt.Errorf("Cloudflare HTTP %d: %s", resp.StatusCode, cfMessage(buf.Bytes()))
}

func cfMessage(b []byte) string {
	var env cfEnvelope
	if json.Unmarshal(b, &env) == nil && len(env.Errors) > 0 {
		return env.Errors[0].Message
	}
	return strings.TrimSpace(string(b[:min(len(b), 120)]))
}

// cfList junta todas as páginas de uma listagem REST.
func cfList[T any](ctx context.Context, token, path string) ([]T, error) {
	var out []T
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for page := 1; ; page++ {
		data, err := cfRequest(ctx, token, http.MethodGet, fmt.Sprintf("%s%spage=%d", path, sep, page), nil)
		if err != nil {
			return nil, err
		}
		var env cfEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, errors.New("resposta inválida da Cloudflare")
		}
		var items []T
		if err := json.Unmarshal(env.Result, &items); err != nil {
			return nil, errors.New("resposta inválida da Cloudflare")
		}
		out = append(out, items...)
		if page >= env.ResultInfo.TotalPages {
			return out, nil
		}
	}
}

// FetchCloudflareAccount lista os domínios e os sites do Pages. Pages é
// opcional: token sem essa permissão ainda mostra os domínios.
func FetchCloudflareAccount(ctx context.Context, token string) (CFAccount, error) {
	type zone struct {
		ID, Name, Status string
		Plan             struct{ Name string } `json:"plan"`
		Account          struct{ ID string }   `json:"account"`
		NameServers      []string              `json:"name_servers"`
	}
	zones, err := cfList[zone](ctx, token, "/zones?per_page=50")
	if err != nil {
		return CFAccount{}, err
	}
	var acc CFAccount
	accounts := map[string]bool{}
	for _, z := range zones {
		acc.Zones = append(acc.Zones, CFZone{ID: z.ID, Name: z.Name, Status: z.Status, Plan: z.Plan.Name, AccountID: z.Account.ID, NameServers: z.NameServers})
		accounts[z.Account.ID] = true
	}
	type pages struct {
		Name, Subdomain string
		Domains         []string `json:"domains"`
		Source          struct {
			Config struct {
				Owner    string `json:"owner"`
				RepoName string `json:"repo_name"`
			} `json:"config"`
		} `json:"source"`
	}
	for id := range accounts {
		list, err := cfList[pages](ctx, token, "/accounts/"+id+"/pages/projects")
		if err != nil {
			continue
		}
		for _, p := range list {
			pp := CFPagesProject{Name: p.Name, Subdomain: p.Subdomain, Domains: p.Domains}
			if c := p.Source.Config; c.Owner != "" && c.RepoName != "" {
				pp.Repo = c.Owner + "/" + c.RepoName
			}
			acc.Pages = append(acc.Pages, pp)
		}
	}
	return acc, nil
}

type CFRecord struct {
	Type, Name, Content string
	Proxied             bool
}

func FetchCloudflareDNS(ctx context.Context, token, zoneID string) ([]CFRecord, error) {
	return cfList[CFRecord](ctx, token, "/zones/"+zoneID+"/dns_records?per_page=100")
}

// ─── tráfego ─────────────────────────────────────────────────────────────────

type CFDay struct {
	Date                                         time.Time
	Requests, Cached, PageViews, Threats, Visits int64
	Bytes, CachedBytes                           int64
	Errors                                       int64 // respostas 5xx
}

type CFCount struct {
	Name     string
	Requests int64
}

type CFTraffic struct {
	Days      []CFDay
	Status    []CFCount // 2xx, 3xx, 4xx, 5xx
	Countries []CFCount // código ISO, mais acessos primeiro
	// Prev soma o período anterior de mesmo tamanho, para a comparação
	// "↑ 12% vs 7 dias antes". HasPrev falso: a conta não guarda tanto.
	Prev    CFDay
	HasPrev bool
}

// Total soma os dias do período.
func (t CFTraffic) Total() CFDay {
	var s CFDay
	for _, d := range t.Days {
		s = sumDays(s, d)
	}
	return s
}

func sumDays(s, d CFDay) CFDay {
	s.Requests += d.Requests
	s.Cached += d.Cached
	s.PageViews += d.PageViews
	s.Threats += d.Threats
	s.Visits += d.Visits
	s.Bytes += d.Bytes
	s.CachedBytes += d.CachedBytes
	s.Errors += d.Errors
	return s
}

// httpRequests1dGroups é o agregado diário, disponível no plano Free.
const cfTrafficQuery = `query($zone: String!, $since: Date!, $until: Date!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    httpRequests1dGroups(limit: 40, filter: {date_geq: $since, date_leq: $until}, orderBy: [date_ASC]) {
      dimensions { date }
      sum { requests cachedRequests bytes cachedBytes threats pageViews
        responseStatusMap { edgeResponseStatus requests }
        countryMap { clientCountryName requests } }
      uniq { uniques }
    }
  } }
}`

// FetchCloudflareTraffic lê os últimos days dias e o período anterior a eles.
// Se a conta não guarda o dobro do período, fica sem a comparação.
func FetchCloudflareTraffic(ctx context.Context, token, zoneID string, days int, now time.Time) (CFTraffic, error) {
	t, err := fetchCloudflareTraffic(ctx, token, zoneID, days, 2*days, now)
	if err != nil && ctx.Err() == nil {
		return fetchCloudflareTraffic(ctx, token, zoneID, days, days, now)
	}
	return t, err
}

func fetchCloudflareTraffic(ctx context.Context, token, zoneID string, days, span int, now time.Time) (CFTraffic, error) {
	now = now.UTC()
	start := now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	vars := map[string]string{
		"zone":  zoneID,
		"since": now.AddDate(0, 0, -(span - 1)).Format("2006-01-02"),
		"until": now.Format("2006-01-02"),
	}
	var zones []struct {
		Groups []struct {
			Dimensions struct{ Date string } `json:"dimensions"`
			Sum        struct {
				Requests       int64 `json:"requests"`
				CachedRequests int64 `json:"cachedRequests"`
				Bytes          int64 `json:"bytes"`
				CachedBytes    int64 `json:"cachedBytes"`
				Threats        int64 `json:"threats"`
				PageViews      int64 `json:"pageViews"`
				Status         []struct {
					Code     int   `json:"edgeResponseStatus"`
					Requests int64 `json:"requests"`
				} `json:"responseStatusMap"`
				Countries []struct {
					Name     string `json:"clientCountryName"`
					Requests int64  `json:"requests"`
				} `json:"countryMap"`
			} `json:"sum"`
			Uniq struct{ Uniques int64 } `json:"uniq"`
		} `json:"httpRequests1dGroups"`
	}
	if err := cfGraphQL(ctx, token, cfTrafficQuery, vars, &zones); err != nil {
		return CFTraffic{}, err
	}
	t := CFTraffic{HasPrev: span > days}
	status, countries := map[string]int64{}, map[string]int64{}
	for _, z := range zones {
		for _, g := range z.Groups {
			date, _ := time.Parse("2006-01-02", g.Dimensions.Date)
			s := g.Sum
			day := CFDay{Date: date, Requests: s.Requests, Cached: s.CachedRequests, PageViews: s.PageViews,
				Threats: s.Threats, Visits: g.Uniq.Uniques, Bytes: s.Bytes, CachedBytes: s.CachedBytes}
			for _, st := range s.Status {
				if st.Code >= 500 && st.Code < 600 {
					day.Errors += st.Requests
				}
			}
			if g.Dimensions.Date < start {
				t.Prev = sumDays(t.Prev, day)
				continue
			}
			t.Days = append(t.Days, day)
			for _, st := range s.Status {
				if st.Code >= 200 && st.Code < 600 {
					status[fmt.Sprintf("%dxx", st.Code/100)] += st.Requests
				}
			}
			for _, c := range s.Countries {
				countries[c.Name] += c.Requests
			}
		}
	}
	for _, k := range []string{"2xx", "3xx", "4xx", "5xx"} {
		t.Status = append(t.Status, CFCount{k, status[k]})
	}
	for k, v := range countries {
		t.Countries = append(t.Countries, CFCount{k, v})
	}
	sort.Slice(t.Countries, func(i, j int) bool {
		if t.Countries[i].Requests != t.Countries[j].Requests {
			return t.Countries[i].Requests > t.Countries[j].Requests
		}
		return t.Countries[i].Name < t.Countries[j].Name
	})
	return t, nil
}

// ─── vínculo com o projeto ───────────────────────────────────────────────────

// CFLink é um domínio da conta que pertence a este projeto.
type CFLink struct {
	Zone CFZone
	Host string // o endereço do site: exemplo.com, app.exemplo.com
	From string // de onde veio o vínculo: "Pages site ← GitHub", ".env APP_URL"
}

// CloudflareLinkFile é o vínculo feito à mão na TUI, para o projeto que não
// escreve o próprio endereço em lugar nenhum. Um endereço por linha.
const CloudflareLinkFile = ".devscope/cloudflare.txt"

// SaveCloudflareLink liga host ao projeto em path.
func SaveCloudflareLink(path, host string) error {
	p := filepath.Join(path, CloudflareLinkFile)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(host + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// cfHostFiles são onde um projeto escreve o próprio endereço.
var cfHostFiles = []string{
	CloudflareLinkFile,
	".env", ".env.production", ".env.local", ".env.example",
	"frontend/.env", "frontend/.env.production",
	"CNAME", "public/CNAME", "static/CNAME",
	"package.json", "frontend/package.json",
	"wrangler.toml", "wrangler.json", "wrangler.jsonc",
	"astro.config.mjs", "astro.config.ts", "nginx.conf",
}

var (
	hostRe   = regexp.MustCompile(`(?i)\b((?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,})\b`)
	envKeyRe = regexp.MustCompile(`^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)
)

// LinkCloudflare acha os domínios da conta que são deste projeto: o site do
// Pages ligado ao mesmo repositório do GitHub e os endereços escritos nos
// arquivos do projeto que caem num domínio da conta.
func LinkCloudflare(path, remote string, acc CFAccount) []CFLink {
	if len(acc.Zones) == 0 {
		return nil
	}
	var out []CFLink
	seen := map[string]bool{}
	add := func(host, from string) {
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		if seen[host] {
			return
		}
		if z, ok := zoneFor(host, acc.Zones); ok {
			seen[host] = true
			out = append(out, CFLink{Zone: z, Host: host, From: from})
		}
	}
	if owner, repo, ok := ParseGitHubRepo(remote); ok {
		for _, p := range acc.Pages {
			if strings.EqualFold(p.Repo, owner+"/"+repo) {
				for _, d := range p.Domains {
					add(d, "Pages "+p.Name+" ← GitHub "+p.Repo)
				}
			}
		}
	}
	for _, name := range cfHostFiles {
		f, err := os.Open(filepath.Join(path, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			from := name
			if m := envKeyRe.FindStringSubmatch(line); m != nil {
				from += " " + m[1]
			}
			for _, h := range hostRe.FindAllString(line, -1) {
				add(h, from)
			}
		}
		f.Close()
	}
	return out
}

// zoneFor devolve o domínio da conta que contém host (o mais específico).
func zoneFor(host string, zones []CFZone) (CFZone, bool) {
	var best CFZone
	for _, z := range zones {
		name := strings.ToLower(z.Name)
		if (host == name || strings.HasSuffix(host, "."+name)) && len(name) > len(best.Name) {
			best = z
		}
	}
	return best, best.ID != ""
}

// cfGraphQL roda uma consulta da Analytics e decodifica viewer.zones em out.
// Erro de GraphQL vem com HTTP 200, no campo errors.
func cfGraphQL(ctx context.Context, token, query string, vars, out any) error {
	data, err := cfRequest(ctx, token, http.MethodPost, "/graphql", map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	var resp struct {
		Data struct {
			Viewer struct {
				Zones json.RawMessage `json:"zones"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct{ Message string } `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return errors.New("resposta inválida da Cloudflare")
	}
	if len(resp.Errors) > 0 {
		return errors.New("Cloudflare: " + resp.Errors[0].Message)
	}
	if len(resp.Data.Viewer.Zones) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Data.Viewer.Zones, out); err != nil {
		return errors.New("resposta inválida da Cloudflare")
	}
	return nil
}

// ─── últimas 24 horas ────────────────────────────────────────────────────────

type CFHour struct {
	Time                     time.Time
	Visits, Requests, Errors int64 // Errors: respostas 5xx
}

const cfHourlyQuery = `query($zone: String!, $since: Time!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    httpRequests1hGroups(limit: 48, filter: {datetime_geq: $since}, orderBy: [datetime_ASC]) {
      dimensions { datetime }
      sum { requests responseStatusMap { edgeResponseStatus requests } }
      uniq { uniques }
    }
  } }
}`

// FetchCloudflareHourly devolve as últimas 24 horas, uma entrada por hora,
// com zero nas horas sem acesso — o gráfico precisa das 24 colunas.
func FetchCloudflareHourly(ctx context.Context, token, zoneID string, now time.Time) ([]CFHour, error) {
	first := now.UTC().Truncate(time.Hour).Add(-23 * time.Hour)
	var zones []struct {
		Groups []struct {
			Dimensions struct{ Datetime time.Time } `json:"dimensions"`
			Sum        struct {
				Requests int64 `json:"requests"`
				Status   []struct {
					Code     int   `json:"edgeResponseStatus"`
					Requests int64 `json:"requests"`
				} `json:"responseStatusMap"`
			} `json:"sum"`
			Uniq struct{ Uniques int64 } `json:"uniq"`
		} `json:"httpRequests1hGroups"`
	}
	if err := cfGraphQL(ctx, token, cfHourlyQuery, map[string]string{"zone": zoneID, "since": first.Format(time.RFC3339)}, &zones); err != nil {
		return nil, err
	}
	out := make([]CFHour, 24)
	for i := range out {
		out[i].Time = first.Add(time.Duration(i) * time.Hour)
	}
	for _, z := range zones {
		for _, g := range z.Groups {
			if i := int(g.Dimensions.Datetime.Sub(first) / time.Hour); i >= 0 && i < 24 {
				out[i].Visits += g.Uniq.Uniques
				out[i].Requests += g.Sum.Requests
				for _, s := range g.Sum.Status {
					if s.Code >= 500 && s.Code < 600 {
						out[i].Errors += s.Requests
					}
				}
			}
		}
	}
	return out, nil
}

// ─── caminhos ────────────────────────────────────────────────────────────────

type CFPathError struct {
	Path   string
	Status int
	Count  int64
}

// O conjunto adaptativo aceita no máximo um dia por consulta em qualquer
// plano: os caminhos são das últimas 24 horas.
const cfPathsQuery = `query($zone: String!, $since: Time!, $until: Time!) {
  viewer { zones(filter: {zoneTag: $zone}) {
    top: httpRequestsAdaptiveGroups(limit: 40, filter: {datetime_geq: $since, datetime_lt: $until}, orderBy: [count_DESC]) {
      count
      dimensions { clientRequestPath }
    }
    failed: httpRequestsAdaptiveGroups(limit: 8, filter: {datetime_geq: $since, datetime_lt: $until, edgeResponseStatus_geq: 400}, orderBy: [count_DESC]) {
      count
      dimensions { clientRequestPath edgeResponseStatus }
    }
  } }
}`

// FetchCloudflarePaths: as páginas mais pedidas (sem arquivos estáticos,
// que dominariam a lista) e os caminhos que mais devolveram erro.
func FetchCloudflarePaths(ctx context.Context, token, zoneID string, now time.Time) ([]CFCount, []CFPathError, error) {
	type group struct {
		Count      int64 `json:"count"`
		Dimensions struct {
			Path   string `json:"clientRequestPath"`
			Status int    `json:"edgeResponseStatus"`
		} `json:"dimensions"`
	}
	var zones []struct {
		Top    []group `json:"top"`
		Failed []group `json:"failed"`
	}
	now = now.UTC()
	vars := map[string]string{"zone": zoneID, "since": now.Add(-24 * time.Hour).Format(time.RFC3339), "until": now.Format(time.RFC3339)}
	if err := cfGraphQL(ctx, token, cfPathsQuery, vars, &zones); err != nil {
		return nil, nil, err
	}
	var pages []CFCount
	var failed []CFPathError
	for _, z := range zones {
		for _, g := range z.Top {
			if isPagePath(g.Dimensions.Path) && len(pages) < 8 {
				pages = append(pages, CFCount{Name: g.Dimensions.Path, Requests: g.Count})
			}
		}
		for _, g := range z.Failed {
			failed = append(failed, CFPathError{Path: g.Dimensions.Path, Status: g.Dimensions.Status, Count: g.Count})
		}
	}
	return pages, failed, nil
}

// isPagePath: /, /sobre e /blog/post.html são páginas; /assets/app.js não.
func isPagePath(p string) bool {
	base := p[strings.LastIndex(p, "/")+1:]
	ext := strings.ToLower(filepath.Ext(base))
	return ext == "" || ext == ".html" || ext == ".htm" || ext == ".php"
}

// ─── segurança ───────────────────────────────────────────────────────────────

type CFCert struct {
	Type, Status, Authority string
	Hosts                   []string
	Expires                 time.Time
}

type CFInsight struct {
	Severity, Subject, Class, Resolve, Link string
}

// CFSecurity junta o que diz se o domínio está bem protegido. Cada parte vem
// de uma rota e falha sozinha: Notes diz o que não deu para ler.
type CFSecurity struct {
	Settings map[string]string // ssl, always_use_https, min_tls_version…
	Certs    []CFCert
	Insights []CFInsight
	Expires  time.Time // vencimento do registro do domínio (RDAP)
	Registry string
	Notes    []string
}

func FetchCloudflareSecurity(ctx context.Context, token string, z CFZone) CFSecurity {
	var sec CFSecurity
	type setting struct {
		ID    string          `json:"id"`
		Value json.RawMessage `json:"value"`
	}
	if list, err := cfList[setting](ctx, token, "/zones/"+z.ID+"/settings"); err == nil {
		sec.Settings = map[string]string{}
		for _, s := range list {
			var v string
			if json.Unmarshal(s.Value, &v) == nil {
				sec.Settings[s.ID] = v
			}
		}
	} else {
		sec.Notes = append(sec.Notes, "configurações: "+err.Error())
	}

	type pack struct {
		Type, Status         string
		Hosts                []string `json:"hosts"`
		CertificateAuthority string   `json:"certificate_authority"`
		Certificates         []struct {
			ExpiresOn time.Time `json:"expires_on"`
		} `json:"certificates"`
	}
	if packs, err := cfList[pack](ctx, token, "/zones/"+z.ID+"/ssl/certificate_packs"); err == nil {
		for _, p := range packs {
			c := CFCert{Type: p.Type, Status: p.Status, Authority: p.CertificateAuthority, Hosts: p.Hosts}
			for _, cert := range p.Certificates {
				if c.Expires.IsZero() || (!cert.ExpiresOn.IsZero() && cert.ExpiresOn.Before(c.Expires)) {
					c.Expires = cert.ExpiresOn
				}
			}
			sec.Certs = append(sec.Certs, c)
		}
	} else {
		sec.Notes = append(sec.Notes, "certificados: "+err.Error())
	}

	if data, err := cfRequest(ctx, token, http.MethodGet, "/zones/"+z.ID+"/security-center/insights?dismissed=false&per_page=50", nil); err == nil {
		var env cfEnvelope
		var res struct {
			Issues []struct {
				Severity    string `json:"severity"`
				Subject     string `json:"subject"`
				IssueClass  string `json:"issue_class"`
				ResolveText string `json:"resolve_text"`
				ResolveLink string `json:"resolve_link"`
				Status      string `json:"status"`
				Dismissed   bool   `json:"dismissed"`
			} `json:"issues"`
		}
		if json.Unmarshal(data, &env) == nil && json.Unmarshal(env.Result, &res) == nil {
			for _, i := range res.Issues {
				if i.Dismissed || i.Status == "resolved" {
					continue
				}
				sec.Insights = append(sec.Insights, CFInsight{Severity: strings.ToLower(i.Severity), Subject: i.Subject,
					Class: i.IssueClass, Resolve: i.ResolveText, Link: i.ResolveLink})
			}
			sort.SliceStable(sec.Insights, func(a, b int) bool {
				return insightRank(sec.Insights[a].Severity) < insightRank(sec.Insights[b].Severity)
			})
		}
	} else {
		sec.Notes = append(sec.Notes, "insights: "+err.Error())
	}

	sec.Expires, sec.Registry = domainExpiry(ctx, z.Name)
	return sec
}

func insightRank(sev string) int {
	switch sev {
	case "critical":
		return 0
	case "moderate":
		return 1
	}
	return 2
}

// rdapURL é variável para os testes. rdap.org encaminha para o RDAP do
// registro de cada extensão (o do registro.br, para .br).
var rdapURL = "https://rdap.org/domain/"

// domainExpiry lê no RDAP público quando o registro do domínio vence. Sem
// resposta, data zero: a tela só não mostra a linha.
func domainExpiry(ctx context.Context, domain string) (time.Time, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rdapURL+domain, nil)
	if err != nil {
		return time.Time{}, ""
	}
	req.Header.Set("Accept", "application/rdap+json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return time.Time{}, ""
	}
	defer resp.Body.Close()
	var body struct {
		Events []struct {
			Action string    `json:"eventAction"`
			Date   time.Time `json:"eventDate"`
		} `json:"events"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&body) != nil {
		return time.Time{}, ""
	}
	for _, e := range body.Events {
		if e.Action == "expiration" {
			return e.Date, resp.Request.URL.Host
		}
	}
	return time.Time{}, ""
}
