package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesAPI(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabAPI {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabAPI missing from AllTabs")
	}
	if TabAPI.String() != "API" {
		t.Fatalf("String=%q", TabAPI.String())
	}
}

func TestInitApiTabSuggestsPort(t *testing.T) {
	a := &App{}
	p := &core.Project{Ports: []int{3000, 8080}}
	a.initApiTab(p)
	if a.apiURL != "http://localhost:3000" {
		t.Fatalf("url=%q", a.apiURL)
	}
	if a.apiMethod != "GET" {
		t.Fatalf("method=%q", a.apiMethod)
	}
	if !strings.Contains(a.apiHeaders, "Accept:") {
		t.Fatalf("headers=%q", a.apiHeaders)
	}
}

func TestInitApiTabKeepsExistingURL(t *testing.T) {
	a := &App{apiURL: "http://example.com/v1"}
	p := &core.Project{Ports: []int{9999}}
	a.initApiTab(p)
	if a.apiURL != "http://example.com/v1" {
		t.Fatalf("url overwritten: %q", a.apiURL)
	}
}

func TestCycleApiPortKeepsPath(t *testing.T) {
	a := &App{apiURL: "http://localhost:3000/api/users"}
	p := &core.Project{Ports: []int{3000, 8080}}
	a.cycleApiPort(p)
	if a.apiURL != "http://localhost:8080/api/users" {
		t.Fatalf("url=%q", a.apiURL)
	}
}

func TestPushApiHistoryDedupAndCap(t *testing.T) {
	a := &App{}
	for i := 0; i < 12; i++ {
		a.pushApiHistory("GET", fmt.Sprintf("http://localhost/%d", i))
	}
	a.pushApiHistory("GET", "http://localhost/0")
	if len(a.apiHistory) != 10 {
		t.Fatalf("len=%d", len(a.apiHistory))
	}
	if a.apiHistory[0].URL != "http://localhost/0" {
		t.Fatalf("first=%+v", a.apiHistory[0])
	}
}

func TestApiSearchMatches(t *testing.T) {
	a := &App{
		apiResponseBody: "hello\nFOO bar\nbaz\nfoo again",
		apiSearchQuery:  "foo",
	}
	matches := a.apiSearchMatches()
	if len(matches) != 2 || matches[0] != 1 || matches[1] != 3 {
		t.Fatalf("matches=%v", matches)
	}
}

func TestApiPrintableKeyIncludesSlash(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	if !apiPrintableKey(msg) {
		t.Fatal("/ should be printable for URL editing")
	}
	slash := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")}
	_ = slash
}

func TestApiLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "p", Ports: []int{3000}}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterApiTab(&p)
	if a.tab != TabAPI || a.apiOpen {
		t.Fatalf("7 should open landing, tab=%v open=%v", a.tab, a.apiOpen)
	}
	landing := stripANSI(a.renderApiLanding(&p))
	if !strings.Contains(landing, "enter") {
		t.Fatalf("landing should prompt enter: %q", landing)
	}

	_, _ = a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.apiOpen || a.tab != TabAPI {
		t.Fatalf("enter should open client, open=%v tab=%v", a.apiOpen, a.tab)
	}

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.apiOpen || a.tab != TabAPI || a.view != ViewProject {
		t.Fatalf("esc should return to tab 8 landing, open=%v tab=%v view=%v", a.apiOpen, a.tab, a.view)
	}
}

func TestFitApiFieldWindowFollowsCursor(t *testing.T) {
	url := "https://pokeapi.co/api/v2/pokemon/ditto"
	cursor := len([]rune(url))
	got := fitApiFieldWindow(url, cursor, 20, true)
	if !strings.Contains(got, "█") {
		t.Fatalf("missing cursor: %q", got)
	}
	if !strings.Contains(got, "ditto") {
		t.Fatalf("should show end while typing: %q", got)
	}
	if strings.HasPrefix(got, "https://") {
		t.Fatalf("should scroll away from start: %q", got)
	}

	idle := fitApiFieldWindow(url, 0, 20, false)
	if !strings.Contains(idle, "ditto") {
		t.Fatalf("idle should show URL end: %q", idle)
	}
}

func TestApiMethodStyleColors(t *testing.T) {
	get := apiMethodStyle("GET").Render("GET")
	post := apiMethodStyle("POST").Render("POST")
	del := apiMethodStyle("DELETE").Render("DELETE")
	if get == post || get == del || post == del {
		t.Fatalf("method colors should differ: get=%q post=%q del=%q", get, post, del)
	}
}

// ── Tela nova (padrão Postman) ────────────────────────────────────────────────

func newApiTestApp() (*App, *core.Project) {
	p := &core.Project{Name: "demo", Path: "/tmp/demo", Ports: []int{3000, 8080}}
	a := &App{width: 110, height: 34, view: ViewProject, tab: TabAPI, apiOpen: true,
		selectedProject: p, now: time.Now()}
	a.initApiTab(p)
	return a, p
}

// As abas são numeradas e o número é a ÚNICA forma de trocar (DESIGN.md §3). E
// trocar de aba leva o foco pro painel: número que não move o foco obriga a
// apertar tab depois, e ninguém lembra disso.
func TestApiTabsSwitchBySpecificNumberAndMoveFocus(t *testing.T) {
	a, p := newApiTestApp()
	for i, want := range []apiSection{apiSecParams, apiSecHeaders, apiSecAuth, apiSecBody, apiSecResponse} {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + i)}}
		_, _ = a.handleApiKeys(key, p)
		if a.apiSection != want {
			t.Fatalf("tecla %d deveria abrir a aba %v, got %v", i+1, want, a.apiSection)
		}
		wantFocus := apiFocusSection
		if want == apiSecResponse {
			wantFocus = apiFocusResponse
		}
		if a.apiFocus != wantFocus {
			t.Fatalf("aba %v deveria focar %v, got %v", want, wantFocus, a.apiFocus)
		}
	}
	// Seta nunca troca de aba.
	a.apiSetSection(apiSecParams)
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRight}, p)
	if a.apiSection != apiSecParams {
		t.Fatalf("seta trocou de aba: %v", a.apiSection)
	}
}

// tab caminha url → aba → resposta → url. Três paradas, sempre na mesma ordem.
func TestApiTabCyclesThreeFocuses(t *testing.T) {
	a, p := newApiTestApp()
	a.apiSetSection(apiSecHeaders)
	a.apiFocus = apiFocusURL
	want := []apiFocus{apiFocusSection, apiFocusResponse, apiFocusURL}
	for i, w := range want {
		_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyTab}, p)
		if a.apiFocus != w {
			t.Fatalf("tab %d: esperava %v, got %v", i+1, w, a.apiFocus)
		}
	}
}

// A tabela de params é uma VISTA da query: mexer na linha reescreve a URL, e
// mexer na URL remonta as linhas. Sem "aplicar" no meio.
func TestApiParamsTableAndURLStayInSync(t *testing.T) {
	a, p := newApiTestApp()
	a.apiURL = "http://localhost:3000/pedidos?page=2&status=pago"
	a.apiSyncParamsFromURL()
	if len(a.apiParams) != 2 || a.apiParams[0].Key != "page" || a.apiParams[1].Value != "pago" {
		t.Fatalf("query não virou tabela: %+v", a.apiParams)
	}

	// Editar o valor da primeira linha reescreve a URL.
	a.apiSetSection(apiSecParams)
	a.apiRowCursor, a.apiRowField = 0, 1
	a.beginApiEdit()
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyCtrlA})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	if a.apiURL != "http://localhost:3000/pedidos?page=7&status=pago" {
		t.Fatalf("URL não seguiu a tabela: %q", a.apiURL)
	}

	// Desligar a linha tira ela da URL mas NÃO a perde.
	a.apiEditing = false
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}, p)
	if a.apiURL != "http://localhost:3000/pedidos?status=pago" {
		t.Fatalf("linha desligada continuou na URL: %q", a.apiURL)
	}
	if len(a.apiParams) != 2 || a.apiParams[0].On {
		t.Fatalf("a linha desligada tinha de continuar na tabela: %+v", a.apiParams)
	}
	// E ligar de volta devolve o parâmetro ao lugar.
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}, p)
	if !strings.Contains(a.apiURL, "page=7") {
		t.Fatalf("religar não devolveu o parâmetro: %q", a.apiURL)
	}
}

func TestApiParamsRowAddAndDelete(t *testing.T) {
	a, p := newApiTestApp()
	a.apiURL = "http://localhost:3000/x"
	a.apiSyncParamsFromURL()
	a.apiSetSection(apiSecParams)

	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, p)
	if !a.apiEditing || len(a.apiParams) != 1 {
		t.Fatalf("n deveria criar a linha e abrir a edição: editing=%v rows=%+v", a.apiEditing, a.apiParams)
	}
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("limit")})
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyTab}) // chave → valor
	_, _ = a.updateApiEdit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("10")})
	if a.apiURL != "http://localhost:3000/x?limit=10" {
		t.Fatalf("linha nova não entrou na URL: %q", a.apiURL)
	}

	a.apiEditing = false
	a.apiRowCursor = 0
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, p)
	if len(a.apiParams) != 0 || a.apiURL != "http://localhost:3000/x" {
		t.Fatalf("d não apagou: rows=%+v url=%q", a.apiParams, a.apiURL)
	}
}

// Header desligado vira comentário no bloco de texto — e parseAPIHeaders já
// ignora `#`, então o cliente HTTP não precisou saber da tabela.
func TestApiHeaderRowsRoundTripThroughText(t *testing.T) {
	rows := []apiKV{
		{On: true, Key: "Accept", Value: "application/json"},
		{On: false, Key: "X-Debug", Value: "1"},
	}
	text := apiBuildHeaderText(rows)
	back := apiParseHeaderRows(text)
	if len(back) != 2 || back[0].Value != "application/json" || back[1].On {
		t.Fatalf("ida e volta perdeu informação: %+v", back)
	}
	h := parseAPIHeaders(text)
	if h.Get("Accept") != "application/json" {
		t.Fatalf("Accept não chegou no cliente: %v", h)
	}
	if h.Get("X-Debug") != "" {
		t.Fatalf("header desligado foi enviado: %v", h)
	}
}

// Enter envia de onde quer que você esteja — menos numa tabela, onde ele edita a
// célula. ctrl+enter envia sempre, inclusive no meio da edição.
func TestApiEnterSendsExceptInsideTable(t *testing.T) {
	a, p := newApiTestApp()
	a.apiFocus = apiFocusURL
	if _, cmd := a.handleApiKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); cmd == nil {
		t.Fatal("enter na URL tinha de enviar")
	}
	a.apiLoading = false

	a.apiSetSection(apiSecParams)
	a.apiRowCursor = 0
	_, cmd := a.handleApiKeys(tea.KeyMsg{Type: tea.KeyEnter}, p)
	if cmd != nil {
		t.Fatal("enter na tabela tinha de editar, não enviar")
	}
	if !a.apiEditing {
		t.Fatal("enter na tabela deveria abrir a edição da célula")
	}
}

func TestApiBodyEditingKeepsBracketsLiteral(t *testing.T) {
	a, p := newApiTestApp()
	a.apiSetSection(apiSecBody)
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, p)
	if a.apiEditing || a.apiBody != "" {
		t.Fatal("digitar no corpo sem e não pode editar")
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, p)
	if !a.apiEditing {
		t.Fatal("e deveria abrir o editor do corpo")
	}
	for _, r := range []rune{'[', ']'} {
		_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, p)
	}
	if a.apiBody != "[]" {
		t.Fatalf("corpo=%q", a.apiBody)
	}
	if a.apiSection != apiSecBody {
		t.Fatal("editar o corpo não pode trocar de aba")
	}
}

// A busca é da resposta — é o único conteúdo rolável da tela.
func TestApiSearchOpensOnResponse(t *testing.T) {
	a, p := newApiTestApp()
	a.apiResponseBody = "{\n  \"foo\": 1\n}"
	a.apiSetSection(apiSecResponse)
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, p)
	if !a.apiSearchOn {
		t.Fatal("/ na resposta deveria abrir a busca")
	}
}

// h alterna corpo e cabeçalhos da resposta, no lugar do antigo H global.
func TestApiResponseViewToggle(t *testing.T) {
	a, p := newApiTestApp()
	a.apiSetSection(apiSecResponse)
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, p)
	if a.apiRespView != apiRespHeaders {
		t.Fatalf("h deveria mostrar os cabeçalhos, got %v", a.apiRespView)
	}
	_, _ = a.handleApiKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, p)
	if a.apiRespView != apiRespBody {
		t.Fatal("h deveria voltar pro corpo")
	}
}

func TestApiCurlCommandCarriesEverything(t *testing.T) {
	a, _ := newApiTestApp()
	a.apiMethod = "POST"
	a.apiURL = "http://localhost:3000/pedidos"
	a.apiHeaders = "Accept: application/json\n# X-Off: 1\n"
	a.apiAuthType = apiAuthBearer
	a.apiAuthToken = "tok123"
	a.apiBody = "{\"a\":1}"
	got := a.apiCurlCommand()
	for _, want := range []string{"-X POST", "localhost:3000/pedidos", "Accept: application/json",
		"Bearer tok123", "--data-raw"} {
		if !strings.Contains(got, want) {
			t.Fatalf("curl sem %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "X-Off") {
		t.Fatalf("header desligado entrou no curl:\n%s", got)
	}
}

// A tela tem a anatomia do padrão (§2) e o vocabulário do Postman — e não tem
// mais as caixas numeradas do modelo antigo.
func TestRenderApiScreenHasPostmanAnatomy(t *testing.T) {
	a, p := newApiTestApp()
	a.apiURL = "http://localhost:3000/api/pedidos?page=2"
	a.apiSyncParamsFromURL()
	a.apiResponseStatus = "200 OK"
	a.apiResponseCode = 200
	a.apiResponseTime = 142 * time.Millisecond
	a.apiResponseBody = "{\n  \"ok\": true\n}"
	out := stripANSI(a.renderApiTab(p))

	for _, want := range []string{
		"API", "1 PARAMS", "2 HEADERS", "3 AUTH", "4 BODY", "5 RESPOSTA",
		"REQUISIÇÃO", "ENVIAR", "localhost:3000", "200 OK", "142ms",
		"RESPOSTA", "1-5", "abas", "enviar",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("tela sem %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{"[1]-Request", "[2]-URL", "[3]-Headers", "[4]-Auth", "AÇÕES"} {
		if strings.Contains(out, gone) {
			t.Fatalf("sobrou do modelo antigo: %q\n%s", gone, out)
		}
	}
	// Requisição e resposta VISÍVEIS JUNTAS: é o ponto da tela.
	if !strings.Contains(out, "PARAMS") || !strings.Contains(out, "\"ok\": true") {
		t.Fatalf("aba e resposta deveriam aparecer juntas:\n%s", out)
	}
}

// Em terminal curto a divisão não paga a borda: mostra a faixa em foco, e
// nenhuma linha estoura a largura.
func TestRenderApiScreenFitsShortTerminal(t *testing.T) {
	a, p := newApiTestApp()
	a.width, a.height = 84, 18
	a.apiResponseBody = "{\n  \"ok\": true\n}"
	a.apiResponseStatus = "200 OK"
	for _, sec := range []apiSection{apiSecParams, apiSecHeaders, apiSecAuth, apiSecBody, apiSecResponse} {
		a.apiSetSection(sec)
		out := a.renderApiTab(p)
		for _, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w > 84 {
				t.Fatalf("aba %v estourou a largura: %d colunas", sec, w)
			}
		}
		if lipgloss.Height(out) > 18 {
			t.Fatalf("aba %v estourou a altura: %d linhas", sec, lipgloss.Height(out))
		}
	}
}

func TestApiHumanSize(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want string
	}{{900, "900 B"}, {2048, "2.0 kB"}, {3 * 1024 * 1024, "3.0 MB"}} {
		if got := apiHumanSize(tc.in); got != tc.want {
			t.Fatalf("apiHumanSize(%d)=%q, esperado %q", tc.in, got, tc.want)
		}
	}
}
