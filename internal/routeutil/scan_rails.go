package routeutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type railsScanner struct{}

func (railsScanner) Name() string { return "rails" }

func (railsScanner) Match(root string, frameworks []string) bool {
	return hasFramework(frameworks, "rails") || fileExists(filepath.Join(root, "config", "routes.rb"))
}

var (
	reRailsVerb     = regexp.MustCompile(`(?m)^\s*(get|post|put|patch|delete)\s+['"]([^'"]+)['"]`)
	reRailsResource = regexp.MustCompile(`(?m)^\s*(resources|resource)\s+:(\w+)`)
)

func (railsScanner) Scan(root string) ([]Route, error) {
	path := filepath.Join(root, "config", "routes.rb")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	content := string(data)
	rel := "config/routes.rb"
	var routes []Route
	for _, loc := range reRailsVerb.FindAllStringSubmatchIndex(content, -1) {
		sub := reRailsVerb.FindStringSubmatch(content[loc[0]:loc[1]])
		if len(sub) != 3 {
			continue
		}
		line := 1 + strings.Count(content[:loc[0]], "\n")
		routes = append(routes, Route{
			Method: strings.ToUpper(sub[1]),
			Path:   sub[2],
			Source: "rails",
			File:   rel,
			Line:   line,
		})
	}
	for _, loc := range reRailsResource.FindAllStringSubmatchIndex(content, -1) {
		sub := reRailsResource.FindStringSubmatch(content[loc[0]:loc[1]])
		if len(sub) != 3 {
			continue
		}
		line := 1 + strings.Count(content[:loc[0]], "\n")
		name := sub[2]
		base := "/" + name
		if sub[1] == "resource" {
			for _, r := range []struct{ m, p string }{
				{"GET", base}, {"POST", base}, {"PUT", base}, {"PATCH", base}, {"DELETE", base},
			} {
				routes = append(routes, Route{Method: r.m, Path: r.p, Source: "rails", File: rel, Line: line, Summary: sub[1]})
			}
			continue
		}
		id := "/{id}"
		for _, r := range []struct{ m, p string }{
			{"GET", base}, {"POST", base},
			{"GET", base + id}, {"PUT", base + id}, {"PATCH", base + id}, {"DELETE", base + id},
		} {
			routes = append(routes, Route{Method: r.m, Path: r.p, Source: "rails", File: rel, Line: line, Summary: "resources"})
		}
	}
	return routes, nil
}
