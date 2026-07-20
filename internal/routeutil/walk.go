package routeutil

import (
	"os"
	"path/filepath"
	"strings"
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "__pycache__": true, "coverage": true,
	".venv": true, "venv": true, "target": true, ".idea": true,
}

func walkProject(root string, fn func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		return fn(path)
	})
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func hasFramework(frameworks []string, names ...string) bool {
	for _, f := range frameworks {
		for _, n := range names {
			if strings.Contains(f, n) {
				return true
			}
		}
	}
	return false
}

func parseQuotedMethods(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.Trim(part, " \t'\"[]")
		part = strings.ToUpper(part)
		switch part {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{"GET"}
	}
	return out
}

func joinRoute(prefix, path string) string {
	prefix = strings.TrimSpace(prefix)
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return normalizePath(prefix)
	}
	if strings.HasPrefix(path, "/") {
		if prefix == "" {
			return normalizePath(path)
		}
		return normalizePath(strings.TrimRight(prefix, "/") + path)
	}
	if prefix == "" {
		return normalizePath("/" + path)
	}
	return normalizePath(strings.TrimRight(prefix, "/") + "/" + path)
}
