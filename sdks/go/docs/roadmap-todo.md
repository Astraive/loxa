# LOZA Program TODO (P0-P17)

Status key: `[x]` done, `[-]` in progress, `[ ]` planned.

- [x] `P0` Tests, docs, API stability baseline
- [x] `P1` Config-first collector (`run/config print/config validate`, YAML precedence)
- [x] `P2` Collector reliability (`direct/spool`, retry, DLQ, dedupe, composite readiness)
- [-] `P3` DuckDB production hardening + benchmark matrix
- [-] `P4` SDK ergonomics/shutdown/context helpers/strict mode
- [-] `P5` Middleware hardening + framework docs
- [-] `P6` OTLP/Loki/Prometheus/Grafana integrations
- [x] `P7` Multi-sink fanout delivery policies
- [ ] `P8` Durable queue mode + `loza-worker`
- [ ] `P9` ClickHouse/S3/GCS/Parquet strategy
- [ ] `P10` Schema governance and validation modes
- [ ] `P11` Security/privacy/tenant controls
- [ ] `P12` Deployment packaging (images/helm/k8s)
- [ ] `P13` Top-level `loza` CLI
- [ ] `P14` DX examples, recipes, migration guides
- [ ] `P15` Performance/load tooling and reports
- [ ] `P16` LOZA self-observability
- [ ] `P17` Packaging/versioning compatibility contracts

## Current Sprint Focus

1. Drive `P3` DuckDB hardening and benchmark matrix to completion.
2. Land `P4` SDK ergonomics (shutdown/context helpers, strict mode) with coverage.
3. Harden `P5` middleware and publish framework-facing docs.
4. Advance `P6` OTLP/Loki/Prometheus/Grafana integrations with runnable examples.
