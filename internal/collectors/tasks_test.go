package collectors

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectTasks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), `CC := gcc
X ::= y
URL = http://x
.PHONY: build test
# compila tudo
build: deps
	go build ./...
test: ## roda os testes
	go test ./...
%.o: %.c
	$(CC) -c $<
build:
	@echo duplicado
`)
	writeFile(t, filepath.Join(dir, "justfile"), `set shell := ["bash", "-c"]
alias b := deploy
# sobe para produção
deploy env='prod':
    echo {{env}}
_privada:
    echo x
`)
	writeFile(t, filepath.Join(dir, "Taskfile.yml"), `version: '3'
tasks:
  lint:
    desc: roda o linter
    cmds: [golangci-lint run]
  helper:
    internal: true
  dev:
    cmds: [air]
`)
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"dev":"vite","build":"vite build"}}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
	writeFile(t, filepath.Join(dir, "composer.json"), `{"scripts":{"test":["phpunit","phpstan"]}}`)

	if !HasProjectTasks(dir) {
		t.Fatal("projeto com Makefile tem tarefas")
	}
	var got []string
	desc := map[string]string{}
	for _, task := range ProjectTasks(dir) {
		got = append(got, task.Key()+"="+strings.Join(task.Argv, " "))
		desc[task.Key()] = task.Desc
	}
	want := []string{
		"make build=make build", "make test=make test",
		"just deploy=just deploy",
		"task lint=task lint", "task dev=task dev",
		"pnpm build=pnpm run build", "pnpm dev=pnpm run dev",
		"composer test=composer run-script test",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tarefas:\n%s\nqueria:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for k, v := range map[string]string{
		"make build": "compila tudo", "make test": "roda os testes", "just deploy": "sobe para produção",
		"task lint": "roda o linter", "pnpm dev": "vite", "composer test": "phpunit && phpstan",
	} {
		if desc[k] != v {
			t.Errorf("%s: descrição %q, queria %q", k, desc[k], v)
		}
	}
}

func TestTaskCommandKeepsExitCode(t *testing.T) {
	cmd := TaskCommand(t.TempDir(), ProjectTask{Argv: []string{"sh", "-c", "exit 3"}})
	cmd.Stdin = strings.NewReader("\n") // o enter que devolve ao DevScope
	err := cmd.Run()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 3 {
		t.Fatalf("o código de saída da tarefa tem de chegar ao DevScope: %v", err)
	}
}

func TestHasProjectTasksEmpty(t *testing.T) {
	if HasProjectTasks(t.TempDir()) {
		t.Fatal("pasta vazia não tem tarefa")
	}
}
