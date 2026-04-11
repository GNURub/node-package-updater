package audit

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubScanner struct {
	lastRequest ScanRequest
	result      *Result
	err         error
}

func (s *stubScanner) Scan(_ context.Context, req ScanRequest) (*Result, error) {
	s.lastRequest = req
	if s.result != nil {
		return s.result, s.err
	}

	return &Result{}, s.err
}

func TestServiceAuditBuildsTargetsFromRootAndWorkspaces(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "packages", "app")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "name": "root",
  "workspaces": ["packages/*"],
  "dependencies": {
    "react": "^19.0.0"
  }
}`), 0o644); err != nil {
		t.Fatalf("write root package.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write root lockfile: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceDir, "package.json"), []byte(`{"name":"app","dependencies":{"zod":"^4.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write workspace package.json: %v", err)
	}

	stub := &stubScanner{result: &Result{}}
	service := NewServiceWithScanner(stub)

	if _, err := service.Audit(context.Background(), Options{Path: workspaceDir}); err != nil {
		t.Fatalf("audit: %v", err)
	}

	if stub.lastRequest.RootDir != root {
		t.Fatalf("expected root dir %q, got %q", root, stub.lastRequest.RootDir)
	}

	if len(stub.lastRequest.Lockfiles) != 1 || stub.lastRequest.Lockfiles[0] != filepath.Join(root, "package-lock.json") {
		t.Fatalf("unexpected lockfiles: %v", stub.lastRequest.Lockfiles)
	}

	if len(stub.lastRequest.Directories) != 0 {
		t.Fatalf("expected no directory targets when root lockfile exists, got %v", stub.lastRequest.Directories)
	}
}

func TestServiceAuditFallsBackToDirectoriesWithoutRootLockfile(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "packages", "app")

	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{
  "name": "root",
  "workspaces": ["packages/*"],
  "dependencies": {
    "react": "^19.0.0"
  }
}`), 0o644); err != nil {
		t.Fatalf("write root package.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(workspaceDir, "package.json"), []byte(`{"name":"app","dependencies":{"zod":"^4.0.0"}}`), 0o644); err != nil {
		t.Fatalf("write workspace package.json: %v", err)
	}

	stub := &stubScanner{result: &Result{}}
	service := NewServiceWithScanner(stub)

	if _, err := service.Audit(context.Background(), Options{Path: root}); err != nil {
		t.Fatalf("audit: %v", err)
	}

	if len(stub.lastRequest.Lockfiles) != 0 {
		t.Fatalf("expected no lockfiles, got %v", stub.lastRequest.Lockfiles)
	}

	if len(stub.lastRequest.Directories) != 2 {
		t.Fatalf("expected root and workspace directories, got %v", stub.lastRequest.Directories)
	}
}

func TestFormatTextIncludesRecommendation(t *testing.T) {
	output, err := Format(&Result{
		Summary: Summary{
			RootDir:            "/tmp/project",
			LockfilesScanned:   1,
			DirectoriesScanned: 0,
			TotalFindings:      1,
			AffectedPackages:   1,
		},
		Findings: []Finding{
			{
				ID:             "OSV-2025-1",
				PackageName:    "react",
				Version:        "18.0.0",
				SourcePath:     "/tmp/project/package-lock.json",
				SourceType:     "lockfile",
				Severity:       "high",
				Summary:        "Example summary",
				Recommendation: "Upgrade react and rerun `npu audit`.",
			},
		},
	}, FormatText)
	if err != nil {
		t.Fatalf("format text: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "OSV-2025-1") || !strings.Contains(rendered, "Suggestion: Upgrade react") {
		t.Fatalf("unexpected text output: %s", rendered)
	}
}
