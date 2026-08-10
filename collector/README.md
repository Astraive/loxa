# LOZA Collector

LOZA Collector is the runtime repository for LOZA ingestion, validation, durability, fanout, worker processing, and deployment assets.

Key binaries:

- `loza-collector`: HTTP ingest server
- `loza-worker`: queue consumer for distributed delivery
- `loza-loadgen`: local load generator

Contract and SDK:

- event contract: `../spec`
- application SDKs emit canonical wide events to the collector over documented ingest protocols
- operations CLI: `../cli`

Local run example:

```bash
go run ./cmd/loza-collector run -c configs/loza.local.yaml
```
