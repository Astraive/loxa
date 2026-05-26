# Release Process

## Versioning

Cortex follows semantic versioning. The current stable release is **v0.2.0**.

## Pre-release Checklist

1. Run the full test suite:

```bash
cd cortex
go test ./... -race -count=1
go vet ./...
```

2. Run the conformance suite:

```bash
go test ./conformance/... -v
```

3. Verify Rust FFI crate builds:

```bash
cd crates/cortex-match
cargo build --release
cargo test
```

4. Verify Docker build:

```bash
docker build -t loxa-cortex:$(git describe --tags --always) .
```

5. Update `CHANGELOG.md` with the new version and date.

6. Verify the shared DuckDB schema is compatible with the collector version being released.

## Build

```bash
cd cortex
go build -o cortex.exe ./cmd/server
```

## Tagging

```bash
git tag -a cortex/v0.2.0 -m "cortex v0.2.0"
git push origin cortex/v0.2.0
```

## Docker Release

```bash
cd cortex/configs
docker build -t ghcr.io/astraive/loxa-cortex:0.2.0 ..
docker push ghcr.io/astraive/loxa-cortex:0.2.0
```

## Kubernetes Deployment

Apply the manifests in `configs/`:

```bash
kubectl apply -f configs/cortex-configmap.yaml
kubectl apply -f configs/cortex-deployment.yaml
kubectl apply -f configs/cortex-service.yaml
```

Or use the Helm chart from `../deploy/helm/cortex`.

## Post-release

1. Verify the deployed `/healthz` and `/readyz` endpoints respond correctly.
2. Run a smoke test: ingest an event, reconstruct it, query the graph.
3. Confirm Prometheus metrics are being scraped.
4. Tag the main branch commit with the release version.
