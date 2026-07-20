package routeutil

import (
	"sort"
	"strings"
)

// Discover finds routes via OpenAPI then scanners for stacks detected in the project.
// stacks lists the resolved route stacks (e.g. nestjs, django, gin).
func Discover(projectPath string, ports []int, frameworks []string) (routes []Route, stacks []string, err error) {
	stacks = DetectStacks(projectPath, frameworks)

	var all []Route
	if r, e := loadOpenAPIFromFiles(projectPath); e == nil {
		all = append(all, r...)
	}
	if len(ports) > 0 {
		if r, e := loadOpenAPIFromHTTP(ports); e == nil {
			all = append(all, r...)
		}
	}

	for _, s := range scanners {
		if !s.Match(projectPath, stacks) {
			continue
		}
		found, e := s.Scan(projectPath)
		if e != nil {
			continue
		}
		all = append(all, found...)
	}

	return mergeRoutes(all), stacks, nil
}

func mergeRoutes(in []Route) []Route {
	type key struct{ m, p string }
	best := map[key]Route{}
	for _, r := range in {
		r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
		r.Path = normalizePath(r.Path)
		if r.Method == "" || r.Path == "" {
			continue
		}
		k := key{r.Method, r.Path}
		prev, ok := best[k]
		if !ok {
			best[k] = r
			continue
		}
		if prev.Source != "openapi" && r.Source == "openapi" {
			best[k] = r
			continue
		}
		if prev.Summary == "" && r.Summary != "" {
			best[k] = r
		}
	}
	out := make([]Route, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return methodOrder(out[i].Method) < methodOrder(out[j].Method)
	})
	return out
}

func methodOrder(m string) int {
	switch m {
	case "GET":
		return 0
	case "POST":
		return 1
	case "PUT":
		return 2
	case "PATCH":
		return 3
	case "DELETE":
		return 4
	default:
		return 9
	}
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}
