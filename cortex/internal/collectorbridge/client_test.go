package collectorbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/models"
)

func TestFetchEventsSinceUsesScopedLQLRouteAndTypedCursorBindings(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody struct {
		Query      string                    `json:"query"`
		Parameters map[string]map[string]any `json:"parameters"`
		Limit      int                       `json:"limit"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if r.URL.Path == "/query" {
			http.Error(w, "legacy raw SQL route is forbidden", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["raw","ts"],"rows":[{"raw":"{\"event_id\":\"evt-1\",\"timestamp\":\"2026-01-01T00:00:01Z\",\"service\":\"checkout\",\"event\":\"checkout.request\"}","ts":"2026-01-01T00:00:01Z"}],"row_count":1}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{
		URL:       server.URL,
		Collector: "demo",
		APIKey:    "secret",
		BatchSize: 10,
	})
	cursor := Cursor{Timestamp: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), EventID: "evt-0"}
	events, next, err := client.FetchEventsSince(context.Background(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch events: %v", err)
	}
	if gotPath != "/collectors/demo/lql/query" {
		t.Fatalf("unexpected route %q", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("unexpected authorization header %q", gotAuth)
	}
	if !strings.Contains(gotBody.Query, "from events") || !strings.Contains(gotBody.Query, "$cursor_ts") || !strings.Contains(gotBody.Query, "$cursor_id") {
		t.Fatalf("query is not source-oriented or cursor-bound: %q", gotBody.Query)
	}
	if gotBody.Parameters["cursor_id"]["value"] != "evt-0" {
		t.Fatalf("cursor id binding missing: %#v", gotBody.Parameters)
	}
	if len(events) != 1 || events[0].ID != "evt-1" || next.EventID != "evt-1" {
		t.Fatalf("unexpected event result: events=%+v next=%+v", events, next)
	}
}

func TestCountByOutcomeUsesLQLSummarize(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotQuery = body.Query
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["outcome","count"],"rows":[{"outcome":"success","count":3}],"row_count":1}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{URL: server.URL, Collector: "demo"})
	counts, err := client.CountByOutcome(context.Background(), "checkout", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("count outcomes: %v", err)
	}
	if gotPath != "/collectors/demo/lql/query" {
		t.Fatalf("unexpected route %q", gotPath)
	}
	if !strings.Contains(gotQuery, "summarize") || !strings.Contains(gotQuery, "count()") {
		t.Fatalf("query does not use LQL aggregation: %q", gotQuery)
	}
	if counts["success"] != 3 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
}
func TestAverageDurationUsesScopedLQLAggregation(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotQuery = body.Query
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["avg_dur"],"rows":[{"avg_dur":42.5}],"row_count":1}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{URL: server.URL, Collector: "demo"})
	average, err := client.AverageDuration(context.Background(), "checkout.request", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("average duration: %v", err)
	}
	if gotPath != "/collectors/demo/lql/query" {
		t.Fatalf("unexpected route %q", gotPath)
	}
	if !strings.Contains(gotQuery, "summarize") || !strings.Contains(gotQuery, "avg(duration_ms)") {
		t.Fatalf("query does not use LQL average aggregation: %q", gotQuery)
	}
	if average != 42.5 {
		t.Fatalf("average = %v, want 42.5", average)
	}
}

func TestPercentileDurationUsesNamedLQLPercentile(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotQuery = body.Query
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["p_dur"],"rows":[{"p_dur":99.0}],"row_count":1}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{URL: server.URL, Collector: "demo"})
	value, err := client.PercentileDuration(context.Background(), "checkout.request", 95, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("percentile duration: %v", err)
	}
	if !strings.Contains(gotQuery, "percentile(duration_ms, 95)") {
		t.Fatalf("query does not use source percentile syntax: %q", gotQuery)
	}
	if value != 99 {
		t.Fatalf("percentile = %v, want 99", value)
	}
}

func TestLifecycleSummariesUseScopedLQLProjectionAndCount(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		queries = append(queries, body.Query)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(body.Query, "count()") {
			_, _ = w.Write([]byte(`{"columns":["total"],"rows":[{"total":2}],"row_count":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"columns":["event_id","event","service","outcome","duration_ms"],"rows":[{"event_id":"evt-1","event":"checkout.request","service":"checkout","outcome":"success","duration_ms":12.5}],"row_count":1}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{URL: server.URL, Collector: "demo"})
	rows, total, err := client.ListLifecycleSummaries(context.Background(), map[string]any{"service": "checkout"}, 100, 0)
	if err != nil {
		t.Fatalf("lifecycle summaries: %v", err)
	}
	if len(queries) != 2 || !strings.Contains(queries[0], "count()") || !strings.Contains(queries[1], "project") {
		t.Fatalf("unexpected lifecycle queries: %#v", queries)
	}
	if total != 2 || len(rows) != 1 || rows[0]["event_id"] != "evt-1" {
		t.Fatalf("rows=%#v total=%d", rows, total)
	}
}
func TestLQLQueriesUseBasicAuthAndScopeHeaders(t *testing.T) {
	var gotPath, gotAuth, gotEnv, gotService string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotEnv = r.Header.Get("X-Loza-Env")
		gotService = r.Header.Get("X-Loza-Service")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"columns":["raw"],"rows":[],"row_count":0}`))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{
		URL:         server.URL,
		Collector:   "demo",
		Username:    "query-user",
		Password:    "secret",
		Environment: "prod",
		Service:     "checkout",
	})
	if _, err := client.FindByService(context.Background(), "checkout", "", "", 10); err != nil {
		t.Fatalf("find by service: %v", err)
	}
	if gotPath != "/collectors/demo/lql/query" {
		t.Fatalf("unexpected route %q", gotPath)
	}
	if gotAuth != "Basic "+base64.StdEncoding.EncodeToString([]byte("query-user:secret")) {
		t.Fatalf("unexpected authorization header %q", gotAuth)
	}
	if gotEnv != "prod" || gotService != "checkout" {
		t.Fatalf("unexpected scope headers env=%q service=%q", gotEnv, gotService)
	}
}

func TestTailURLUsesConfiguredCollectorScope(t *testing.T) {
	client := &Client{cfg: config.CollectorConfig{
		URL:       "https://collector.example/base",
		Collector: "demo",
	}}

	if got, want := client.tailURL("/tail", false), "https://collector.example/base/collectors/demo/tail"; got != want {
		t.Fatalf("HTTP tail URL = %q, want %q", got, want)
	}
	if got, want := client.tailURL("/ws/tail", true), "wss://collector.example/base/collectors/demo/ws/tail"; got != want {
		t.Fatalf("WebSocket tail URL = %q, want %q", got, want)
	}
}

func TestStreamTailHTTPSendsConfiguredScope(t *testing.T) {
	var gotPath, gotEnvironment string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotEnvironment = r.Header.Get("X-Loza-Env")
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"id":"tail-1","schema_version":"v1","event_version":"v1","event_id":"tail-1","timestamp":"2026-01-01T00:00:00Z","service":"checkout","event":"checkout.request","kind":"event"}` + "\n"))
	}))
	defer server.Close()

	client := NewClient(config.CollectorConfig{
		URL:         server.URL,
		Collector:   "demo",
		Environment: "prod",
	})
	var received int
	if err := client.StreamTail(context.Background(), func(_ *models.Event) error {
		received++
		return nil
	}); err != nil {
		t.Fatalf("stream tail: %v", err)
	}
	if gotPath != "/collectors/demo/tail" {
		t.Fatalf("tail path = %q, want scoped collector path", gotPath)
	}
	if gotEnvironment != "prod" {
		t.Fatalf("environment header = %q, want prod", gotEnvironment)
	}
	if received != 1 {
		t.Fatalf("received %d events, want 1", received)
	}
}

func TestCollectorAuthUsesConfiguredAPIKeyHeader(t *testing.T) {
	header := http.Header{}
	setCollectorAuth(header, config.CollectorConfig{
		APIKey:       "secret",
		APIKeyHeader: "X-Collector-Key",
	})

	if got := header.Get("X-Collector-Key"); got != "secret" {
		t.Fatalf("custom API-key header = %q, want secret", got)
	}
	if got := header.Get("Authorization"); got != "" {
		t.Fatalf("unexpected bearer authorization %q", got)
	}
}

func TestCursorPersistenceIsAtomicAndRecoversBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.cursor")
	client := NewClient(config.CollectorConfig{CursorPath: path})
	first := Cursor{Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EventID: "evt-1"}
	second := Cursor{Timestamp: time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC), EventID: "evt-2"}

	if err := client.SaveCursor(first); err != nil {
		t.Fatalf("save first cursor: %v", err)
	}
	if err := client.SaveCursor(second); err != nil {
		t.Fatalf("save second cursor: %v", err)
	}
	backupData, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read cursor backup: %v", err)
	}
	backup, err := decodeCursor(backupData)
	if err != nil || backup != first {
		t.Fatalf("backup = %+v, err=%v; want %+v", backup, err, first)
	}

	if err := os.WriteFile(path, []byte(`{"timestamp":`), 0o600); err != nil {
		t.Fatalf("corrupt primary cursor: %v", err)
	}
	recovered, err := client.LoadCursor()
	if !errors.Is(err, ErrCursorRecovered) {
		t.Fatalf("load error = %v, want ErrCursorRecovered", err)
	}
	if recovered != first {
		t.Fatalf("recovered cursor = %+v, want %+v", recovered, first)
	}
}

func TestCursorPersistenceRejectsCorruptPrimaryAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.cursor")
	if err := os.WriteFile(path, []byte(`not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", []byte(`also-not-json`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewClient(config.CollectorConfig{CursorPath: path})
	if _, err := client.LoadCursor(); err == nil || errors.Is(err, ErrCursorRecovered) {
		t.Fatalf("load error = %v, want unrecoverable corruption", err)
	}
	if err := client.SaveCursor(Cursor{EventID: "evt-new"}); err == nil {
		t.Fatal("save must not overwrite an invalid primary cursor")
	}
}
