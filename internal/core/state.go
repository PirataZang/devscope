package core

import (
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
		for i := range snap.Projects {
			if snap.Projects[i].Git != nil {
				prevGit[snap.Projects[i].Path] = snap.Projects[i].Git
			}
		}
		for i := range projects {
			if projects[i].Git == nil {
				if git, ok := prevGit[projects[i].Path]; ok {
					projects[i].Git = git
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
	s.Update(func(snap *Snapshot) {
		for i := range snap.Projects {
			if snap.Projects[i].Path == path {
				copy := git
				snap.Projects[i].Git = &copy
				return
			}
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
