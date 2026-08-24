package collectors

import (
	"os"
	"sort"
)

// ComposeDependency is one service's depends_on edges from the project's
// compose file (supports both the list form and the newer map-with-condition
// form).
type ComposeDependency struct {
	Service   string
	DependsOn []string
}

// ParseComposeDependencies reads depends_on relationships from the project's
// compose file, sorted by service name.
func ParseComposeDependencies(projectPath string) []ComposeDependency {
	file := ComposeFile(projectPath)
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	services, err := parseComposeServices(string(data))
	if err != nil || len(services) == 0 {
		return nil
	}

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)

	deps := make([]ComposeDependency, 0, len(names))
	for _, name := range names {
		cfg, _ := asStringMap(services[name])
		deps = append(deps, ComposeDependency{Service: name, DependsOn: dependsOnNames(cfg["depends_on"])})
	}
	return deps
}

func dependsOnNames(raw any) []string {
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case map[string]any:
		out := make([]string, 0, len(v))
		for k := range v {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	default:
		return nil
	}
}
