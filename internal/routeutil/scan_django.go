package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type djangoScanner struct{}

func (djangoScanner) Name() string { return "django" }

func (djangoScanner) Match(root string, frameworks []string) bool {
	return hasFramework(frameworks, "django") || fileExists(filepath.Join(root, "manage.py"))
}

var (
	reDjangoPath   = regexp.MustCompile(`(?i)\b(?:path|re_path)\s*\(\s*['"]([^'"]+)['"]`)
	reDjangoRouter = regexp.MustCompile(`(?i)\.register\s*\(\s*['"]([^'"]+)['"]`)
)

func (djangoScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		if !strings.HasSuffix(path, ".py") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "path(") && !strings.Contains(content, "re_path(") && !strings.Contains(content, ".register(") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, loc := range reDjangoPath.FindAllStringSubmatchIndex(content, -1) {
			sub := reDjangoPath.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 2 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			p := sub[1]
			if !strings.HasPrefix(p, "^") && !strings.HasPrefix(p, "/") {
				p = "/" + strings.Trim(p, "/")
			}
			if strings.HasPrefix(p, "^") {
				p = strings.TrimPrefix(p, "^")
				p = strings.TrimSuffix(p, "$")
				if !strings.HasPrefix(p, "/") {
					p = "/" + p
				}
			}
			routes = append(routes, Route{Method: "GET", Path: p, Source: "django", File: rel, Line: line})
		}
		for _, loc := range reDjangoRouter.FindAllStringSubmatchIndex(content, -1) {
			sub := reDjangoRouter.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 2 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			base := "/" + strings.Trim(sub[1], "/")
			for _, r := range []struct{ m, p string }{
				{"GET", base},
				{"POST", base},
				{"GET", base + "/{id}"},
				{"PUT", base + "/{id}"},
				{"PATCH", base + "/{id}"},
				{"DELETE", base + "/{id}"},
			} {
				routes = append(routes, Route{Method: r.m, Path: r.p, Source: "django", File: rel, Line: line, Summary: "router"})
			}
		}
		return nil
	})
	return routes, nil
}
