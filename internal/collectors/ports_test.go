package collectors

import (
	"reflect"
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestAssignPortConflicts(t *testing.T) {
	projects := []core.Project{
		{Name: "api", Ports: []int{3000, 5432}},
		{Name: "web", Ports: []int{3000, 8080}},
		{Name: "db", Ports: []int{5432}},
		{Name: "lonely", Ports: []int{9999}},
	}

	assignPortConflicts(projects)

	// 3000 é disputada por api e web; 5432 por api e db.
	if got := projects[0].PortConflicts; !reflect.DeepEqual(got, map[int][]string{3000: {"web"}, 5432: {"db"}}) {
		t.Fatalf("api conflicts = %v", got)
	}
	if got := projects[1].PortConflicts; !reflect.DeepEqual(got, map[int][]string{3000: {"api"}}) {
		t.Fatalf("web conflicts = %v", got)
	}
	if got := projects[2].PortConflicts; !reflect.DeepEqual(got, map[int][]string{5432: {"api"}}) {
		t.Fatalf("db conflicts = %v", got)
	}
	// porta exclusiva não vira conflito
	if got := projects[3].PortConflicts; got != nil {
		t.Fatalf("lonely should have no conflicts, got %v", got)
	}
}

func TestAssignPortConflictsThreeWay(t *testing.T) {
	projects := []core.Project{
		{Name: "c", Ports: []int{80}},
		{Name: "a", Ports: []int{80}},
		{Name: "b", Ports: []int{80}},
	}
	assignPortConflicts(projects)
	// "others" vem ordenado, sem incluir o próprio projeto.
	if got := projects[0].PortConflicts[80]; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("c others = %v", got)
	}
	if got := projects[1].PortConflicts[80]; !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("a others = %v", got)
	}
}
