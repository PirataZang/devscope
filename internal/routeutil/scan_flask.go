package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type flaskScanner struct{}

func (flaskScanner) Name() string { return "flask" }

func (flaskScanner) Match(_ string, frameworks []string) bool {
	return hasFramework(frameworks, "flask")
}

var reFlaskRoute = regexp.MustCompile(`@(?:app|bp|blueprint|api)\.route\s*\(\s*['"]([^'"]+)['"]([^)]*)\)`)

func (flaskScanner) Scan(root string) ([]Route, error) {
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
		if !strings.Contains(content, ".route(") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, loc := range reFlaskRoute.FindAllStringSubmatchIndex(content, -1) {
			sub := reFlaskRoute.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) < 2 {
				continue
			}
			methods := []string{"GET"}
			if len(sub) > 2 && strings.Contains(sub[2], "methods") {
				methods = parseQuotedMethods(sub[2])
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			for _, m := range methods {
				routes = append(routes, Route{
					Method: m, Path: sub[1], Source: "flask", File: rel, Line: line,
				})
			}
		}
		return nil
	})
	return routes, nil
}
