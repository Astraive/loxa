package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsCollectorExportsPrometheusText(t *testing.T) {
	mc := NewMetricsCollector("loxa_test", 8)
	mc.OnEventCreated()
	mc.OnRetry(12)
	mc.OnBackpressure()

	rec := httptest.NewRecorder()
	mc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `loxa_test_events_created_total 1`) {
		t.Fatalf("expected events_created_total in metrics output, got %s", body)
	}
	if !strings.Contains(body, `loxa_test_retry_total{attempt="12"} 1`) {
		t.Fatalf("expected retry_total label in metrics output, got %s", body)
	}
	if !strings.Contains(body, `loxa_test_backpressure_total 1`) {
		t.Fatalf("expected backpressure_total in metrics output, got %s", body)
	}
}

func TestPrometheusStatsHandlerExposesMetrics(t *testing.T) {
	h := NewPrometheusStatsHandler("loxa_obs", 4)
	h.OnEmit(nil)
	h.OnDeliveryFailed(nil, nil)

	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `loxa_obs_events_emitted_total{status="success"} 1`) {
		t.Fatalf("expected emitted success metric, got %s", body)
	}
	if !strings.Contains(body, `loxa_obs_events_emitted_total{status="failure"} 1`) {
		t.Fatalf("expected emitted failure metric, got %s", body)
	}
}

func TestLoggerFeedsPrometheusStatsHandler(t *testing.T) {
	sink, _ := MemorySink()
	stats := NewPrometheusStatsHandler("loxa_hook", 4)
	l, err := New(Test().WithSink(sink).WithStatsHandler(stats))
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	ctx := l.StartEvent(context.Background(), Params{Event: "metrics.test"})
	if err := l.Finish(ctx, "success"); err != nil {
		t.Fatalf("finish: %v", err)
	}
	if err := l.Emit(ctx); err != nil {
		t.Fatalf("emit: %v", err)
	}

	rec := httptest.NewRecorder()
	stats.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := rec.Body.String()
	if !strings.Contains(body, `loxa_hook_events_created_total 1`) {
		t.Fatalf("expected created counter in metrics output, got %s", body)
	}
	if !strings.Contains(body, `loxa_hook_events_finished_total 1`) {
		t.Fatalf("expected finished counter in metrics output, got %s", body)
	}
	if !strings.Contains(body, `loxa_hook_emit_duration_seconds_count 1`) {
		t.Fatalf("expected emit duration histogram in metrics output, got %s", body)
	}
}
