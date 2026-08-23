package correlation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/models"
)

type failingGraphStore struct {
	err      error
	attempts int
}

func (s *failingGraphStore) SaveNode(context.Context, *models.Node) error { return nil }
func (s *failingGraphStore) GetNode(context.Context, string) (*models.Node, error) {
	return nil, nil
}
func (s *failingGraphStore) ListNodes(context.Context, string, int) ([]*models.Node, error) {
	return nil, nil
}
func (s *failingGraphStore) SaveEdge(context.Context, *models.Edge) error {
	s.attempts++
	return s.err
}
func (s *failingGraphStore) GetEdges(context.Context, string, string) ([]*models.Edge, error) {
	return nil, nil
}
func (s *failingGraphStore) Traverse(context.Context, string, models.TraversalOptions) (*models.GraphView, error) {
	return nil, nil
}

func TestSynthesizeCoOccurrenceReturnsPersistenceErrors(t *testing.T) {
	persistErr := errors.New("persist edge")
	graph := &failingGraphStore{err: persistErr}
	analyzer := NewAnalyzer(DefaultConfig(), nil, graph)
	now := time.Now()
	events := []*models.Event{
		{ID: "evt-a", Service: "api", Kind: models.EventKindLog, Timestamp: now, Raw: map[string]any{"level": "error"}},
		{ID: "evt-b", Service: "db", Kind: models.EventKindLog, Timestamp: now, Raw: map[string]any{"level": "error"}},
	}

	err := analyzer.synthesizeCoOccurrence(context.Background(), events)
	if !errors.Is(err, persistErr) {
		t.Fatalf("expected persistence error, got %v", err)
	}
	if graph.attempts != 3 {
		t.Fatalf("expected three idempotent save attempts, got %d", graph.attempts)
	}
}
