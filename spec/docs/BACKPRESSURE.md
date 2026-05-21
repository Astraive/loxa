# Collector Backpressure Contract

Canonical ingest response statuses:

- `200`: accepted and synchronously written.
- `202`: accepted and queued/stored.
- `207`: partial success.
- `400`: invalid request shape.
- `401`: authentication failed.
- `403`: forbidden by authorization policy.
- `409`: duplicate conflict or idempotency conflict.
- `413`: payload too large.
- `422`: semantic validation failed.
- `429`: backpressure or rate limit.
- `503`: temporarily unavailable.

For `429` and `503`, collectors SHOULD return:

```json
{
  "retry_after_ms": 1000,
  "reason": "queue_full"
}
```

SDKs MUST honor `retry_after_ms` when retrying retryable collector responses.

