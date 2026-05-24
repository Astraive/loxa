# Ingest API

Collector endpoints:

- `POST /events`
- `POST /events/batch`
- `POST /events/ndjson`
- `POST /ingest`
- `POST /query`
- `POST /query`
- `GET /status`
- `GET /sinks`
- `GET /dlq`
- `GET /healthz`
- `GET /health`
- `GET /readyz`
- `GET /ready`
- `GET /metrics`

Supported `POST /ingest` payload shapes:

- single JSON object
- JSON array
- wrapped payload: `{ "events": [...] }`
- NDJSON
- gzip-compressed variants of the above

Canonical response:

```json
{
  "request_id": "ing_1770000000000000000",
  "status": "partial",
  "accepted": 99,
  "rejected": 1,
  "duplicates": 0,
  "errors": [
    {
      "index": 5,
      "event_id": "evt_...",
      "code": "schema_invalid",
      "message": "field service.name is required",
      "retryable": false
    }
  ]
}
```

Compatibility response fields `invalid`, `deduped`, and `acks` remain allowed.
Backpressure responses use `retry_after_ms` and `reason`; see `BACKPRESSURE.md`.
