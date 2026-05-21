# Ingest API

Collector-specific ingest behavior:

- `POST /ingest`
- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

The collector accepts single-object JSON, arrays, wrapped events, NDJSON, and gzip-compressed variants.
