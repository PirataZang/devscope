package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

var (
	cfTestZone  = collectors.CFZone{ID: "z1", Name: "exemplo.dev.br", Status: "active", Plan: "Free Website", AccountID: "acc1", NameServers: []string{"ana.ns.cloudflare.com", "bob.ns.cloudflare.com"}}
	cfOtherZone = collectors.CFZone{ID: "z2", Name: "outro.com", Status: "pending", Plan: "Free Website", AccountID: "acc1"}
)

func cfTestDay(d, visits, req int64) collectors.CFDay {
	return collectors.CFDay{Date: time.Date(2026, 10, int(d), 0, 0, 0, 0, time.UTC), Visits: visits, Requests: req, Cached: req / 2, Bytes: 4 << 20}
}

// cloudflareTestApp: lista carregada, com outro.com antes na conta para
// conferir que o domínio do projeto sobe para o topo.
func cloudflareTestApp(width int) (*App, *core.Project) {
	p := &core.Project{Name: "site", Path: "/p/site"}
	a := &App{width: width, height: 44, view: ViewProject, tab: TabCloudflare, selectedProject: p,
		snapshot: core.Snapshot{Projects: []core.Project{*p}}}
	a.cfd = cloudflareState{token: true, acctOK: true, path: p.Path, days: 7,
		acct: collectors.CFAccount{Zones: []collectors.CFZone{cfOtherZone, cfTestZone}}}
	week := collectors.CFTraffic{Days: []collectors.CFDay{cfTestDay(21, 40, 400), cfTestDay(22, 120, 900), cfTestDay(23, 80, 700)},
		Prev: collectors.CFDay{Visits: 200}, HasPrev: true}
	a.handleCloudflareList(cfdListMsg{path: p.Path,
		links: []collectors.CFLink{{Zone: cfTestZone, Host: "exemplo.dev.br", From: ".env APP_URL"}},
		week:  map[string]collectors.CFTraffic{"z1": week}, today: time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC)})
	return a, p
}

// openTestDomain: enter no primeiro da lista e a leitura do domínio chegando.
func openTestDomain(t *testing.T, a *App, p *core.Project) {
	t.Helper()
	if _, ok := a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); !ok || !a.cfd.open || a.cfd.zone.ID != "z1" {
		t.Fatalf("enter abre o domínio: %+v", a.cfd.zone)
	}
	day := a.cfd.days
	traffic := collectors.CFTraffic{
		Days:      []collectors.CFDay{cfTestDay(21, 40, 400), cfTestDay(22, 120, 900), cfTestDay(23, 80, 700)},
		Status:    []collectors.CFCount{{Name: "2xx", Requests: 1800}, {Name: "3xx", Requests: 0}, {Name: "4xx", Requests: 150}, {Name: "5xx", Requests: 50}},
		Countries: []collectors.CFCount{{Name: "BR", Requests: 1600}, {Name: "US", Requests: 400}},
		Prev:      collectors.CFDay{Visits: 200}, HasPrev: true,
	}
	traffic.Days[0].Errors = 3
	first := time.Date(2026, 10, 22, 16, 0, 0, 0, time.UTC)
	var hours []collectors.CFHour
	for i := 0; i < 24; i++ {
		hours = append(hours, collectors.CFHour{Time: first.Add(time.Duration(i) * time.Hour), Visits: int64((i * 7) % 30)})
	}
	a.handleCloudflareDetail(cfdDetailMsg{zone: "z1", days: day, today: time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC),
		traffic: traffic, hourly: hours,
		pages:  []collectors.CFCount{{Name: "/", Requests: 320}, {Name: "/sobre", Requests: 80}},
		failed: []collectors.CFPathError{{Path: "/wp-login.php", Status: 404, Count: 52}},
		dns: []collectors.CFRecord{{Type: "A", Name: "exemplo.dev.br", Content: "178.104.78.64", Proxied: true},
			{Type: "CNAME", Name: "www.exemplo.dev.br", Content: "exemplo.dev.br"}},
		sec: collectors.CFSecurity{
			Settings: map[string]string{"ssl": "flexible", "always_use_https": "on", "min_tls_version": "1.2", "security_level": "medium"},
			Certs:    []collectors.CFCert{{Type: "universal", Authority: "google", Status: "active", Expires: time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)}},
			Insights: []collectors.CFInsight{{Severity: "moderate", Subject: "exemplo.dev.br", Resolve: "Ative o DNSSEC"}},
			Expires:  time.Date(2027, 9, 3, 13, 0, 0, 0, time.UTC), Registry: "rdap.registro.br",
		}})
}

func TestCloudflareListThenDomainThenBack(t *testing.T) {
	a, p := cloudflareTestApp(140)
	got := stripANSI(a.renderCloudflareTab(p))
	for _, want := range []string{
		"⇅ CLOUDFLARE", "DOMÍNIOS · 2", "DOMÍNIO", "VISITANTES · 7 DIAS",
		"▸ exemplo.dev.br", "◆ ativo", "240", "↑ 20%", "◆ este projeto", "outro.com", "◌ pending",
		"https://exemplo.dev.br", ".env APP_URL", "ana.ns.cloudflare.com", "ÚLTIMOS 7 DIAS", "ontem",
		"enter abrir o domínio",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("lista: faltou %q:\n%s", want, got)
		}
	}
	// O domínio do projeto vem primeiro, mesmo sendo o segundo da conta.
	if strings.Index(got, "exemplo.dev.br") > strings.Index(got, "outro.com") {
		t.Fatalf("ordem:\n%s", got)
	}

	openTestDomain(t, a, p)
	got = stripANSI(a.renderCloudflareTab(p))
	for _, want := range []string{
		"‹ esc", "1 TRÁFEGO", "2 DNS", "3 SEGURANÇA",
		"RESUMO · últimos 7 dias", "visitantes  240", "↑ 20% vs 7 dias antes",
		"50% servidas do cache", "2xx 90%", "4xx 7.5%", "5xx 2.5%", "Brasil 80%",
		"ÚLTIMAS 24 HORAS", "┤", "agora",
		"POR DIA", "hoje", "ontem", "qua 21/10", "3 erros 5xx", "pico",
		"PÁGINAS MAIS ACESSADAS · 24h", "/sobre", "CAMINHOS COM ERRO · 24h", "404", "/wp-login.php",
		"esc domínios",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("tráfego: faltou %q:\n%s", want, got)
		}
	}

	a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}, p)
	got = stripANSI(a.renderCloudflareTab(p))
	for _, want := range []string{"REGISTROS · 2", "◆ 1 passam pelo proxy", "APONTA PARA", "178.104.78.64", "www", "· só DNS"} {
		if !strings.Contains(got, want) {
			t.Fatalf("DNS: faltou %q:\n%s", want, got)
		}
	}

	a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")}, p)
	got = stripANSI(a.renderCloudflareTab(p))
	for _, want := range []string{
		"SSL / TLS", "⚠ Flexible", "sem criptografia", "◆ sempre redireciona para HTTPS", "1.2",
		"Universal (grátis)", "vence 01/11/2026 (em 9 dias)",
		"AVISOS DE SEGURANÇA · 1", "◌ moderado", "Ative o DNSSEC",
		"registro", "vence 03/09/2027", "rdap.registro.br", "nível medium",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("segurança: faltou %q:\n%s", want, got)
		}
	}

	if _, ok := a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyEsc}, p); !ok || a.cfd.open {
		t.Fatal("esc volta para a lista")
	}
	if got = stripANSI(a.renderCloudflareTab(p)); !strings.Contains(got, "DOMÍNIOS · 2") {
		t.Fatalf("de volta à lista:\n%s", got)
	}
	// Na lista, esc não é do módulo: sai dele como em qualquer outro.
	if _, ok := a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyEsc}, p); ok {
		t.Fatal("esc na lista sai do módulo")
	}
}

func TestCloudflarePeriodTogglesAndReloads(t *testing.T) {
	a, p := cloudflareTestApp(120)
	openTestDomain(t, a, p)
	if cmd, ok := a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyRight}, p); !ok || cmd == nil || a.cfd.days != 30 {
		t.Fatalf("←→ troca o período e recarrega: %d", a.cfd.days)
	}
	// Resposta atrasada do período antigo não sobrescreve o novo.
	a.handleCloudflareDetail(cfdDetailMsg{zone: "z1", days: 7, traffic: collectors.CFTraffic{Days: []collectors.CFDay{cfTestDay(1, 999, 1)}}})
	if a.cfd.traffic.Total().Visits == 999 {
		t.Fatal("resposta de 7 dias chegou depois e venceu")
	}
}

func TestCloudflareCharts(t *testing.T) {
	if s := cfdSpark([]int64{0, 4, 8}); s != "▁▄█" {
		t.Fatalf("spark %q", s)
	}
	if s := stripANSI(cfdHBar(5, 10, 4, StyleNormal)); s != "██░░" {
		t.Fatalf("barra %q", s)
	}
	if s := stripANSI(cfdHBar(3, 8, 2, StyleNormal)); s != "▊░" {
		t.Fatalf("ponta em oitavos %q", s)
	}
	if s := stripANSI(cfdStatusBar([]collectors.CFCount{{Name: "2xx", Requests: 999}, {Name: "5xx", Requests: 1}}, 10)); !strings.HasPrefix(s, "██████████  2xx 100% · 5xx 0.1%") {
		t.Fatalf("5xx raro não some da legenda: %q", s)
	}
}

// Nenhuma largura nem altura estoura a tela, em nenhuma das telas.
func TestCloudflareTabFits(t *testing.T) {
	for _, size := range [][2]int{{60, 44}, {80, 30}, {100, 24}, {160, 50}, {140, 16}} {
		for sub := -1; sub < 3; sub++ {
			a, p := cloudflareTestApp(size[0])
			a.height = size[1]
			if sub >= 0 {
				openTestDomain(t, a, p)
				a.cfd.sub = sub
			}
			out := a.renderCloudflareTab(p)
			_, h := a.moduleSize()
			if lines := strings.Split(out, "\n"); len(lines) > h {
				t.Fatalf("%v aba %d: %d linhas para %d", size, sub, len(lines), h)
			}
			for i, l := range strings.Split(out, "\n") {
				if w := lipgloss.Width(l); w > size[0] {
					t.Fatalf("%v aba %d: linha %d com %d colunas: %q", size, sub, i, w, stripANSI(l))
				}
			}
		}
	}
}

func TestCloudflareTabWithoutToken(t *testing.T) {
	a, p := cloudflareTestApp(120)
	a.cfd = cloudflareState{acctOK: true, path: p.Path}
	got := stripANSI(a.renderCloudflareTab(p))
	for _, want := range []string{"CONECTAR A CLOUDFLARE", "devscope cloudflare login", "CLOUDFLARE_API_TOKEN", "o criar o token"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
}

// Projeto sem vínculo: v liga o domínio selecionado ao projeto.
func TestCloudflareTabLinksByHand(t *testing.T) {
	a, _ := cloudflareTestApp(120)
	dir := t.TempDir()
	p := &core.Project{Name: "portfolio", Path: dir}
	a.selectedProject, a.snapshot = p, core.Snapshot{Projects: []core.Project{*p}}
	a.cfd.path, a.cfd.links, a.cfd.cursor = dir, nil, 0
	if got := stripANSI(a.renderCloudflareTab(p)); !strings.Contains(got, "nenhum domínio da conta aparece neste projeto") || !strings.Contains(got, "v vincular a este projeto") {
		t.Fatalf("%s", got)
	}
	a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyDown}, p)
	cmd, _ := a.handleCloudflareKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")}, p)
	a.handleCloudflareLinked(cmd().(cfdLinkedMsg))
	links := collectors.LinkCloudflare(dir, "", a.cfd.acct)
	if len(links) != 1 || links[0].Host != "exemplo.dev.br" || links[0].From != collectors.CloudflareLinkFile {
		t.Fatalf("%+v", links)
	}
}
