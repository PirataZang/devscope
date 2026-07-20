package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type nestjsScanner struct{}

func (nestjsScanner) Name() string { return "nestjs" }

func (nestjsScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "nest") {
		return true
	}
	return depsContain(root, "@nestjs/core")
}

var (
	reNestController = regexp.MustCompile(`@Controller\s*\(\s*(?:['"]([^'"]*)['"])?\s*\)`)
	reNestMethod     = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete)\s*\(\s*(?:['"]([^'"]*)['"])?\s*\)`)
)

func (nestjsScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".js") {
			return nil
		}
		if strings.HasSuffix(path, ".spec.ts") || strings.HasSuffix(path, ".test.ts") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "@Controller") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		prefix := ""
		if m := reNestController.FindStringSubmatch(content); len(m) > 0 {
			prefix = m[1]
		}
		for _, loc := range reNestMethod.FindAllStringSubmatchIndex(content, -1) {
			sub := reNestMethod.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) < 2 {
				continue
			}
			method := strings.ToUpper(sub[1])
			subpath := ""
			if len(sub) > 2 {
				subpath = sub[2]
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: method,
				Path:   joinRoute(prefix, nestPath(subpath)),
				Source: "nestjs",
				File:   rel,
				Line:   line,
			})
		}
		return nil
	})
	return routes, nil
}

func nestPath(p string) string {
	p = strings.TrimSpace(p)
	// Nest :id → {id}
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func depsContain(root, pkg string) bool {
	for _, name := range []string{"package.json"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "\""+pkg+"\"") {
			return true
		}
	}
	return false
}
