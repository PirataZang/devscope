package routeutil

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectStacks resolves which route stacks apply to the project:
// frameworks already known on the Project plus markers in the tree (deps, configs).
func DetectStacks(root string, frameworks []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(raw string) {
		s := canonicalizeStack(raw)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	for _, f := range frameworks {
		add(f)
	}

	if fileExists(filepath.Join(root, "artisan")) {
		add("laravel")
	}
	if fileExists(filepath.Join(root, "manage.py")) {
		add("django")
	}
	if fileExists(filepath.Join(root, "config", "routes.rb")) {
		add("rails")
	}
	for _, f := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		if fileExists(filepath.Join(root, f)) {
			add("nextjs")
			break
		}
	}
	for _, f := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs"} {
		if fileExists(filepath.Join(root, f)) {
			add("nuxt")
			break
		}
	}
	if fileExists(filepath.Join(root, "go.mod")) {
		add("go")
		mod := readFileString(filepath.Join(root, "go.mod"))
		for _, pair := range []struct{ needle, stack string }{
			{"gin-gonic/gin", "gin"},
			{"labstack/echo", "echo"},
			{"gofiber/fiber", "fiber"},
			{"go-chi/chi", "chi"},
			{"gorilla/mux", "mux"},
		} {
			if strings.Contains(mod, pair.needle) {
				add(pair.stack)
			}
		}
	}
	if fileExists(filepath.Join(root, "pom.xml")) || fileExists(filepath.Join(root, "build.gradle")) || fileExists(filepath.Join(root, "build.gradle.kts")) {
		add("java")
		build := readFileString(filepath.Join(root, "pom.xml")) +
			readFileString(filepath.Join(root, "build.gradle")) +
			readFileString(filepath.Join(root, "build.gradle.kts"))
		if strings.Contains(strings.ToLower(build), "spring") {
			add("spring")
		}
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		add("rust")
		cargo := readFileString(filepath.Join(root, "Cargo.toml"))
		if strings.Contains(cargo, "axum") {
			add("axum")
		}
		if strings.Contains(cargo, "actix-web") {
			add("actix")
		}
	}

	pkg := readFileString(filepath.Join(root, "package.json"))
	if pkg != "" {
		add("node")
		for _, pair := range []struct{ needle, stack string }{
			{"@nestjs/core", "nestjs"},
			{"\"express\"", "express"},
			{"\"fastify\"", "fastify"},
			{"\"hono\"", "hono"},
			{"\"koa\"", "koa"},
			{"\"next\"", "nextjs"},
			{"\"nuxt\"", "nuxt"},
		} {
			if strings.Contains(pkg, pair.needle) {
				add(pair.stack)
			}
		}
	}

	pyDeps := false
	for _, name := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		py := strings.ToLower(readFileString(filepath.Join(root, name)))
		if py == "" {
			continue
		}
		pyDeps = true
		add("python")
		if strings.Contains(py, "fastapi") {
			add("fastapi")
		}
		if strings.Contains(py, "flask") {
			add("flask")
		}
		if strings.Contains(py, "django") {
			add("django")
		}
	}
	if fileExists(filepath.Join(root, "manage.py")) {
		pyDeps = true
	}

	// Sniff .py only when Python is present and a web stack is still unknown.
	if pyDeps && !seen["fastapi"] && !seen["flask"] && !seen["django"] {
		_ = walkProject(root, func(path string) error {
			if !strings.HasSuffix(path, ".py") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			s := string(data)
			if strings.Contains(s, "FastAPI") || strings.Contains(s, "APIRouter") {
				add("fastapi")
			}
			if strings.Contains(s, "Flask(") || strings.Contains(s, "from flask") {
				add("flask")
			}
			if strings.Contains(s, "urlpatterns") {
				add("django")
			}
			return nil
		})
	}

	return out
}

func canonicalizeStack(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	switch {
	case strings.Contains(s, "nest"):
		return "nestjs"
	case strings.Contains(s, "laravel"):
		return "laravel"
	case strings.Contains(s, "django"):
		return "django"
	case strings.Contains(s, "fastapi"):
		return "fastapi"
	case strings.Contains(s, "flask"):
		return "flask"
	case strings.Contains(s, "next"):
		return "nextjs"
	case strings.Contains(s, "nuxt"):
		return "nuxt"
	case strings.Contains(s, "express"):
		return "express"
	case strings.Contains(s, "fastify"):
		return "fastify"
	case strings.Contains(s, "hono"):
		return "hono"
	case strings.Contains(s, "koa"):
		return "koa"
	case strings.Contains(s, "spring"):
		return "spring"
	case strings.Contains(s, "rails"):
		return "rails"
	case strings.Contains(s, "gin"):
		return "gin"
	case strings.Contains(s, "echo"):
		return "echo"
	case strings.Contains(s, "fiber"):
		return "fiber"
	case strings.Contains(s, "chi"):
		return "chi"
	case strings.Contains(s, "mux"):
		return "mux"
	case strings.Contains(s, "axum"):
		return "axum"
	case strings.Contains(s, "actix"):
		return "actix"
	case s == "go" || s == "golang":
		return "go"
	case s == "python":
		return "python"
	case s == "node.js" || s == "nodejs" || s == "node":
		return "node"
	case s == "java":
		return "java"
	case s == "php":
		return "php"
	case s == "rust":
		return "rust"
	default:
		return s
	}
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
