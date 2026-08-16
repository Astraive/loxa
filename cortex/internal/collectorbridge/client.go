package collectorbridge

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/eventconv"
	"github.com/astraive/loza/cortex/internal/models"
	transportcontracts "github.com/astraive/loza/spec/transport/contracts"
	lqlclient "github.com/astraive/lql/client/go"
	"github.com/gorilla/websocket"
)

const maxCollectorQueryLimit = 1000

type Cursor struct {
	Timestamp time.Time `json:"timestamp"`
	EventID   string    `json:"event_id"`
}

type Client struct {
	cfg        config.CollectorConfig
	httpClient *http.Client
	lqlClient  *lqlclient.Client
	initErr    error
}

func NewClient(cfg config.CollectorConfig) *Client {
	httpClient, err := collectorHTTPClient(cfg)
	if err != nil {
		return &Client{cfg: cfg, initErr: err}
	}
	live, err := lqlclient.New(lqlclient.ConnectionConfig{
		DSN:              cfg.DSN,
		Endpoint:         cfg.URL,
		Collector:        cfg.Collector,
		APIKey:           cfg.APIKey,
		Username:         cfg.Username,
		Password:         cfg.Password,
		Env:              cfg.Environment,
		Service:          cfg.Service,
		HTTPClient:       httpClient,
		Timeout:          cfg.Timeout,
		MaxResponseBytes: cfg.MaxResponseBytes,
	})
	return &Client{cfg: cfg, httpClient: httpClient, lqlClient: live, initErr: err}
}

func collectorHTTPClient(cfg config.CollectorConfig) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if path := strings.TrimSpace(cfg.TLSCAFile); path != "" {
		pem, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("collector TLS CA file: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("collector TLS CA file contains no certificates")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &http.Client{Transport: transport, Timeout: timeout}, nil
}
func setCollectorAuth(header http.Header, cfg config.CollectorConfig) {
	if strings.TrimSpace(cfg.APIKey) != "" {
		header.Set("Authorization", "Bearer "+cfg.APIKey)
		return
	}
	if cfg.Username != "" {
		header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.Username+":"+cfg.Password)))
		return
	}
	if cfg.APIKeyHeader != "" {
		header.Set(cfg.APIKeyHeader, cfg.APIKey)
	}
}

func (c *Client) FetchEventsSince(ctx context.Context, cursor Cursor, limit int) ([]*models.Event, Cursor, error) {
	limit = clampQueryLimit(limit, c.cfg.BatchSize)
	conditions := make([]string, 0, 1)
	parameters := make(map[string]lqlclient.QueryValue, 2)
	if !cursor.Timestamp.IsZero() {
		conditions = append(conditions, "(timestamp > $cursor_ts or (timestamp = $cursor_ts and event_id > $cursor_id))")
		parameters["cursor_ts"] = lqlclient.QueryValue{Type: "timestamp", Value: cursor.Timestamp.UTC().Format(time.RFC3339Nano)}
		parameters["cursor_id"] = lqlclient.QueryValue{Type: "string", Value: cursor.EventID}
	}
	source := "from events"
	if len(conditions) > 0 {
		source += " | where " + strings.Join(conditions, " and ")
	}
	source += " | sort timestamp asc | limit " + strconv.Itoa(limit)
	rows, err := c.queryLQLRows(ctx, source, parameters, limit)
	if err != nil {
		return nil, cursor, err
	}

	events := make([]*models.Event, 0, len(rows))
	next := cursor
	for _, row := range rows {
		event, err := c.rowToEvent(row)
		if err != nil {
			return nil, cursor, err
		}
		events = append(events, event)
		next = Cursor{Timestamp: event.Timestamp, EventID: event.ID}
	}
	return events, next, nil
}

func (c *Client) StreamTail(ctx context.Context, handle func(*models.Event) error) error {
	switch strings.ToLower(strings.TrimSpace(c.cfg.TailTransport)) {
	case "", "http":
		return c.streamTailHTTP(ctx, handle)
	case "websocket":
		return c.streamTailWebSocket(ctx, handle)
	default:
		return fmt.Errorf("unsupported collector tail transport %q", c.cfg.TailTransport)
	}
}

func (c *Client) streamTailHTTP(ctx context.Context, handle func(*models.Event) error) error {
	if c.initErr != nil {
		return c.initErr
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.tailURL("/tail", false), nil)
	if err != nil {
		return err
	}
	setCollectorAuth(req.Header, c.cfg)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("collector tail failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			return err
		}
		event, err := eventconv.FromRawMap(payload, "collector")
		if err != nil {
			return err
		}
		if err := handle(event); err != nil {
			return err
		}
	}
	return scanner.Err()
}
func (c *Client) streamTailWebSocket(ctx context.Context, handle func(*models.Event) error) error {
	if c.initErr != nil {
		return c.initErr
	}
	header := http.Header{}
	setCollectorAuth(header, c.cfg)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.tailURL("/ws/tail", true), header)
	if err != nil {
		return err
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		payload = bytes.TrimSpace(payload)
		if len(payload) == 0 {
			continue
		}
		var envelope transportcontracts.WebSocketResponse
		if err := json.Unmarshal(payload, &envelope); err == nil && (envelope.Type != "" || envelope.Error != "") {
			if envelope.Error != "" {
				return fmt.Errorf("collector websocket tail error: %s", envelope.Error)
			}
			if envelope.Type == "event" {
				if dataMap, ok := envelope.Data.(map[string]any); ok {
					event, err := eventconv.FromRawMap(dataMap, "collector")
					if err != nil {
						return err
					}
					if err := handle(event); err != nil {
						return err
					}
					continue
				}
				reencoded, err := json.Marshal(envelope.Data)
				if err != nil {
					return err
				}
				payload = reencoded
			}
		}
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return err
		}
		event, err := eventconv.FromRawMap(raw, "collector")
		if err != nil {
			return err
		}
		if err := handle(event); err != nil {
			return err
		}
	}
}

func (c *Client) FindByTraceIDPage(ctx context.Context, traceID string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where trace_id = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: traceID},
	}, limit, offset)
}

func (c *Client) FindByTraceID(ctx context.Context, traceID string, limit int) ([]*models.Event, error) {
	return c.queryEvents(ctx, "from events | where trace_id = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: traceID},
	}, limit)
}

func (c *Client) GetByID(ctx context.Context, id string) (*models.Event, error) {
	events, err := c.queryEvents(ctx, "from events | where event_id = $value | limit 1", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: id},
	}, 1)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return events[0], nil
}

func (c *Client) ListRecentPage(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | sort timestamp desc", nil, limit, offset)
}

func (c *Client) ListRecent(ctx context.Context, limit int) ([]*models.Event, error) {
	return c.ListRecentPage(ctx, limit, 0)
}

func (c *Client) FindByIncidentIDPage(ctx context.Context, incidentID string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where incident_id = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: incidentID},
	}, limit, offset)
}

func (c *Client) FindByIncidentID(ctx context.Context, incidentID string, limit int) ([]*models.Event, error) {
	return c.FindByIncidentIDPage(ctx, incidentID, limit, 0)
}

func (c *Client) FindByService(ctx context.Context, service, from, to string, limit int) ([]*models.Event, error) {
	return c.FindByServicePage(ctx, service, from, to, limit, 0)
}

func (c *Client) FindByServicePage(ctx context.Context, service, from, to string, limit, offset int) ([]*models.Event, error) {
	conditions := []string{"service = $service"}
	parameters := map[string]lqlclient.QueryValue{"service": {Type: "string", Value: service}}
	appendTimeConditions(&conditions, parameters, from, to)
	source := "from events | where " + strings.Join(conditions, " and ") + " | sort timestamp asc"
	return c.queryEventsPage(ctx, source, parameters, limit, offset)
}

func appendTimeConditions(conditions *[]string, parameters map[string]lqlclient.QueryValue, from, to string) {
	if strings.TrimSpace(from) != "" {
		*conditions = append(*conditions, "timestamp >= $from")
		parameters["from"] = lqlclient.QueryValue{Type: "timestamp", Value: from}
	}
	if strings.TrimSpace(to) != "" {
		*conditions = append(*conditions, "timestamp <= $to")
		parameters["to"] = lqlclient.QueryValue{Type: "timestamp", Value: to}
	}
}

func (c *Client) LoadCursor() (Cursor, error) {
	path := strings.TrimSpace(c.cfg.CursorPath)
	if path == "" {
		return Cursor{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Cursor{}, nil
		}
		return Cursor{}, err
	}
	var cur Cursor
	if err := json.Unmarshal(data, &cur); err != nil {
		return Cursor{}, err
	}
	return cur, nil
}

func (c *Client) SaveCursor(cur Cursor) error {
	path := strings.TrimSpace(c.cfg.CursorPath)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cur)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Client) queryLQLRows(ctx context.Context, source string, parameters map[string]lqlclient.QueryValue, limit int) ([]map[string]any, error) {
	if c.lqlClient == nil {
		if c.initErr != nil {
			return nil, c.initErr
		}
		return nil, fmt.Errorf("collector LQL client is unavailable")
	}
	result, err := c.lqlClient.Query(ctx, source, parameters, limit)
	if err != nil {
		return nil, err
	}
	return result.Rows, nil
}

func (c *Client) queryEventsPage(ctx context.Context, source string, parameters map[string]lqlclient.QueryValue, limit, offset int) ([]*models.Event, error) {
	if offset < 0 {
		offset = 0
	}
	if offset > 0 {
		source += " | offset " + strconv.Itoa(offset)
	}
	rows, err := c.queryLQLRows(ctx, source, parameters, limit)
	if err != nil {
		return nil, err
	}
	events := make([]*models.Event, 0, len(rows))
	for _, row := range rows {
		event, err := c.rowToEvent(row)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil

}
func (c *Client) queryEvents(ctx context.Context, source string, parameters map[string]lqlclient.QueryValue, limit int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, source, parameters, limit, 0)
}

func (c *Client) rowToEvent(row map[string]any) (*models.Event, error) {
	rawValue, ok := row[c.cfg.RawColumn]
	if !ok {
		rawValue, ok = row["raw"]
	}
	if !ok {
		return nil, fmt.Errorf("collector query row missing raw column")
	}

	var payload map[string]any
	switch typed := rawValue.(type) {
	case string:
		if err := json.Unmarshal([]byte(typed), &payload); err != nil {
			return nil, err
		}
	case []byte:
		if err := json.Unmarshal(typed, &payload); err != nil {
			return nil, err
		}
	case map[string]any:
		payload = typed
	default:
		return nil, fmt.Errorf("unsupported raw column type %T", rawValue)
	}
	if _, ok := payload["schema_version"]; !ok {
		payload["schema_version"] = "v1"
	}
	if _, ok := payload["event_version"]; !ok {
		payload["event_version"] = "v1"
	}
	if _, ok := payload["version"]; !ok {
		payload["version"] = "1"
	}
	if _, ok := payload["kind"]; !ok {
		payload["kind"] = "event"
	}
	if _, ok := payload["id"]; !ok {
		if eventID, exists := payload["event_id"]; exists {
			payload["id"] = eventID
		}
	}
	event, err := eventconv.FromRawMap(payload, "collector")
	if err != nil {
		return nil, err
	}
	if event.ID == "" {
		event.ID = event.EventID
	}
	return event, nil

}

func clampQueryLimit(limit, fallback int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > maxCollectorQueryLimit {
		return maxCollectorQueryLimit
	}
	return limit
}

func (c *Client) FindByEventName(ctx context.Context, eventName string, limit int) ([]*models.Event, error) {
	return c.FindByEventNamePage(ctx, eventName, limit, 0)
}

func (c *Client) FindByEventNamePage(ctx context.Context, eventName string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where event = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: eventName},
	}, limit, offset)
}

func (c *Client) FindByOutcome(ctx context.Context, outcome string, limit int) ([]*models.Event, error) {
	return c.FindByOutcomePage(ctx, outcome, limit, 0)
}

func (c *Client) FindByOutcomePage(ctx context.Context, outcome string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where outcome = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: outcome},
	}, limit, offset)
}

func (c *Client) DistinctServices(ctx context.Context) ([]string, error) {
	rows, err := c.queryLQLRows(ctx, "from events | distinct service | sort service asc", nil, 10000)
	if err != nil {
		return nil, err
	}
	services := make([]string, 0, len(rows))
	for _, row := range rows {
		if svc, ok := row["service"].(string); ok && strings.TrimSpace(svc) != "" {
			services = append(services, svc)
		}
	}
	return services, nil
}

func (c *Client) FindByLevel(ctx context.Context, level string, limit int) ([]*models.Event, error) {
	return c.FindByLevelPage(ctx, level, limit, 0)
}

func (c *Client) FindByLevelPage(ctx context.Context, level string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where level = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: level},
	}, limit, offset)
}

func (c *Client) FindByEnvironment(ctx context.Context, env string, limit int) ([]*models.Event, error) {
	return c.FindByEnvironmentPage(ctx, env, limit, 0)
}

func (c *Client) FindByEnvironmentPage(ctx context.Context, env string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where environment = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: env},
	}, limit, offset)
}

func (c *Client) FindByRelease(ctx context.Context, release string, limit int) ([]*models.Event, error) {
	return c.FindByReleasePage(ctx, release, limit, 0)
}

func (c *Client) FindByReleasePage(ctx context.Context, release string, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where release = $value | sort timestamp asc", map[string]lqlclient.QueryValue{
		"value": {Type: "string", Value: release},
	}, limit, offset)
}

func (c *Client) FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit int) ([]*models.Event, error) {
	return c.FindByDurationRangePage(ctx, minMs, maxMs, limit, 0)
}

func (c *Client) FindByDurationRangePage(ctx context.Context, minMs, maxMs float64, limit, offset int) ([]*models.Event, error) {
	return c.queryEventsPage(ctx, "from events | where duration_ms between $min and $max | sort timestamp asc", map[string]lqlclient.QueryValue{
		"min": {Type: "float", Value: minMs},
		"max": {Type: "float", Value: maxMs},
	}, limit, offset)
}

func (c *Client) countByField(ctx context.Context, service, field string, from, to time.Time) (map[string]int64, error) {
	conditions := []string{"service = $service"}
	parameters := map[string]lqlclient.QueryValue{
		"service": {Type: "string", Value: service},
	}
	if !from.IsZero() {
		conditions = append(conditions, "timestamp >= $from")
		parameters["from"] = lqlclient.QueryValue{Type: "timestamp", Value: from.UTC().Format(time.RFC3339Nano)}
	}
	if !to.IsZero() {
		conditions = append(conditions, "timestamp <= $to")
		parameters["to"] = lqlclient.QueryValue{Type: "timestamp", Value: to.UTC().Format(time.RFC3339Nano)}
	}
	source := "from events | where " + strings.Join(conditions, " and ") +
		" | summarize count() as count by " + field
	rows, err := c.queryLQLRows(ctx, source, parameters, 1000)
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		name, _ := row[field].(string)
		switch count := row["count"].(type) {
		case float64:
			result[name] = int64(count)
		case int64:
			result[name] = count
		case int:
			result[name] = int64(count)
		}
	}
	return result, nil
}

func (c *Client) CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return c.countByField(ctx, service, "outcome", from, to)
}

func (c *Client) CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return c.countByField(ctx, service, "event", from, to)
}

func (c *Client) durationConditions(eventName string, from, to time.Time) ([]string, map[string]lqlclient.QueryValue) {
	conditions := []string{"event = $event"}
	parameters := map[string]lqlclient.QueryValue{
		"event": {Type: "string", Value: eventName},
	}
	if !from.IsZero() {
		conditions = append(conditions, "timestamp >= $from")
		parameters["from"] = lqlclient.QueryValue{Type: "timestamp", Value: from.UTC().Format(time.RFC3339Nano)}
	}
	if !to.IsZero() {
		conditions = append(conditions, "timestamp <= $to")
		parameters["to"] = lqlclient.QueryValue{Type: "timestamp", Value: to.UTC().Format(time.RFC3339Nano)}
	}
	return conditions, parameters
}

func numericValue(row map[string]any, field string) float64 {
	switch value := row[field].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		n, _ := value.Float64()
		return n
	default:
		return 0
	}
}

func (c *Client) AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error) {
	conditions, parameters := c.durationConditions(eventName, from, to)
	source := "from events | where " + strings.Join(conditions, " and ") +
		" | summarize avg(duration_ms) as avg_dur"
	rows, err := c.queryLQLRows(ctx, source, parameters, 1)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return numericValue(rows[0], "avg_dur"), nil
}

func (c *Client) PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error) {
	conditions, parameters := c.durationConditions(eventName, from, to)
	source := fmt.Sprintf(
		"from events | where %s | summarize percentile(duration_ms, %g) as p_dur",
		strings.Join(conditions, " and "),
		percentile,
	)
	rows, err := c.queryLQLRows(ctx, source, parameters, 1)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return numericValue(rows[0], "p_dur"), nil
}

func (c *Client) DistinctEventNames(ctx context.Context) ([]string, error) {
	rows, err := c.queryLQLRows(ctx, "from events | distinct event | sort event asc", nil, 1000)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if name, ok := row["event"].(string); ok && strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (c *Client) ListLifecycleSummaries(ctx context.Context, filter map[string]any, limit, offset int) ([]map[string]any, int, error) {
	conditions := make([]string, 0, 3)
	parameters := make(map[string]lqlclient.QueryValue, 3)
	if service, ok := filter["service"].(string); ok && strings.TrimSpace(service) != "" {
		conditions = append(conditions, "service = $service")
		parameters["service"] = lqlclient.QueryValue{Type: "string", Value: service}
	}
	if eventName, ok := filter["event_name"].(string); ok && strings.TrimSpace(eventName) != "" {
		conditions = append(conditions, "event = $event")
		parameters["event"] = lqlclient.QueryValue{Type: "string", Value: eventName}
	}
	if outcome, ok := filter["outcome"].(string); ok && strings.TrimSpace(outcome) != "" {
		conditions = append(conditions, "outcome = $outcome")
		parameters["outcome"] = lqlclient.QueryValue{Type: "string", Value: outcome}
	}
	source := "from events"
	if len(conditions) > 0 {
		source += " | where " + strings.Join(conditions, " and ")
	}
	countRows, err := c.queryLQLRows(ctx, source+" | summarize count() as total", parameters, 1)
	if err != nil {
		return nil, 0, err
	}
	total := 0
	if len(countRows) > 0 {
		total = int(numericValue(countRows[0], "total"))
	}

	limit = clampQueryLimit(limit, 100)
	if offset < 0 {
		offset = 0
	}
	dataSource := source + " | sort timestamp desc | project event_id, event, service, outcome, duration_ms"
	if offset > 0 {
		dataSource += " | offset " + strconv.Itoa(offset)
	}
	dataRows, err := c.queryLQLRows(ctx, dataSource, parameters, limit)
	if err != nil {
		return nil, 0, err
	}
	return dataRows, total, nil
}
func (c *Client) tailURL(path string, websocketScheme bool) string {
	base := strings.TrimRight(c.cfg.URL, "/")
	if websocketScheme {
		base = strings.Replace(base, "http://", "ws://", 1)
		base = strings.Replace(base, "https://", "wss://", 1)
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return base + path
	}
	q := u.Query()
	cursor, err := c.LoadCursor()
	if err == nil && !cursor.Timestamp.IsZero() {
		q.Set("since", cursor.Timestamp.UTC().Format(time.RFC3339Nano))
	}
	if strings.TrimSpace(cursor.EventID) != "" {
		q.Set("after_event_id", cursor.EventID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
