# CORTEX Architecture

## System Model

```
┌─ Collectors ─────────────────┐
│ Loxa Collector              │
│ OTel Receiver               │
│ Log Shipper                 │
└──────────────┬──────────────┘
               │
        ┌──────▼─────────┐
        │ Cortex Ingest  │
        │ (HTTP/gRPC)    │
        └──────┬─────────┘
               │
        ┌──────▼──────────────┐
        │ Event Normalization │
        │ & Validation        │
        └──────┬──────────────┘
               │
        ┌──────▼──────────────┐
        │ Event Processor     │
        │ - Validation        │
        │ - Storage           │
        │ - Graph Build       │
        └──────┬──────────────┘
               │
      ┌────────┴────────┬─────────────┐
      │                 │             │
  ┌───▼────┐    ┌──────▼─────┐   ┌──▼────────┐
  │ Storage │    │ Graph DBs  │   │ Detector  │
  │ (Events)│    │ (Service,  │   │ (Pattern  │
  │         │    │  Incident) │   │  Matching)│
  └────────┘    └────────────┘   └──┬────────┘
                                     │
                            ┌────────▼────────┐
                            │ Reconstructor   │
                            │ (Timeline, RCA) │
                            └────────┬────────┘
                                     │
                            ┌────────▼────────┐
                            │ Feedback Loop   │
                            │ (Learning)      │
                            └─────────────────┘
```

## Components

### 1. Ingest Layer
- HTTP endpoint: `POST /v1/events`, `/v1/events/batch`, `/v1/events/jsonl`
- gRPC: `CortexService.IngestEvent`, `IngestBatch`
- Validates incoming data against cortex-event schema
- Applies strict mode for collector data, loose mode for raw logs

### 2. Event Processor
- Normalizes events (canonicalize fields, apply aliases)
- Validates required fields and enums
- Assigns canonical event_id if missing
- Stores event in event table
- Creates graph nodes for service, event, trace

### 3. Graph Builder
- Creates service dependency graph
- Links events by trace_id (same trace)
- Links events by incident_id (same incident)
- Links services by deployment ordering
- Links events by causality signals (error rate spikes, etc.)

### 4. Reconstructor
- Builds incident timeline from events
- Performs graph traversal to find root causes
- Computes confidence scores
- Generates affected services list
- Returns reconstruction response with evidence

### 5. Feedback Loop
- Records remediation outcomes
- Updates event confidence based on feedback
- Learns signature patterns from successful remediations
- Updates edge weights based on causality effectiveness

## Data Model

### Events
- Canonical event schema (cortex-event.schema.json)
- Stored in events table
- Indexed by: event_id, trace_id, incident_id, service, timestamp

### Graph Nodes
- Types: service, event, trace, span, request, deployment, metric, log, incident, remediation, resource
- Stored in graph_nodes table
- Indexed by: id, type, service

### Graph Edges
- Types: depends_on, same_trace, same_incident, parent_span, calls, deployed_before, metric_spiked_after, log_error_after, caused_probably, remediated_by, similar_shape
- Stored in graph_edges table
- Weighted by confidence/evidence

## Request/Response Flows

### Ingest Single Event
```
POST /v1/events
{
  schema_version, event_version, event_id, timestamp, service, event, kind, ...
}
→ Validate (cortex-event schema)
→ Store (events table)
→ Create nodes (service, event)
→ Return: { event_id, status, incident_id }
```

### Ingest Batch
```
POST /v1/events/batch
[{event1}, {event2}, ...]
→ Validate each event
→ Store all
→ Link by trace_id
→ Return: { accepted, rejected, errors }
```

### Reconstruct
```
POST /v1/incidents/{incident_id}/reconstruct
→ Query events by incident_id
→ Build timeline by timestamp
→ Graph traversal for causality
→ Compute root causes
→ Return reconstruction response with nodes, edges, evidence
```

### Record Feedback
```
POST /v1/feedback/remediation
{
  incident_id, remediation_id, success, notes, time_to_resolve_seconds
}
→ Store feedback
→ Update edge weights
→ Learn patterns
→ Return: { status }
```

## Determinism Guarantees

1. **Same Events → Same Graph**: Idempotent operations; repeated ingest produces no new nodes
2. **Same Graph → Same Reconstruction**: Reconstruction is deterministic given a graph snapshot
3. **Reproducible Causality**: Edge weights and scores deterministic from incident data
4. **Audit Trail**: All decisions logged and refeedback-traceable

## Extensibility

- New event kinds can be added to cortex-event schema
- New graph node/edge types can be added with migration
- New detector patterns can be added without schema change
- New storage backends can implement storage interfaces
