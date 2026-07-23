package ui

import (
	"strings"
	"testing"

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
	for _, want := range []string{"Novo serviço Docker", "Recusar buscar do docker hub"} {
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
		width: 100, height: 30,
		view: ViewProject, tab: TabContainers,
		selectedProject: &p, snapshot: core.Snapshot{Projects: []core.Project{p}},
	}
	a.startDockerAdd(&p)
	a.handleDockerHubSearchDone(dockerHubSearchDoneMsg{
		query: "postgres",
		results: []collectors.DockerHubRepo{
			{Name: "postgres", Stars: 10, Official: true, Description: "db"},
		},
	})
	if a.dockerAddStep != dockerAddStepResults {
		t.Fatal("expected results")
	}
	_, _ = a.updateDockerAdd(tea.KeyMsg{Type: tea.KeyEnter})
	if a.dockerAddStep != dockerAddStepEdit {
		t.Fatal("expected edit after select")
	}
	if !strings.Contains(a.dockerAddEdit, "image: postgres") {
		t.Fatalf("edit=%q", a.dockerAddEdit)
	}
}
