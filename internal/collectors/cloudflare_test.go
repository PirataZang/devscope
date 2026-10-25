package collectors

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeCloudflare(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"success":false,"errors":[{"message":"Invalid API Token"}]}`)
			return
		}
		switch r.URL.Path {
		case "/zones":
			_, _ = io.WriteString(w, `{"success":true,"result_info":{"total_pages":1},"result":[
				{"id":"z1","name":"exemplo.dev.br","status":"active","plan":{"name":"Free Website"},"account":{"id":"acc1"}},
				{"id":"z2","name":"outro.com","status":"pending","plan":{"name":"Free Website"},"account":{"id":"acc1"}}]}`)
		case "/accounts/acc1/pages/projects":
			_, _ = io.WriteString(w, `{"success":true,"result_info":{"total_pages":1},"result":[
				{"name":"site","subdomain":"site.pages.dev","domains":["site.pages.dev","www.outro.com"],
				 "source":{"type":"github","config":{"owner":"igor","repo_name":"site"}}}]}`)
		case "/zones/z1/dns_records":
			_, _ = io.WriteString(w, `{"success":true,"result_info":{"total_pages":1},"result":[
				{"type":"A","name":"exemplo.dev.br","content":"178.104.78.64","proxied":true}]}`)
		case "/graphql":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"zone":"z1"`) {
				t.Errorf("graphql sem a zona: %s", body)
			}
			_, _ = io.WriteString(w, `{"data":{"viewer":{"zones":[{"httpRequests1dGroups":[
				{"dimensions":{"date":"2026-10-12"},"uniq":{"uniques":90},"sum":{"requests":10,"pageViews":5,
				 "responseStatusMap":[{"edgeResponseStatus":500,"requests":4}],"countryMap":[{"clientCountryName":"DE","requests":10}]}},
				{"dimensions":{"date":"2026-10-22"},"uniq":{"uniques":100},"sum":{"requests":1000,"cachedRequests":400,"bytes":2048,"cachedBytes":1024,"threats":0,"pageViews":300,
				 "responseStatusMap":[{"edgeResponseStatus":200,"requests":900},{"edgeResponseStatus":404,"requests":80},{"edgeResponseStatus":502,"requests":20}],
				 "countryMap":[{"clientCountryName":"BR","requests":800},{"clientCountryName":"US","requests":200}]}},
				{"dimensions":{"date":"2026-10-23"},"uniq":{"uniques":50},"sum":{"requests":500,"cachedRequests":100,"bytes":1024,"cachedBytes":0,"threats":3,"pageViews":120,
				 "responseStatusMap":[{"edgeResponseStatus":200,"requests":500}],"countryMap":[{"clientCountryName":"US","requests":500}]}}]}]}},"errors":null}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := cfAPI
	cfAPI = srv.URL
	t.Cleanup(func() { cfAPI = old })
}

func TestCloudflareAccountTrafficAndDNS(t *testing.T) {
	fakeCloudflare(t)
	ctx := context.Background()
	if _, err := FetchCloudflareAccount(ctx, "errado"); err == nil || !strings.Contains(err.Error(), "Invalid API Token") {
		t.Fatalf("token recusado: %v", err)
	}
	acc, err := FetchCloudflareAccount(ctx, "tok")
	if err != nil || len(acc.Zones) != 2 || len(acc.Pages) != 1 || acc.Pages[0].Repo != "igor/site" {
		t.Fatalf("%+v %v", acc, err)
	}
	if CloudflareDashURL(acc.Zones[0]) != "https://dash.cloudflare.com/acc1/exemplo.dev.br" {
		t.Fatal(CloudflareDashURL(acc.Zones[0]))
	}

	tr, err := FetchCloudflareTraffic(ctx, "tok", "z1", 7, time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if tot := tr.Total(); tot.Visits != 150 || tot.Requests != 1500 || tot.Threats != 3 || len(tr.Days) != 2 || tot.Errors != 20 {
		t.Fatalf("%+v", tot)
	}
	// O dia de antes da janela é o período anterior, e não entra nos países.
	if !tr.HasPrev || tr.Prev.Visits != 90 || tr.Prev.Errors != 4 || tr.Days[0].Errors != 20 {
		t.Fatalf("anterior: %+v %v", tr.Prev, tr.HasPrev)
	}
	if tr.Status[0] != (CFCount{"2xx", 1400}) || tr.Status[2] != (CFCount{"4xx", 80}) || tr.Status[3] != (CFCount{"5xx", 20}) {
		t.Fatalf("%+v", tr.Status)
	}
	if tr.Countries[0] != (CFCount{"BR", 800}) || tr.Countries[1] != (CFCount{"US", 700}) {
		t.Fatalf("%+v", tr.Countries)
	}
	dns, err := FetchCloudflareDNS(ctx, "tok", "z1")
	if err != nil || len(dns) != 1 || !dns[0].Proxied || dns[0].Content != "178.104.78.64" {
		t.Fatalf("%+v %v", dns, err)
	}
}

// O domínio é do projeto pelo Pages ligado ao mesmo repositório ou por um
// endereço da conta escrito nos arquivos; endereço de fora da conta não liga.
func TestLinkCloudflare(t *testing.T) {
	acc := CFAccount{
		Zones: []CFZone{{ID: "z1", Name: "exemplo.dev.br"}, {ID: "z2", Name: "outro.com"}},
		Pages: []CFPagesProject{{Name: "site", Repo: "igor/site", Domains: []string{"site.pages.dev", "www.outro.com"}}},
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_URL=https://app.exemplo.dev.br\nMAIL=no-reply@gmail.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := LinkCloudflare(dir, "git@github.com:igor/site.git", acc)
	if len(links) != 2 {
		t.Fatalf("%+v", links)
	}
	if links[0].Host != "www.outro.com" || links[0].Zone.ID != "z2" || links[0].From != "Pages site ← GitHub igor/site" {
		t.Fatalf("%+v", links[0])
	}
	if links[1].Host != "app.exemplo.dev.br" || links[1].Zone.ID != "z1" || links[1].From != ".env APP_URL" {
		t.Fatalf("%+v", links[1])
	}
	if got := LinkCloudflare(dir, "https://github.com/igor/outro", CFAccount{Zones: acc.Zones[1:]}); len(got) != 0 {
		t.Fatalf("sem vínculo: %+v", got)
	}
}
