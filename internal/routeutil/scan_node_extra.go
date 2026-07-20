package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// fastify / hono / koa — same shape as Express-style verb calls.
type nodeExtraScanner struct{}

func (nodeExtraScanner) Name() string { return "node" }

func (nodeExtraScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "fastify", "hono", "koa") {
		return true
	}
	return depsContainAny(root, "\"fastify\"", "\"hono\"", "\"koa\"")
}

var reNodeExtra = regexp.MustCompile(`(?i)\b(?:app|server|fastify|router|r|api)\s*\.\s*(get|post|put|patch|delete|all|route)\s*\(\s*['"]([^'"]+)['"]`)

func (nodeExtraScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		ext := filepath.Ext(path)
		if ext != ".js" && ext != ".ts" && ext != ".mjs" && ext != ".cjs" && ext != ".tsx" {
			return nil
		}
		if strings.Contains(path, ".test.") || strings.Contains(path, ".spec.") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !reNodeExtra.MatchString(content) {
			return nil
		}
		// Avoid double-counting pure Express files already handled — still OK via merge.
		rel, _ := filepath.Rel(root, path)
		src := "node"
		switch {
		case strings.Contains(content, "fastify") || depsContain(root, "\"fastify\""):
			src = "fastify"
		case strings.Contains(content, "hono") || depsContain(root, "\"hono\""):
			src = "hono"
		case strings.Contains(content, "koa") || depsContain(root, "\"koa\""):
			src = "koa"
		}
		for _, loc := range reNodeExtra.FindAllStringSubmatchIndex(content, -1) {
			sub := reNodeExtra.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			methods := []string{strings.ToUpper(sub[1])}
			if methods[0] == "ALL" || methods[0] == "ROUTE" {
				methods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			for _, m := range methods {
				routes = append(routes, Route{
					Method: m, Path: nestPath(sub[2]), Source: src, File: rel, Line: line,
				})
			}
		}
		return nil
	})
	return routes, nil
}

func depsContainAny(root string, pkgs ...string) bool {
	pkg := readFileString(filepath.Join(root, "package.json"))
	for _, p := range pkgs {
		if strings.Contains(pkg, p) {
			return true
		}
	}
	return false
}
