package cortex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.Health(context.Background()) {
		t.Fatal("expected health=true")
	}
}

func TestClientReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.Ready(context.Background()) {
		t.Fatal("expected ready=true")
	}
}

func TestClientHealthUnhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.Health(context.Background()) {
		t.Fatal("expected health=false")
	}
}

func TestClientAuthHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL).WithAPIKey("test-key")
	c.Health(context.Background())

	if gotHeader != "test-key" {
		t.Fatalf("expected auth header 'test-key', got '%s'", gotHeader)
	}
}

func TestClientReconstruct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/reconstruct" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["incident_id"] != "inc-123" {
			t.Errorf("expected incident_id=inc-123, got %s", body["incident_id"])
		}
		_ = json.NewEncoder(w).Encode(IncidentContext{
			IncidentID:      "inc-123",
			Timestamp:       "2026-05-20T00:00:00Z",
			CausalChain:     []map[string]any{{"cause": "deploy", "effect": "error"}},
			RelatedServices: []string{"svc-a", "svc-b"},
			Confidence:      0.85,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, err := c.Reconstruct(context.Background(), "inc-123", "fast")
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if ctx.IncidentID != "inc-123" {
		t.Fatalf("expected incident_id=inc-123, got %s", ctx.IncidentID)
	}
	if ctx.Confidence != 0.85 {
		t.Fatalf("expected confidence=0.85, got %f", ctx.Confidence)
	}
	if len(ctx.RelatedServices) != 2 {
		t.Fatalf("expected 2 related services, got %d", len(ctx.RelatedServices))
	}
}

func TestClientServiceGraph(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/graph/service/svc-a" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("depth") != "2" {
			t.Errorf("expected depth=2, got %s", r.URL.Query().Get("depth"))
		}
		_ = json.NewEncoder(w).Encode(GraphView{
			Nodes: []map[string]any{{"id": "svc-a", "type": "service"}, {"id": "svc-b", "type": "service"}},
			Edges: []map[string]any{{"source": "svc-a", "target": "svc-b", "type": "depends_on"}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	gv, err := c.ServiceGraph(context.Background(), "svc-a", 2)
	if err != nil {
		t.Fatalf("service_graph: %v", err)
	}
	if len(gv.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(gv.Nodes))
	}
	if len(gv.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(gv.Edges))
	}
}

func TestClientRecordRemediation(t *testing.T) {
	var received Remediation
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/feedback/remediation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.RecordRemediation(context.Background(), &Remediation{
		IncidentID: "inc-123",
		Action:     "rollback",
	})
	if err != nil {
		t.Fatalf("record_remediation: %v", err)
	}
	if received.Action != "rollback" {
		t.Fatalf("expected action=rollback, got %s", received.Action)
	}
}

func TestClientRecordFeedback(t *testing.T) {
	var received RemediationFeedback
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/feedback/incident" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.RecordFeedback(context.Background(), &RemediationFeedback{
		RemediationID: "rem-1",
		IncidentID:    "inc-123",
		Outcome:       "success",
	})
	if err != nil {
		t.Fatalf("record_feedback: %v", err)
	}
	if received.Outcome != "success" {
		t.Fatalf("expected outcome=success, got %s", received.Outcome)
	}
}

func TestClientIngestBatch(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/batch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	events := []map[string]any{
		{"event_id": "e1", "event": "test", "kind": "event", "service": "svc"},
	}
	err := c.IngestBatch(context.Background(), events)
	if err != nil {
		t.Fatalf("ingest_batch: %v", err)
	}
	evts, ok := received["events"].([]any)
	if !ok || len(evts) != 1 {
		t.Fatalf("expected 1 event, got %v", received["events"])
	}
}

func TestClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Reconstruct(context.Background(), "inc-123", "fast")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientValidation(t *testing.T) {
	c := NewClient("http://localhost:1")

	// Missing incident_id
	err := c.RecordRemediation(context.Background(), &Remediation{Action: "rollback"})
	if err == nil {
		t.Fatal("expected validation error for missing incident_id")
	}

	// Missing action
	err = c.RecordRemediation(context.Background(), &Remediation{IncidentID: "inc-1"})
	if err == nil {
		t.Fatal("expected validation error for missing action")
	}

	// Missing remediation_id
	err = c.RecordFeedback(context.Background(), &RemediationFeedback{IncidentID: "inc-1", Outcome: "success"})
	if err == nil {
		t.Fatal("expected validation error for missing remediation_id")
	}
}

func TestClientMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# HELP cortex_events_total Total events\n# TYPE cortex_events_total counter\ncortex_events_total 42\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	metrics, err := c.Metrics(context.Background())
	if err != nil {
		t.Fatalf("metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected non-empty metrics")
	}
}

func TestClientHealthWithJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.Health(context.Background()) {
		t.Fatal("expected health=true with status:ok body")
	}
}

func TestClientReadyWithJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ready":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if !c.Ready(context.Background()) {
		t.Fatal("expected ready=true with ready:true body")
	}
}

func TestClientReadyDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"degraded"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if c.Ready(context.Background()) {
		t.Fatal("expected ready=false with status:degraded body")
	}
}
