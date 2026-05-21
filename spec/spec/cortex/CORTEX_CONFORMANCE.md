# CORTEX Conformance

## Test Fixtures

All fixture files located in `fixtures/cortex/`

### Valid Fixtures

#### loxa_event_ingest.json
Single Loxa event ingested successfully

#### cortex_event_ingest.json
Single Cortex event ingested successfully

#### batch_ingest.json
Batch of events ingested successfully

#### reconstruct_response_fast.json
Fast mode reconstruction returns valid response

#### graph_response.json
Service graph query returns valid node/edge set

#### feedback_success.json
Remediation feedback recorded successfully

### Invalid Fixtures

#### missing_event_id.json
Event without event_id field rejected

#### missing_service.json
Event without service field rejected

#### bad_timestamp.json
Event with invalid RFC3339 timestamp rejected

#### invalid_kind.json
Event with unknown kind enum rejected

#### invalid_graph_edge.json
Edge with unknown type enum rejected

#### invalid_feedback.json
Feedback missing required fields rejected

## Validation Rules

### Event Validation
- schema_version must exist in enums
- event_version must exist in enums
- event_id must be non-empty string
- timestamp must be valid RFC3339
- service must be non-empty string or object with name
- event must be non-empty string
- kind must be in allowed enum
- level (if present) must be in allowed enum
- outcome (if present) must be in allowed enum
- status_code (if present) must be 100-599

### Graph Validation
- Node id must be unique
- Node type must be in allowed enum
- Edge from_node_id and to_node_id must exist
- Edge type must be in allowed enum
- Edge weight must be 0-1
- No cycles allowed (DAG property)

### Reconstruction Validation
- incident_id must exist
- Must return at least primary_service
- nodes and edges must be non-empty
- confidence must be 0-1
- evidence must have confidence scores

## Conformance Runner

```
python conformance/runner.py [--manifest <path>] [--check]

--manifest: Path to conformance manifest (default: conformance/manifest.json)
--check: Verify fixtures pass without updating

Output:
  ✓ <fixture>: PASS
  ✗ <fixture>: FAIL (reason)
  
Summary:
  123 passed, 0 failed
```

## CI Integration

```yaml
cortex-conformance:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v3
    - run: python conformance/runner.py --check
```

## Fixture Format

Each fixture is a JSON file containing:

```
{
  "description": "What this tests",
  "mode": "strict" | "loose",
  "input": {...},
  "expected_status": 202 | 400 | 422,
  "expected_errors"?: [
    {field, code, message}
  ]
}
```
