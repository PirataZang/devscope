package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type laravelScanner struct{}

func (laravelScanner) Name() string { return "laravel" }

func (laravelScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "laravel") {
		return true
	}
	return fileExists(filepath.Join(root, "artisan"))
}

var (
	reLaravelRoute = regexp.MustCompile(`(?i)Route::(get|post|put|patch|delete|any|match)\s*\(\s*['"]([^'"]+)['"]`)
	reLaravelRes   = regexp.MustCompile(`(?i)Route::(apiResource|resource)\s*\(\s*['"]([^'"]+)['"]`)
)

func (laravelScanner) Scan(root string) ([]Route, error) {
	dir := filepath.Join(root, "routes")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, nil
	}
	var routes []Route
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".php") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		content := string(data)
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if m := reLaravelRoute.FindStringSubmatch(line); len(m) == 3 {
				method := strings.ToUpper(m[1])
				if method == "ANY" || method == "MATCH" {
					for _, meth := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
						routes = append(routes, Route{
							Method: meth, Path: m[2], Source: "laravel", File: rel, Line: i + 1,
						})
					}
					continue
				}
				routes = append(routes, Route{
					Method: method, Path: m[2], Source: "laravel", File: rel, Line: i + 1,
				})
			}
			if m := reLaravelRes.FindStringSubmatch(line); len(m) == 3 {
				base := strings.Trim(m[2], "/")
				name := strings.TrimSuffix(filepath.Base(base), "s")
				if name == "" {
					name = "id"
				}
				id := "{" + name + "}"
				prefix := "/" + base
				for _, r := range []struct{ m, p string }{
					{"GET", prefix},
					{"POST", prefix},
					{"GET", prefix + "/" + id},
					{"PUT", prefix + "/" + id},
					{"PATCH", prefix + "/" + id},
					{"DELETE", prefix + "/" + id},
				} {
					routes = append(routes, Route{
						Method: r.m, Path: r.p, Source: "laravel", File: rel, Line: i + 1,
						Summary: m[1],
					})
				}
			}
		}
		return nil
	})
	return routes, nil
}
