package detectors

import (
	"context"
	"sort"
)

type Detector interface {
	Name() string
	Priority() int
	Detect(ctx context.Context, root string) (*FrameworkInfo, error)
}

type FrameworkInfo struct {
	Name     string
	Version  string
	Language string
}

var registry []Detector

func Register(d Detector) {
	registry = append(registry, d)
}

func DetectAll(root string) FrameworkInfo {
	matches := DetectMatches(root)
	if len(matches) == 0 {
		return FrameworkInfo{Name: "Unknown", Language: "Unknown"}
	}
	return matches[0]
}

// DetectMatches returns every framework detected in the project root.
func DetectMatches(root string) []FrameworkInfo {
	ctx := context.Background()
	detectors := make([]Detector, len(registry))
	copy(detectors, registry)
	sort.Slice(detectors, func(i, j int) bool {
		return detectors[i].Priority() > detectors[j].Priority()
	})

	seen := make(map[string]bool)
	var matches []FrameworkInfo
	for _, d := range detectors {
		info, err := d.Detect(ctx, root)
		if err != nil || info == nil || info.Name == "" || seen[info.Name] {
			continue
		}
		seen[info.Name] = true
		matches = append(matches, *info)
	}
	return matches
}

func init() {
	Register(&NestJS{})
	Register(&Laravel{})
	Register(&Django{})
	Register(&NextJS{})
	Register(&NuxtJS{})
	Register(&Vue{})
	Register(&React{})
	Register(&Node{})
	Register(&PHP{})
	Register(&GoLang{})
	Register(&Rust{})
	Register(&Python{})
	Register(&Java{})
	Register(&DockerOnly{})
}
