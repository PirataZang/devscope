package hub

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

var cfZone = collectors.CFZone{ID: "z1", Name: "exemplo.dev.br", Status: "active", Plan: "Free Website", AccountID: "acc"}

// hoursWith: 24 horas com `base` visitantes; a penúltima (a última completa)
// com `last` visitantes e `errs` respostas 5xx em 200 requisições.
func hoursWith(base, last, errs int64) []collectors.CFHour {
	out := make([]collectors.CFHour, 24)
	first := fixedNow.Truncate(time.Hour).Add(-23 * time.Hour)
	for i := range out {
		out[i] = collectors.CFHour{Time: first.Add(time.Duration(i) * time.Hour), Visits: base, Requests: 200}
	}
	out[22].Visits, out[22].Errors = last, errs
	return out
}

func TestCloudflarePeaks(t *testing.T) {
	w := newCFWatcher()
	target := cfTarget{collectors.CFLink{Zone: cfZone, Host: "exemplo.dev.br"}, core.Project{ID: pid, Name: "site"}}

	// Hora normal: nada.
	if evs := w.peaks(target, hoursWith(20, 25, 0), fixedNow); len(evs) != 0 {
		t.Fatalf("hora normal avisou: %+v", evs)
	}
	// 200 visitantes contra ~20 por hora: pico.
	evs := w.peaks(target, hoursWith(20, 200, 0), fixedNow)
	if len(evs) != 1 || evs[0].Kind != "cf_traffic" || !strings.Contains(evs[0].Title, "200 na última hora") ||
		!strings.Contains(evs[0].Detail, "~20 por hora: 10×") || evs[0].ProjectID != pid {
		t.Fatalf("pico: %+v", evs)
	}
	// O mesmo pico 5 minutos depois não repete.
	if evs := w.peaks(target, hoursWith(20, 200, 0), fixedNow.Add(5*time.Minute)); len(evs) != 0 {
		t.Fatalf("repetiu o pico: %+v", evs)
	}
	// Pouca visita, mesmo em proporção alta, não é pico.
	w2 := newCFWatcher()
	if evs := w2.peaks(target, hoursWith(1, 30, 0), fixedNow); len(evs) != 0 {
		t.Fatalf("30 visitas não é pico: %+v", evs)
	}

	// 5xx: abre uma vez, fecha com "resolved".
	evs = w.peaks(target, hoursWith(20, 20, 30), fixedNow)
	if len(evs) != 1 || evs[0].Kind != "cf_errors" || !strings.Contains(evs[0].Title, "30 erros 5xx") || !strings.Contains(evs[0].Detail, "15%") {
		t.Fatalf("5xx: %+v", evs)
	}
	if evs := w.peaks(target, hoursWith(20, 20, 30), fixedNow); len(evs) != 0 {
		t.Fatalf("5xx aberto repetiu: %+v", evs)
	}
	if evs := w.peaks(target, hoursWith(20, 20, 0), fixedNow); len(evs) != 1 || evs[0].Kind != "resolved" {
		t.Fatalf("5xx resolvido: %+v", evs)
	}
}

func TestCloudflareExpiry(t *testing.T) {
	target := cfTarget{collectors.CFLink{Zone: cfZone}, core.Project{ID: pid, Name: "site"}}
	sec := collectors.CFSecurity{
		Certs:    []collectors.CFCert{{Expires: fixedNow.Add(90 * 24 * time.Hour)}, {Expires: fixedNow.Add(5 * 24 * time.Hour)}},
		Expires:  fixedNow.Add(20 * 24 * time.Hour),
		Registry: "rdap.registro.br",
	}
	evs := expiryEvents(target, sec, fixedNow)
	if len(evs) != 2 || evs[0].Kind != "ssl" || !strings.Contains(evs[0].Title, "vence em 5 dias") ||
		evs[1].Kind != "cf_domain" || !strings.Contains(evs[1].Detail, "rdap.registro.br") {
		t.Fatalf("%+v", evs)
	}
	if evs := expiryEvents(target, collectors.CFSecurity{Expires: fixedNow.Add(200 * 24 * time.Hour)}, fixedNow); len(evs) != 0 {
		t.Fatalf("longe de vencer: %+v", evs)
	}
}

// O comando junta tráfego, horas, páginas e segurança do domínio do projeto,
// e o snapshot leva o endereço para o bot saber onde mostrar o botão.
func TestHandleCloudflare(t *testing.T) {
	a := fixtureAgent(t)
	stub(t, &cfToken, func() string { return "" })
	_, err := run(t, a, "cloudflare", `{"project_id":"`+pid+`"}`)
	expectCode(t, err, "cloudflare_off")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_URL=https://app.exemplo.dev.br\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := a.store.Get().Projects
	projects[0].Path = dir
	a.store.SetProjects(projects)

	stub(t, &cfToken, func() string { return "tok" })
	stub(t, &cfAccount, func(context.Context, string) (collectors.CFAccount, error) {
		return collectors.CFAccount{Zones: []collectors.CFZone{cfZone}}, nil
	})
	stub(t, &cfTraffic, func(context.Context, string, string, int, time.Time) (collectors.CFTraffic, error) {
		return collectors.CFTraffic{
			Days:      []collectors.CFDay{{Visits: 40, Requests: 400, Cached: 100}, {Visits: 60, Requests: 600, Cached: 300, Errors: 6}},
			Countries: []collectors.CFCount{{Name: "BR", Requests: 900}},
			Prev:      collectors.CFDay{Visits: 80}, HasPrev: true,
		}, nil
	})
	stub(t, &cfHourly, func(context.Context, string, string, time.Time) ([]collectors.CFHour, error) {
		return hoursWith(2, 9, 0), nil
	})
	stub(t, &cfPaths, func(context.Context, string, string, time.Time) ([]collectors.CFCount, []collectors.CFPathError, error) {
		return []collectors.CFCount{{Name: "/", Requests: 50}}, []collectors.CFPathError{{Path: "/wp-login.php", Status: 404, Count: 12}}, nil
	})
	calls := 0
	stub(t, &cfSecurity, func(context.Context, string, collectors.CFZone) collectors.CFSecurity {
		calls++
		return collectors.CFSecurity{Settings: map[string]string{"ssl": "strict", "always_use_https": "on"},
			Certs: []collectors.CFCert{{Expires: fixedNow.Add(60 * 24 * time.Hour)}}}
	})

	a.cloudflareAccount(context.Background())
	if s := a.summaries(a.store.Get().Projects); len(s[0].Domains) != 1 || s[0].Domains[0] != "app.exemplo.dev.br" {
		t.Fatalf("snapshot sem o domínio: %+v", s[0].Domains)
	}

	out := mustRun(t, a, "cloudflare", `{"project_id":"`+pid+`"}`)
	for _, want := range []string{
		`"zone":"exemplo.dev.br"`, `"host":"app.exemplo.dev.br"`, `"linked":".env APP_URL"`,
		`"site":"https://app.exemplo.dev.br"`, `"dash":"https://dash.cloudflare.com/acc/exemplo.dev.br"`,
		`"visits":100`, `"prev_visits":80`, `"errors":6`, `"cache_pct":40`, `"daily":[40,60]`, `"peak_visits":9`,
		`"countries":[{"name":"BR","count":900}]`, `"top_pages":[{"name":"/","count":50}]`,
		`"failed":[{"path":"/wp-login.php","status":404,"count":12}]`, `"ssl_mode":"strict"`, `"always_https":true`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("faltou %s em %s", want, out)
		}
	}
	// A segurança vem do cache na segunda consulta: o Telegram não espera o RDAP.
	mustRun(t, a, "cloudflare", `{"project_id":"`+pid+`"}`)
	if calls != 1 {
		t.Fatalf("segurança lida %d vezes", calls)
	}
	_, err = run(t, a, "cloudflare", `{"project_id":"0000000000000001"}`)
	expectCode(t, err, "not_found")
}
