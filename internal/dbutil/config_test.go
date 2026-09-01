package dbutil

import "testing"

func TestSaveLoadRoundTripAndUpsert(t *testing.T) {
	dir := t.TempDir()
	cfg := LoadProject(dir)
	cfg.Upsert(Credential{Name: "prod", Engine: "postgres", Host: "db.example.com", Port: 5432, User: "app", Password: "s3cr3t", Database: "app"})
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	loaded := LoadProject(dir)
	if len(loaded.Credentials) != 1 || loaded.Credentials[0].Host != "db.example.com" {
		t.Fatalf("unexpected load: %+v", loaded.Credentials)
	}

	loaded.Upsert(Credential{Name: "prod", Engine: "postgres", Host: "db2.example.com", User: "app", Database: "app"})
	if len(loaded.Credentials) != 1 || loaded.Credentials[0].Host != "db2.example.com" {
		t.Fatalf("upsert should replace by name, got: %+v", loaded.Credentials)
	}

	loaded.Remove("prod")
	if len(loaded.Credentials) != 0 {
		t.Fatalf("remove should drop the credential, got: %+v", loaded.Credentials)
	}
}
