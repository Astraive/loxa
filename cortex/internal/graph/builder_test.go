package graph

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

type fakeGraphStore struct {
	nodes map[string]*models.Node
	edges map[string][]*models.Edge
	savedNodes []*models.Node
	savedEdges []*models.Edge
}

func (f *fakeGraphStore) SaveNode(_ context.Context, node *models.Node) error {
	f.savedNodes = append(f.savedNodes, node)
	f.nodes[node.ID] = node
	return nil
}

func (f *fakeGraphStore) GetNode(_ context.Context, id string) (*models.Node, error) {
	if node, ok := f.nodes[id]; ok {
		return node, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeGraphStore) ListNodes(_ context.Context, nodeType string, limit int) ([]*models.Node, error) {
	var out []*models.Node
	for _, node := range f.nodes {
		if nodeType == "" || string(node.Type) == nodeType {
			out = append(out, node)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeGraphStore) SaveEdge(_ context.Context, edge *models.Edge) error {
	f.savedEdges = append(f.savedEdges, edge)
	f.edges[edge.FromNodeID] = append(f.edges[edge.FromNodeID], edge)
	return nil
}

func (f *fakeGraphStore) GetEdges(_ context.Context, nodeID string, edgeType string) ([]*models.Edge, error) {
	var out []*models.Edge
	for _, edge := range f.edges[nodeID] {
		if edgeType == "" || string(edge.Type) == edgeType {
			out = append(out, edge)
		}
	}
	return out, nil
}

func (f *fakeGraphStore) Traverse(ctx context.Context, startNodeID string, opts models.TraversalOptions) (*models.GraphView, error) {
	return nil, nil
}

func TestAddNodeAndEdge(t *testing.T) {
	store := &fakeGraphStore{nodes: map[string]*models.Node{}, edges: map[string][]*models.Edge{}}
	builder := NewBuilder(store)

	node := &models.Node{ID: "svc-a", Type: models.NodeTypeService, Label: "svc-a"}
	if err := builder.AddNode(context.Background(), node); err != nil {
		t.Fatalf("add node: %v", err)
	}
	if node.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}

	edge := &models.Edge{ID: "e1", FromNodeID: "svc-a", ToNodeID: "svc-a", Type: models.EdgeTypeDependsOn, Weight: 1}
	if err := builder.AddEdge(context.Background(), edge); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if edge.CreatedAt.IsZero() {
		t.Fatal("expected edge created_at to be set")
	}
}

func TestTraverseGraph(t *testing.T) {
	store := &fakeGraphStore{
		nodes: map[string]*models.Node{},
		edges: map[string][]*models.Edge{},
	}
	now := time.Now()
	store.nodes["n1"] = &models.Node{ID: "n1", Type: models.NodeTypeIncident, Label: "n1", CreatedAt: now}
	store.nodes["n2"] = &models.Node{ID: "n2", Type: models.NodeTypeMetric, Label: "n2", CreatedAt: now.Add(time.Second)}
	store.nodes["n3"] = &models.Node{ID: "n3", Type: models.NodeTypeLog, Label: "n3", CreatedAt: now.Add(2 * time.Second)}
	store.edges["n1"] = []*models.Edge{{ID: "e1", FromNodeID: "n1", ToNodeID: "n2", Type: models.EdgeTypeDependsOn, Weight: 1}}
	store.edges["n2"] = []*models.Edge{{ID: "e2", FromNodeID: "n2", ToNodeID: "n3", Type: models.EdgeTypeDependsOn, Weight: 1}}

	view, err := NewBuilder(store).TraverseGraph(context.Background(), "n1", models.TraversalOptions{
		MaxDepth: 2,
		EdgeTypes: []models.EdgeType{models.EdgeTypeDependsOn},
	})
	if err != nil {
		t.Fatalf("traverse: %v", err)
	}
	if len(view.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(view.Nodes))
	}
	if len(view.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(view.Edges))
	}
}
