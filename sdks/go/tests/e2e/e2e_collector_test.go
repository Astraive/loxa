package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	loza "github.com/astraive/loza/sdks/go"
)

const defaultCollectorURL = "http://127.0.0.1:9308"

func collectorURL() string {
	if value := strings.TrimRight(os.Getenv("LOZA_TEST_COLLECTOR_URL"), "/"); value != "" {
		return value
	}
	return defaultCollectorURL
}

func ingestAPIKey() string {
	return os.Getenv("LOZA_API_KEY")
}

func adminAPIKey() string {
	return os.Getenv("LOZA_TEST_COLLECTOR_ADMIN_KEY")
}

// newTestClient creates a SDK Logger that auto-installs HTTPBatchSink
// when CollectorURL is set — this is the production path.
func newTestClient(t *testing.T, opts ...loza.ConfigOption) *loza.Logger {
	t.Helper()
	cfg := loza.Config{
		Service:      "e2e-test",
		CollectorURL: collectorURL(),
		APIKey:       ingestAPIKey(),
	}
	for _, opt := range opts {
		cfg = opt(cfg)
	}
	l, err := loza.NewClient(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	t.Cleanup(func() { l.Shutdown(context.Background()) })
	return l
}

func TestE2E_HTTPBatchSink_BasicEvent(t *testing.T) {
	l := newTestClient(t)

	ctx := context.Background()
	l.StartEvent(ctx, loza.Params{Event: "e2e.batch.basic"})
	loza.Set(ctx, loza.String("test_id", "batch-001"))
	loza.Set(ctx, loza.String("env", "e2e"))
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	if err := l.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	body := getCollectorStatus(t)
	t.Logf("Collector status: %s", string(body))
}

func TestE2E_HTTPBatchSink_WithRedaction(t *testing.T) {
	l := newTestClient(t, loza.WithRedactor(loza.DefaultRedactor()))

	ctx := context.Background()
	l.StartEvent(ctx, loza.Params{Event: "e2e.batch.redaction"})
	loza.Set(ctx, loza.String("password", "supersecret"))
	loza.Set(ctx, loza.String("user_input", "normal value"))
	if err := loza.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}

	if err := l.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	getCollectorStatus(t)
}

func TestE2E_HTTPBatchSink_AsyncBatch(t *testing.T) {
	l := newTestClient(t,
		loza.WithAsync(true),
		loza.WithWorkers(2),
	)

	ctx := context.Background()
	for i := range 10 {
		l.StartEvent(ctx, loza.Params{Event: "e2e.batch.async"})
		loza.Set(ctx, loza.Int("seq", i))
		if err := loza.Finish(ctx, "success"); err != nil {
			t.Fatalf("finish %d: %v", i, err)
		}
	}

	if err := l.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	time.Sleep(1 * time.Second)
	getCollectorStatus(t)
}

func TestE2E_HTTPBatchSink_VersionCheck(t *testing.T) {
	_ = newTestClient(t)

	resp, err := http.Get(collectorURL() + "/version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var version map[string]any
	if err := json.Unmarshal(body, &version); err != nil {
		t.Fatalf("parse version: %v", err)
	}
	t.Logf("Collector version: %+v", version)

	if _, ok := version["ingest_api_version"]; !ok {
		t.Fatal("expected ingest_api_version in version response")
	}
}

func TestE2E_HTTPBatchSink_MultipleFlushes(t *testing.T) {
	l := newTestClient(t)

	ctx := context.Background()
	for batch := range 3 {
		for i := range 5 {
			l.StartEvent(ctx, loza.Params{Event: "e2e.batch.multi"})
			loza.Set(ctx, loza.Int("batch", batch))
			loza.Set(ctx, loza.Int("seq", i))
			if err := loza.Finish(ctx, "success"); err != nil {
				t.Fatalf("finish batch=%d seq=%d: %v", batch, i, err)
			}
		}
		if err := l.Flush(ctx); err != nil {
			t.Fatalf("flush batch %d: %v", batch, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)
	body := getCollectorStatus(t)
	t.Logf("After 3 batches of 5: %s", string(body))
}

func getCollectorStatus(t *testing.T) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, collectorURL()+"/status", nil)
	if err != nil {
		t.Fatalf("create status request: %v", err)
	}
	if token := adminAPIKey(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, string(body))
	}
	return body
}
