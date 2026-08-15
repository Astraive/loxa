package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxCollectorClientResponseBytes = 10 << 20

// CollectorClient communicates with the LOZA collector REST API.
type CollectorClient struct {
	endpoint      string
	collectorName string
	apiKey        string
	basicUsername string
	basicPassword string
	initErr       error
	client        *http.Client
}

// CollectorClientConfig configures the collector client.
type CollectorClientConfig struct {
	Endpoint      string
	CollectorName string
	APIKey        string
	BasicUsername string
	BasicPassword string
	Insecure      bool
	Client        *http.Client
}

// NewCollectorClient creates a new collector client.
func NewCollectorClient(cfg CollectorClientConfig) *CollectorClient {
	endpoint := safeCollectorEndpoint(cfg.Endpoint)
	initErr := validateCollectorCredentials(
		cfg.Endpoint,
		cfg.BasicUsername,
		cfg.BasicPassword,
		cfg.Insecure,
	)
	if cfg.Client == nil {
		cfg.Client = &http.Client{}
	}
	return &CollectorClient{
		endpoint:      strings.TrimRight(endpoint, "/"),
		collectorName: cfg.CollectorName,
		apiKey:        cfg.APIKey,
		basicUsername: cfg.BasicUsername,
		basicPassword: cfg.BasicPassword,
		initErr:       initErr,
		client:        cfg.Client,
	}
}

func (c *CollectorClient) collectorPath(route string) string {
	if c.collectorName == "" {
		return route
	}
	return "/collectors/" + url.PathEscape(c.collectorName) + route
}

func (c *CollectorClient) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	endpoint := c.endpoint + path
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, endpoint, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("collector: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else if c.basicUsername != "" {
		req.Header.Set("Authorization", basicAuthorization(c.basicUsername, c.basicPassword))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("collector: do: %w", err)
	}
	defer resp.Body.Close()
	raw, err := readCollectorResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("collector: read: %w", err)
	}
	if resp.StatusCode >= 300 {
		return raw, fmt.Errorf("collector: status %d: %s", resp.StatusCode, truncateErrorBody(string(raw)))
	}
	return raw, nil
}

// Validate sends an event to the collector for validation without ingesting it.
func (c *CollectorClient) Validate(ctx context.Context, event json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/validate", event)
}

// Ingest sends events to the collector for ingestion.
func (c *CollectorClient) Ingest(ctx context.Context, events []json.RawMessage) ([]byte, error) {
	body, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("collector: marshal: %w", err)
	}
	return c.do(ctx, http.MethodPost, c.collectorPath("/events"), body)
}

// Query queries events from the collector.
func (c *CollectorClient) Query(ctx context.Context, query json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/query", query)
}

// Tail tails events from the collector (server-sent events).
func (c *CollectorClient) Tail(ctx context.Context, filter json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, c.collectorPath("/tail"), filter)
}

// Delete deletes events from the collector.
func (c *CollectorClient) Delete(ctx context.Context, filter json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, c.collectorPath("/events"), filter)
}

// Replay replays events through the collector.
func (c *CollectorClient) Replay(ctx context.Context, request json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/replay", request)
}

// DLQList lists dead-letter queue entries.
func (c *CollectorClient) DLQList(ctx context.Context, filter json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/dlq/list", filter)
}

// DLQRead reads a dead-letter queue entry by ID.
func (c *CollectorClient) DLQRead(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/dlq/"+url.PathEscape(id), nil)
}

// DLQReplay replays a dead-letter queue entry.
func (c *CollectorClient) DLQReplay(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/dlq/"+url.PathEscape(id)+"/replay", nil)
}

// KeysCreate creates a new API key.
func (c *CollectorClient) KeysCreate(ctx context.Context, keyReq json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/keys", keyReq)
}

// KeysRevoke revokes an API key.
func (c *CollectorClient) KeysRevoke(ctx context.Context, keyID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, "/keys/"+url.PathEscape(keyID), nil)
}

func (c *CollectorClient) KeysRotate(ctx context.Context, keyID string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/keys/"+url.PathEscape(keyID)+"/rotate", nil)
}

// SinksList lists configured sinks from the collector.
func (c *CollectorClient) SinksList(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/sinks", nil)
}

func (c *CollectorClient) SinksTest(ctx context.Context, name string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/sinks/"+url.PathEscape(name)+"/test", nil)
}

func readCollectorResponse(body io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxCollectorClientResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxCollectorClientResponseBytes {
		return raw[:maxCollectorClientResponseBytes], fmt.Errorf("response exceeds %d bytes", maxCollectorClientResponseBytes)
	}
	return raw, nil
}

func (c *CollectorClient) PolicyValidate(ctx context.Context, policy json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/policy/validate", policy)
}

func (c *CollectorClient) SchemaCheck(ctx context.Context, event json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/schema/check", event)
}

func (c *CollectorClient) SchemaPublish(ctx context.Context, schema json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/schema/publish", schema)
}

func (c *CollectorClient) RetentionApply(ctx context.Context, policy json.RawMessage) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/retention/apply", policy)
}

// Health checks the collector health endpoint.
func (c *CollectorClient) Health(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/health", nil)
	return err
}
