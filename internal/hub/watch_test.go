package hub

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func snapWith(php core.Container, disk float64, ssl ...core.SSLCert) core.Snapshot {
	return core.Snapshot{
		HostMetrics: core.HostMetrics{DiskPercent: disk, DiskUsedGB: disk, DiskTotalGB: 100},
		Projects: []core.Project{{ID: pid, Name: "digiliza", SSL: ssl, Containers: []core.Container{
			php,
			{ID: "bbb", Name: "laradock-postgres-1", Service: "postgres", Status: "running"},
		}}},
	}
}

func php(status, health string) core.Container {
	return core.Container{ID: "aaa", Name: "laradock-php-fpm-1", Service: "php-fpm", Status: status, Health: health}
}

func kinds(evs []Event) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Kind)
	}
	return out
}

func TestWatcherContainers(t *testing.T) {
	isolate(t)
	stub(t, &dockerExitState, func(string) (int, bool, error) { return 137, true, nil })
	w := newWatcher()
	at := fixedNow
	step := func(c core.Container, after time.Duration) ([]Event, bool) {
		at = at.Add(after)
		return w.step(snapWith(c, 50), at)
	}

	// Primeira leitura com o container já caído: só referência, nenhum aviso.
	if evs, changed := step(php("exited", ""), 0); len(evs) != 0 || changed {
		t.Fatalf("primeira leitura gerou %v", evs)
	}
	if evs, _ := step(php("running", ""), 5*time.Second); len(evs) != 0 {
		t.Fatalf("subir sem ter avisado a queda não é aviso: %v", evs)
	}

	evs, changed := step(php("exited", ""), 5*time.Second)
	if len(evs) != 1 || !changed {
		t.Fatalf("queda: %v changed=%v", evs, changed)
	}
	down := evs[0]
	if down.Kind != "container_down" || down.Key != "ctr:laradock-php-fpm-1" || down.Container != "php-fpm" ||
		down.Title != "php-fpm caiu" || down.Detail != "saiu com código 137 (sem memória)" ||
		down.ExitCode == nil || *down.ExitCode != 137 || down.Count != 1 || down.ProjectID != pid || down.Project != "digiliza" {
		t.Fatalf("evento de queda: %+v", down)
	}
	if evs, changed := step(php("exited", ""), 5*time.Second); len(evs) != 0 || changed {
		t.Fatalf("continuar caído não repete o aviso: %v", evs)
	}

	evs, _ = step(php("running", ""), 5*time.Second)
	if len(evs) != 1 || evs[0].Kind != "container_up" || evs[0].Key != down.Key || evs[0].Title != "php-fpm voltou" {
		t.Fatalf("volta: %+v", evs)
	}

	// Cair de novo em menos de 10 minutos sobe o count, com a mesma key.
	evs, _ = step(php("restarting", ""), time.Minute)
	if len(evs) != 1 || evs[0].Count != 2 || evs[0].Key != down.Key || evs[0].Detail != "reiniciando sem parar (último código 137) (sem memória)" {
		t.Fatalf("repetição: %+v", evs)
	}
	step(php("running", ""), 5*time.Second)
	evs, _ = step(php("dead", ""), 11*time.Minute)
	if len(evs) != 1 || evs[0].Count != 1 {
		t.Fatalf("depois de 10 minutos o count recomeça: %+v", evs)
	}

	// Healthcheck: unhealthy é queda, healthy de novo é resolved.
	step(php("running", "healthy"), 5*time.Second)
	evs, _ = step(php("running", "unhealthy"), 5*time.Second)
	if len(evs) != 1 || evs[0].Kind != "container_down" || evs[0].Title != "php-fpm está unhealthy" || evs[0].ExitCode != nil {
		t.Fatalf("unhealthy: %+v", evs)
	}
	evs, _ = step(php("running", "healthy"), 5*time.Second)
	if len(evs) != 1 || evs[0].Kind != "resolved" {
		t.Fatalf("healthy de novo: %+v", evs)
	}
}

func TestWatcherDiskAndSSL(t *testing.T) {
	isolate(t)
	w := newWatcher()
	at := fixedNow
	step := func(disk float64, ssl ...core.SSLCert) []Event {
		at = at.Add(5 * time.Second)
		evs, _ := w.step(snapWith(php("running", ""), disk, ssl...), at)
		return evs
	}
	old := core.SSLCert{Domain: "velho.com.br", DaysLeft: 5}
	if evs := step(95, old); len(evs) != 0 {
		t.Fatalf("primeira leitura: %v", evs)
	}
	if evs := step(95, old); len(evs) != 0 {
		t.Fatalf("já estava acima de 90 e o SSL já era conhecido: %v", evs)
	}
	if evs := step(88, old); len(evs) != 0 {
		t.Fatalf("88%% não desarma (precisa cair 5 pontos): %v", evs)
	}
	if evs := step(84, old); len(evs) != 1 || evs[0].Kind != "resolved" || evs[0].Key != "disk" {
		t.Fatalf("caiu abaixo de 85: %v", evs)
	}
	evs := step(92, old)
	if len(evs) != 1 || evs[0].Kind != "disk" || evs[0].Title != "disco em 92%" || evs[0].Detail != "92 de 100 GB usados" {
		t.Fatalf("subiu de novo: %+v", evs)
	}
	if evs := step(97, old); len(evs) != 0 {
		t.Fatalf("continua alto, sem repetir: %v", evs)
	}

	novo := core.SSLCert{Domain: "loja.com.br", DaysLeft: 3}
	evs = step(97, old, novo)
	if len(evs) != 1 || evs[0].Kind != "ssl" || evs[0].Key != "ssl:loja.com.br" || evs[0].Title != "SSL de loja.com.br vence em 3 dias" {
		t.Fatalf("SSL perto de vencer: %+v", evs)
	}
	if evs := step(97, old, novo); len(evs) != 0 {
		t.Fatalf("SSL só uma vez por dia: %v", evs)
	}
	at = at.Add(24 * time.Hour)
	if evs := step(97, old, novo); len(kinds(evs)) != 2 {
		t.Fatalf("um dia depois os dois avisam de novo: %v", kinds(evs))
	}
}

func TestCIWatcher(t *testing.T) {
	a := fixtureAgent(t)
	runs := []collectors.GHARun{
		{ID: "2", Workflow: "tests", Branch: "main", Status: "in_progress"},
		{ID: "1", Workflow: "tests", Branch: "main", Status: "completed", Conclusion: "success"},
	}
	stub(t, &ghaListRuns, func(_ string, limit int) ([]collectors.GHARun, error) {
		if limit != 5 {
			t.Fatalf("limit = %d", limit)
		}
		return runs, nil
	})
	projects := a.store.Get().Projects[:1]
	w := newCIWatcher()
	if evs := w.step(a, projects); len(evs) != 0 {
		t.Fatalf("primeira leitura: %v", evs)
	}
	runs = []collectors.GHARun{
		{ID: "3", Workflow: "deploy", Branch: "main", Status: "completed", Conclusion: "success"},
		{ID: "2", Workflow: "tests", Branch: "main", Status: "completed", Conclusion: "failure", DisplayTitle: "token sk-proj-abcdefghijklmnop"},
		{ID: "1", Workflow: "tests", Branch: "main", Status: "completed", Conclusion: "success"},
	}
	evs := w.step(a, projects)
	if len(evs) != 2 || evs[0].Kind != "ci_failed" || evs[0].Key != "ci:"+pid+":tests:main" ||
		evs[0].Title != "CI falhou: tests (main)" || evs[0].Detail != "token sk-p••••••••" || evs[1].Kind != "ci_ok" {
		t.Fatalf("runs que terminaram: %+v", evs)
	}
	if evs := w.step(a, projects); len(evs) != 0 {
		t.Fatalf("o mesmo run não avisa duas vezes: %v", evs)
	}
	if ci := a.projectCI(projects[0]); ci == nil || ci.Workflow != "deploy" {
		t.Fatalf("o vigia de CI alimenta o cache do snapshot: %+v", ci)
	}
}

func TestDelayedEventsQueue(t *testing.T) {
	isolate(t)
	key := pairMachine(t)
	f := newFakeHub(t, nil)
	url := f.URL
	f.Close() // hub fora do ar
	a := newAgent(NewClient(url, key), core.NewStateStore(nil), testConfig(url), "service")
	a.now = func() time.Time { return fixedNow }
	ctx := context.Background()

	old := Event{Key: "disk", Kind: "disk", Title: "velho", At: fixedNow.Add(-25 * time.Hour)}
	a.queue = []Event{old}
	a.sendEvents(ctx, []Event{{Key: "ctr:x", Kind: "container_down", Title: "x caiu", At: fixedNow}})
	q := LoadState().Queue
	if len(q) != 1 || q[0].Key != "ctr:x" || !q[0].Delayed {
		t.Fatalf("fila: %+v", q)
	}

	// Até 100 eventos: os mais velhos saem.
	for i := range 120 {
		a.sendEvents(ctx, []Event{{Key: "k", Kind: "disk", Title: string(rune('a' + i%26)), At: fixedNow}})
	}
	if n := len(LoadState().Queue); n != 100 {
		t.Fatalf("fila com %d eventos", n)
	}

	// O hub volta: a fila sai inteira, marcada como atrasada, e esvazia.
	var got []any
	f2 := newFakeHub(t, map[string]func(http.ResponseWriter, map[string]any){
		"/v1/events": func(w http.ResponseWriter, body map[string]any) {
			got = body["events"].([]any)
			_, _ = io.WriteString(w, `{}`)
		},
	})
	a.client = NewClient(f2.URL, key)
	a.sendEvents(ctx, nil)
	if len(got) != 100 || got[0].(map[string]any)["delayed"] != true {
		t.Fatalf("eventos atrasados: %d", len(got))
	}
	if len(LoadState().Queue) != 0 || len(a.queue) != 0 {
		t.Fatal("a fila não esvaziou")
	}
}
