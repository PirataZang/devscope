package ui

import "path/filepath"

func pathsMatch(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}
