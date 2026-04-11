package packagemanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetWorkspacesPathsFromPnpmWorkspaceYAML(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "packages", "app")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "pnpm-workspace.yaml"), []byte("packages:\n  - packages/*\n"), 0o644); err != nil {
		t.Fatalf("write pnpm-workspace.yaml: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceDir, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
		t.Fatalf("write workspace package.json: %v", err)
	}

	paths := Pnpm.GetWorkspacesPaths(root, nil)
	if len(paths) != 1 {
		t.Fatalf("expected 1 workspace path, got %d (%v)", len(paths), paths)
	}

	if paths[0] != workspaceDir {
		t.Fatalf("expected workspace path %q, got %q", workspaceDir, paths[0])
	}
}
