package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type rustScanner struct{}

func (rustScanner) Name() string { return "rust" }

func (rustScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "rust", "axum", "actix") {
		return fileExists(filepath.Join(root, "Cargo.toml"))
	}
	cargo := readFileString(filepath.Join(root, "Cargo.toml"))
	return strings.Contains(cargo, "axum") || strings.Contains(cargo, "actix-web")
}

var (
	reAxum   = regexp.MustCompile(`\.route\s*\(\s*"([^"]+)"\s*,\s*(get|post|put|patch|delete)\s*\(`)
	reActixM = regexp.MustCompile(`#\[(get|post|put|patch|delete)\s*\(\s*"([^"]+)"\s*\)\]`)
)

func (rustScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		if !strings.HasSuffix(path, ".rs") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		rel, _ := filepath.Rel(root, path)
		for _, loc := range reAxum.FindAllStringSubmatchIndex(content, -1) {
			sub := reAxum.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: strings.ToUpper(sub[2]), Path: sub[1], Source: "axum", File: rel, Line: line,
			})
		}
		for _, loc := range reActixM.FindAllStringSubmatchIndex(content, -1) {
			sub := reActixM.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: strings.ToUpper(sub[1]), Path: sub[2], Source: "actix", File: rel, Line: line,
			})
		}
		return nil
	})
	return routes, nil
}
