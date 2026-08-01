package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func TestDockerAddRefuseOpensEditor(t *testing.T) {
	p := core.Project{Path: "/tmp/proj", Name: "proj"}
	a := &App{
		width: 100, height: 30,
		view: ViewProject, tab: TabContainers, containerSubview: containerSubviewList,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.startDockerAdd(&p)
	if !a.dockerAddOn || a.dockerAddStep != dockerAddStepSearch {
		t.Fatal("search step not open")
	}
	got := stripANSI(a.renderDockerAdd())
	for _, want := range []string{"DOCKER HUB", "1 Busca", "Buscar imagem", "Escrever YAML manualmente"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, truncate(got, 280))
		}
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyTab})
	if a.dockerAddSearchFocus != dockerAddSearchRefuse {
		t.Fatal("tab should focus refuse")
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyEnter})
	if a.dockerAddStep != dockerAddStepEdit {
		t.Fatalf("expected edit step, got %v", a.dockerAddStep)
	}
	if !strings.Contains(a.dockerAddEdit, "services:") {
		t.Fatalf("expected template, got %q", a.dockerAddEdit)
	}
	editView := stripANSI(a.renderDockerAdd())
	if !strings.Contains(editView, "Salvar no compose") {
		t.Fatalf("edit missing save button: %q", truncate(editView, 200))
	}
}

func TestDockerAddSelectImageOpensEditor(t *testing.T) {
	p := core.Project{Path: "/tmp/proj", Name: "proj"}
	a := &App{
		width: 120, height: 48,
		view: ViewProject, tab: TabContainers,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.startDockerAdd(&p)
	a.dockerAddQuery = "postgres"
	_ = a.handleDockerHubSearchDone(dockerHubSearchDoneMsg{
		query: "postgres",
		page:  1,
		results: []collectors.DockerHubRepo{
			{Name: "postgres", Stars: 10, Pulls: 1_100_000_000, Official: true, Description: "The PostgreSQL object-relational database"},
		},
		hasMore: true,
	})
	if a.dockerAddStep != dockerAddStepResults {
		t.Fatal("expected results")
	}
	if a.dockerAddImage != "postgres" {
		t.Fatalf("image field=%q", a.dockerAddImage)
	}
	a.dockerAddDetailsLoading = false
	a.dockerAddDetailsCache = map[string]collectors.DockerHubDetails{
		"postgres": {
			Name:           "postgres",
			Description:    "The PostgreSQL object-relational database",
			Overview:       "PostgreSQL is a powerful open source database.",
			Stars:          14976,
			Pulls:          11_000_000_000,
			Official:       true,
			Tag:            "latest",
			TagSize:        162_351_975,
			Architectures:  []string{"amd64", "arm64"},
			OS:             []string{"linux"},
			Categories:     []string{"Databases & storage"},
			LastUpdated:    time.Now().Add(-30 * time.Hour),
			DateRegistered: time.Date(2014, 6, 5, 0, 0, 0, 0, time.UTC),
			Status:         "active",
		},
	}
	a.dockerAddTagsRepo = "postgres"
	a.dockerAddTags = []collectors.DockerHubTag{
		{Name: "latest", Size: 162_351_975},
		{Name: "16", Size: 150_000_000},
		{Name: "16-alpine", Size: 80_000_000},
	}
	resultsView := stripANSI(a.renderDockerAdd())
	for _, want := range []string{
		"DOCKER HUB", "2 Imagem", "postgres", "Detalhes",
		"The PostgreSQL", "hub.docker.com", "Imagem",
		"STARS", "PULLS", "SIZE", "UPDATE", "155 MB", "amd64", "overview",
		"tags recentes", "latest", "16-alpine",
	} {
		if !strings.Contains(resultsView, want) {
			t.Fatalf("results missing %q in %q", want, truncate(resultsView, 900))
		}
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyTab})
	if a.dockerAddResultsFocus != dockerAddResultsImage {
		t.Fatal("tab should focus image field")
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyDown})
	if a.dockerAddImage != "postgres:16" {
		t.Fatalf("tag select image=%q", a.dockerAddImage)
	}
	// no fim da lista de tags, ↓ não troca de painel
	a.dockerAddTagCursor = len(a.dockerAddTags) - 1
	a.applyDockerAddTagAtCursor()
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyDown})
	if a.dockerAddResultsFocus != dockerAddResultsImage {
		t.Fatalf("↓ no fim das tags não deve sair do painel, focus=%v", a.dockerAddResultsFocus)
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyUp})
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyUp})
	if a.dockerAddResultsFocus != dockerAddResultsImage {
		t.Fatalf("↑ no início das tags não deve sair do painel, focus=%v", a.dockerAddResultsFocus)
	}
	a.dockerAddImage = "postgres:16"
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-bookworm")})
	if a.dockerAddImage != "postgres:16-bookworm" {
		t.Fatalf("edited image=%q", a.dockerAddImage)
	}
	_, cmd := a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyEnter})
	if a.dockerAddStep != dockerAddStepEdit {
		t.Fatal("expected edit after select")
	}
	if cmd != nil {
		t.Fatal("postgres preset should not need async cmd")
	}
	if !strings.Contains(a.dockerAddEdit, "image: postgres:16-bookworm") {
		t.Fatalf("edit=%q", a.dockerAddEdit)
	}
	if !strings.Contains(a.dockerAddEdit, "5432:5432") || !strings.Contains(a.dockerAddEdit, "POSTGRES_PASSWORD") {
		t.Fatalf("expected postgres preset YAML:\n%s", a.dockerAddEdit)
	}
	if a.dockerAddComposeSource != collectors.ComposeSourcePreset {
		t.Fatalf("source=%q", a.dockerAddComposeSource)
	}
}

func TestDockerAddUnknownImageUsesAsyncCompose(t *testing.T) {
	p := core.Project{Path: "/tmp/proj", Name: "proj"}
	a := &App{
		width: 100, height: 40,
		view: ViewProject, tab: TabContainers,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
		dockerAddOn: true, dockerAddStep: dockerAddStepResults,
		dockerAddResults: []collectors.DockerHubRepo{{Name: "itzg/minecraft-server"}},
		dockerAddImage:   "itzg/minecraft-server",
	}
	_, cmd := a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("unknown image should fetch manifest async")
	}
	if a.dockerAddStep == dockerAddStepEdit {
		t.Fatal("should stay on results until ready")
	}
	a.handleDockerAddComposeReady(dockerAddComposeReadyMsg{
		image:  "itzg/minecraft-server",
		yaml:   "services:\n  minecraft-server:\n    image: itzg/minecraft-server\n",
		source: collectors.ComposeSourceManifest,
	})
	if a.dockerAddStep != dockerAddStepEdit {
		t.Fatal("expected edit after ready")
	}
	if a.dockerAddComposeSource != collectors.ComposeSourceManifest {
		t.Fatalf("source=%q", a.dockerAddComposeSource)
	}
}

func TestDockerAddLoadMoreAppends(t *testing.T) {
	a := &App{dockerAddOn: true, dockerAddStep: dockerAddStepResults, dockerAddQuery: "itzg"}
	_ = a.handleDockerHubSearchDone(dockerHubSearchDoneMsg{
		query: "itzg", page: 1,
		results: []collectors.DockerHubRepo{{Name: "itzg/a"}, {Name: "itzg/b"}},
		hasMore: true,
	})
	_ = a.handleDockerHubSearchDone(dockerHubSearchDoneMsg{
		query: "itzg", page: 2, append: true,
		results: []collectors.DockerHubRepo{{Name: "itzg/c"}},
		hasMore: false,
	})
	if len(a.dockerAddResults) != 3 {
		t.Fatalf("len=%d", len(a.dockerAddResults))
	}
	if a.dockerAddHasMore {
		t.Fatal("expected no more pages")
	}
	if a.dockerAddCursor != 2 || a.dockerAddImage != "itzg/c" {
		t.Fatalf("cursor=%d image=%q", a.dockerAddCursor, a.dockerAddImage)
	}
}

func TestDockerHubDetailsDoneCaches(t *testing.T) {
	a := &App{
		dockerAddOn: true, dockerAddStep: dockerAddStepResults,
		dockerAddResults:     []collectors.DockerHubRepo{{Name: "redis"}},
		dockerAddDetailsName: "redis", dockerAddDetailsSeq: 3, dockerAddDetailsLoading: true,
	}
	a.handleDockerHubDetailsDone(dockerHubDetailsDoneMsg{
		seq: 3, name: "redis",
		details: collectors.DockerHubDetails{Name: "redis", Tag: "latest", TagSize: 40_000_000},
	})
	if a.dockerAddDetailsLoading {
		t.Fatal("loading should clear")
	}
	d, ok := a.dockerAddDetailsCache["redis"]
	if !ok || d.Tag != "latest" {
		t.Fatalf("cache=%v ok=%v", d, ok)
	}
	a.handleDockerHubDetailsDone(dockerHubDetailsDoneMsg{
		seq: 2, name: "redis",
		details: collectors.DockerHubDetails{Name: "redis", Tag: "stale"},
	})
	if a.dockerAddDetailsCache["redis"].Tag != "latest" {
		t.Fatal("stale seq should be ignored")
	}
}
