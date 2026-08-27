# LOZA configuration reference

## Configuration discovery

The Collector and CLI commonly discover `loza.yaml` in the current directory, parent directories, or the executable directory. An environment override such as `LOZA_COLLECTOR_DEFAULTS` may select an alternative defaults file. Confirm discovery with the installed release's `loza config print`; never assume the working directory in a service manager or container.

## Precedence

Unless the release README says otherwise, use this precedence:

1. Explicit code or command-line options.
2. Environment variables, especially secret variables.
3. Explicit configuration file.
4. Release defaults.

Verify the effective result with `loza config print` and validate it with `loza config validate` before starting the service.

## Secure baseline

```yaml
collector:
  addr: ":9308"
  read_header_timeout: 5s
  shutdown_timeout: 10s
  max_body_bytes: 10485760
  max_events_per_request: 5000

auth:
  enabled: true
  server_secret: "${COLLECTOR_AUTH_SERVER_SECRET}"
  cache_ttl: 5m
  negative_cache_ttl: 30s

storage:
  primary: duckdb
  encryption_key_env: LOZA_STORAGE_ENCRYPTION_KEY

duckdb:
  path: /var/lib/loza/duckdb/loza.db
  checkpoint_on_shutdown: true

rate_limit:
  enabled: true
  rps: 100
  burst: 200

privacy:
  mode: enforce
```

Field names and supported sections are release-specific. The example is a secure contract pattern, not permission to copy unsupported fields into an older image.

## Collector-managed database connections

The `0.4.0` Collector keeps database credentials server-side. Configure named
connections under `database.connections` and select the primary with
`storage.connection`; supported backends are `duckdb`, `postgres`, and
`clickhouse`.

```yaml
storage:
  primary: postgres
  connection: analytics

database:
  connections:
    - name: analytics
      type: postgres
      enabled: true
      host: postgres.internal
      port: 5432
      database: loza
      username_env: LOZA_PG_USER
      password_env: LOZA_PG_PASSWORD
      ssl_mode: verify-full
      table: events
      raw_column: raw
      connection_timeout: 5s
      query_timeout: 10s
```

DuckDB uses a server-managed `path` and optional `driver`; it is not a TCP
listener. PostgreSQL uses `host`/`port` and ClickHouse uses a `hosts`
(`host:port`) list. Never put credentials or DSNs in browser state.

Authenticated clients use `GET /collectors/{collector}/database/connections`,
`POST /collectors/{collector}/database/connections/{name}/test`, and
`POST /collectors/{collector}/database/query` with `events:read`. The query
body contains a configured `connection` name and LQL source; arbitrary SQL,
database targets, credentials, and direct browser database sockets are
rejected.

## Independent secrets

`auth.server_secret` protects key-record hashing. `storage.encryption_key_env` protects raw event persistence. They are different secrets and must not be reused. Keep API key secrets, DSNs, database passwords, and encryption keys in a secret manager or protected environment—not committed YAML, command arguments, URLs, or logs.

## Scoped credentials

A scoped key record normally identifies:

- `key_id` and a secret environment variable;
- `kind` (`sec` for server, `pub` for browser/client where supported);
- `mode` (`private` or `public`);
- an explicit `collector`;
- exact `permissions`, such as `events:read` or `events:write`;
- allowed environments and, for public credentials, allowed origins.

Use separate write, read, delete, schema/admin, and service-to-service credentials. Configure `default_collector` only when legacy unscoped root routes are intentionally required; prefer `/collectors/{collector}/...` for data-plane access.

## Operational limits

Set bounded body/event sizes, header and idle timeouts, queue/spool limits, retry/backoff, retention, and rate limits. A full disk, spool, queue, or sink failure must become an explicit readiness or ingestion failure; it must not silently drop events. Restart after configuration changes unless the installed release explicitly documents safe hot reload.
