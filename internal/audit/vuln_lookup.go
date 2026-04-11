package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/GNURub/node-package-updater/internal/dependency"
	"github.com/GNURub/node-package-updater/internal/styles"
)

// CacheAccessor is the minimal cache interface required by AuditDependencies.
// Both *cache.Cache and test fakes satisfy it.
type CacheAccessor interface {
	Get(key string) ([]byte, error)
	Set(key string, data []byte) error
}

// auditCacheEntry is what we store in the cache for a package@version audit.
type auditCacheEntry struct {
	Severity string `json:"severity"`
	Count    int    `json:"count"`
	Clean    bool   `json:"clean"`
}

func auditCacheKey(name, version string) string {
	return fmt.Sprintf("audit:osv:npm:%s@%s", name, version)
}

// ExtractMaxSeverity derives the highest severity label from a set of vulnerability
// objects. It prefers database_specific.severity (human-readable string) and falls
// back to parsing CVSS base scores when that field is absent.
func ExtractMaxSeverity(vulns []Vulnerability) string {
	rank := map[string]int{
		"unknown":  0,
		"low":      1,
		"moderate": 2,
		"high":     3,
		"critical": 4,
	}

	best := "unknown"
	for _, v := range vulns {
		sev := styles.NormalizeSeverity(v.DatabaseSeverity)
		if rank[sev] > rank[best] {
			best = sev
			continue
		}
		// Try CVSS vectors as fallback
		for _, vec := range v.CVSSVectors {
			if s := severityFromCVSS(vec); rank[s] > rank[best] {
				best = s
			}
		}
	}
	return best
}

// severityFromCVSS extracts the base score from a CVSS vector string and maps
// it to a canonical severity bucket. Returns "unknown" if parsing fails.
//
// Format: CVSS:3.1/AV:N/.../... or just a numeric score.
func severityFromCVSS(vector string) string {
	// Some feeds store just the base score as a number string.
	score, err := strconv.ParseFloat(strings.TrimSpace(vector), 64)
	if err != nil {
		// Try extracting the base score from the full CVSS vector.
		// CVSS v3 base score is not embedded in the vector string itself;
		// the vector is just the parameters. We can't extract score from
		// the vector without a full CVSS library, so fall back to unknown.
		return "unknown"
	}
	return scoreToSeverity(score)
}

func scoreToSeverity(score float64) string {
	switch {
	case score >= 9.0:
		return "critical"
	case score >= 7.0:
		return "high"
	case score >= 4.0:
		return "moderate"
	case score > 0:
		return "low"
	default:
		return "unknown"
	}
}

// ResultCallback is called from worker goroutines for each completed audit.
// It is safe to call tea.Program.Send() inside this callback.
type ResultCallback func(index int, status dependency.AuditStatus, severity string, count int)

// AuditDependencies runs the two-phase OSV audit for all deps that have a
// NextVersion set. It calls onResult for each dependency as soon as its state
// is resolved — the caller should translate those calls into tea.Program.Send().
//
// c may be nil to skip caching. It respects ctx cancellation — call cancel()
// when the user closes the TUI.
func AuditDependencies(
	ctx context.Context,
	deps dependency.Dependencies,
	c CacheAccessor,
	client *OSVClient,
	onResult ResultCallback,
) {
	// Phase 0: separate deps by cache hit vs miss.
	type missEntry struct {
		index int
		dep   *dependency.Dependency
	}
	var misses []missEntry

	for i, dep := range deps {
		if dep.NextVersion == nil {
			continue
		}
		dep.SetAuditScanning()

		if c != nil {
			key := auditCacheKey(dep.PackageName, dep.NextVersion.String())
			if raw, err := c.Get(key); err == nil {
				var entry auditCacheEntry
				if json.Unmarshal(raw, &entry) == nil {
					if entry.Clean {
						onResult(i, dependency.AuditClean, "", 0)
					} else {
						onResult(i, dependency.AuditVulnerable, entry.Severity, entry.Count)
					}
					continue
				}
			}
		}
		misses = append(misses, missEntry{index: i, dep: dep})
	}

	if len(misses) == 0 {
		return
	}

	// Phase 1: single /v1/querybatch for all cache misses.
	queries := make([]BatchQuery, len(misses))
	for i, m := range misses {
		queries[i] = BatchQuery{
			Name:    m.dep.PackageName,
			Version: m.dep.NextVersion.String(),
		}
	}

	batchResults, err := client.QueryBatch(ctx, queries)
	if err != nil {
		// Network failure for phase 1 — mark everything as error.
		for _, m := range misses {
			m.dep.SetAuditError()
			onResult(m.index, dependency.AuditError, "", 0)
		}
		return
	}

	// Emit clean results immediately; collect dirty for phase 2.
	type dirtyEntry struct {
		index int
		dep   *dependency.Dependency
		count int
	}
	var dirty []dirtyEntry

	for i, m := range misses {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := batchResults[i]
		if result.Count == 0 {
			m.dep.SetAuditClean()
			onResult(m.index, dependency.AuditClean, "", 0)
			if c != nil {
				saveToCache(c, m.dep.PackageName, m.dep.NextVersion.String(), auditCacheEntry{Clean: true})
			}
			continue
		}
		dirty = append(dirty, dirtyEntry{index: m.index, dep: m.dep, count: result.Count})
	}

	if len(dirty) == 0 {
		return
	}

	// Phase 2: concurrent /v1/query only for flagged packages.
	numWorkers := min(runtime.NumCPU()*2, len(dirty))

	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	for _, d := range dirty {
		if ctx.Err() != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, dep *dependency.Dependency, knownCount int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			select {
			case <-ctx.Done():
				dep.SetAuditError()
				onResult(idx, dependency.AuditError, "", 0)
				return
			default:
			}

			vulns, err := client.QueryPackage(ctx, dep.PackageName, dep.NextVersion.String())
			if err != nil {
				dep.SetAuditError()
				onResult(idx, dependency.AuditError, "", 0)
				return
			}

			count := len(vulns)
			if count == 0 {
				count = knownCount
			}
			severity := ExtractMaxSeverity(vulns)

			dep.SetAuditResult(severity, count)
			onResult(idx, dependency.AuditVulnerable, severity, count)

			if c != nil {
				saveToCache(c, dep.PackageName, dep.NextVersion.String(),
					auditCacheEntry{Severity: severity, Count: count, Clean: false})
			}
		}(d.index, d.dep, d.count)
	}

	wg.Wait()
}

func saveToCache(c CacheAccessor, name, version string, entry auditCacheEntry) {
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = c.Set(auditCacheKey(name, version), raw)
}
