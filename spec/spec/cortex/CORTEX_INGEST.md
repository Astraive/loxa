# CORTEX Ingest API

## HTTP Endpoints

### Single Event Ingest
```
POST /v1/events
Content-Type: application/json

{schema_version, event_version, event_id, timestamp, service, event, kind, ...}

Response 202:
{
  status: "accepted" | "invalid" | "duplicate",
  event_id: string,
  incident_id?: string
}

Response 400 (validation error):
{
  errors: [{field, code, message}]
}
```

### Batch Ingest
```
POST /v1/events/batch
Content-Type: application/json

[{event1}, {event2}, ...]

Response 202:
{
  status: "accepted" | "partial" | "rejected",
  accepted: integer,
  rejected: integer,
  invalid: integer,
  errors: [{event_id, code, message}]
}
```

### JSONL Ingest
```
POST /v1/events/jsonl
Content-Type: application/x-ndjson

{event1}\n
{event2}\n
{event3}\n

Response 202:
{
  status: "accepted" | "partial",
  accepted: integer,
  rejected: integer,
  errors: [{line, code, message}]
}
```

## gRPC Endpoints

### IngestEvent
```proto
rpc IngestEvent(IngestEventRequest) returns (IngestEventResponse);

message IngestEventRequest {
  CortexEvent event = 1;
}

message IngestEventResponse {
  string status = 1;
  string event_id = 2;
  string incident_id = 3;
}
```

### IngestBatch
```proto
rpc IngestBatch(IngestBatchRequest) returns (IngestBatchResponse);

message IngestBatchRequest {
  repeated CortexEvent events = 1;
}

message IngestBatchResponse {
  string status = 1;
  int32 accepted = 2;
  int32 rejected = 3;
  int32 invalid = 4;
  repeated CortexIngestError errors = 5;
}

message CortexIngestError {
  string event_id = 1;
  string code = 2;
  string message = 3;
  bool retryable = 4;
}
```

## Validation

- All events validated against cortex-event schema
- Required fields: event_id, timestamp, service, event, kind
- Enum values validated
- Status codes validated (100-599)
- Timestamps validated (RFC3339)
- Deduplicated by event_id (within 24h window)

## Rate Limiting

- Default: 10,000 events/sec per service
- Configurable per API key
- Returns 429 if exceeded

## Retry Behavior

- 5xx responses: retryable
- Validation errors (4xx): non-retryable
- Duplicate event (event_id exists): returns 202 (idempotent)
