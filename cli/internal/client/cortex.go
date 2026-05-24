package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

// RunCortexCommand executes the local cortex binary from the cortex repo.
func RunCortexCommand(ctx context.Context, cortexRepoPath string, args []string) error {
	return runGoCommand(ctx, cortexRepoPath, filepath.Join(".", "cmd", "server"), args)
}

// CheckCortexHealth checks the cortex health endpoint.
func CheckCortexHealth(baseURL string) error {
	return expectStatus(strings.TrimRight(baseURL, "/")+"/healthz", http.StatusOK)
}

// CheckCortexReady checks the cortex ready endpoint.
func CheckCortexReady(baseURL string) error {
	return expectStatus(strings.TrimRight(baseURL, "/")+"/readyz", http.StatusOK)
}

// FetchCortexMetrics fetches Prometheus metrics from cortex.
func FetchCortexMetrics(baseURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/metrics", nil)
	if err != nil {
		return nil, err
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cortex returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// IngestCortexBatch sends a batch of events to cortex.
func IngestCortexBatch(ctx context.Context, baseURL string, events []map[string]any) error {
	raw, err := json.Marshal(map[string]any{"events": events})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/events/batch", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("cortex returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func ReconstructCortexIncident(ctx context.Context, baseURL, incidentID, mode string, limit int) ([]byte, error) {
	payload := map[string]any{
		"incident_id": incidentID,
	}
	if strings.TrimSpace(mode) != "" {
		payload["mode"] = mode
	}
	if limit > 0 {
		payload["limit"] = limit
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return postCortexJSON(ctx, baseURL, "/reconstruct", raw)
}

func RecordCortexRemediation(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postCortexJSON(ctx, baseURL, "/feedback/remediation", payload)
}

func RecordCortexFeedback(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postCortexJSON(ctx, baseURL, "/feedback/incident", payload)
}

func FetchCortexServiceGraph(ctx context.Context, baseURL, service string, depth int) ([]byte, error) {
	path := fmt.Sprintf("/graph/service/%s?depth=%d", service, depth)
	return getCortexJSON(ctx, baseURL, path)
}

func FetchCortexIncidentGraph(ctx context.Context, baseURL, incidentID string, depth int) ([]byte, error) {
	path := fmt.Sprintf("/graph/incident/%s?depth=%d", incidentID, depth)
	return getCortexJSON(ctx, baseURL, path)
}

func getCortexJSON(ctx context.Context, baseURL, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cortex returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func QueryCortexGraphQL(ctx context.Context, baseURL, query string, variables map[string]any) ([]byte, error) {
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return postCortexJSON(ctx, baseURL, "/graphql", raw)
}

func FetchSignatures(ctx context.Context, baseURL string, limit int) ([]byte, error) {
	query := fmt.Sprintf(`{ signatures(limit: %d) { id pattern count lastSeen } }`, limit)
	return QueryCortexGraphQL(ctx, baseURL, query, nil)
}

func postCortexJSON(ctx context.Context, baseURL, path string, payload []byte) ([]byte, error) {
	if payload == nil {
		payload = []byte("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cortex returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
