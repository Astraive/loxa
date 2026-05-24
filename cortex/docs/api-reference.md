# API Reference

Cortex exposes HTTP REST, gRPC, WebSocket, and GraphQL APIs. This document covers the HTTP REST endpoints.

## Authentication

All API endpoints require authentication when `authentication.enabled` is true in the config. Authenticate by passing an API key in the `Authorization` header:

```
Authorization: Bearer <api-key>
```

The `/healthz`, `/readyz`, and `/metrics` endpoints do not require authentication.

## Rate Limiting

When `rate_limit.enabled` is true, requests are throttled per API key and per IP address. Limits are configured via `rate_limit.per_api_key_rpm` and `rate_limit.per_ip_rpm`.

## Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/healthz` | GET | Liveness probe |
| `/readyz` | GET | Readiness probe |
| `/metrics` | GET | Prometheus metrics |
| `/events` | POST | Ingest a single event |
| `/events/batch` | POST | Ingest a batch of events |
| `/events/jsonl` | POST | Ingest NDJSON stream |
| `/reconstruct` | POST | Reconstruct incident context |
| `/incidents/{incident_id}/reconstruct` | POST | Reconstruct by incident ID |
| `/graph/service/{service}` | GET | Get service graph neighborhood |
| `/graph/incident/{incident_id}` | GET | Get incident graph |
| `/feedback/remediation` | POST | Record remediation feedback |
| `/feedback/incident` | POST | Record incident feedback |
| `/graphql` | POST | GraphQL query endpoint |
| `/ws` | GET | WebSocket live event stream |

---

### GET /healthz

Liveness probe. Returns 200 OK if the process is running.

**Response**

```
200 OK
OK
```

---

### GET /readyz

Readiness probe. Returns 200 OK if the server is ready to accept traffic, 503 if not.

**Response**

```
200 OK
OK
```

```
503 Service Unavailable
NOT READY
```

---

### GET /metrics

Prometheus metrics endpoint. Returns metrics in Prometheus exposition format.

**Response**

```
200 OK
Content-Type: text/plain; version=0.0.4; charset=utf-8
```

---

### POST /events

Ingest a single event.

**Request Body**

```json
{
  "id": "evt-001",
  "event_type": "http_request",
  "service": "payment-service",
  "timestamp": "2026-05-20T10:00:00Z",
  "trace_id": "abc123",
  "attributes": {
    "status_code": 500,
    "path": "/api/charge",
    "latency_ms": 4500
  }
}
```

**Response**

```json
{
  "status": "accepted"
}
```

| Status | Description |
|---|---|
| 202 Accepted | Event accepted for processing |
| 400 Bad Request | Invalid event payload |
| 401 Unauthorized | Missing or invalid API key |
| 429 Too Many Requests | Rate limit exceeded |

---

### POST /events/batch

Ingest a batch of events.

**Request Body**

```json
{
  "events": [
    {
      "id": "evt-001",
      "event_type": "http_request",
      "service": "payment-service",
      "timestamp": "2026-05-20T10:00:00Z"
    },
    {
      "id": "evt-002",
      "event_type": "http_request",
      "service": "auth-service",
      "timestamp": "2026-05-20T10:00:01Z"
    }
  ]
}
```

**Response**

```json
{
  "status": "accepted"
}
```

| Status | Description |
|---|---|
| 202 Accepted | Batch accepted for processing |
| 400 Bad Request | Invalid batch payload |
| 401 Unauthorized | Missing or invalid API key |
| 429 Too Many Requests | Rate limit exceeded |

---

### POST /events/jsonl

Ingest events in NDJSON format (one JSON object per line).

**Request Body**

```
{"id":"evt-001","event_type":"http_request","service":"payment-service","timestamp":"2026-05-20T10:00:00Z"}
{"id":"evt-002","event_type":"http_request","service":"auth-service","timestamp":"2026-05-20T10:00:01Z"}
```

**Response**

```json
{
  "status": "accepted",
  "count": 2
}
```

---

### POST /reconstruct

Reconstruct an incident context from stored events.

**Request Body**

```json
{
  "incident_id": "inc-001",
  "mode": "fast"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| incident_id | string | Yes | The incident identifier |
| mode | string | No | Reconstruction mode: `fast` or `deep`. Default: `fast` |

**Response**

```json
{
  "incident_id": "inc-001",
  "mode": "fast",
  "causal_chain": [
    {
      "event_id": "evt-001",
      "event_type": "deploy",
      "service": "payment-service",
      "timestamp": "2026-05-20T09:55:00Z",
      "edges": [
        {"to": "evt-002", "type": "triggered", "confidence": 0.9}
      ]
    }
  ],
  "symptoms": ["5xx_spike", "timeout", "connection_pool_exhausted"],
  "similar_incidents": [
    {
      "incident_id": "inc-087",
      "similarity": 0.85,
      "matched_symptoms": ["5xx_spike", "timeout"],
      "matched_services": ["payment-service"]
    }
  ],
  "suggested_remediations": [
    {
      "action": "rollback_deploy",
      "confidence": 0.9,
      "success_rate": 0.85,
      "historical_count": 12
    }
  ],
  "confidence": 0.82,
  "elapsed_ms": 45
}
```

---

### POST /incidents/{incident_id}/reconstruct

Reconstruct an incident by path parameter.

**Path Parameters**

| Parameter | Type | Description |
|---|---|---|
| incident_id | string | The incident identifier |

**Query Parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| mode | string | fast | Reconstruction mode: `fast` or `deep` |

**Response** -- same as `POST /reconstruct`.

---

### GET /graph/service/{service}

Get the service graph neighborhood for a given service.

**Path Parameters**

| Parameter | Type | Description |
|---|---|---|
| service | string | The service name |

**Query Parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| depth | int | 3 | Graph traversal depth |

**Response**

```json
{
  "service": "payment-service",
  "nodes": [
    {"id": "payment-service", "type": "service", "label": "payment-service"},
    {"id": "auth-service", "type": "service", "label": "auth-service"}
  ],
  "edges": [
    {"from": "payment-service", "to": "auth-service", "type": "calls", "weight": 0.95}
  ]
}
```

---

### GET /graph/incident/{incident_id}

Get the incident graph showing events and their relationships.

**Path Parameters**

| Parameter | Type | Description |
|---|---|---|
| incident_id | string | The incident identifier |

**Response**

```json
{
  "incident_id": "inc-001",
  "nodes": [
    {"id": "evt-001", "type": "event", "event_type": "deploy"},
    {"id": "evt-002", "type": "event", "event_type": "http_request"}
  ],
  "edges": [
    {"from": "evt-001", "to": "evt-002", "type": "triggered", "confidence": 0.9}
  ]
}
```

---

### POST /feedback/remediation

Record operator feedback on a remediation suggestion.

**Request Body**

```json
{
  "incident_id": "inc-001",
  "action": "rollback_deploy",
  "outcome": "resolved",
  "confidence": 0.9
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| incident_id | string | Yes | The incident identifier |
| action | string | Yes | The remediation action taken |
| outcome | string | Yes | One of: `resolved`, `partially_resolved`, `not_resolved`, `worsened` |
| confidence | float | No | Operator confidence in the outcome (0.0 -- 1.0) |

**Response**

```json
{
  "status": "recorded"
}
```

---

### POST /feedback/incident

Record feedback on an incident reconstruction.

**Request Body**

```json
{
  "incident_id": "inc-001",
  "reconstruction_quality": "good",
  "comments": "Root cause was correctly identified"
}
```

**Response**

```json
{
  "status": "recorded"
}
```

---

### POST /graphql

GraphQL endpoint for flexible queries. Supports querying incidents, events, signatures, graph, and remediations.

**Example Query**

```graphql
query {
  incident(id: "inc-001") {
    id
    symptoms
    similarIncidents {
      id
      similarity
    }
    suggestedRemediations {
      action
      confidence
    }
  }
}
```

---

### GET /ws

WebSocket endpoint for live event streaming. Connect and send a subscription message to receive events in real time.

**Connection**

```
ws://localhost:9091/ws
```

**Subscribe Message**

```json
{
  "type": "subscribe",
  "filter": {
    "services": ["payment-service"],
    "event_types": ["http_request"]
  }
}
```

**Event Message**

```json
{
  "type": "event",
  "data": {
    "id": "evt-001",
    "event_type": "http_request",
    "service": "payment-service",
    "timestamp": "2026-05-20T10:00:00Z"
  }
}
```
