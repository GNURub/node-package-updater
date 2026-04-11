package packagemanager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestFindLockfiles(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "bun.lock"), []byte("lock"), 0o644); err != nil {
		t.Fatalf("write bun.lock: %v", err)
	}

	lockfiles := Bun.FindLockfiles(root)
	if len(lockfiles) != 1 || lockfiles[0] != filepath.Join(root, "bun.lock") {
		t.Fatalf("unexpected lockfiles: %v", lockfiles)
	}
}

func TestInstallInDirUsesTargetDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script test")
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	workDir := filepath.Join(root, "project")
	outputFile := filepath.Join(root, "install.out")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir binDir: %v", err)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir workDir: %v", err)
	}

	npmScript := filepath.Join(binDir, "npm")
	script := "#!/bin/sh\npwd > \"" + outputFile + "\"\nprintf '%s' \"$*\" >> \"" + outputFile + "\"\n"
	if err := os.WriteFile(npmScript, []byte(script), 0o755); err != nil {
		t.Fatalf("write npm script: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	if err := Npm.InstallInDir(workDir, "--frozen-lockfile"); err != nil {
		t.Fatalf("install in dir: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	output := string(content)
	if !strings.Contains(output, workDir) {
		t.Fatalf("expected cwd %q in output %q", workDir, output)
	}
	if !strings.Contains(output, "install --frozen-lockfile") {
		t.Fatalf("expected install args in output %q", output)
	}
}
