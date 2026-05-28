package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/astraive/loxa/cortex/internal/models"
	"github.com/astraive/loxa/cortex/internal/storage"
)

// AdjacencyCache preloads edges into memory for fast in-memory BFS.
type AdjacencyCache struct {
	edges    map[string][]*models.Edge // nodeID -> edges
	loaded   bool
	mu       sync.RWMutex
}

func NewAdjacencyCache() *AdjacencyCache {
	return &AdjacencyCache{
		edges: make(map[string][]*models.Edge),
	}
}

func (c *AdjacencyCache) GetEdges(nodeID string) []*models.Edge {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.edges[nodeID]
}

func (c *AdjacencyCache) SetEdges(nodeID string, edges []*models.Edge) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.edges[nodeID] = edges
}

func (c *AdjacencyCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.edges = make(map[string][]*models.Edge)
	c.loaded = false
}

type Builder struct {
	graphStore      storage.GraphStore
	adjCache        *AdjacencyCache
	defaultMaxDepth int
}

func NewBuilder(store storage.GraphStore) *Builder {
	return &Builder{
		graphStore:      store,
		adjCache:        NewAdjacencyCache(),
		defaultMaxDepth: 3,
	}
}

// WithDefaultMaxDepth sets the default traversal depth.
func (b *Builder) WithDefaultMaxDepth(depth int) *Builder {
	if depth > 0 {
		b.defaultMaxDepth = depth
	}
	return b
}

func (b *Builder) GraphStore() storage.GraphStore {
	return b.graphStore
}

func (b *Builder) AddNode(ctx context.Context, node *models.Node) error {
	if err := node.Validate(); err != nil {
		return fmt.Errorf("node validation failed: %w", err)
	}
	node.CreatedAt = time.Now()
	return b.graphStore.SaveNode(ctx, node)
}

func (b *Builder) AddEdge(ctx context.Context, edge *models.Edge) error {
	if err := edge.Validate(); err != nil {
		return fmt.Errorf("edge validation failed: %w", err)
	}

	_, err := b.graphStore.GetNode(ctx, edge.FromNodeID)
	if err != nil {
		return fmt.Errorf("from_node_id %s does not exist: %w", edge.FromNodeID, err)
	}

	_, err = b.graphStore.GetNode(ctx, edge.ToNodeID)
	if err != nil {
		return fmt.Errorf("to_node_id %s does not exist: %w", edge.ToNodeID, err)
	}

	edge.CreatedAt = time.Now()
	return b.graphStore.SaveEdge(ctx, edge)
}

func (b *Builder) GetNode(ctx context.Context, nodeID string) (*models.Node, error) {
	return b.graphStore.GetNode(ctx, nodeID)
}

func (b *Builder) GetEdges(ctx context.Context, nodeID string, edgeType string) ([]*models.Edge, error) {
	return b.graphStore.GetEdges(ctx, nodeID, edgeType)
}

func (b *Builder) TraverseGraph(ctx context.Context, startNodeID string, opts models.TraversalOptions) (*models.GraphView, error) {
	if opts.MaxDepth == 0 {
		opts.MaxDepth = b.defaultMaxDepth
	}

	visited := make(map[string]bool)
	var queue []string
	queue = append(queue, startNodeID)

	var allNodes []*models.Node
	var allEdges []*models.Edge

	currentDepth := 0
	for len(queue) > 0 && currentDepth < opts.MaxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			nodeID := queue[i]
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			node, err := b.graphStore.GetNode(ctx, nodeID)
			if err != nil {
				continue
			}

			if opts.TimeWindowMin != nil && node.CreatedAt.Before(*opts.TimeWindowMin) {
				continue
			}
			if opts.TimeWindowMax != nil && node.CreatedAt.After(*opts.TimeWindowMax) {
				continue
			}

			allNodes = append(allNodes, node)

			// Use adjacency cache for edges
			edges := b.adjCache.GetEdges(nodeID)
			if edges == nil {
				fetched, err := b.graphStore.GetEdges(ctx, nodeID, "")
				if err != nil {
					continue
				}
				edges = fetched
				b.adjCache.SetEdges(nodeID, edges)
			}

			for _, edge := range edges {
				if len(opts.EdgeTypes) > 0 {
					found := false
					for _, t := range opts.EdgeTypes {
						if edge.Type == t {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				allEdges = append(allEdges, edge)

				if !visited[edge.ToNodeID] {
					queue = append(queue, edge.ToNodeID)
				}
			}
		}
		queue = queue[levelSize:]
		currentDepth++
	}

	return &models.GraphView{Nodes: allNodes, Edges: allEdges}, nil
}

func (b *Builder) GetServiceGraph(ctx context.Context, service string, depth int) (*models.GraphView, error) {
	opts := models.TraversalOptions{
		MaxDepth: depth,
	}
	return b.TraverseGraph(ctx, service, opts)
}

func (b *Builder) GetIncidentGraph(ctx context.Context, incidentID string, depth int) (*models.GraphView, error) {
	_, err := b.graphStore.GetNode(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("incident node not found: %w", err)
	}

	opts := models.TraversalOptions{
		MaxDepth: depth,
	}
	return b.TraverseGraph(ctx, incidentID, opts)
}

func init() {
	_ = models.NodeTypeService
}