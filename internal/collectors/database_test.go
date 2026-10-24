package collectors

import (
	"testing"

	"github.com/devscope/devscope/internal/core"
)

func TestParseDatabaseURL(t *testing.T) {
	user, db, pass := parseDatabaseURL("postgres://igor:s3cret@localhost:5432/digiliza?sslmode=disable")
	if user != "igor" || db != "digiliza" || pass != "s3cret" {
		t.Fatalf("got user=%q db=%q pass=%q", user, db, pass)
	}
}

func TestDetectContainerDBEngine(t *testing.T) {
	if detectContainerDBEngine(core.Container{Image: "postgres:16", Name: "db"}) != DBEnginePostgres {
		t.Fatal("expected postgres")
	}
	if detectContainerDBEngine(core.Container{Image: "mysql:8", Name: "mysql"}) != DBEngineMySQL {
		t.Fatal("expected mysql")
	}
	if detectContainerDBEngine(core.Container{Image: "nginx", Name: "web"}) != "" {
		t.Fatal("expected empty")
	}
}

func TestDetectProjectDatabases(t *testing.T) {
	p := &core.Project{
		Containers: []core.Container{
			{Name: "app-db-1", Image: "postgres:15", State: "running"},
			{Name: "web", Image: "node:20", State: "running"},
		},
	}
	got := DetectProjectDatabases(p)
	if len(got) != 1 || got[0].Engine != DBEnginePostgres {
		t.Fatalf("got %+v", got)
	}
}

func TestDetectContainerDBEngineExtras(t *testing.T) {
	if detectContainerDBEngine(core.Container{Image: "timescale/timescaledb:latest"}) != DBEnginePostgres {
		t.Fatal("timescale")
	}
	if detectContainerDBEngine(core.Container{Image: "mariadb:11"}) != DBEngineMySQL {
		t.Fatal("mariadb")
	}
}

func TestEscapeSQLLiteral(t *testing.T) {
	if escapeSQLLiteral("a'b") != "a''b" {
		t.Fatal(escapeSQLLiteral("a'b"))
	}
}

func TestIsDataContainer(t *testing.T) {
	for _, c := range []core.Container{
		{Image: "postgres:16"}, {Image: "mariadb:11"}, {Image: "redis:7-alpine"},
		{Image: "mongo:7"}, {Image: "memcached"}, {Image: "docker.elastic.co/elasticsearch/elasticsearch:8"},
		{Image: "opensearchproject/opensearch"}, {Image: "rabbitmq:3-management"},
		{Image: "getmeili/meilisearch"}, {Image: "custom:1", Name: "laradock-redis-1"},
	} {
		if !IsDataContainer(c) {
			t.Errorf("%+v deveria ser de dados", c)
		}
	}
	for _, c := range []core.Container{{Image: "nginx"}, {Image: "laradock-php-fpm", Name: "laradock-php-fpm-1"}} {
		if IsDataContainer(c) {
			t.Errorf("%+v não é de dados", c)
		}
	}
}
