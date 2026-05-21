# Public API

Cross-language stable-v1 parity is defined by `docs/sdk-parity-manifest.json`.
This document describes the stable lifecycle-facing Go surface that must stay in sync with that manifest.

Primary lifecycle APIs:

- `StartEvent`
- `StartHTTPEvent`
- `StartJobEvent`
- `StartQueueEvent`
- `StartCLIEvent`
- `StartCronEvent`
- `StartJob`
- `StartQueueJob`
- `StartCron`
- `Enrich`
- `Append`
- `Set`
- `Merge`
- `Delete`
- `Get`
- `GetGroup`
- `Checkpoint`
- `Finish`
- `FinishError`
- `Emit`
- `EmitEvent`
- `Flush`
- `Shutdown`

The root package also exposes config helpers, built-in schemas, core sinks, samplers, redactors, and HTTP client instrumentation.
