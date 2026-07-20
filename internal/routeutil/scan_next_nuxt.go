package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type nextjsScanner struct{}

func (nextjsScanner) Name() string { return "nextjs" }

func (nextjsScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "nextjs", "next") {
		return true
	}
	for _, f := range []string{"next.config.js", "next.config.mjs", "next.config.ts"} {
		if fileExists(filepath.Join(root, f)) {
			return true
		}
	}
	return depsContain(root, "\"next\"")
}

type nuxtScanner struct{}

func (nuxtScanner) Name() string { return "nuxt" }

func (nuxtScanner) Match(root string, frameworks []string) bool {
	if hasFramework(frameworks, "nuxt") {
		return true
	}
	for _, f := range []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs"} {
		if fileExists(filepath.Join(root, f)) {
			return true
		}
	}
	return depsContain(root, "\"nuxt\"")
}

var reNextExport = regexp.MustCompile(`(?m)^export\s+(?:async\s+)?function\s+(GET|POST|PUT|PATCH|DELETE)\b`)

func (nextjsScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		base := filepath.Base(path)
		rel, _ := filepath.Rel(root, path)
		relSlash := filepath.ToSlash(rel)

		if base == "route.ts" || base == "route.js" || base == "route.tsx" || base == "route.jsx" {
			apiPath := nextAppRoutePath(relSlash)
			if apiPath == "" {
				return nil
			}
			data, _ := os.ReadFile(path)
			methods := reNextExport.FindAllStringSubmatch(string(data), -1)
			if len(methods) == 0 {
				routes = append(routes, Route{Method: "GET", Path: apiPath, Source: "nextjs", File: rel})
				return nil
			}
			for _, m := range methods {
				routes = append(routes, Route{Method: m[1], Path: apiPath, Source: "nextjs", File: rel})
			}
			return nil
		}

		if strings.Contains(relSlash, "/pages/api/") || strings.HasPrefix(relSlash, "pages/api/") {
			if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".js") &&
				!strings.HasSuffix(path, ".tsx") && !strings.HasSuffix(path, ".jsx") {
				return nil
			}
			p := pagesAPIPath(relSlash)
			routes = append(routes, Route{Method: "GET", Path: p, Source: "nextjs", File: rel})
			routes = append(routes, Route{Method: "POST", Path: p, Source: "nextjs", File: rel})
		}
		return nil
	})
	return routes, nil
}

func nextAppRoutePath(rel string) string {
	rel = filepath.ToSlash(rel)
	idx := strings.Index(rel, "app/")
	if idx < 0 {
		return ""
	}
	rest := rel[idx+len("app/"):]
	for _, suf := range []string{"/route.ts", "/route.js", "/route.tsx", "/route.jsx"} {
		rest = strings.TrimSuffix(rest, suf)
	}
	parts := strings.Split(rest, "/")
	var segs []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
			continue
		}
		if strings.HasPrefix(p, "@") {
			continue
		}
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			name := strings.TrimSuffix(strings.TrimPrefix(p, "["), "]")
			name = strings.TrimPrefix(name, "...")
			segs = append(segs, "{"+name+"}")
			continue
		}
		segs = append(segs, p)
	}
	if len(segs) == 0 {
		return "/"
	}
	return "/" + strings.Join(segs, "/")
}

func pagesAPIPath(rel string) string {
	const marker = "pages/api/"
	i := strings.Index(rel, marker)
	if i < 0 {
		return "/api"
	}
	rest := rel[i+len(marker):]
	rest = strings.TrimSuffix(rest, filepath.Ext(rest))
	if rest == "index" || rest == "" {
		return "/api"
	}
	rest = strings.TrimSuffix(rest, "/index")
	parts := strings.Split(rest, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			parts[i] = "{" + strings.TrimSuffix(strings.TrimPrefix(p, "["), "]") + "}"
		}
	}
	return "/api/" + strings.Join(parts, "/")
}

func (nuxtScanner) Scan(root string) ([]Route, error) {
	var routes []Route
	_ = walkProject(root, func(path string) error {
		rel, _ := filepath.Rel(root, path)
		relSlash := filepath.ToSlash(rel)
		if !strings.Contains(relSlash, "server/api/") && !strings.HasPrefix(relSlash, "server/api/") {
			return nil
		}
		if !strings.HasSuffix(path, ".ts") && !strings.HasSuffix(path, ".js") {
			return nil
		}
		p, method := nuxtAPIRoute(relSlash)
		routes = append(routes, Route{Method: method, Path: p, Source: "nuxt", File: rel})
		return nil
	})
	return routes, nil
}

func nuxtAPIRoute(rel string) (string, string) {
	const marker = "server/api/"
	i := strings.Index(rel, marker)
	if i < 0 {
		return "/api", "GET"
	}
	rest := rel[i+len(marker):]
	rest = strings.TrimSuffix(rest, filepath.Ext(rest))
	method := "GET"
	for _, m := range []string{"get", "post", "put", "patch", "delete"} {
		suf := "." + m
		if strings.HasSuffix(strings.ToLower(rest), suf) {
			method = strings.ToUpper(m)
			rest = rest[:len(rest)-len(suf)]
			break
		}
	}
	if rest == "index" || rest == "" {
		return "/api", method
	}
	rest = strings.TrimSuffix(rest, "/index")
	parts := strings.Split(rest, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
			parts[i] = "{" + strings.TrimSuffix(strings.TrimPrefix(p, "["), "]") + "}"
		}
	}
	return "/api/" + strings.Join(parts, "/"), method
}
