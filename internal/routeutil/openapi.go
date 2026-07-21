package routeutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var openAPIFileNames = []string{
	"openapi.json", "openapi.yaml", "openapi.yml",
	"swagger.json", "swagger.yaml", "swagger.yml",
}

var openAPIHTTPPaths = []string{
	"/openapi.json",
	"/swagger/doc.json",
	"/swagger.json",
	"/v3/api-docs",
	"/api-docs",
}

func loadOpenAPIFromFiles(root string) ([]Route, error) {
	var routes []Route
	err := walkProject(root, func(path string) error {
		base := strings.ToLower(filepath.Base(path))
		ok := false
		for _, n := range openAPIFileNames {
			if base == n {
				ok = true
				break
			}
		}
		if !ok {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		parsed, err := parseOpenAPI(data)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for i := range parsed {
			parsed[i].Source = "openapi"
			parsed[i].File = rel
		}
		routes = append(routes, parsed...)
		return nil
	})
	return routes, err
}

func loadOpenAPIFromHTTP(ports []int) ([]Route, error) {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	var routes []Route
	seen := map[string]bool{}
	for _, port := range ports {
		if port <= 0 {
			continue
		}
		for _, p := range openAPIHTTPPaths {
			url := fmt.Sprintf("http://127.0.0.1:%d%s", port, p)
			if seen[url] {
				continue
			}
			seen[url] = true
			resp, err := client.Get(url)
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				continue
			}
			parsed, err := parseOpenAPI(body)
			if err != nil || len(parsed) == 0 {
				continue
			}
			for i := range parsed {
				parsed[i].Source = "openapi"
				parsed[i].Summary = strings.TrimSpace(parsed[i].Summary)
			}
			routes = append(routes, parsed...)
			return routes, nil
		}
	}
	return routes, nil
}

func parseOpenAPI(data []byte) ([]Route, error) {
	var doc map[string]any
	trimmed := bytesTrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty")
	}
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &doc); err != nil {
			return nil, err
		}
	} else {
		if err := yaml.Unmarshal(trimmed, &doc); err != nil {
			return nil, err
		}
	}
	paths, _ := doc["paths"].(map[string]any)
	if paths == nil {
		return nil, fmt.Errorf("no paths")
	}
	var routes []Route
	for path, raw := range paths {
		ops, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for method, opRaw := range ops {
			m := strings.ToUpper(method)
			switch m {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
			default:
				continue
			}
			summary := ""
			auth := false
			if op, ok := opRaw.(map[string]any); ok {
				if s, ok := op["summary"].(string); ok {
					summary = s
				} else if s, ok := op["operationId"].(string); ok {
					summary = s
				}
				if sec, ok := op["security"].([]any); ok && len(sec) > 0 {
					auth = true
					if summary == "" {
						summary = "secured"
					}
				}
			}
			routes = append(routes, Route{
				Method:  m,
				Path:    path,
				Source:  "openapi",
				Summary: summary,
				Auth:    auth,
			})
		}
	}
	return routes, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
