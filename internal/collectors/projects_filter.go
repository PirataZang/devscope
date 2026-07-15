package collectors

import (
	"os"
	"strings"

	"github.com/devscope/devscope/internal/core"
)

func filterNestedProjectList(projects []core.Project) []core.Project {
	if len(projects) < 2 {
		return projects
	}
	var result []core.Project
	for _, p := range projects {
		if isNestedPath(p.Path, projects) {
			continue
		}
		result = append(result, p)
	}
	return result
}

func isNestedPath(path string, projects []core.Project) bool {
	path = cleanPath(path)
	for _, other := range projects {
		otherPath := cleanPath(other.Path)
		if path == otherPath {
			continue
		}
		if strings.HasPrefix(path, otherPath+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func cleanPath(path string) string {
	return strings.TrimRight(path, string(os.PathSeparator))
}

// FilterNestedProjects removes nested duplicate project paths.
func FilterNestedProjects(projects []core.Project) []core.Project {
	return filterNestedProjectList(projects)
}
