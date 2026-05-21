# LOXA Deployment Guide

This guide documents the deployment paths that are implemented in this repo today.

## Local Development

Run the collector directly:

```bash
cd collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

Run the worker when you are testing queue mode:

```bash
cd collector
go run ./cmd/loxa-worker run
```

Run the CLI against the same workspace:

```bash
cd cli
go run ./cmd/loxa query --sql "SELECT * FROM events LIMIT 10"
```

## Packaged Assets

Current deployment assets are split by ownership:

- Collector runtime assets live under [collector/deploy](collector/deploy)
- `schema-service` and `stager` container and Helm assets live under [spec](spec), with the canonical chart at [spec/charts/loxa](spec/charts/loxa)
- Root-level [docker-compose.yml](docker-compose.yml) is the local Kafka/Redis/schema-service/stager integration stack

Treat `spec/deploy/k8s` as thin example manifests that mirror the chart-managed deploy path.

## Health and Metrics

Current HTTP endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `GET /v1/status`

## Authentication

Current enforced HTTP auth is API-key based.

Example:

```yaml
auth:
  enabled: true
  header: X-API-Key
  value_env: COLLECTOR_API_KEY
```

JWT and mTLS appear in configuration and backlog material, but they are not the current HTTP ingest contract to rely on for release usage.

## Reliability Modes

The collector currently supports:

- `direct`
- `spool`
- `queue`

Do not assume `hybrid` mode is available in the current release.

## Compression

Current ingest support:

- gzip-compressed request bodies

Do not assume zstd or compressed responses are available in the current release.

## Configuration Reload

The current release should be operated with restart-based config changes.

Do not assume SIGHUP hot reload is available.
