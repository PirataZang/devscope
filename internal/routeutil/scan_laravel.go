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
	// Chained: Route::middleware('auth')->get(...), Route::prefix('v1')->post(...)
	reLaravelRoute = regexp.MustCompile(`(?i)Route::(?:[A-Za-z_]+\s*\([^)]*\)\s*->\s*)*(get|post|put|patch|delete|any|match)\s*\(\s*['"]([^'"]+)['"]`)
	reLaravelRes   = regexp.MustCompile(`(?i)Route::(?:[A-Za-z_]+\s*\([^)]*\)\s*->\s*)*(apiResource|resource)\s*\(\s*['"]([^'"]+)['"]`)
	reLaravelAuthGroup = regexp.MustCompile(`(?i)middleware\s*\([^)]*(?:auth|sanctum|passport|can:|permission)[^)]*\)\s*->\s*group\s*\(`)
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
		routes = append(routes, scanLaravelFile(rel, string(data))...)
		return nil
	})
	return routes, nil
}

func scanLaravelFile(rel, content string) []Route {
	var routes []Route
	lines := strings.Split(content, "\n")
	brace := 0
	var authAt []int // brace levels that opened an auth group
	pendingAuth := false

	for i, line := range lines {
		if reLaravelAuthGroup.MatchString(line) {
			pendingAuth = true
		}
		inAuth := len(authAt) > 0 || laravelLineLooksPrivate(line)

		if m := reLaravelRoute.FindStringSubmatch(line); len(m) == 3 {
			method := strings.ToUpper(m[1])
			path := m[2]
			summary := ""
			if inAuth {
				summary = "auth"
			}
			if method == "ANY" || method == "MATCH" {
				for _, meth := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
					routes = append(routes, Route{
						Method: meth, Path: path, Source: "laravel", File: rel, Line: i + 1,
						Summary: summary, Auth: inAuth,
					})
				}
			} else {
				routes = append(routes, Route{
					Method: method, Path: path, Source: "laravel", File: rel, Line: i + 1,
					Summary: summary, Auth: inAuth,
				})
			}
		}
		if m := reLaravelRes.FindStringSubmatch(line); len(m) == 3 {
			base := strings.Trim(m[2], "/")
			name := strings.TrimSuffix(filepath.Base(base), "s")
			if name == "" {
				name = "id"
			}
			id := "{" + name + "}"
			prefix := "/" + base
			sum := m[1]
			if inAuth {
				sum = m[1] + "+auth"
			}
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
					Summary: sum, Auth: inAuth,
				})
			}
		}

		for _, ch := range line {
			switch ch {
			case '{':
				brace++
				if pendingAuth {
					authAt = append(authAt, brace)
					pendingAuth = false
				}
			case '}':
				if n := len(authAt); n > 0 && authAt[n-1] == brace {
					authAt = authAt[:n-1]
				}
				if brace > 0 {
					brace--
				}
			}
		}
	}
	return routes
}

func laravelLineLooksPrivate(line string) bool {
	low := strings.ToLower(line)
	if !strings.Contains(low, "middleware") && !strings.Contains(low, "can:") {
		return false
	}
	return strings.Contains(low, "auth") ||
		strings.Contains(low, "sanctum") ||
		strings.Contains(low, "passport") ||
		strings.Contains(low, "permission") ||
		strings.Contains(low, "can:")
}
