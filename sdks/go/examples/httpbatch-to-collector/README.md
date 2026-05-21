# HTTP Batch To Collector

Run the collector:

```bash
cd ../loxa-collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

Run a Go app that emits with the HTTP batch sink:

```bash
cd ../loxa-go/examples/httpbatch-to-collector
go run .
```

Use this example to validate:

- `loxa-go` emits spec-compatible JSON
- `loxa-collector` accepts the public ingest format
