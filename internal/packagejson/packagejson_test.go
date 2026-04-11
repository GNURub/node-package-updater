package packagejson

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/packagemanager"
)

func TestLoadPackageJSONLoadsWorkspacePatterns(t *testing.T) {
	tests := []struct {
		name               string
		rootPackageJSON    string
		expectedWorkspaces []string
	}{
		{
			name: "array workspaces",
			rootPackageJSON: `{
  "name": "root",
  "workspaces": ["packages/*"]
}`,
			expectedWorkspaces: []string{"packages/app"},
		},
		{
			name: "object workspaces",
			rootPackageJSON: `{
  "name": "root",
  "workspaces": {
    "packages": ["packages/*"]
  }
}`,
			expectedWorkspaces: []string{"packages/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			workspaceDir := filepath.Join(root, "packages", "app")

			if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
				t.Fatalf("mkdir workspace: %v", err)
			}

			if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(tt.rootPackageJSON), 0o644); err != nil {
				t.Fatalf("write root package.json: %v", err)
			}

			if err := os.WriteFile(filepath.Join(workspaceDir, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
				t.Fatalf("write workspace package.json: %v", err)
			}

			pkg, err := LoadPackageJSON(
				root,
				EnableWorkspaces(),
				WithPackageManager(packagemanager.Npm),
			)
			if err != nil {
				t.Fatalf("load package json: %v", err)
			}

			for _, expected := range tt.expectedWorkspaces {
				workspacePath := filepath.Join(root, expected)
				if _, ok := pkg.WorkspacesPkgs[workspacePath]; !ok {
					t.Fatalf("expected workspace %q to be loaded, got keys %v", workspacePath, keys(pkg.WorkspacesPkgs))
				}
			}
		})
	}
}

func TestGroupDependenciesByWorkspaceDoesNotDuplicateFirstDependency(t *testing.T) {
	depA1, err := dependency.NewDependency("dep-a1", "^1.0.0", constants.Dependencies, "packages/a")
	if err != nil {
		t.Fatalf("new depA1: %v", err)
	}

	depA2, err := dependency.NewDependency("dep-a2", "^1.0.0", constants.DevDependencies, "packages/a")
	if err != nil {
		t.Fatalf("new depA2: %v", err)
	}

	depB1, err := dependency.NewDependency("dep-b1", "^1.0.0", constants.Dependencies, "packages/b")
	if err != nil {
		t.Fatalf("new depB1: %v", err)
	}

	grouped := groupDependenciesByWorkspace(dependency.Dependencies{depA1, depA2, depB1})

	if got := len(grouped["packages/a"]); got != 2 {
		t.Fatalf("expected 2 dependencies in packages/a, got %d", got)
	}

	if got := len(grouped["packages/b"]); got != 1 {
		t.Fatalf("expected 1 dependency in packages/b, got %d", got)
	}

	if grouped["packages/a"][0] != depA1 || grouped["packages/a"][1] != depA2 {
		t.Fatalf("unexpected grouping order/content for packages/a")
	}
}

func TestResolveProjectRoot(t *testing.T) {
	t.Run("returns monorepo root for workspace path", func(t *testing.T) {
		root := t.TempDir()
		workspaceDir := filepath.Join(root, "packages", "app")

		if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
			t.Fatalf("mkdir workspace: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "name": "root",
  "workspaces": ["packages/*"]
}`), 0o644); err != nil {
			t.Fatalf("write root package.json: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write package-lock: %v", err)
		}

		if err := os.WriteFile(filepath.Join(workspaceDir, "package.json"), []byte(`{"name":"app"}`), 0o644); err != nil {
			t.Fatalf("write workspace package.json: %v", err)
		}

		resolvedRoot, pm, err := ResolveProjectRoot(workspaceDir, "")
		if err != nil {
			t.Fatalf("resolve project root: %v", err)
		}

		if resolvedRoot != root {
			t.Fatalf("expected root %q, got %q", root, resolvedRoot)
		}

		if pm != packagemanager.Npm {
			t.Fatalf("expected npm package manager, got %v", pm)
		}
	})

	t.Run("returns nearest package root for standalone project", func(t *testing.T) {
		root := t.TempDir()
		subDir := filepath.Join(root, "src")

		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("mkdir subdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"standalone"}`), 0o644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'"), 0o644); err != nil {
			t.Fatalf("write pnpm lockfile: %v", err)
		}

		resolvedRoot, pm, err := ResolveProjectRoot(subDir, "")
		if err != nil {
			t.Fatalf("resolve project root: %v", err)
		}

		if resolvedRoot != root {
			t.Fatalf("expected root %q, got %q", root, resolvedRoot)
		}

		if pm != packagemanager.Pnpm {
			t.Fatalf("expected pnpm package manager, got %v", pm)
		}
	})
}

func keys(m map[string]*PackageJSON) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	return out
}
