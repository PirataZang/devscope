package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func TestAllTabsIncludesKubernetes(t *testing.T) {
	found := false
	for _, tab := range AllTabs {
		if tab == TabKubernetes {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("TabKubernetes missing from AllTabs")
	}
	if TabKubernetes.String() != "Kubernetes" {
		t.Fatalf("String=%q", TabKubernetes.String())
	}
}

func TestK8sLandingEnterAndEsc(t *testing.T) {
	p := core.Project{Path: "/p", Name: "demo"}
	a := &App{
		width:           100,
		height:          30,
		view:            ViewProject,
		tab:             TabOverview,
		selectedProject: &p,
		snapshot:        core.Snapshot{Projects: []core.Project{p}},
	}
	a.enterK8sTab(&p)
	if a.tab != TabKubernetes || a.k8sOpen {
		t.Fatalf("4 should open landing, tab=%v open=%v", a.tab, a.k8sOpen)
	}
	landing := stripANSI(a.renderK8sLanding(&p))
	if !strings.Contains(landing, "enter") || !strings.Contains(landing, "KUBERNETES") || !strings.Contains(landing, "AÇÕES") {
		t.Fatalf("landing missing prompt: %q", landing)
	}

	_, cmd := a.updateProject(tea.KeyMsg{Type: tea.KeyEnter})
	if !a.k8sOpen || a.tab != TabKubernetes {
		t.Fatalf("enter should open client, open=%v tab=%v", a.k8sOpen, a.tab)
	}
	_ = cmd

	_, _ = a.handleK8sKeys(tea.KeyMsg{Type: tea.KeyEsc}, &p)
	if a.k8sOpen || a.tab != TabKubernetes || a.view != ViewProject {
		t.Fatalf("esc should return to tab 4 landing, open=%v tab=%v view=%v", a.k8sOpen, a.tab, a.view)
	}
}

func TestSidebarShowsKubernetesInScope(t *testing.T) {
	a := &App{width: 120, height: 40, tab: TabKubernetes}
	got := stripANSI(a.renderProjectSidebar())
	if !strings.Contains(got, "MANAGER") || !strings.Contains(got, "Kubernetes") {
		t.Fatalf("sidebar missing Kubernetes in MANAGER: %q", got)
	}
	if !strings.Contains(got, "tab · shift+tab") {
		t.Fatalf("footer should mention tab · shift+tab: %q", got)
	}
}

func TestSanitizeK8sName(t *testing.T) {
	if got := sanitizeK8sName("My App!"); got != "my-app" {
		t.Fatalf("got %q", got)
	}
}

func TestK8sApplyKeyAndEnterNewline(t *testing.T) {
	if !isK8sApplyKey(tea.KeyMsg{Type: tea.KeyCtrlS}) {
		t.Fatal("ctrl+s must apply")
	}
	if isK8sApplyKey(tea.KeyMsg{Type: tea.KeyEnter}) {
		t.Fatal("plain enter must NOT apply (nova linha)")
	}

	a := &App{k8sEditing: true, k8sYAML: "a: 1", k8sEditorCursor: 4, k8sPane: k8sPaneEditor}
	_, _ = a.updateK8sEdit(tea.KeyMsg{Type: tea.KeyEnter}, nil)
	if !strings.Contains(a.k8sYAML, "\n") || !a.k8sEditing {
		t.Fatalf("enter should insert newline and stay editing: yaml=%q editing=%v", a.k8sYAML, a.k8sEditing)
	}

	a = &App{k8sEditing: true, k8sYAML: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: t\n", k8sPane: k8sPaneEditor}
	_, cmd := a.updateK8sEdit(tea.KeyMsg{Type: tea.KeyCtrlS}, nil)
	if a.k8sEditing || cmd == nil {
		t.Fatalf("ctrl+s should apply: editing=%v cmd=%v", a.k8sEditing, cmd)
	}
}

func TestK8sCreateStartsEditing(t *testing.T) {
	a := &App{k8sNamespace: "default"}
	_ = a.k8sBeginCreate()
	if !a.k8sEditing || a.k8sYAML == "" || a.k8sPane != k8sPaneEditor {
		t.Fatalf("create: editing=%v pane=%v yaml empty=%v", a.k8sEditing, a.k8sPane, a.k8sYAML == "")
	}
}

func TestK8sOverviewLayout(t *testing.T) {
	a := &App{
		width:        120,
		height:       40,
		k8sOpen:      true,
		k8sNamespace: "default",
		k8sContext:   "kind-dev",
		k8sVersion:   "v1.31.0",
		k8sNodeCount: 3,
		k8sKind:      k8sKindPods,
		k8sSubTab:    k8sTabOverview,
		k8sFocus:     k8sFocusTable,
		k8sResources: []collectors.K8sResource{
			{Kind: "Pod", Name: "frontend-1", Status: "Running", Ready: "1/1", Restarts: "0", Node: "node-1", IP: "10.0.0.1", Age: "2d"},
		},
	}
	view := stripANSI(a.renderK8sTab(&core.Project{Name: "demo", Path: "/p"}))
	for _, want := range []string{
		"KUBERNETES", "kind-dev", "default", "PODS", "POD LOGS", "YAML", "DETAILS", "RELATION",
		"NOME", "ESTADO", "RESTARTS", "frontend-1",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("overview missing %q in:\n%s", want, view)
		}
	}
	// O seletor de tipo virou régua horizontal; a coluna comia 26 col da tabela.
	for _, gone := range []string{"CLUSTER EXPLORER", "QUICK STATS"} {
		if strings.Contains(view, gone) {
			t.Fatalf("sobra do layout antigo %q:\n%s", gone, view)
		}
	}
}

// A coluna do explorer saiu, então o foco não pode mais parar nela — e o tipo
// de recurso precisa continuar alcançável só pelo teclado.
func TestK8sKindStaysReachableWithoutExplorer(t *testing.T) {
	a := &App{width: 120, height: 40, k8sOpen: true, k8sNamespace: "default", k8sKind: k8sKindPods}

	// tab percorre só painéis que existem na tela
	seen := map[k8sFocus]bool{}
	for i := 0; i < 8; i++ {
		seen[a.k8sFocus] = true
		a.k8sFocus = (a.k8sFocus + 1) % 4
	}
	for f := range seen {
		if f > k8sFocusDetail {
			t.Fatalf("foco %v não tem painel", f)
		}
	}

	a.k8sShiftKind(1)
	if a.k8sKind != k8sKindDeploys {
		t.Fatalf("] deve avançar o tipo, got %v", a.k8sKind)
	}
	a.k8sShiftKind(-1)
	if a.k8sKind != k8sKindPods {
		t.Fatalf("[ deve voltar o tipo, got %v", a.k8sKind)
	}
	a.k8sShiftKind(-1)
	if a.k8sKind != k8sKindManifests {
		t.Fatalf("ciclo deve dar a volta, got %v", a.k8sKind)
	}

	// A régua anuncia a tecla e não repete a contagem que a caixa já mostra.
	strip := stripANSI(a.renderK8sKindStrip(120))
	if !strings.Contains(strip, "[ ]") {
		t.Fatalf("régua deve mostrar a tecla que troca o tipo: %q", strip)
	}
	if strings.Contains(strip, "PODS 0") {
		t.Fatalf("contagem duplica o título da caixa: %q", strip)
	}
}

// Rodar kubectl no contexto errado é o acidente clássico — a tela marca prod.
func TestK8sProdContextIsFlagged(t *testing.T) {
	for _, ctx := range []string{"gke_acme_sa-east1_prod", "prd-cluster", "acme-live"} {
		if !k8sContextLooksProd(ctx) {
			t.Fatalf("%q deveria ser marcado como produção", ctx)
		}
	}
	for _, ctx := range []string{"kind-dev", "minikube", "acme-nonprod", "staging"} {
		if k8sContextLooksProd(ctx) {
			t.Fatalf("%q não é produção", ctx)
		}
	}
	a := &App{width: 120, height: 40, k8sOpen: true, k8sContext: "gke_acme_prod", k8sNamespace: "default"}
	if !strings.Contains(stripANSI(a.renderK8sHeader(120)), "produção") {
		t.Fatal("header deve avisar quando o contexto é produção")
	}
}

// Restarts e ready são o sintoma mais barato de instabilidade.
func TestK8sReadyAndRestartStyles(t *testing.T) {
	if k8sReadyStyle("1/1").GetForeground() == k8sReadyStyle("0/1").GetForeground() {
		t.Fatal("1/1 e 0/1 devem ler diferente")
	}
	zero := k8sRestartStyle("0").GetForeground()
	few := k8sRestartStyle("2").GetForeground()
	many := k8sRestartStyle("7").GetForeground()
	if zero == few || few == many || zero == many {
		t.Fatalf("0, 2 e 7 restarts devem ler diferente: %v %v %v", zero, few, many)
	}
	if _, st := k8sStatusDot("CrashLoopBackOff", 0); st.GetForeground() == StyleHealthy.GetForeground() {
		t.Fatal("CrashLoopBackOff não pode ler como saudável")
	}
}

func TestK8sFilterKey(t *testing.T) {
	a := &App{k8sOpen: true, k8sFocus: k8sFocusTable}
	_, _ = a.handleK8sKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}, nil)
	if !a.k8sFilterOn {
		t.Fatal("b should start filter")
	}
}
