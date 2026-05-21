# LOXA Go SDK

LOXA-Go is the Go SDK for creating canonical LOXA wide events.

Core lifecycle:

- `StartEvent`
- `Enrich`
- `Append`
- `Set`
- `Merge`
- `Delete`
- `Checkpoint`
- `Finish`
- `FinishError`
- `Emit`
- `Flush`
- `Shutdown`

See `spec/` for the shared contract and `collector/` for the runtime ingest service.
