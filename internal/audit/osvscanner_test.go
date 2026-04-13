package audit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePackageLockPackagesObject(t *testing.T) {
	tempDir := t.TempDir()
	lockfile := filepath.Join(tempDir, "package-lock.json")
	content := `{
	  "packages": {
	    "": {"name": "root", "version": "1.0.0"},
	    "node_modules/react": {"version": "18.2.0"},
	    "node_modules/@types/node": {"version": "20.11.0"}
	  }
	}`
	if err := os.WriteFile(lockfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	packages, err := parsePackageLock(lockfile)
	if err != nil {
		t.Fatalf("parsePackageLock: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
	if packages[0].SourceType != "lockfile" || packages[1].SourceType != "lockfile" {
		t.Fatalf("expected lockfile source type, got %+v", packages)
	}
}

func TestParsePNPMPackageKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantName   string
		wantVer    string
	}{
		{name: "plain", key: "react@18.2.0", wantName: "react", wantVer: "18.2.0"},
		{name: "scoped", key: "/@types/node@20.11.0", wantName: "@types/node", wantVer: "20.11.0"},
		{name: "peer suffix", key: "/react@18.2.0(typescript@5.0.0)", wantName: "react", wantVer: "18.2.0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotVer := parsePNPMPackageKey(tc.key)
			if gotName != tc.wantName || gotVer != tc.wantVer {
				t.Fatalf("parsePNPMPackageKey(%q) = (%q, %q), want (%q, %q)", tc.key, gotName, gotVer, tc.wantName, tc.wantVer)
			}
		})
	}
}

func TestParseYarnLock(t *testing.T) {
	tempDir := t.TempDir()
	lockfile := filepath.Join(tempDir, "yarn.lock")
	content := `react@^18.0.0:
	  version "18.2.0"

"@types/node@^20.0.0":
	version "20.11.0"
`
	if err := os.WriteFile(lockfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write yarn.lock: %v", err)
	}

	packages, err := parseYarnLock(lockfile)
	if err != nil {
		t.Fatalf("parseYarnLock: %v", err)
	}

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
	if packages[0].Version == "" || packages[1].Version == "" {
		t.Fatalf("expected resolved versions, got %+v", packages)
	}
}

func TestOSVScannerScanBuildsFindingsFromLockfile(t *testing.T) {
	tempDir := t.TempDir()
	lockfile := filepath.Join(tempDir, "package-lock.json")
	content := `{
	  "packages": {
	    "node_modules/react": {"version": "18.2.0"}
	  }
	}`
	if err := os.WriteFile(lockfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/querybatch":
			_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"GHSA-test"}]}]}`))
		case "/query":
			_, _ = w.Write([]byte(`{"vulns":[{"id":"GHSA-test","summary":"Example vuln","details":"A long explanation","aliases":["CVE-2026-1"],"database_specific":{"severity":"HIGH"}}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	scanner := osvScanner{client: newTestClient(srv.URL)}
	result, err := scanner.Scan(context.Background(), ScanRequest{
		RootDir:   tempDir,
		Lockfiles: []string{lockfile},
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if result.Summary.TotalFindings != 1 {
		t.Fatalf("expected 1 finding, got %d", result.Summary.TotalFindings)
	}
	if result.Findings[0].PackageName != "react" {
		t.Fatalf("expected react package, got %+v", result.Findings[0])
	}
	if result.Findings[0].Severity != "high" {
		t.Fatalf("expected high severity, got %q", result.Findings[0].Severity)
	}
	if result.Findings[0].SourcePath != lockfile {
		t.Fatalf("expected lockfile source path, got %q", result.Findings[0].SourcePath)
	}
}