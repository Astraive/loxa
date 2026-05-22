package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	speccontract "github.com/astraive/loxa-spec/generated/go/contract"
)

const defaultTimeout = 30 * time.Second

// RunCollectorCommand executes the local collector binary from the collector repo.
func RunCollectorCommand(ctx context.Context, collectorRepoPath string, args []string) error {
	return runGoCommand(ctx, collectorRepoPath, filepath.Join(".", "cmd", "loxa-collector"), args)
}

// RunWorkerCommand executes the local worker binary from the collector repo.
func RunWorkerCommand(ctx context.Context, collectorRepoPath string, args []string) error {
	return runGoCommand(ctx, collectorRepoPath, filepath.Join(".", "cmd", "loxa-worker"), args)
}

// RunLoadgenCommand executes the local load generator from the collector repo.
func RunLoadgenCommand(ctx context.Context, collectorRepoPath string, args []string) error {
	return runGoCommand(ctx, collectorRepoPath, filepath.Join(".", "cmd", "loxa-loadgen"), args)
}

// CheckHealth checks the collector health endpoint.
func CheckHealth(baseURL string) error {
	if err := expectStatus(strings.TrimRight(baseURL, "/")+"/health", http.StatusOK); err == nil {
		return nil
	}
	return expectStatus(strings.TrimRight(baseURL, "/")+"/healthz", http.StatusOK)
}

// CheckReady checks the collector ready endpoint.
func CheckReady(baseURL string) error {
	if err := expectStatus(strings.TrimRight(baseURL, "/")+"/ready", http.StatusOK); err == nil {
		return nil
	}
	return expectStatus(strings.TrimRight(baseURL, "/")+"/readyz", http.StatusOK)
}

// PostIngest posts event data to the collector ingest endpoint.
func PostIngest(baseURL, contentType string, payload []byte) error {
	if err := validateIngestPayload(contentType, payload); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/events", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", contentType)
		applyAPIKeyAuth(req)

		resp, err := clientWithTimeout(defaultTimeout).Do(req)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				time.Sleep(backoffDelay(attempt, ""))
				continue
			}
			return err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < 2 {
				time.Sleep(backoffDelay(attempt, ""))
				continue
			}
			return readErr
		}

		outcome, reason := collectorResponseOutcome(resp.StatusCode, body)
		switch outcome {
		case "success":
			return nil
		case "retryable":
			lastErr = fmt.Errorf("collector reported retryable response: %s", reason)
			if attempt < 2 {
				time.Sleep(backoffDelay(attempt, resp.Header.Get("Retry-After")))
				continue
			}
			return lastErr
		default:
			if resp.StatusCode >= 300 {
				return fmt.Errorf("collector returned %d: %s", resp.StatusCode, string(body))
			}
			return fmt.Errorf("collector rejected batch: %s", reason)
		}
	}
	return fmt.Errorf("collector send failed: %w", lastErr)
}

// FetchStatus fetches the collector status endpoint.
func FetchStatus(baseURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/status", nil)
	if err != nil {
		return nil, err
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status returned %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// FetchMetrics fetches Prometheus metrics from the collector.
func FetchMetrics(baseURL string) ([]byte, error) {
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
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("metrics returned %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// Query runs a query against the collector's query endpoint.
func Query(baseURL, engine, sqlQuery string) ([]byte, error) {
	payload := map[string]string{"engine": engine, "query": sqlQuery}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/query", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query returned %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// TailStream opens an HTTP stream to the collector for tailing events.
func TailStream(ctx context.Context, baseURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/tail", nil)
	if err != nil {
		return nil, fmt.Errorf("tail request failed: %w", err)
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return nil, fmt.Errorf("tail request failed: %w", err)
	}
	if resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("tail returned %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// ReplayEvents replays NDJSON events to the collector ingest endpoint.
func ReplayEvents(baseURL string, payload []byte) error {
	return PostIngest(baseURL, "application/x-ndjson", payload)
}

func FetchDLQ(baseURL string) ([]byte, error) {
	return getJSON(baseURL, "/v1/dlq")
}

func FetchDLQItem(baseURL, id string) ([]byte, error) {
	return getJSON(baseURL, "/v1/dlq/"+id)
}

func ReplayDLQ(ctx context.Context, baseURL string) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/dlq/replay", nil)
}

func ReplayDLQItem(ctx context.Context, baseURL, id string) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/dlq/"+id+"/replay", nil)
}

func FetchSchema(baseURL string) ([]byte, error) {
	return getJSON(baseURL, "/v1/schema")
}

func DiffSchema(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/schema/diff", payload)
}

func PublishSchema(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/schema/publish", payload)
}

func AuditPII(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/audit/pii", payload)
}

func DeleteEventsByTenant(ctx context.Context, baseURL, tenantID, reason string) ([]byte, error) {
	return deleteJSON(ctx, baseURL, "/v1/events/by-tenant/"+tenantID, reason)
}

func DeleteEventsByUser(ctx context.Context, baseURL, userID, reason string) ([]byte, error) {
	return deleteJSON(ctx, baseURL, "/v1/events/by-user/"+userID, reason)
}

func DeleteEventByID(ctx context.Context, baseURL, eventID, reason string) ([]byte, error) {
	return deleteJSON(ctx, baseURL, "/v1/events/"+eventID, reason)
}

func FetchSinks(baseURL string) ([]byte, error) {
	return getJSON(baseURL, "/v1/sinks")
}

func FetchSinkHealth(baseURL, name string) ([]byte, error) {
	return getJSON(baseURL, "/v1/sinks/"+name)
}

func DeleteDLQItem(ctx context.Context, baseURL, id string) ([]byte, error) {
	return deleteJSON(ctx, baseURL, "/v1/dlq/"+id, "")
}

func PublishBlueprint(ctx context.Context, baseURL string, payload []byte) ([]byte, error) {
	return postJSON(ctx, baseURL, "/v1/schema/blueprint", payload)
}

func ListBlueprints(ctx context.Context, baseURL string) ([]byte, error) {
	return getJSON(baseURL, "/v1/schema/blueprint")
}

func WatchStream(ctx context.Context, baseURL string, filters map[string]string) (io.ReadCloser, error) {
	base := strings.TrimRight(baseURL, "/") + "/v1/tail"
	if len(filters) > 0 {
		params := []string{}
		for k, v := range filters {
			params = append(params, k+"="+v)
		}
		base += "?" + strings.Join(params, "&")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		return nil, err
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(0).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("watch returned %d: %s", resp.StatusCode, string(body))
	}
	return resp.Body, nil
}

func expectStatus(url string, want int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyAPIKeyAuth(req)
	resp, err := clientWithTimeout(defaultTimeout).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return nil
}

func clientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func getJSON(baseURL, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
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
		return nil, fmt.Errorf("collector returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func postJSON(ctx context.Context, baseURL, path string, payload []byte) ([]byte, error) {
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
		return nil, fmt.Errorf("collector returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func deleteJSON(ctx context.Context, baseURL, path, reason string) ([]byte, error) {
	payload := []byte("{}")
	if strings.TrimSpace(reason) != "" {
		raw, err := json.Marshal(map[string]string{"reason": reason})
		if err != nil {
			return nil, err
		}
		payload = raw
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(payload))
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
		return nil, fmt.Errorf("collector returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func validateIngestPayload(contentType string, payload []byte) error {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if strings.Contains(ct, "ndjson") || !strings.Contains(ct, "json") {
		return nil
	}
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("collector ingest payload must be valid JSON: %w", err)
	}
	return validateIngestEnvelopeShape(envelope)
}

func validateIngestEnvelopeShape(envelope map[string]any) error {
	if version, ok := envelope["api_version"].(string); !ok || strings.TrimSpace(version) == "" {
		return fmt.Errorf("collector envelope must include api_version")
	}
	source, ok := envelope["source"].(map[string]any)
	if !ok {
		return fmt.Errorf("collector envelope must include a source object")
	}
	for _, key := range []string{"sdk", "version", "service"} {
		value, ok := source[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("collector envelope source.%s must be a non-empty string", key)
		}
	}
	events, ok := envelope["events"].([]any)
	if !ok {
		return fmt.Errorf("collector envelope must include an events array")
	}
	if len(events) == 0 {
		return fmt.Errorf("collector envelope must include at least one event")
	}
	for idx, event := range events {
		if _, ok := event.(map[string]any); !ok {
			return fmt.Errorf("collector envelope events[%d] must be JSON objects", idx)
		}
	}
	return nil
}

func collectorResponseOutcome(status int, body []byte) (string, string) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			return "retryable", fmt.Sprintf("collector returned %d", status)
		}
		if status >= 300 {
			return "permanent", fmt.Sprintf("collector returned %d", status)
		}
		return "success", ""
	}

	resp, err := speccontract.ParseCollectorResponse(trimmed)
	if err != nil {
		if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
			return "retryable", fmt.Sprintf("collector response decode failed: %v", err)
		}
		if status >= 300 {
			return "permanent", fmt.Sprintf("collector response decode failed: %v", err)
		}
		return "success", ""
	}
	if retryable, reason := resp.RetryableError(); retryable {
		return "retryable", reason
	}
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		return "retryable", collectorResponseSummary(resp)
	}
	if failed, reason := resp.PermanentFailure(); failed || status >= 300 {
		if strings.TrimSpace(reason) == "" {
			reason = collectorResponseSummary(resp)
		}
		return "permanent", reason
	}
	return "success", ""
}

func collectorResponseSummary(resp speccontract.CollectorResponse) string {
	if strings.TrimSpace(resp.Error) != "" {
		return resp.Error
	}
	if strings.TrimSpace(resp.Reason) != "" {
		return resp.Reason
	}
	return fmt.Sprintf("accepted=%d rejected=%d invalid=%d", resp.Accepted, resp.Rejected, resp.Invalid)
}

func backoffDelay(attempt int, retryAfter string) time.Duration {
	if delay := parseRetryAfter(retryAfter); delay > 0 {
		return delay
	}
	delay := time.Duration(math.Pow(2, float64(attempt))) * 50 * time.Millisecond
	if delay > time.Second {
		return time.Second
	}
	return delay
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		if delay := time.Until(t); delay > 0 {
			return delay
		}
	}
	return 0
}

func applyAPIKeyAuth(req *http.Request) {
	if req == nil {
		return
	}
	apiKey := getConfiguredAPIKey(req.URL.Host)
	if apiKey == "" {
		return
	}
	// Use Authorization: Bearer format (matches collector auth middleware)
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func getConfiguredAPIKey(host string) string {
	// Primary: LOXA_API_KEY (works for all services)
	if apiKey := strings.TrimSpace(os.Getenv("LOXA_API_KEY")); apiKey != "" {
		return apiKey
	}
	// Fallback: service-specific keys
	host = strings.ToLower(strings.TrimSpace(host))
	if strings.Contains(host, "cortex") {
		return strings.TrimSpace(os.Getenv("LOXA_CORTEX_API_KEY"))
	}
	return strings.TrimSpace(os.Getenv("LOXA_COLLECTOR_API_KEY"))
}

func runGoCommand(ctx context.Context, repoPath, packagePath string, args []string) error {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return fmt.Errorf("repo path is not configured")
	}

	cmdArgs := []string{"run", packagePath}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", filepath.Base(packagePath), err)
	}
	return nil
}
