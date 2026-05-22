# HTTP Batch To Collector

Run the collector:

```bash
cd collector
go run ./cmd/loxa-collector run -c configs/loxa.local.yaml
```

Run a Go app that emits with the HTTP batch sink:

```bash
cd sdks/go/examples/httpbatch-to-collector
go run .
```

Use this example to validate:

- The Go SDK emits spec-compatible JSON
- The collector accepts the public ingest format
