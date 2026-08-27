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

Named database connections are configured under `database.connections`. Use
`storage.connection` to select a named DuckDB, PostgreSQL, or ClickHouse
primary. DuckDB uses a server-side file path; PostgreSQL uses server-side
host/port credentials; ClickHouse uses server-side `hosts`. Keep usernames and
passwords in environment variables referenced by `username_env` and
`password_env`. The browser must use Collector HTTP and never opens a database
socket.

Focused verification:

```bash
go test ./internal/config ./internal/database ./server/http ./cmd/loza-collector ./internal/sinks/...
```

Integration environments provide `LOZA_PG_USER`, `LOZA_PG_PASSWORD`, and
ClickHouse credentials without committing them to YAML.
