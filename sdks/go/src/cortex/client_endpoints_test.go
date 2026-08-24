package cortex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loza/sdks/go/src/core"
)

func TestClientEndpointVariantsAndOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/incidents/inc%2F1/reconstruct", "/incidents/inc/1/reconstruct":
			_ = json.NewEncoder(w).Encode(IncidentContext{IncidentID: "inc/1", Timestamp: "2026-01-01T00:00:00Z", Confidence: 0.9})
		case "/reconstruct":
			_ = json.NewEncoder(w).Encode(IncidentContext{IncidentID: "inc-1", SimilarIncidents: []map[string]any{{"id": "similar"}}})
		case "/graph/incident/inc%2F1", "/graph/incident/inc/1", "/graph/service/api":
			_ = json.NewEncoder(w).Encode(GraphView{Nodes: []map[string]any{{"id": "api", "type": "service"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL+"/").WithAPIKey("secret").WithAuthHeader("Authorization").WithHTTPClient(srv.Client())
	if c == nil {
		t.Fatal("options returned nil client")
	}
	if got, err := c.ReconstructIncident(context.Background(), "inc/1", "fast"); err != nil || got == nil {
		t.Fatalf("ReconstructIncident = %#v, %v", got, err)
	}
	if got, err := c.IncidentGraph(context.Background(), "inc/1", 2); err != nil || len(got.Nodes) != 1 {
		t.Fatalf("IncidentGraph = %#v, %v", got, err)
	}
	if got, err := c.ServiceGraph(context.Background(), "api", 1); err != nil || len(got.Nodes) != 1 {
		t.Fatalf("ServiceGraph = %#v, %v", got, err)
	}
}

func TestClientHealthReadyAndMetricsBoundaries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"degraded"}`))
		case "/metrics":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("bad"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if c.Health(context.Background()) || c.Ready(context.Background()) {
		t.Fatal("degraded health/readiness reported true")
	}
	if _, err := c.Metrics(context.Background()); err == nil {
		t.Fatal("Metrics accepted non-2xx response")
	}
	if got := NewClient(""); got == nil {
		t.Fatal("default endpoint client is nil")
	}
}

func TestClientSimilarIncidentsAndSchemaConstructors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(IncidentContext{SimilarIncidents: []map[string]any{{"id": "similar"}}})
	}))
	defer srv.Close()
	similar, err := NewClient(srv.URL).SimilarIncidents(context.Background(), "inc-1")
	if err != nil || len(similar) != 1 {
		t.Fatalf("SimilarIncidents = %#v, %v", similar, err)
	}
	if DefaultSchema() == nil || FlatSchema() == nil || NestedSchema() == nil || OTelLogSchema() == nil || ECSchema() == nil || DatadogSchema() == nil || CustomSchema(func(core.EventView) map[string]any { return nil }) == nil {
		t.Fatal("schema constructor returned nil")
	}
}
