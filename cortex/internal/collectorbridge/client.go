package collectorbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	transportcontracts "github.com/astraive/loxa/spec/transport/contracts"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/eventconv"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/gorilla/websocket"
)

var sqlIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Cursor struct {
	Timestamp time.Time `json:"timestamp"`
	EventID   string    `json:"event_id"`
}

type Client struct {
	cfg        config.CollectorConfig
	httpClient *http.Client
}

func NewClient(cfg config.CollectorConfig) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) FetchEventsSince(ctx context.Context, cursor Cursor, limit int) ([]*models.Event, Cursor, error) {
	if limit <= 0 {
		limit = c.cfg.BatchSize
	}
	query, err := c.buildIncrementalQuery(cursor, limit)
	if err != nil {
		return nil, cursor, err
	}
	rows, err := c.queryRows(ctx, query, limit)
	if err != nil {
		return nil, cursor, err
	}

	events := make([]*models.Event, 0, len(rows))
	next := cursor
	for _, row := range rows {
		event, err := c.rowToEvent(row)
		if err != nil {
			continue
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.tailURL("/v1/tail", false), nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set(c.cfg.APIKeyHeader, c.cfg.APIKey)
	}
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
	header := http.Header{}
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		header.Set(c.cfg.APIKeyHeader, c.cfg.APIKey)
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.tailURL("/v1/ws/tail", true), header)
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

func (c *Client) FindByTraceID(ctx context.Context, traceID string, limit int) ([]*models.Event, error) {
	query, err := c.buildJSONFieldQuery("trace_id", traceID, limit)
	if err != nil {
		return nil, err
	}
	return c.queryEvents(ctx, query, limit)
}

func (c *Client) GetByID(ctx context.Context, id string) (*models.Event, error) {
	query, err := c.buildJSONFieldQuery("id", id, 1)
	if err != nil {
		return nil, err
	}
	events, err := c.queryEvents(ctx, query, 1)
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return events[0], nil
}

func (c *Client) ListRecent(ctx context.Context, limit int) ([]*models.Event, error) {
	if limit <= 0 {
		limit = 1000
	}
	table, rawCol, tsCol, err := c.sqlParts()
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s DESC LIMIT %d", rawCol, table, tsCol, limit)
	return c.queryEvents(ctx, query, limit)
}

func (c *Client) FindByIncidentID(ctx context.Context, incidentID string, limit int) ([]*models.Event, error) {
	query, err := c.buildJSONFieldQuery("incident_id", incidentID, limit)
	if err != nil {
		return nil, err
	}
	return c.queryEvents(ctx, query, limit)
}

func (c *Client) FindByService(ctx context.Context, service, from, to string, limit int) ([]*models.Event, error) {
	if limit <= 0 {
		limit = 1000
	}
	table, rawCol, tsCol, err := c.sqlParts()
	if err != nil {
		return nil, err
	}
	conds := []string{fmt.Sprintf("json_extract_string(%s, '$.service') = %s", rawCol, quoteSQLString(service))}
	if strings.TrimSpace(from) != "" {
		conds = append(conds, fmt.Sprintf("%s >= TIMESTAMP %s", tsCol, quoteSQLString(from)))
	}
	if strings.TrimSpace(to) != "" {
		conds = append(conds, fmt.Sprintf("%s <= TIMESTAMP %s", tsCol, quoteSQLString(to)))
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s ASC LIMIT %d", rawCol, table, strings.Join(conds, " AND "), tsCol, limit)
	return c.queryEvents(ctx, query, limit)
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

func (c *Client) queryEvents(ctx context.Context, query string, limit int) ([]*models.Event, error) {
	rows, err := c.queryRows(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	events := make([]*models.Event, 0, len(rows))
	for _, row := range rows {
		event, err := c.rowToEvent(row)
		if err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
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

	return eventconv.FromRawMap(payload, "collector")
}

func (c *Client) buildIncrementalQuery(cursor Cursor, limit int) (string, error) {
	table, rawCol, tsCol, err := c.sqlParts()
	if err != nil {
		return "", err
	}
	idExpr := fmt.Sprintf("coalesce(json_extract_string(%s, '$.id'), '')", rawCol)
	query := fmt.Sprintf("SELECT %s FROM %s", rawCol, table)
	if !cursor.Timestamp.IsZero() {
		ts := cursor.Timestamp.UTC().Format(time.RFC3339Nano)
		query += fmt.Sprintf(" WHERE (%s > TIMESTAMP %s OR (%s = TIMESTAMP %s AND %s > %s))",
			tsCol, quoteSQLString(ts),
			tsCol, quoteSQLString(ts),
			idExpr, quoteSQLString(cursor.EventID))
	}
	query += fmt.Sprintf(" ORDER BY %s ASC, %s ASC LIMIT %d", tsCol, idExpr, limit)
	return query, nil
}

func (c *Client) buildJSONFieldQuery(field, value string, limit int) (string, error) {
	table, rawCol, tsCol, err := c.sqlParts()
	if err != nil {
		return "", err
	}
	if limit <= 0 {
		limit = 1000
	}
	return fmt.Sprintf(
		"SELECT %s FROM %s WHERE json_extract_string(%s, '$.%s') = %s ORDER BY %s ASC LIMIT %d",
		rawCol, table, rawCol, field, quoteSQLString(value), tsCol, limit,
	), nil
}

func (c *Client) sqlParts() (table, rawCol, tsCol string, err error) {
	for _, ident := range []string{c.cfg.QueryTable, c.cfg.RawColumn, c.cfg.TimestampColumn} {
		if !sqlIdentPattern.MatchString(strings.TrimSpace(ident)) {
			return "", "", "", fmt.Errorf("invalid collector SQL identifier %q", ident)
		}
	}
	return c.cfg.QueryTable, c.cfg.RawColumn, c.cfg.TimestampColumn, nil
}

func (c *Client) queryRows(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	payload := map[string]any{
		"query": query,
		"limit": limit,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.URL, "/")+"/v1/query", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set(c.cfg.APIKeyHeader, c.cfg.APIKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("collector query failed with status %d", resp.StatusCode)
	}
	var out struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Rows, nil
}

func quoteSQLString(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
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
		if strings.TrimSpace(cursor.EventID) != "" {
			q.Set("after_event_id", cursor.EventID)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
