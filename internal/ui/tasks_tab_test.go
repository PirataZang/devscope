package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/devscope/devscope/internal/collectors"
	"github.com/devscope/devscope/internal/core"
)

func sampleTasks() []collectors.ProjectTask {
	return []collectors.ProjectTask{
		{Source: "make", Name: "build", Desc: "compila tudo", Argv: []string{"make", "build"}},
		{Source: "make", Name: "test", Desc: "roda os testes", Argv: []string{"make", "test"}},
		{Source: "pnpm", Name: "dev", Desc: "vite", Argv: []string{"pnpm", "run", "dev"}},
		{Source: "pnpm", Name: "build", Desc: "vite build", Argv: []string{"pnpm", "run", "build"}},
	}
}

func TestTasksListShowsLastRunWithoutColor(t *testing.T) {
	p := &core.Project{Name: "app", Path: "/p/app"}
	a := &App{width: 140, height: 40, view: ViewProject, tab: TabTasks, selectedProject: p, taskList: sampleTasks()}
	a.handleTaskDone(taskDoneMsg{key: "make test", code: 2, dur: 14e9})
	a.handleTaskDone(taskDoneMsg{key: "pnpm dev", code: 130, dur: 90e9})
	a.handleTaskDone(taskDoneMsg{key: "pnpm build", dur: 3e9})
	got := stripANSI(a.renderTasksTab(p))
	for _, want := range []string{"TAREFAS · 4", "compila tudo", "saiu 2 · 14s", "parada · 1m", "ok · 3s", "⚠", "◌", "◆", "roda     make build", "enter rodar"} {
		if !strings.Contains(got, want) {
			t.Fatalf("faltou %q:\n%s", want, got)
		}
	}
}

func TestTasksEmptySaysWhereTheyComeFrom(t *testing.T) {
	p := &core.Project{Name: "app", Path: "/p/app"}
	a := &App{width: 140, height: 40, view: ViewProject, tab: TabTasks, selectedProject: p}
	if got := stripANSI(a.renderTasksTab(p)); !strings.Contains(got, "nenhuma tarefa") || !strings.Contains(got, "Makefile") {
		t.Fatalf("estado vazio diz de onde as tarefas vêm:\n%s", got)
	}
}

func TestTasksKeys(t *testing.T) {
	p := &core.Project{Name: "app", Path: t.TempDir()}
	a := &App{width: 140, height: 40, view: ViewProject, tab: TabTasks, selectedProject: p, taskList: sampleTasks()}
	if _, ok := a.handleTaskKeys(tea.KeyMsg{Type: tea.KeyDown}, p); !ok || a.taskCursor != 1 {
		t.Fatalf("↓ move o cursor: %d", a.taskCursor)
	}
	if cmd, ok := a.handleTaskKeys(tea.KeyMsg{Type: tea.KeyEnter}, p); !ok || cmd == nil {
		t.Fatal("enter roda a tarefa selecionada")
	}
	if _, ok := a.handleTaskKeys(tea.KeyMsg{Type: tea.KeyTab}, p); ok {
		t.Fatal("tab não é do módulo — troca de módulo")
	}
}
