package collectors

import (
	"path/filepath"
	"testing"
)

func TestProjectShellUsesProjectDir(t *testing.T) {
	path := filepath.Clean("/tmp/my-project")
	cmd := ProjectShell(path)
	if cmd.Dir != path {
		t.Fatalf("expected Dir %q, got %q", path, cmd.Dir)
	}
	if len(cmd.Args) == 0 {
		t.Fatal("expected shell command")
	}
}
