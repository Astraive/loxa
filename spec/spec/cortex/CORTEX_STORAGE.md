# CORTEX Storage

## Storage Interface

```
type EventStore interface {
  // Store a single event
  StoreEvent(event CortexEvent) error
  
  // Query events
  QueryEvents(filter EventFilter) ([]CortexEvent, error)
  QueryEventsByTraceID(traceID string) ([]CortexEvent, error)
  QueryEventsByIncidentID(incidentID string) ([]CortexEvent, error)
  
  // Check idempotency
  EventExists(eventID string) (bool, error)
}

type GraphStore interface {
  // Store nodes and edges
  StoreNode(node GraphNode) error
  StoreEdge(edge GraphEdge) error
  
  // Query graph
  GetNode(nodeID string) (GraphNode, error)
  GetNodesOfType(nodeType string) ([]GraphNode, error)
  GetEdgesBetween(fromID, toID string) ([]GraphEdge, error)
  GetServiceDependencies(service string) ([]GraphNode, error)
}

type FeedbackStore interface {
  StoreFeedback(feedback Feedback) error
  QueryFeedback(filter FeedbackFilter) ([]Feedback, error)
}
```

## Backend Implementations

### PostgreSQL (Recommended)
- events table: event_id, timestamp, service, trace_id, incident_id, raw_payload
- graph_nodes table: id, type, label, attributes, service
- graph_edges table: id, from_id, to_id, type, weight, attributes
- feedback table: feedback_id, incident_id, remediation_id, success, notes

Indexes:
- events (event_id, incident_id, trace_id, service, timestamp)
- graph_nodes (id, type, service)
- graph_edges (from_id, to_id, type)

### DuckDB (For Analysis)
- Optimized for OLAP queries (analytics)
- Used for signature learning batch jobs
- Reads from PostgreSQL events/feedback

## Query Patterns

### Recent Events for Service
```sql
SELECT * FROM events 
WHERE service = 'api-server' 
AND timestamp > now() - interval '1h'
ORDER BY timestamp DESC
```

### Trace Reconstruction
```sql
SELECT * FROM events
WHERE trace_id = 'abc123'
ORDER BY timestamp
```

### Service Dependencies
```sql
SELECT DISTINCT from_node FROM graph_edges
WHERE to_node = 'api-server' AND type = 'calls'
```

## Data Retention

- Events: 30 days (configurable)
- Graph nodes: indefinite
- Graph edges: indefinite
- Feedback: indefinite (learning history)

## Performance

- Event ingest: < 10ms p99
- Reconstruction query: < 500ms p99
- Graph traversal (BFS): < 100ms for 100k nodes

## Backup & Recovery

- Daily snapshots of PostgreSQL
- Incident graphs backed up after reconstruction
- Feedback logs archived for learning audits
