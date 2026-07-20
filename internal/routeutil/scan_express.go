package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type expressScanner struct{}

func (expressScanner) Name() string { return "express" }

func (expressScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "express") {
		return true
	}
	return depsContain(root, "express")
}

var reExpressRoute = regexp.MustCompile(`(?i)\b(?:app|router|r)\s*\.\s*(get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]`)

func (expressScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		ext := filepath.Ext(path)
		if ext != ".js" && ext != ".ts" && ext != ".mjs" && ext != ".cjs" {
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
		if !reExpressRoute.MatchString(content) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, loc := range reExpressRoute.FindAllStringSubmatchIndex(content, -1) {
			sub := reExpressRoute.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: strings.ToUpper(sub[1]),
				Path:   nestPath(sub[2]),
				Source: "express",
				File:   rel,
				Line:   line,
			})
		}
		return nil
	})
	return routes, nil
}
