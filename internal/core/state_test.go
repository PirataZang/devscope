package core

import (
	"sync"
	"testing"
)

func TestStateStoreConcurrency(t *testing.T) {
	store := NewStateStore([]string{"/tmp"})
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			store.SetHostMetrics(HostMetrics{CPUPercent: float64(n)})
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = store.Get()
		}()
	}

	wg.Wait()
	snap := store.Get()
	if snap.HostMetrics.CPUPercent < 0 {
		t.Error("invalid cpu percent")
	}
}

func TestSetProjectsPreservesGit(t *testing.T) {
	store := NewStateStore([]string{"/tmp"})
	git := &GitInfo{IsRepo: true, Branch: "main"}
	store.SetProjects([]Project{{Path: "/p1", Name: "one", Git: git}})

	store.SetProjects([]Project{{Path: "/p1", Name: "one-updated"}})

	snap := store.Get()
	if len(snap.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(snap.Projects))
	}
	if snap.Projects[0].Git == nil || snap.Projects[0].Git.Branch != "main" {
		t.Fatal("git info should be preserved across SetProjects")
	}
	if snap.Projects[0].Name != "one-updated" {
		t.Fatal("project fields should still update")
	}
}

func TestSnapshotClone(t *testing.T) {
	orig := &Snapshot{
		Projects: []Project{{Name: "a"}, {Name: "b"}},
		ScanPaths: []string{"/tmp"},
	}
	clone := orig.Clone()
	clone.Projects[0].Name = "changed"

	if orig.Projects[0].Name == "changed" {
		t.Error("clone should be independent copy")
	}
}
