package core

import (
	"path/filepath"
	"sync"
	"time"
)

type StateStore struct {
	mu       sync.RWMutex
	snapshot *Snapshot
}

func NewStateStore(scanPaths []string) *StateStore {
	return &StateStore{
		snapshot: &Snapshot{
			Projects:  []Project{},
			ScanPaths: scanPaths,
			ScannedAt: time.Now(),
		},
	}
}

func (s *StateStore) Get() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.Clone()
}

func (s *StateStore) Update(fn func(*Snapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := s.snapshot.Clone()
	fn(&clone)
	s.snapshot = &clone
}

func (s *Snapshot) Clone() Snapshot {
	clone := *s
	if s.Projects != nil {
		clone.Projects = make([]Project, len(s.Projects))
		copy(clone.Projects, s.Projects)
	}
	if s.ScanPaths != nil {
		clone.ScanPaths = make([]string, len(s.ScanPaths))
		copy(clone.ScanPaths, s.ScanPaths)
	}
	clone.ProjectCount = len(clone.Projects)
	return clone
}

func (s *StateStore) SetProjects(projects []Project) {
	s.Update(func(snap *Snapshot) {
		prevGit := make(map[string]*GitInfo, len(snap.Projects))
		prevContainers := make(map[string][]Container, len(snap.Projects))
		for i := range snap.Projects {
			key := filepath.Clean(snap.Projects[i].Path)
			if snap.Projects[i].Git != nil {
				prevGit[key] = snap.Projects[i].Git
			}
			if len(snap.Projects[i].Containers) > 0 {
				prevContainers[key] = snap.Projects[i].Containers
			}
		}
		for i := range projects {
			key := filepath.Clean(projects[i].Path)
			if prev, ok := prevGit[key]; ok {
				incomingIsSummary := projects[i].Git != nil &&
					projects[i].Git.IsRepo &&
					len(projects[i].Git.Branches) == 0 &&
					len(projects[i].Git.Commits) == 0 &&
					projects[i].Git.LastCommit == ""
				prevHasDetails := len(prev.Branches) > 0 ||
					len(prev.Commits) > 0 ||
					prev.LastCommit != ""
				if projects[i].Git == nil || !projects[i].Git.IsRepo || (incomingIsSummary && prevHasDetails) {
					projects[i].Git = prev
				}
			}
			if len(projects[i].Containers) == 0 {
				if containers, ok := prevContainers[key]; ok {
					projects[i].Containers = containers
					projects[i].ContainerCount = len(containers)
				}
			}
		}
		snap.Projects = projects
		snap.ScannedAt = time.Now()
		snap.ProjectCount = len(projects)
	})
}

func (s *StateStore) SetHostMetrics(m HostMetrics) {
	s.Update(func(snap *Snapshot) {
		snap.HostMetrics = m
	})
}

func (s *StateStore) UpdateProjectGit(path string, git GitInfo) {
	path = filepath.Clean(path)
	s.Update(func(snap *Snapshot) {
		for i := range snap.Projects {
			if filepath.Clean(snap.Projects[i].Path) == path {
				copy := git
				snap.Projects[i].Git = &copy
				return
			}
		}
	})
}

func (s *StateStore) UpdateProjectRuntime(path string, src Project) {
	path = filepath.Clean(path)
	s.Update(func(snap *Snapshot) {
		for i := range snap.Projects {
			if filepath.Clean(snap.Projects[i].Path) != path {
				continue
			}
			p := &snap.Projects[i]
			p.Containers = src.Containers
			p.ContainerCount = src.ContainerCount
			p.Metrics = src.Metrics
			p.Health = src.Health
			p.HealthChecks = src.HealthChecks
			p.Workers = src.Workers
			p.WorkerCount = src.WorkerCount
			p.Status = src.Status
			return
		}
	})
}

func (s *StateStore) UpdateProjectStatus(path string, status ProjectStatus) {
	s.Update(func(snap *Snapshot) {
		for i := range snap.Projects {
			if snap.Projects[i].Path == path {
				snap.Projects[i].Status = status
				return
			}
		}
	})
}
