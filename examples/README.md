# Examples

Cross-component examples for the LOZA monorepo.

| Example | Languages | Description |
|---------|-----------|-------------|
| [quickstart/](quickstart/) | Go, Python, Rust, JS | Minimal SDK usage: configure, emit, shutdown |
| [collector-docker/](collector-docker/) | Docker Compose | Collector with DuckDB storage |
| [full-stack/](full-stack/) | Docker Compose | Collector + Grafana + Prometheus observability stack |

## Running Quickstart Examples

Each quickstart example requires a running collector:

```bash
cd collector && go run ./cmd/loza-collector run -c configs/loza.local.yaml
```

Then run the example for your SDK:

```bash
cd examples/quickstart/go && go run main.go
cd examples/quickstart/py && python main.py
cd examples/quickstart/rs && cargo run
cd examples/quickstart/js && bun src/main.ts
```
