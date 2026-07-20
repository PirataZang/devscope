package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type springScanner struct{}

func (springScanner) Name() string { return "spring" }

func (springScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "spring") {
		return true
	}
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if strings.Contains(strings.ToLower(readFileString(filepath.Join(root, f))), "spring") {
			return true
		}
	}
	return false
}

var (
	reSpringClass = regexp.MustCompile(`@RequestMapping\s*\(\s*(?:value\s*=\s*)?["']([^"']+)["']`)
	reSpringMeth  = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete)Mapping\s*(?:\(\s*(?:value\s*=\s*)?["']([^"']*)["']\s*\))?`)
	reSpringReq   = regexp.MustCompile(`@RequestMapping\s*\([^)]*method\s*=\s*RequestMethod\.(GET|POST|PUT|PATCH|DELETE)[^)]*(?:value\s*=\s*)?["']([^"']+)["']`)
)

func (springScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		if !strings.HasSuffix(path, ".java") && !strings.HasSuffix(path, ".kt") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "Mapping") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		prefix := ""
		if m := reSpringClass.FindStringSubmatch(content); len(m) == 2 {
			// Prefer class-level: first RequestMapping before methods — take first match near "class"
			prefix = m[1]
		}
		for _, loc := range reSpringMeth.FindAllStringSubmatchIndex(content, -1) {
			sub := reSpringMeth.FindStringSubmatch(content[loc[0]:loc[1]])
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
				Path:   joinRoute(prefix, subpath),
				Source: "spring",
				File:   rel,
				Line:   line,
			})
		}
		for _, loc := range reSpringReq.FindAllStringSubmatchIndex(content, -1) {
			sub := reSpringReq.FindStringSubmatch(content[loc[0]:loc[1]])
			if len(sub) != 3 {
				continue
			}
			line := 1 + strings.Count(content[:loc[0]], "\n")
			routes = append(routes, Route{
				Method: strings.ToUpper(sub[1]),
				Path:   joinRoute(prefix, sub[2]),
				Source: "spring",
				File:   rel,
				Line:   line,
			})
		}
		return nil
	})
	return routes, nil
}
