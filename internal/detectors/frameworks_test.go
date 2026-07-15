package detectors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNestJS(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"@nestjs/core":"^10.0.0"}}`), 0644)

	info := DetectAll(dir)
	if info.Name != "NestJS" {
		t.Errorf("expected NestJS, got %s", info.Name)
	}
}

func TestDetectLaravel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "artisan"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{}`), 0644)

	info := DetectAll(dir)
	if info.Name != "Laravel" {
		t.Errorf("expected Laravel, got %s", info.Name)
	}
}

func TestDetectGo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com\n\ngo 1.22\n"), 0644)

	info := DetectAll(dir)
	if info.Name != "Go" {
		t.Errorf("expected Go, got %s", info.Name)
	}
}

func TestDetectDockerOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}"), 0644)

	info := DetectAll(dir)
	if info.Name != "Docker" {
		t.Errorf("expected Docker, got %s", info.Name)
	}
}

func TestDetectDjango(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manage.py"), []byte("# django"), 0644)

	info := DetectAll(dir)
	if info.Name != "Django" {
		t.Errorf("expected Django, got %s", info.Name)
	}
}
