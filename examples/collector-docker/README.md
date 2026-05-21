# Collector Docker Example

Runs the LOXA collector with DuckDB storage and optional Grafana dashboards.

## Usage

```bash
docker compose up -d
```

## Verify

```bash
# Health check
curl http://localhost:9090/healthz

# Emit a test event
curl -X POST http://localhost:9090/v1/events \
  -H "Content-Type: application/json" \
  -d '{"events":[{"event_id":"test-001","event_type":"test.hello","timestamp":"2026-01-01T00:00:00Z","service":{"name":"test","version":"1.0.0"},"deployment":{"environment":"dev"}}]}'

# Query events
curl http://localhost:9090/v1/events?limit=10
```

## Grafana

Open http://localhost:3000 (anonymous access enabled).
