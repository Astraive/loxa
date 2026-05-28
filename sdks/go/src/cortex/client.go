package cortex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEndpoint   = "http://localhost:9312"
	defaultAuthHeader = "X-API-Key"
	defaultTimeout    = 10 * time.Second
)

// Client is an HTTP client for the Cortex incident intelligence API.
type Client struct {
	endpoint   string
	apiKey     string
	authHeader string
	httpClient *http.Client
}

// NewClient creates a CortexClient pointing at the given endpoint.
func NewClient(endpoint string) *Client {
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Client{
		endpoint:   endpoint,
		authHeader: defaultAuthHeader,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// WithAPIKey sets the API key for authentication.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

// WithAuthHeader overrides the auth header name (default: x-loxa-api-key).
func (c *Client) WithAuthHeader(header string) *Client {
	c.authHeader = header
	return c
}

// WithHTTPClient sets a custom http.Client.
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.httpClient = hc
	return c
}

// Health checks if cortex is healthy.
func (c *Client) Health(ctx context.Context) bool {
	req, err := c.newRequest(ctx, http.MethodGet, "/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return true // 200 with non-JSON body is still healthy
	}
	if s, ok := body["status"].(string); ok {
		return s == "ok"
	}
	return true
}

// Ready checks if cortex is ready to accept requests.
func (c *Client) Ready(ctx context.Context) bool {
	req, err := c.newRequest(ctx, http.MethodGet, "/readyz", nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return true
	}
	if s, ok := body["status"].(string); ok {
		return s == "ok"
	}
	if r, ok := body["ready"].(bool); ok {
		return r
	}
	return true
}

// Metrics fetches Prometheus metrics text from cortex.
func (c *Client) Metrics(ctx context.Context) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/metrics", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cortex: metrics request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("cortex: read metrics response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cortex: /metrics returned %d: %s", resp.StatusCode, string(body))
	}
	return string(body), nil
}

// Reconstruct reconstructs an incident timeline with root cause analysis.
func (c *Client) Reconstruct(ctx context.Context, incidentID, mode string) (*IncidentContext, error) {
	body := map[string]string{
		"incident_id": incidentID,
		"mode":        mode,
	}
	var result IncidentContext
	if err := c.postJSON(ctx, "/reconstruct", body, &result); err != nil {
		return nil, err
	}
	NormalizeIncidentContext(&result)
	if err := ValidateIncidentContext(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReconstructIncident reconstructs an incident using the URL-path variant.
func (c *Client) ReconstructIncident(ctx context.Context, incidentID, mode string) (*IncidentContext, error) {
	body := map[string]string{"mode": mode}
	path := "/incidents/" + url.PathEscape(incidentID) + "/reconstruct"
	var result IncidentContext
	if err := c.postJSON(ctx, path, body, &result); err != nil {
		return nil, err
	}
	NormalizeIncidentContext(&result)
	if err := ValidateIncidentContext(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ServiceGraph fetches the dependency graph for a service.
func (c *Client) ServiceGraph(ctx context.Context, service string, depth int) (*GraphView, error) {
	path := "/graph/service/" + url.PathEscape(service) + "?depth=" + strconv.Itoa(depth)
	var result GraphView
	if err := c.getJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	NormalizeGraphNodes(&result)
	if err := ValidateGraphView(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// IncidentGraph fetches the graph for a specific incident.
func (c *Client) IncidentGraph(ctx context.Context, incidentID string, depth int) (*GraphView, error) {
	path := "/graph/incident/" + url.PathEscape(incidentID) + "?depth=" + strconv.Itoa(depth)
	var result GraphView
	if err := c.getJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	NormalizeGraphNodes(&result)
	if err := ValidateGraphView(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RecordRemediation records a remediation action.
func (c *Client) RecordRemediation(ctx context.Context, r *Remediation) error {
	NormalizeRemediation(r)
	if err := ValidateRemediation(r); err != nil {
		return err
	}
	return c.postJSON(ctx, "/feedback/remediation", r, nil)
}

// RecordFeedback records feedback on a remediation outcome.
func (c *Client) RecordFeedback(ctx context.Context, rf *RemediationFeedback) error {
	NormalizeRemediationFeedback(rf)
	if err := ValidateRemediationFeedback(rf); err != nil {
		return err
	}
	return c.postJSON(ctx, "/feedback/incident", rf, nil)
}

// SimilarIncidents finds incidents similar to the given one.
func (c *Client) SimilarIncidents(ctx context.Context, incidentID string) ([]map[string]any, error) {
	body := map[string]string{
		"incident_id": incidentID,
		"mode":        "fast",
	}
	var result IncidentContext
	if err := c.postJSON(ctx, "/reconstruct", body, &result); err != nil {
		return nil, err
	}
	return result.SimilarIncidents, nil
}

// IngestBatch sends events directly to cortex.
func (c *Client) IngestBatch(ctx context.Context, events []map[string]any) error {
	body := map[string]any{"events": events}
	return c.postJSON(ctx, "/events/batch", body, nil)
}

// --- internal helpers ---

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, body)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set(c.authHeader, c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cortex: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cortex: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cortex: %s %s returned %d: %s", req.Method, req.URL.Path, resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("cortex: decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) postJSON(ctx context.Context, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("cortex: encode request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}
