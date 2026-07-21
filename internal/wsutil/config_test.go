package wsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadProject(t *testing.T) {
	dir := t.TempDir()
	cfg := ProjectConfig{URLs: []string{"ws://localhost:3000/ws", "wss://echo.test/"}}
	if err := SaveProject(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".devscope", "ws.json")); err != nil {
		t.Fatal(err)
	}
	got := LoadProject(dir)
	if len(got.URLs) != 2 || got.URLs[0] != "ws://localhost:3000/ws" {
		t.Fatalf("%+v", got)
	}
}

func TestLoadProjectMissing(t *testing.T) {
	got := LoadProject(t.TempDir())
	if len(got.URLs) != 0 {
		t.Fatalf("%+v", got)
	}
}
