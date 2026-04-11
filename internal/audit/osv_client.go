package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/valyala/fasthttp"
)

const osvBaseURL = "https://api.osv.dev/v1"

// BatchQuery identifies a single package@version to query.
type BatchQuery struct {
	Name    string
	Version string
}

// BatchResult holds the vulnerability IDs returned for one query entry.
type BatchResult struct {
	VulnIDs []string
	Count   int
}

// Vulnerability holds the resolved details for a single vulnerability.
type Vulnerability struct {
	ID               string
	Summary          string
	DatabaseSeverity string   // database_specific.severity — human-readable
	CVSSVectors      []string // severity[].score when type is CVSS_V3/V4
}

// OSVClient makes direct HTTP calls to the public OSV API.
// It reuses the fasthttp client already used by the registry layer.
type OSVClient struct {
	client  *fasthttp.Client
	baseURL string
}

// NewOSVClient returns a ready-to-use OSVClient.
func NewOSVClient() *OSVClient {
	return &OSVClient{
		client:  &fasthttp.Client{},
		baseURL: osvBaseURL,
	}
}

// osvPackage is the wire representation of the package field in OSV requests.
type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// osvQueryEntry is one entry inside a /v1/querybatch request.
type osvQueryEntry struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}

// osvBatchRequest is the body sent to /v1/querybatch.
type osvBatchRequest struct {
	Queries []osvQueryEntry `json:"queries"`
}

// osvBatchResultEntry is one entry in the results array returned by /v1/querybatch.
type osvBatchResultEntry struct {
	Vulns []struct {
		ID string `json:"id"`
	} `json:"vulns"`
}

// osvBatchResponse is the top-level response from /v1/querybatch.
type osvBatchResponse struct {
	Results []osvBatchResultEntry `json:"results"`
}

// osvSeverityEntry is one item in vulnerability.severity[].
type osvSeverityEntry struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

// osvVuln is the wire representation of a vulnerability object from /v1/query.
type osvVuln struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Severity []osvSeverityEntry `json:"severity"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
}

// osvQueryRequest is the body sent to /v1/query.
type osvQueryRequest struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}

// osvQueryResponse is the top-level response from /v1/query.
type osvQueryResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

// QueryBatch calls /v1/querybatch with up to 1000 queries.
// Returns one BatchResult per input query, in the same order.
// Does not include severity — use QueryPackage for flagged entries.
func (c *OSVClient) QueryBatch(ctx context.Context, queries []BatchQuery) ([]BatchResult, error) {
	entries := make([]osvQueryEntry, len(queries))
	for i, q := range queries {
		entries[i] = osvQueryEntry{
			Package: osvPackage{Name: q.Name, Ecosystem: "npm"},
			Version: q.Version,
		}
	}

	body, err := json.Marshal(osvBatchRequest{Queries: entries})
	if err != nil {
		return nil, fmt.Errorf("osv: marshal batch request: %w", err)
	}

	respBody, err := c.post(ctx, "/querybatch", body)
	if err != nil {
		return nil, err
	}

	var resp osvBatchResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("osv: unmarshal batch response: %w", err)
	}

	results := make([]BatchResult, len(queries))
	for i, r := range resp.Results {
		ids := make([]string, len(r.Vulns))
		for j, v := range r.Vulns {
			ids[j] = v.ID
		}
		results[i] = BatchResult{VulnIDs: ids, Count: len(ids)}
	}
	// pad results if the server returned fewer entries than queries
	for len(results) < len(queries) {
		results = append(results, BatchResult{})
	}
	return results, nil
}

// QueryPackage calls /v1/query for a single package@version and returns the
// full vulnerability objects including severity information.
func (c *OSVClient) QueryPackage(ctx context.Context, name, version string) ([]Vulnerability, error) {
	body, err := json.Marshal(osvQueryRequest{
		Package: osvPackage{Name: name, Ecosystem: "npm"},
		Version: version,
	})
	if err != nil {
		return nil, fmt.Errorf("osv: marshal query request: %w", err)
	}

	respBody, err := c.post(ctx, "/query", body)
	if err != nil {
		return nil, err
	}

	var resp osvQueryResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("osv: unmarshal query response: %w", err)
	}

	vulns := make([]Vulnerability, len(resp.Vulns))
	for i, v := range resp.Vulns {
		vectors := make([]string, 0, len(v.Severity))
		for _, s := range v.Severity {
			if s.Score != "" {
				vectors = append(vectors, s.Score)
			}
		}
		vulns[i] = Vulnerability{
			ID:               v.ID,
			Summary:          v.Summary,
			DatabaseSeverity: v.DatabaseSpecific.Severity,
			CVSSVectors:      vectors,
		}
	}
	return vulns, nil
}

// post executes a JSON POST request against the OSV API and returns the raw body.
func (c *OSVClient) post(ctx context.Context, path string, body []byte) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer func() {
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
	}()

	req.SetRequestURI(c.baseURL + path)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(body)

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.client.Do(req, resp)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("osv: request to %s: %w", path, err)
		}
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		return nil, fmt.Errorf("osv: unexpected status %d for %s", resp.StatusCode(), path)
	}

	return resp.Body(), nil
}
