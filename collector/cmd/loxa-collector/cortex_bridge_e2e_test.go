package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	loxav1 "github.com/astraive/loxa/gen/go/loxa/core"
	"github.com/astraive/loxa/collector/internal/ingest"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestCollectorHTTPIngestBridgesToCortexBatch(t *testing.T) {
	endpoint, fake, stop := startFakeCortexBridge(t)
	defer stop()

	cfg := testCollectorConfig()
	cfg.cortexBridgeEnabled = true
	cfg.cortexBridgeMode = "sync"
	cfg.cortexBridgeEndpoint = endpoint
	cfg.cortexBridgeInsecure = true
	cfg.cortexBridgeTimeout = 3 * time.Second

	sink := &fakeSink{}
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	var err error
	state.cortexBridge, err = newCortexBridgeClient(cfg, &state.metrics)
	require.NoError(t, err)
	defer state.cortexBridge.Close()

	body := `{"event_id":"evt-bridge-http","timestamp":"` + time.Now().UTC().Add(-time.Minute).Format(time.RFC3339) + `","service":"checkout","kind":"http","event":"payment.completed","trace_id":"tr-http-1"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	var out ingest.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, 1, out.Accepted)

	require.Eventually(t, func() bool {
		return fake.batchCount() >= 1
	}, 5*time.Second, 50*time.Millisecond)

	batch := fake.lastBatch()
	require.NotNil(t, batch)
	require.Len(t, batch.Events, 1)
	require.Equal(t, "evt-bridge-http", batch.Events[0].EventId)
	require.Equal(t, "checkout", batch.Events[0].Service)
	require.Equal(t, loxav1.EventKind_EVENT_KIND_EVENT, batch.Events[0].Kind)
	require.Equal(t, "tr-http-1", batch.Events[0].TraceId)
}

func TestCollectorBridgeMetricsIncrement(t *testing.T) {
	endpoint, _, stop := startFakeCortexBridge(t)
	defer stop()

	cfg := testCollectorConfig()
	cfg.cortexBridgeEnabled = true
	cfg.cortexBridgeMode = "sync"
	cfg.cortexBridgeEndpoint = endpoint
	cfg.cortexBridgeInsecure = true
	cfg.cortexBridgeTimeout = 3 * time.Second

	sink := &fakeSink{}
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	var err error
	state.cortexBridge, err = newCortexBridgeClient(cfg, &state.metrics)
	require.NoError(t, err)
	defer state.cortexBridge.Close()

	body := `{"event_id":"evt-bridge-metrics","timestamp":"` + time.Now().UTC().Add(-time.Minute).Format(time.RFC3339) + `","service":"billing","kind":"event","event":"invoice.generated"}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code)

	require.Eventually(t, func() bool {
		return state.metrics.cortexBridgeFlushes.Load() >= 1 &&
			state.metrics.cortexBridgeEvents.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)
	require.EqualValues(t, 0, state.metrics.cortexBridgeErrors.Load())
}
