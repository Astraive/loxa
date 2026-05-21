# Connect Go SDK

Use the SDK HTTP batch sink to emit spec-compatible events to the collector:

```go
cfg := loxa.Production().WithService("checkout")
cfg.Sinks = []loxa.Sink{httpbatch.New(httpbatch.Config{URL: "http://localhost:9090/ingest"})}
```
