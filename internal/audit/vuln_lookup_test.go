package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/GNURub/node-package-updater/internal/constants"
	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/semver"
	"github.com/valyala/fasthttp"
)

// ── ExtractMaxSeverity ───────────────────────────────────────────────────────

func TestExtractMaxSeverityDatabaseField(t *testing.T) {
	vulns := []Vulnerability{
		{DatabaseSeverity: "MODERATE"},
		{DatabaseSeverity: "HIGH"},
		{DatabaseSeverity: "LOW"},
	}
	got := ExtractMaxSeverity(vulns)
	if got != "high" {
		t.Fatalf("expected high, got %s", got)
	}
}

func TestExtractMaxSeverityCriticalBeatsHigh(t *testing.T) {
	vulns := []Vulnerability{
		{DatabaseSeverity: "HIGH"},
		{DatabaseSeverity: "CRITICAL"},
	}
	got := ExtractMaxSeverity(vulns)
	if got != "critical" {
		t.Fatalf("expected critical, got %s", got)
	}
}

func TestExtractMaxSeverityCVSSFallback(t *testing.T) {
	// database_specific.severity is absent; fallback uses CVSS numeric score.
	vulns := []Vulnerability{
		{CVSSVectors: []string{"7.5"}}, // high
	}
	got := ExtractMaxSeverity(vulns)
	if got != "high" {
		t.Fatalf("expected high, got %s", got)
	}
}

func TestExtractMaxSeverityEmptyReturnsUnknown(t *testing.T) {
	got := ExtractMaxSeverity(nil)
	if got != "unknown" {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestScoreToSeverityBuckets(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{9.5, "critical"},
		{9.0, "critical"},
		{8.9, "high"},
		{7.0, "high"},
		{6.9, "moderate"},
		{4.0, "moderate"},
		{3.9, "low"},
		{0.1, "low"},
		{0.0, "unknown"},
	}
	for _, tc := range cases {
		got := scoreToSeverity(tc.score)
		if got != tc.want {
			t.Errorf("scoreToSeverity(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

// ── OSVClient (HTTP layer) ───────────────────────────────────────────────────

func newTestClient(serverURL string) *OSVClient {
	return &OSVClient{
		client:  &fasthttp.Client{},
		baseURL: serverURL,
	}
}

func TestQueryBatchReturnsVulnIDs(t *testing.T) {
	payload := `{"results":[{"vulns":[{"id":"GHSA-xxx"},{"id":"OSV-yyy"}]},{"vulns":[]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/querybatch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	results, err := client.QueryBatch(context.Background(), []BatchQuery{
		{Name: "lodash", Version: "4.17.15"},
		{Name: "react", Version: "18.2.0"},
	})
	if err != nil {
		t.Fatalf("QueryBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Count != 2 {
		t.Errorf("expected 2 vulns for lodash, got %d", results[0].Count)
	}
	if results[1].Count != 0 {
		t.Errorf("expected 0 vulns for react, got %d", results[1].Count)
	}
}

func TestQueryPackageReturnsSeverity(t *testing.T) {
	payload := `{"vulns":[{
		"id":"GHSA-abc",
		"summary":"SQL injection",
		"database_specific":{"severity":"HIGH"},
		"severity":[]
	}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/query" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	vulns, err := client.QueryPackage(context.Background(), "lodash", "4.17.15")
	if err != nil {
		t.Fatalf("QueryPackage: %v", err)
	}
	if len(vulns) != 1 {
		t.Fatalf("expected 1 vuln, got %d", len(vulns))
	}
	if vulns[0].DatabaseSeverity != "HIGH" {
		t.Errorf("expected HIGH severity, got %q", vulns[0].DatabaseSeverity)
	}
}

func TestQueryBatchHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	_, err := client.QueryBatch(context.Background(), []BatchQuery{{Name: "x", Version: "1.0.0"}})
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

// ── AuditDependencies ───────────────────────────────────────────────────────

func TestAuditDependenciesCleanPath(t *testing.T) {
	batchResp := `{"results":[{"vulns":[]},{"vulns":[]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(batchResp))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	deps := makeDeps([]string{"react@18.2.0", "zod@3.22.0"})

	var mu sync.Mutex
	results := map[int]dependency.AuditStatus{}

	AuditDependencies(context.Background(), deps, nil, client, func(idx int, status dependency.AuditStatus, _ string, _ int) {
		mu.Lock()
		results[idx] = status
		mu.Unlock()
	})

	if results[0] != dependency.AuditClean {
		t.Errorf("dep[0] expected AuditClean, got %v", results[0])
	}
	if results[1] != dependency.AuditClean {
		t.Errorf("dep[1] expected AuditClean, got %v", results[1])
	}
}

func TestAuditDependenciesVulnerablePath(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if r.URL.Path == "/querybatch" {
			w.Write([]byte(`{"results":[{"vulns":[{"id":"GHSA-x"}]},{"vulns":[]}]}`))
		} else {
			w.Write([]byte(`{"vulns":[{"id":"GHSA-x","database_specific":{"severity":"HIGH"}}]}`))
		}
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	deps := makeDeps([]string{"lodash@4.17.15", "react@18.2.0"})

	var mu sync.Mutex
	statuses := map[int]dependency.AuditStatus{}
	severities := map[int]string{}

	AuditDependencies(context.Background(), deps, nil, client, func(idx int, status dependency.AuditStatus, severity string, _ int) {
		mu.Lock()
		statuses[idx] = status
		severities[idx] = severity
		mu.Unlock()
	})

	if statuses[0] != dependency.AuditVulnerable {
		t.Errorf("lodash expected AuditVulnerable, got %v", statuses[0])
	}
	if severities[0] != "high" {
		t.Errorf("lodash expected high severity, got %q", severities[0])
	}
	if statuses[1] != dependency.AuditClean {
		t.Errorf("react expected AuditClean, got %v", statuses[1])
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 HTTP calls (batch + detail), got %d", callCount)
	}
}

func TestAuditDependenciesNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijacker", 500)
			return
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	deps := makeDeps([]string{"lodash@4.17.15"})

	var got dependency.AuditStatus
	AuditDependencies(context.Background(), deps, nil, client, func(_ int, status dependency.AuditStatus, _ string, _ int) {
		got = status
	})

	if got != dependency.AuditError {
		t.Errorf("expected AuditError on network failure, got %v", got)
	}
}

func TestAuditDependenciesCacheHit(t *testing.T) {
	calledHTTP := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledHTTP = true
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	client := newTestClient(srv.URL)
	deps := makeDeps([]string{"lodash@4.17.15"})

	fc := newFakeCache()
	entry, _ := json.Marshal(auditCacheEntry{Clean: false, Severity: "critical", Count: 3})
	fc.Set(auditCacheKey("lodash", "4.17.15"), entry)

	var gotStatus dependency.AuditStatus
	var gotSeverity string
	AuditDependencies(context.Background(), deps, fc, client, func(_ int, status dependency.AuditStatus, severity string, _ int) {
		gotStatus = status
		gotSeverity = severity
	})

	if calledHTTP {
		t.Error("HTTP should not be called when cache has a valid entry")
	}
	if gotStatus != dependency.AuditVulnerable {
		t.Errorf("expected AuditVulnerable from cache, got %v", gotStatus)
	}
	if gotSeverity != "critical" {
		t.Errorf("expected critical from cache, got %q", gotSeverity)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// makeDeps builds a slice of Dependencies from "name@version" strings.
func makeDeps(specs []string) dependency.Dependencies {
	deps := make(dependency.Dependencies, 0, len(specs))
	for _, spec := range specs {
		at := len(spec) - 1
		for at >= 0 && spec[at] != '@' {
			at--
		}
		name := spec[:at]
		ver := spec[at+1:]

		d, err := dependency.NewDependency(name, ver, constants.Dependencies, "")
		if err != nil {
			panic("makeDeps: " + err.Error())
		}
		d.NextVersion = &dependency.Version{
			Version: semver.NewSemver(ver),
		}
		deps = append(deps, d)
	}
	return deps
}

// fakeCache is an in-memory CacheAccessor for tests.
type fakeCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string][]byte)}
}

func (f *fakeCache) Get(key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}

func (f *fakeCache) Set(key string, val []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = val
	return nil
}

type notFoundErr string

func (e notFoundErr) Error() string { return string(e) }

var errNotFound = notFoundErr("key not found")
