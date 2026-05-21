# CORTEX Graph Model

## Graph Nodes

### Node Types

```
service        - Microservice, application, or system component
event          - Individual event occurrence
trace          - Distributed trace (aggregated from events)
span           - Single span within a trace
request        - HTTP/gRPC request
deployment     - Application deployment or release
metric         - Time-series metric aggregation
log            - Log message or entry
incident       - Incident (aggregated from events)
remediation    - Remediation action applied
resource       - Infrastructure resource (host, pod, container)
```

### Node Schema
See: `schemas/json/cortex-graph-node.schema.json`

```
{
  id: string,
  type: NodeType,
  label: string,
  service?: string,
  attributes: object,
  created_at: timestamp,
  updated_at: timestamp
}
```

## Graph Edges

### Edge Types

```
depends_on          - A depends on B (service/resource dependency)
same_trace          - Events belong to same trace
same_incident       - Events belong to same incident
parent_span         - Span hierarchy
calls               - Service A calls service B
deployed_before     - Deployment A happened before B
metric_spiked_after - Metric spike occurred after event
log_error_after     - Error log appeared after event
caused_probably     - A probably caused B (RCA suggestion)
remediated_by       - Incident resolved by remediation
similar_shape       - Similar pattern/signature (ML-based)
```

### Edge Schema
See: `schemas/json/cortex-graph-edge.schema.json`

```
{
  id: string,
  from_node_id: string,
  to_node_id: string,
  type: EdgeType,
  weight: number [0-1],  // confidence/evidence
  attributes: object,
  created_at: timestamp
}
```

## Graph APIs

### Get Service Graph
```
GET /v1/graph/service/{service}

Query params:
- depth: 2 (default), go N hops from service
- include_incidents: boolean

Response:
{
  nodes: [{...}],
  edges: [{...}]
}
```

### Get Incident Graph
```
GET /v1/graph/incident/{incident_id}

Query params:
- include_evidence: boolean

Response:
{
  nodes: [{...}],
  edges: [{...}],
  affected_services: [string],
  primary_service: string
}
```

## Graph Maintenance

- Nodes created automatically on first event with given id
- Edges created based on event relationships (trace_id, incident_id, deployment order)
- Edge weights updated based on evidence and feedback
- Incident nodes created by detector; incident_id assigned to events

## Deterministic Properties

- Graph is acyclic DAG (directed acyclic graph)
- Causality edges always respect timestamp ordering
- Same input events always produce same graph
- Graph transitive closure is computed for RCA
