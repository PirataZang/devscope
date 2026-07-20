package routeutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverOpenAPIAndParsers(t *testing.T) {
	root := t.TempDir()

	mustWrite(t, filepath.Join(root, "openapi.json"), `{
  "openapi": "3.0.0",
  "paths": {
    "/health": { "get": { "summary": "ok" } },
    "/users": { "get": {}, "post": { "summary": "create" } }
  }
}`)

	mustWrite(t, filepath.Join(root, "package.json"), `{"dependencies":{"@nestjs/core":"10.0.0","express":"4.18.0"}}`)
	mustWrite(t, filepath.Join(root, "src", "users.controller.ts"), `
@Controller('users')
export class UsersController {
  @Get()
  list() {}
  @Get(':id')
  one() {}
  @Post()
  create() {}
}
`)
	mustWrite(t, filepath.Join(root, "src", "app.js"), `
const express = require('express');
const app = express();
app.get('/ping', (req,res)=>res.send('ok'));
app.delete('/ping', (req,res)=>res.send('ok'));
`)
	mustWrite(t, filepath.Join(root, "main.py"), `
from fastapi import FastAPI
app = FastAPI()
@app.get("/items")
def items(): pass
@app.post("/items/{item_id}")
def create(item_id: int): pass
`)
	mustWrite(t, filepath.Join(root, "requirements.txt"), "fastapi\n")
	mustWrite(t, filepath.Join(root, "artisan"), "#!/usr/bin/env php\n")
	mustWrite(t, filepath.Join(root, "routes", "api.php"), `
Route::get('/legacy', fn () => null);
Route::apiResource('posts', PostController::class);
`)

	routes, stacks, err := Discover(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stacks) < 3 {
		t.Fatalf("expected auto-detected stacks, got %v", stacks)
	}
	if len(routes) < 8 {
		t.Fatalf("expected several routes, got %d: %+v", len(routes), routes)
	}

	mustHave := []struct{ m, p string }{
		{"GET", "/health"},
		{"POST", "/users"},
		{"GET", "/users/{id}"},
		{"GET", "/ping"},
		{"GET", "/items"},
		{"GET", "/legacy"},
		{"GET", "/posts"},
	}
	for _, want := range mustHave {
		if !hasRoute(routes, want.m, want.p) {
			t.Fatalf("missing %s %s in %+v", want.m, want.p, routes)
		}
	}
	for _, r := range routes {
		if r.Method == "GET" && r.Path == "/health" && r.Source != "openapi" {
			t.Fatalf("health should come from openapi, got %s", r.Source)
		}
	}
}

func TestDetectStacksAndExtraFrameworks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "manage.py"), "#!/usr/bin/env python\n")
	mustWrite(t, filepath.Join(root, "app", "urls.py"), `
urlpatterns = [
    path('api/health/', views.health),
]
`)
	mustWrite(t, filepath.Join(root, "go.mod"), "module demo\n\nrequire github.com/gin-gonic/gin v1.9.0\n")
	mustWrite(t, filepath.Join(root, "main.go"), `
package main
func main() {
  r.GET("/v1/ping", h)
  r.POST("/v1/ping", h)
}
`)
	mustWrite(t, filepath.Join(root, "app", "api", "hello", "route.ts"), `
export async function GET() {}
export async function POST() {}
`)
	mustWrite(t, filepath.Join(root, "next.config.mjs"), "export default {}\n")
	mustWrite(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  get '/up', to: 'rails/health#show'
  resources :books
end
`)

	stacks := DetectStacks(root, []string{"Django"})
	for _, want := range []string{"django", "gin", "go", "nextjs", "rails"} {
		if !containsStr(stacks, want) {
			t.Fatalf("missing stack %q in %v", want, stacks)
		}
	}

	routes, _, err := Discover(root, nil, []string{"Django"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ m, p string }{
		{"GET", "/api/health"},
		{"GET", "/v1/ping"},
		{"POST", "/v1/ping"},
		{"GET", "/api/hello"},
		{"POST", "/api/hello"},
		{"GET", "/up"},
		{"GET", "/books"},
	} {
		if !hasRoute(routes, want.m, want.p) {
			t.Fatalf("missing %s %s in %+v", want.m, want.p, routes)
		}
	}
}

func TestParseOpenAPIYAML(t *testing.T) {
	data := []byte(`
openapi: "3.0.0"
paths:
  /v1/ping:
    get:
      summary: ping
`)
	routes, err := parseOpenAPI(data)
	if err != nil || !hasRoute(routes, "GET", "/v1/ping") {
		t.Fatalf("yaml parse: err=%v routes=%+v", err, routes)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasRoute(routes []Route, method, path string) bool {
	for _, r := range routes {
		if r.Method == method && r.Path == path {
			return true
		}
	}
	return false
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
