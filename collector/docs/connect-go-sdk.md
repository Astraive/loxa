# Connect Go SDK

Use the SDK HTTP batch sink to emit spec-compatible events to the collector:

```go
cfg := loza.Production().WithService("checkout")
cfg.Sinks = []loza.Sink{httpbatch.New(httpbatch.Config{URL: "http://localhost:9308/ingest"})}
```
