# CORTEX Reconstruction

## Timeline Reconstruction

For a given incident:

1. Fetch all events with matching incident_id
2. Sort by timestamp (ascending)
3. Group by service
4. Build causal relationships using trace_id and span_id
5. Identify critical path (events that directly impact outcome)
6. Compute root cause candidates

## Reconstruction Response

See: `schemas/json/cortex-reconstruct-response.schema.json`

```
{
  incident_id: string,
  status: "open" | "investigating" | "mitigated" | "resolved" | "unknown",
  severity: "critical" | "high" | "medium" | "low" | "unknown",
  primary_service: string,
  affected_services: [string],
  nodes: [GraphNode],          // incident + service + event nodes
  edges: [GraphEdge],          // causality + dependency edges
  evidence: [                  // why we think this is root cause
    {
      event_id: string,
      reason: string,
      confidence: number [0-1]
    }
  ],
  summary: string,             // human-readable RCA summary
  confidence: number [0-1]     // overall confidence in reconstruction
}
```

## Reconstruction Modes

### Fast Mode
- Returns immediately
- Based on graph state only
- No ML model invocation
- Returns existing incident/service nodes + critical path edges

### Deep Mode
- Runs statistical analysis
- Invokes ML models for pattern matching
- Computes all transitive causality
- Takes longer but more accurate

## Root Cause Analysis (RCA)

Algorithm:
1. Build trace tree (parent_span_id relationships)
2. Identify first error (earliest timestamp with level=error)
3. Walk backwards in graph following depends_on, calls edges
4. Score each potential root cause by:
   - Temporal proximity to critical event
   - Edge weight (confidence)
   - ML pattern similarity
5. Return top N candidates sorted by confidence

## Reconstruction API

```
POST /v1/incidents/{incident_id}/reconstruct

Request:
{
  mode?: "fast" | "deep"  // default: fast
}

Response:
<ReconstructResponse as above>
```

## Determinism Guarantees

- Same incident_id + same event state → same reconstruction
- Reconstruction is idempotent (calling twice = same result)
- Evidence scores reproducible from event data
