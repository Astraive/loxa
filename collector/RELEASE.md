# Release Process

## Versioning

The collector follows semantic versioning. The current stable release is **v0.0.1**.

## Pre-release Checklist

1. Run the full test suite with race detection:

```bash
cd collector
go test ./... -race -count=1
go vet ./...
```

2. Run the sink conformance suite:

```bash
go test ./internal/sinks/conformance/... -v
```

3. Run benchmarks and verify no regressions:

```bash
go test ./bench/... -bench=. -benchmem
```

4. Verify Docker build:

```bash
docker build -t loxa:$(git describe --tags --always) .
```

5. Update `CHANGELOG.md` with the new version and date.

6. Verify config defaults file (`loxa-collector.defaults.yaml`) is up to date.

## Build

```bash
cd collector
go build -o loxa-collector.exe ./cmd/loxa-collector
go build -o loxa-worker.exe ./cmd/loxa-worker
go build -o loxa-loadgen.exe ./cmd/loxa-loadgen
```

## Tagging

```bash
git tag -a collector/v0.0.1 -m "collector v0.0.1"
git push origin collector/v0.0.1
```

## Docker Release

The Dockerfile expects the build context to be the **repo root** (not `collector/`):

```bash
# Build from repo root (required for proto file access)
docker build -t ghcr.io/astraive/loxa:0.0.1 -f collector/Dockerfile .

# Or via docker-compose (already sets context correctly)
docker compose -f collector/deploy/docker-compose.yml build

docker push ghcr.io/astraive/loxa:0.0.1
```

## Kubernetes Deployment

Apply the Helm chart:

```bash
helm install loxa-collector ../deploy/helm/collector
```

Or apply raw manifests from `deploy/`.

## Post-release

1. Verify `/healthz` and `/readyz` respond on the deployed instance.
2. Send a test event through the ingest endpoint and confirm delivery to sinks.
3. Check DLQ is empty and no alerts are firing.
4. Verify Cortex is receiving events via the gRPC push sink.
