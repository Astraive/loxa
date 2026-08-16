package collectorbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
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
