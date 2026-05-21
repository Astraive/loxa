# Full Stack Example

Complete observability stack: LOXA Collector + Prometheus + Grafana.

## Components

| Component | Port | Description |
|-----------|------|-------------|
| Collector | 9090 | Event ingestion and storage |
| Prometheus | 9091 | Metrics collection |
| Grafana | 3000 | Dashboards and visualization |

## Usage

```bash
docker compose up -d
```

## Verify

```bash
# Collector health
curl http://localhost:9090/healthz

# Prometheus targets
curl http://localhost:9091/api/v1/targets

# Grafana dashboards
open http://localhost:3000
```
