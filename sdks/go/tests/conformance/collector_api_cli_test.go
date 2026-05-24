package conformance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestCollectorAndCortexClientFamilies(t *testing.T) {
	collectorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/validate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/ingest":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"accepted":1}`))
		case "/query":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"events":[]}`))
		case "/tail":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"events":[]}`))
		case "/events":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"deleted":0}`))
		case "/replay":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"replayed":0}`))
		case "/dlq/list":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entries":[]}`))
		case "/dlq/item":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"entry":null}`))
		case "/dlq/item/replay":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"replayed":1}`))
		case "/keys":
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"id":"key_123"}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "/keys/key_123":
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"revoked":true}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "/keys/key_123/rotate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"rotated":true}`))
		case "/sinks":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"sinks":[]}`))
		case "/sinks/stdout/test":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		case "/policy/validate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"valid":true,"errors":[]}`))
		case "/schema/check":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"valid":true}`))
		case "/schema/publish":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"published":true}`))
		case "/retention/apply":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"applied":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer collectorServer.Close()

	collectorClient := loxa.NewCollectorClient(loxa.CollectorClientConfig{
		Endpoint: collectorServer.URL,
		Client:   collectorServer.Client(),
	})
	if err := collectorClient.Health(context.Background()); err != nil {
		t.Fatalf("collector health: %v", err)
	}
	if _, err := collectorClient.Validate(context.Background(), json.RawMessage(`{"event":"verification"}`)); err != nil {
		t.Fatalf("collector validate: %v", err)
	}
	if _, err := collectorClient.Ingest(context.Background(), []json.RawMessage{json.RawMessage(`{"event":"verification"}`)}); err != nil {
		t.Fatalf("collector ingest: %v", err)
	}
	if _, err := collectorClient.Query(context.Background(), json.RawMessage(`{"query":"select 1"}`)); err != nil {
		t.Fatalf("collector query: %v", err)
	}
	if _, err := collectorClient.Tail(context.Background(), json.RawMessage(`{"limit":1}`)); err != nil {
		t.Fatalf("collector tail: %v", err)
	}
	if _, err := collectorClient.Delete(context.Background(), json.RawMessage(`{"event":"verification"}`)); err != nil {
		t.Fatalf("collector delete: %v", err)
	}
	if _, err := collectorClient.Replay(context.Background(), json.RawMessage(`{"event_ids":["evt_1"]}`)); err != nil {
		t.Fatalf("collector replay: %v", err)
	}
	if _, err := collectorClient.DLQList(context.Background(), json.RawMessage(`{"limit":1}`)); err != nil {
		t.Fatalf("collector dlq list: %v", err)
	}
	if _, err := collectorClient.DLQRead(context.Background(), "item"); err != nil {
		t.Fatalf("collector dlq read: %v", err)
	}
	if _, err := collectorClient.DLQReplay(context.Background(), "item"); err != nil {
		t.Fatalf("collector dlq replay: %v", err)
	}
	if _, err := collectorClient.KeysCreate(context.Background(), json.RawMessage(`{"name":"verification"}`)); err != nil {
		t.Fatalf("collector keys create: %v", err)
	}
	if _, err := collectorClient.KeysRevoke(context.Background(), "key_123"); err != nil {
		t.Fatalf("collector keys revoke: %v", err)
	}
	if _, err := collectorClient.KeysRotate(context.Background(), "key_123"); err != nil {
		t.Fatalf("collector keys rotate: %v", err)
	}
	if _, err := collectorClient.SinksList(context.Background()); err != nil {
		t.Fatalf("collector sinks list: %v", err)
	}
	if _, err := collectorClient.SinksTest(context.Background(), "stdout"); err != nil {
		t.Fatalf("collector sinks test: %v", err)
	}
	if _, err := collectorClient.PolicyValidate(context.Background(), json.RawMessage(`{"sample_rate":1.0}`)); err != nil {
		t.Fatalf("collector policy validate: %v", err)
	}
	if _, err := collectorClient.SchemaCheck(context.Background(), json.RawMessage(`{"event":"verification"}`)); err != nil {
		t.Fatalf("collector schema check: %v", err)
	}
	if _, err := collectorClient.SchemaPublish(context.Background(), json.RawMessage(`{"schema":"v1"}`)); err != nil {
		t.Fatalf("collector schema publish: %v", err)
	}
	if _, err := collectorClient.RetentionApply(context.Background(), json.RawMessage(`{"days":7}`)); err != nil {
		t.Fatalf("collector retention apply: %v", err)
	}

	cortexServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","ready":true}`))
		case "/reconstruct":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"incident_id":"inc_123","timestamp":"2026-05-23T00:00:00Z","confidence":0.9,"related_services":["checkout"],"related_events":[],"causal_chain":[],"similar_past_incidents":[],"suggested_remediations":[],"symptoms":[],"explain":"ok"}`))
		case "/graph/service/checkout":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"nodes":[{"id":"checkout"}],"edges":[]}`))
		case "/graph/incident/inc_123":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"nodes":[{"id":"checkout"}],"edges":[]}`))
		case "/feedback/remediation", "/feedback/incident", "/events/batch":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer cortexServer.Close()

	cortexClient := loxa.NewCortexClient(cortexServer.URL)
	if !cortexClient.Health(context.Background()) {
		t.Fatalf("expected cortex health")
	}
	if !cortexClient.Ready(context.Background()) {
		t.Fatalf("expected cortex ready")
	}
	if _, err := cortexClient.Reconstruct(context.Background(), "inc_123", "fast"); err != nil {
		t.Fatalf("cortex reconstruct: %v", err)
	}
	if _, err := cortexClient.ServiceGraph(context.Background(), "checkout", 1); err != nil {
		t.Fatalf("cortex service graph: %v", err)
	}
	if _, err := cortexClient.IncidentGraph(context.Background(), "inc_123", 1); err != nil {
		t.Fatalf("cortex incident graph: %v", err)
	}
	if err := cortexClient.RecordRemediation(context.Background(), &loxa.Remediation{IncidentID: "inc_123", Action: "restart"}); err != nil {
		t.Fatalf("cortex remediation: %v", err)
	}
	if err := cortexClient.RecordFeedback(context.Background(), &loxa.RemediationFeedback{RemediationID: "rem_123", IncidentID: "inc_123", Outcome: "success"}); err != nil {
		t.Fatalf("cortex feedback: %v", err)
	}
	if err := cortexClient.IngestBatch(context.Background(), []map[string]any{{"event": "verification"}}); err != nil {
		t.Fatalf("cortex ingest batch: %v", err)
	}
	if err := loxa.ValidateIncidentContext(&loxa.IncidentContext{}); err == nil {
		t.Fatalf("expected incident context validation error")
	}
}
