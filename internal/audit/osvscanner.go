package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	osvmodels "github.com/google/osv-scanner/v2/pkg/models"
	"github.com/google/osv-scanner/v2/pkg/osvscanner"
)

type osvScanner struct{}

func newOSVScanner() scanner {
	return osvScanner{}
}

func (osvScanner) Scan(_ context.Context, req ScanRequest) (*Result, error) {
	result, err := osvscanner.DoScan(osvscanner.ScannerActions{
		LockfilePaths:     req.Lockfiles,
		DirectoryPaths:    req.Directories,
		CompareOffline:    req.Offline,
		DownloadDatabases: req.DownloadDB,
		LocalDBPath:       req.DBPath,
		ShowAllVulns:      true,
	})
	if err != nil && !errors.Is(err, osvscanner.ErrVulnerabilitiesFound) && !errors.Is(err, osvscanner.ErrNoPackagesFound) {
		return nil, err
	}

	mapped := mapResults(req, result)
	return mapped, nil
}

func mapResults(req ScanRequest, raw osvmodels.VulnerabilityResults) *Result {
	findings := make([]Finding, 0)
	affectedPackages := make(map[string]struct{})

	for _, flattened := range raw.Flatten() {
		if flattened.Vulnerability == nil || flattened.Vulnerability.GetId() == "" {
			continue
		}

		packageKey := fmt.Sprintf("%s@%s", flattened.Package.Name, flattened.Package.Version)
		affectedPackages[packageKey] = struct{}{}

		findings = append(findings, Finding{
			ID:             flattened.Vulnerability.GetId(),
			PackageName:    flattened.Package.Name,
			Version:        flattened.Package.Version,
			Ecosystem:      flattened.Package.Ecosystem,
			Severity:       flattened.GroupInfo.MaxSeverity,
			SourcePath:     flattened.Source.Path,
			SourceType:     string(flattened.Source.Type),
			Summary:        strings.TrimSpace(flattened.Vulnerability.GetSummary()),
			Details:        compactDetails(flattened.Vulnerability.GetDetails()),
			Aliases:        flattened.Vulnerability.GetAliases(),
			Recommendation: buildRecommendation(flattened),
		})
	}

	return &Result{
		Summary: Summary{
			RootDir:            req.RootDir,
			LockfilesScanned:   len(req.Lockfiles),
			DirectoriesScanned: len(req.Directories),
			TotalFindings:      len(findings),
			AffectedPackages:   len(affectedPackages),
		},
		Findings: findings,
	}
}

func compactDetails(details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return ""
	}

	details = strings.Join(strings.Fields(details), " ")
	if len(details) > 180 {
		return details[:177] + "..."
	}

	return details
}

func buildRecommendation(flattened osvmodels.VulnerabilityFlattened) string {
	return fmt.Sprintf(
		"Upgrade %s to a patched version with your project package manager and rerun `npu audit`.",
		flattened.Package.Name,
	)
}
