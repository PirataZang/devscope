package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type goScanner struct{}

func (goScanner) Name() string { return "go" }

func (goScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "go", "gin", "echo", "fiber", "chi", "mux") {
		return true
	}
	return fileExists(filepath.Join(root, "go.mod"))
}

var (
	reGin    = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE|Any|Handle)\s*\(\s*"([^"]+)"`)
	reEcho   = regexp.MustCompile(`\.(GET|POST|PUT|PATCH|DELETE|Any|Add)\s*\(\s*"([^"]+)"`)
	reFiber  = regexp.MustCompile(`\.(Get|Post|Put|Patch|Delete|All|Add)\s*\(\s*"([^"]+)"`)
	reChi    = regexp.MustCompile(`\.(Get|Post|Put|Patch|Delete|Handle|Method)\s*\(\s*"([^"]+)"`)
	reStdlib = regexp.MustCompile(`http\.(HandleFunc|Handle)\s*\(\s*"([^"]+)"`)
)

func (goScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		rel, _ := filepath.Rel(root, path)
		for _, re := range []*regexp.Regexp{reGin, reEcho, reFiber, reChi, reStdlib} {
			for _, loc := range re.FindAllStringSubmatchIndex(content, -1) {
				sub := re.FindStringSubmatch(content[loc[0]:loc[1]])
				if len(sub) != 3 {
					continue
				}
				methods := goMethods(sub[1])
				line := 1 + strings.Count(content[:loc[0]], "\n")
				for _, m := range methods {
					routes = append(routes, Route{
						Method: m, Path: goPath(sub[2]), Source: "go", File: rel, Line: line,
					})
				}
			}
		}
		return nil
	})
	return routes, nil
}

func goMethods(m string) []string {
	m = strings.ToUpper(m)
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return []string{m}
	case "ANY", "ALL", "HANDLE", "HANDLEFUNC", "ADD", "METHOD":
		return []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	default:
		// Fiber Get -> GET
		switch strings.ToUpper(m) {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			return []string{strings.ToUpper(m)}
		}
		return []string{"GET"}
	}
}

func goPath(p string) string {
	// :id or {id} or *path
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
		if strings.HasPrefix(part, "*") {
			parts[i] = "{" + strings.TrimPrefix(part, "*") + "}"
		}
	}
	return strings.Join(parts, "/")
}
