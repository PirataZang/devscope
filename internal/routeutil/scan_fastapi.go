package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type fastapiScanner struct{}

func (fastapiScanner) Name() string { return "fastapi" }

func (fastapiScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "fastapi") {
		return true
	}
	return fastapiMarker(root)
}

func fastapiMarker(root string) bool {
	for _, name := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		low := strings.ToLower(string(data))
		if strings.Contains(low, "fastapi") {
			return true
		}
	}
	found := false
	_ = walkProject(root, func(path string) error {
		if found || !strings.HasSuffix(path, ".py") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "FastAPI") || strings.Contains(string(data), "APIRouter") {
			found = true
		}
		return nil
	})
	return found
}

var reFastAPI = regexp.MustCompile(`@(?:app|router)\.(get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]`)

func (fastapiScanner) Scan(root string) ([]Route, error) {
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
		if !reFastAPI.MatchString(content) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, loc := range reFastAPI.FindAllStringSubmatchIndex(content, -1) {
			sub := reFastAPI.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: strings.ToUpper(sub[1]),
				Path:   sub[2],
				Source: "fastapi",
				File:   rel,
				Line:   line,
			})
		}
		return nil
	})
	return routes, nil
}
