// Package devscopeutil manages the per-project .devscope directory, where
// devscope stores tunnel and database configs that can hold credentials.
package devscopeutil

import (
	"os"
	"path/filepath"
	"strings"
)

const ignoreLine = ".devscope/"

// EnsureDir creates projectPath/.devscope and makes sure the project's
// .gitignore excludes it, since files written there (ssh.json, ngrok.json,
// database.json...) can hold passwords and tokens that must never be
// committed.
func EnsureDir(projectPath string) (string, error) {
	dir := filepath.Join(projectPath, ".devscope")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ensureGitignore(projectPath)
	return dir, nil
}

func ensureGitignore(projectPath string) {
	path := filepath.Join(projectPath, ".gitignore")
	b, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == ignoreLine {
			return
		}
	}
	content := string(b)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += ignoreLine + "\n"
	_ = os.WriteFile(path, []byte(content), 0o644)
}
