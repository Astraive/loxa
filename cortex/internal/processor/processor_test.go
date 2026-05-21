package processor

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

type fakeEventStore struct {
	saved         []*models.Event
	batchSaved    [][]*models.Event
	byTraceID     map[string][]*models.Event
	byIncident    map[string][]*models.Event
	traceCalls    int
	incidentCalls int
}

func (f *fakeEventStore) Save(_ context.Context, event *models.Event) error {
	f.saved = append(f.saved, event)
	return nil
}

func (f *fakeEventStore) SaveBatch(_ context.Context, events []*models.Event) error {
	batch := make([]*models.Event, len(events))
	copy(batch, events)
	f.batchSaved = append(f.batchSaved, batch)
	return nil
}

func (f *fakeEventStore) Get(_ context.Context, id string) (*models.Event, error) {
	return nil, errors.New("not found")
}

func (f *fakeEventStore) List(context.Context, int, int) ([]*models.Event, error) {
	return nil, nil
}

func (f *fakeEventStore) FindByTraceID(_ context.Context, traceID string) ([]*models.Event, error) {
	f.traceCalls++
	return f.byTraceID[traceID], nil
}

func (f *fakeEventStore) FindByIncidentID(_ context.Context, incidentID string) ([]*models.Event, error) {
	f.incidentCalls++
	return f.byIncident[incidentID], nil
}

func (f *fakeEventStore) FindByService(context.Context, string, string, string) ([]*models.Event, error) {
	return nil, nil
}

type fakeTopologyStoreForProcessor struct {
	alias *models.ServiceAlias
}

func (f *fakeTopologyStoreForProcessor) SaveAlias(context.Context, *models.ServiceAlias) error {
	return nil
}

func (f *fakeTopologyStoreForProcessor) GetAlias(_ context.Context, alias, timestamp string) (*models.ServiceAlias, error) {
	if f.alias != nil && f.alias.Alias == alias {
		return f.alias, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeTopologyStoreForProcessor) GetHistory(context.Context, string) ([]*models.ServiceAlias, error) {
	return nil, nil
}

type fakeGraphStoreForProcessor struct {
	nodes []*models.Node
	edges []*models.Edge
}

func (f *fakeGraphStoreForProcessor) SaveNode(_ context.Context, node *models.Node) error {
	f.nodes = append(f.nodes, node)
	return nil
}

func (f *fakeGraphStoreForProcessor) GetNode(context.Context, string) (*models.Node, error) {
	return nil, errors.New("not found")
}

func (f *fakeGraphStoreForProcessor) ListNodes(context.Context, string, int) ([]*models.Node, error) {
	return nil, nil
}

func (f *fakeGraphStoreForProcessor) SaveEdge(_ context.Context, edge *models.Edge) error {
	f.edges = append(f.edges, edge)
	return nil
}

func (f *fakeGraphStoreForProcessor) GetEdges(context.Context, string, string) ([]*models.Edge, error) {
	return nil, nil
}

func (f *fakeGraphStoreForProcessor) Traverse(context.Context, string, models.TraversalOptions) (*models.GraphView, error) {
	return nil, nil
}

func TestProcessEventCreatesGraphNodes(t *testing.T) {
	events := &fakeEventStore{}
	topology := &fakeTopologyStoreForProcessor{
		alias: &models.ServiceAlias{Alias: "svc-a", Canonical: "svc-b"},
	}
	graphStore := &fakeGraphStoreForProcessor{}
	p := NewEventProcessor(events, topology, graphStore)

	event := &models.Event{
		ID:         "evt-1",
		Timestamp:  time.Now(),
		Kind:       models.EventKindDeploy,
		Service:    "svc-a",
		Provenance: "jsonl",
		Raw: map[string]any{
			"message":  "deploy failed",
			"secret":   "should-not-be-copied",
			"version":  "1.2.3",
			"trace_id": "trace-1",
		},
	}
	if err := p.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("process event: %v", err)
	}
	if len(events.saved) != 1 || events.saved[0].CreatedAt.IsZero() {
		t.Fatalf("expected normalized saved event, got %+v", events.saved)
	}
	if len(graphStore.nodes) != 3 {
		t.Fatalf("expected service, event, and deploy nodes, got %d", len(graphStore.nodes))
	}
	if len(graphStore.edges) != 2 {
		t.Fatalf("expected event-service and deploy-service edges, got %d", len(graphStore.edges))
	}
	if _, ok := graphStore.nodes[1].Attributes["secret"]; ok {
		t.Fatalf("expected graph node attrs to avoid copying raw secrets, got %+v", graphStore.nodes[1].Attributes)
	}
	if graphStore.nodes[1].Attributes["message"] != "deploy failed" {
		t.Fatalf("expected graph node attrs to keep derived message, got %+v", graphStore.nodes[1].Attributes)
	}
}

func TestProcessJSONLDefaultsProvenance(t *testing.T) {
	events := &fakeEventStore{}
	topology := &fakeTopologyStoreForProcessor{}
	graphStore := &fakeGraphStoreForProcessor{}
	p := NewEventProcessor(events, topology, graphStore)

	payload := bytes.NewBufferString(`{"id":"evt-2","timestamp":"2026-01-01T00:00:00Z","kind":"log","service":"svc-a"}` + "\n")
	if err := p.ProcessJSONL(context.Background(), payload); err != nil {
		t.Fatalf("process jsonl: %v", err)
	}
	if len(events.batchSaved) != 1 || len(events.batchSaved[0]) != 1 {
		t.Fatalf("expected one batch event, got %+v", events.batchSaved)
	}
	if got := events.batchSaved[0][0].Provenance; got != "jsonl" {
		t.Fatalf("expected jsonl provenance, got %s", got)
	}
}

func TestProcessBatchLinksLocalTraceWithoutStoreLookup(t *testing.T) {
	events := &fakeEventStore{}
	topology := &fakeTopologyStoreForProcessor{}
	graphStore := &fakeGraphStoreForProcessor{}
	p := NewEventProcessor(events, topology, graphStore)

	ts := time.Now()
	batch := []*models.Event{
		{
			ID:         "evt-1",
			Timestamp:  ts,
			Kind:       models.EventKindLog,
			Service:    "svc-a",
			TraceID:    "trace-1",
			Provenance: "jsonl",
		},
		{
			ID:         "evt-2",
			Timestamp:  ts.Add(time.Second),
			Kind:       models.EventKindMetric,
			Service:    "svc-a",
			TraceID:    "trace-1",
			Provenance: "jsonl",
		},
	}

	if err := p.ProcessBatch(context.Background(), batch); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if events.traceCalls != 0 {
		t.Fatalf("expected same-batch trace linking without store lookup, got %d trace lookups", events.traceCalls)
	}

	foundTraceEdge := false
	for _, edge := range graphStore.edges {
		if edge.Type == models.EdgeTypeSameTrace {
			foundTraceEdge = true
			break
		}
	}
	if !foundTraceEdge {
		t.Fatalf("expected same-trace edge in batch graph build, got %+v", graphStore.edges)
	}
}
