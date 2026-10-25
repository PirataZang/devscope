package hub

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

// Cloudflare no bot: o vigia que avisa de pico de visitas, de pico de erros
// 5xx e de certificado ou registro perto de vencer, e o comando "cloudflare"
// que o Telegram usa para mostrar os números do domínio de um projeto.
//
// Usa o mesmo token e os mesmos coletores do módulo Cloudflare da TUI. Sem
// token, nada disso roda e o comando responde dizendo como configurar.

// Coletores da Cloudflare: variáveis para os testes.
var (
	cfToken    = collectors.CloudflareToken
	cfAccount  = collectors.FetchCloudflareAccount
	cfHourly   = collectors.FetchCloudflareHourly
	cfTraffic  = collectors.FetchCloudflareTraffic
	cfPaths    = collectors.FetchCloudflarePaths
	cfSecurity = collectors.FetchCloudflareSecurity
)

const (
	cfEvery     = 5 * time.Minute
	cfAcctEvery = 30 * time.Minute

	// Pico de visitas: a última hora completa com pelo menos cfSpikeMin
	// visitantes e cfSpikeFactor vezes a mediana das 22 horas anteriores.
	cfSpikeMin    = 50
	cfSpikeFactor = 3.0
	cfSpikeQuiet  = 3 * time.Hour
	// Pico de erros: cfErrMin respostas 5xx e cfErrShare% das requisições.
	cfErrMin   = 10
	cfErrShare = 5.0

	cfCertWarn   = 14 // dias
	cfDomainWarn = 30 // dias
)

// ─── conta ──────────────────────────────────────────────────────────────────

// cloudflareAccount devolve a conta lida há menos de 30 min, ou lê de novo.
// Falhou a leitura, fica a anterior: um soluço da API não some com os
// domínios do bot.
func (a *Agent) cloudflareAccount(ctx context.Context) (collectors.CFAccount, string) {
	token := cfToken()
	if token == "" {
		return collectors.CFAccount{}, ""
	}
	a.cfMu.Lock()
	acct, at := a.cfAcct, a.cfAt
	a.cfMu.Unlock()
	if !at.IsZero() && a.now().Sub(at) < cfAcctEvery {
		return acct, token
	}
	if fresh, err := cfAccount(ctx, token); err == nil {
		acct = fresh
		a.cfMu.Lock()
		a.cfAcct, a.cfAt = fresh, a.now()
		a.cfMu.Unlock()
	}
	return acct, token
}

type cfSecEntry struct {
	sec collectors.CFSecurity
	at  time.Time
}

// zoneSecurity lê SSL, certificados, avisos e o RDAP — a parte lenta e que
// quase não muda — no máximo a cada maxAge. O comando do Telegram usa 6 h e
// responde dentro do prazo do hub; o vigia, 24 h.
func (a *Agent) zoneSecurity(ctx context.Context, token string, z collectors.CFZone, maxAge time.Duration) collectors.CFSecurity {
	a.cfMu.Lock()
	e, ok := a.cfSec[z.ID]
	a.cfMu.Unlock()
	if ok && a.now().Sub(e.at) < maxAge {
		return e.sec
	}
	sec := cfSecurity(ctx, token, z)
	a.cfMu.Lock()
	if a.cfSec == nil {
		a.cfSec = map[string]cfSecEntry{}
	}
	a.cfSec[z.ID] = cfSecEntry{sec, a.now()}
	a.cfMu.Unlock()
	return sec
}

// projectDomains: os endereços da Cloudflare deste projeto, com a conta que
// já está em memória — o snapshot não espera a API.
func (a *Agent) projectDomains(p core.Project) []collectors.CFLink {
	a.cfMu.Lock()
	acct := a.cfAcct
	a.cfMu.Unlock()
	if len(acct.Zones) == 0 {
		return nil
	}
	remote := ""
	if p.Git != nil {
		remote = p.Git.Remote
	}
	return collectors.LinkCloudflare(p.Path, remote, acct)
}

// cfTarget é um domínio vigiado e o projeto a que ele pertence.
type cfTarget struct {
	link    collectors.CFLink
	project core.Project
}

// cfTargets: cada domínio da conta uma vez só, no primeiro projeto que o usa.
func (a *Agent) cfTargets(projects []core.Project) []cfTarget {
	var out []cfTarget
	seen := map[string]bool{}
	for _, p := range projects {
		for _, l := range a.projectDomains(p) {
			if !seen[l.Zone.ID] {
				seen[l.Zone.ID] = true
				out = append(out, cfTarget{l, p})
			}
		}
	}
	return out
}

// ─── vigia ──────────────────────────────────────────────────────────────────

func (a *Agent) cfLoop(ctx context.Context) {
	w := newCFWatcher()
	t := time.NewTicker(cfEvery)
	defer t.Stop()
	for {
		if _, token := a.cloudflareAccount(ctx); token != "" {
			// A conta acabou de chegar: o snapshot já pode levar os domínios.
			select {
			case a.snapNow <- struct{}{}:
			default:
			}
			a.sendEvents(ctx, w.step(ctx, a, a.store.Get().Projects))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type cfWatcher struct {
	spike   map[string]time.Time // domínio → último aviso de pico
	errOpen map[string]bool      // domínio com pico de 5xx ainda aberto
	checked map[string]time.Time // domínio → última conferência de vencimentos
}

func newCFWatcher() *cfWatcher {
	return &cfWatcher{spike: map[string]time.Time{}, errOpen: map[string]bool{}, checked: map[string]time.Time{}}
}

func (w *cfWatcher) step(ctx context.Context, a *Agent, projects []core.Project) []Event {
	_, token := a.cloudflareAccount(ctx)
	now := a.now()
	var evs []Event
	for _, t := range a.cfTargets(projects) {
		if hours, err := cfHourly(ctx, token, t.link.Zone.ID, now); err == nil {
			evs = append(evs, w.peaks(t, hours, now)...)
		}
		if now.Sub(w.checked[t.link.Zone.ID]) >= 24*time.Hour {
			w.checked[t.link.Zone.ID] = now
			evs = append(evs, expiryEvents(t, a.zoneSecurity(ctx, token, t.link.Zone, 0), now)...)
		}
	}
	return evs
}

// peaks olha a última hora COMPLETA — a corrente ainda está pela metade e
// sempre pareceria uma queda.
func (w *cfWatcher) peaks(t cfTarget, hours []collectors.CFHour, now time.Time) []Event {
	if len(hours) < 3 {
		return nil
	}
	last := hours[len(hours)-2]
	prev := make([]int64, 0, len(hours)-2)
	for _, h := range hours[:len(hours)-2] {
		prev = append(prev, h.Visits)
	}
	base := median(prev)
	zone := t.link.Zone.Name
	ev := Event{ProjectID: t.project.ID, Project: t.project.Name, At: now}
	var evs []Event

	ratio := float64(last.Visits) / float64(max(base, 5))
	if last.Visits >= cfSpikeMin && ratio >= cfSpikeFactor && now.Sub(w.spike[zone]) >= cfSpikeQuiet {
		w.spike[zone] = now
		e := ev
		e.Key, e.Kind = "cf:traffic:"+zone, "cf_traffic"
		e.Title = fmt.Sprintf("pico de visitas em %s: %d na última hora", zone, last.Visits)
		e.Detail = fmt.Sprintf("o normal é ~%d por hora: %.0f× acima (às %s)", base, ratio, last.Time.Local().Format("15h"))
		evs = append(evs, e)
	}

	share := 0.0
	if last.Requests > 0 {
		share = float64(last.Errors) * 100 / float64(last.Requests)
	}
	bad := last.Errors >= cfErrMin && share >= cfErrShare
	switch {
	case bad && !w.errOpen[zone]:
		w.errOpen[zone] = true
		e := ev
		e.Key, e.Kind = "cf:errors:"+zone, "cf_errors"
		e.Title = fmt.Sprintf("%s: %d erros 5xx na última hora", zone, last.Errors)
		e.Detail = fmt.Sprintf("%.0f%% das requisições falharam no servidor atrás da Cloudflare", share)
		evs = append(evs, e)
	case !bad && w.errOpen[zone]:
		w.errOpen[zone] = false
		e := ev
		e.Key, e.Kind = "cf:errors:"+zone, "resolved"
		e.Title = zone + ": erros 5xx voltaram ao normal"
		evs = append(evs, e)
	}
	return evs
}

// expiryEvents: certificado a menos de 14 dias (o Universal se renova sozinho
// bem antes; se chegou aqui, a renovação travou) e registro a menos de 30.
func expiryEvents(t cfTarget, sec collectors.CFSecurity, now time.Time) []Event {
	zone := t.link.Zone.Name
	ev := Event{ProjectID: t.project.ID, Project: t.project.Name, At: now}
	var evs []Event
	if exp := earliestCert(sec); !exp.IsZero() {
		if days := daysUntil(exp, now); days < cfCertWarn {
			e := ev
			e.Key, e.Kind = "cf:cert:"+zone, "ssl"
			e.Title = fmt.Sprintf("certificado de %s vence em %d dias", zone, days)
			if days < 0 {
				e.Title = "certificado de " + zone + " venceu"
			}
			e.Detail = "confira o SSL/TLS do domínio no painel da Cloudflare"
			evs = append(evs, e)
		}
	}
	if !sec.Expires.IsZero() {
		if days := daysUntil(sec.Expires, now); days < cfDomainWarn {
			e := ev
			e.Key, e.Kind = "cf:domain:"+zone, "cf_domain"
			e.Title = fmt.Sprintf("o registro de %s vence em %d dias", zone, days)
			if days < 0 {
				e.Title = "o registro de " + zone + " venceu"
			}
			e.Detail = "renove em " + firstNonEmpty(sec.Registry, "quem registrou o domínio") + " antes que o site saia do ar"
			evs = append(evs, e)
		}
	}
	return evs
}

func earliestCert(sec collectors.CFSecurity) time.Time {
	var out time.Time
	for _, c := range sec.Certs {
		if !c.Expires.IsZero() && (out.IsZero() || c.Expires.Before(out)) {
			out = c.Expires
		}
	}
	return out
}

func daysUntil(t, now time.Time) int { return int(t.Sub(now).Hours() / 24) }

func median(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int64{}, v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ─── comando "cloudflare" ───────────────────────────────────────────────────

// CFDomain é o que o Telegram mostra de um domínio: a semana, as últimas 24
// horas e o que precisa de atenção. Países vão em código ISO; o hub desenha a
// bandeira.
type CFDomain struct {
	Zone      string `json:"zone"`
	Host      string `json:"host"`
	Status    string `json:"status"`
	Plan      string `json:"plan"`
	Linked    string `json:"linked,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Project   string `json:"project,omitempty"`
	Site      string `json:"site"`
	Dash      string `json:"dash"`

	Days       int     `json:"days"`
	Visits     int64   `json:"visits"`
	PrevVisits *int64  `json:"prev_visits,omitempty"`
	PageViews  int64   `json:"page_views"`
	Requests   int64   `json:"requests"`
	Errors     int64   `json:"errors"`
	Threats    int64   `json:"threats"`
	CachePct   float64 `json:"cache_pct"`
	Daily      []int64 `json:"daily"`  // visitantes por dia, do mais antigo a hoje
	Hourly     []int64 `json:"hourly"` // visitantes por hora; a última é a corrente
	PeakHour   string  `json:"peak_hour,omitempty"`
	PeakVisits int64   `json:"peak_visits"`

	Countries []CFCount     `json:"countries"`
	TopPages  []CFCount     `json:"top_pages"`
	Failed    []CFPathError `json:"failed"`

	SSLMode       string     `json:"ssl_mode,omitempty"`
	AlwaysHTTPS   bool       `json:"always_https"`
	CertExpires   *time.Time `json:"cert_expires,omitempty"`
	DomainExpires *time.Time `json:"domain_expires,omitempty"`
	Insights      int        `json:"insights"`
}

type CFCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type CFPathError struct {
	Path   string `json:"path"`
	Status int    `json:"status"`
	Count  int64  `json:"count"`
}

// handleCloudflare: com project_id, os domínios daquele projeto; sem, os
// domínios da conta que algum projeto desta máquina usa (até 5).
func handleCloudflare(ctx context.Context, a *Agent, args Args) (any, error) {
	acct, token := a.cloudflareAccount(ctx)
	if token == "" {
		return nil, cmdErr("cloudflare_off", "esta máquina não tem token da Cloudflare: rode devscope cloudflare login")
	}
	var targets []cfTarget
	if args.ProjectID != "" {
		p, err := a.project(args.ProjectID)
		if err != nil {
			return nil, err
		}
		for _, l := range a.projectDomains(p) {
			targets = append(targets, cfTarget{l, p})
		}
		if len(targets) == 0 {
			return nil, cmdErr("not_found", "nenhum domínio da Cloudflare ligado a "+p.Name)
		}
	} else {
		targets = a.cfTargets(a.store.Get().Projects)
		if len(targets) == 0 {
			for _, z := range acct.Zones {
				targets = append(targets, cfTarget{link: collectors.CFLink{Zone: z, Host: z.Name}})
			}
		}
	}
	if len(targets) > 5 {
		targets = targets[:5]
	}
	days := 7
	if args.Limit == 30 {
		days = 30
	}
	out := make([]CFDomain, len(targets))
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = a.cfDomain(ctx, token, t, days)
		}()
	}
	wg.Wait()
	return map[string]any{"domains": out}, nil
}

// cfDomain lê as quatro partes em paralelo; a que falhar só fica vazia.
func (a *Agent) cfDomain(ctx context.Context, token string, t cfTarget, days int) CFDomain {
	now := a.now()
	z := t.link.Zone
	d := CFDomain{Zone: z.Name, Host: t.link.Host, Status: z.Status, Plan: z.Plan, Linked: t.link.From,
		ProjectID: t.project.ID, Project: t.project.Name, Site: "https://" + t.link.Host,
		Dash: collectors.CloudflareDashURL(z), Days: days,
		Daily: []int64{}, Hourly: []int64{}, Countries: []CFCount{}, TopPages: []CFCount{}, Failed: []CFPathError{}}

	var (
		wg      sync.WaitGroup
		traffic collectors.CFTraffic
		hours   []collectors.CFHour
		pages   []collectors.CFCount
		failed  []collectors.CFPathError
		sec     collectors.CFSecurity
	)
	run := func(f func()) {
		wg.Add(1)
		go func() { defer wg.Done(); f() }()
	}
	run(func() { traffic, _ = cfTraffic(ctx, token, z.ID, days, now) })
	run(func() { hours, _ = cfHourly(ctx, token, z.ID, now) })
	run(func() { pages, failed, _ = cfPaths(ctx, token, z.ID, now) })
	run(func() { sec = a.zoneSecurity(ctx, token, z, 6*time.Hour) })
	wg.Wait()

	tot := traffic.Total()
	d.Visits, d.PageViews, d.Requests, d.Errors, d.Threats = tot.Visits, tot.PageViews, tot.Requests, tot.Errors, tot.Threats
	if tot.Requests > 0 {
		d.CachePct = float64(tot.Cached) * 100 / float64(tot.Requests)
	}
	if traffic.HasPrev {
		prev := traffic.Prev.Visits
		d.PrevVisits = &prev
	}
	for _, day := range traffic.Days {
		d.Daily = append(d.Daily, day.Visits)
	}
	for i, c := range traffic.Countries {
		if i == 5 {
			break
		}
		d.Countries = append(d.Countries, CFCount{c.Name, c.Requests})
	}
	for _, h := range hours {
		d.Hourly = append(d.Hourly, h.Visits)
		if h.Visits > d.PeakVisits {
			d.PeakVisits, d.PeakHour = h.Visits, h.Time.Local().Format("15h")
		}
	}
	for i, p := range pages {
		if i == 5 {
			break
		}
		d.TopPages = append(d.TopPages, CFCount{p.Name, p.Requests})
	}
	for i, f := range failed {
		if i == 3 {
			break
		}
		d.Failed = append(d.Failed, CFPathError{f.Path, f.Status, f.Count})
	}
	d.SSLMode, d.AlwaysHTTPS = sec.Settings["ssl"], sec.Settings["always_use_https"] == "on"
	if exp := earliestCert(sec); !exp.IsZero() {
		d.CertExpires = &exp
	}
	if !sec.Expires.IsZero() {
		exp := sec.Expires
		d.DomainExpires = &exp
	}
	d.Insights = len(sec.Insights)
	return d
}
