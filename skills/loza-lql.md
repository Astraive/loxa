# LQL user reference

LQL is a typed, Kusto-inspired query language for LOZA wide events. The compiler targets DuckDB and ClickHouse. The checked-in LQL crate is version `0.4.2`; use the installed release's grammar and `loza query --help` rather than assuming this version is deployed.

## Pipeline shape

Queries begin with `from` and compose stages with `|`:

```lql
from events | where level = "error" | summarize count() by service | sort count desc | limit 10
```

Common stages include `where`, `project`, `extend`, `summarize`, `sort`, and `limit`. Common predicates include equality, comparisons, and `contains`.

## CLI usage

Development of the LQL repository can use:

```bash
cargo run --bin lql -- compile 'from events | where duration_ms > 1000 | limit 20'
cargo run --bin lql -- compile-ch 'from events | where service contains "api" | limit 20'
cargo run --bin lql -- check 'from events | where level = "error"'
```

Release users should query through the installed Collector-facing CLI:

```bash
loza query --engine duckdb --format table --limit 100 \
  -q 'from events | where service = "checkout" and level = "error" | limit 100'
```

Use `--raw-sql` only for an explicitly supported engine and release. LQL compilation does not grant authorization; the Collector still enforces collector scope and permissions.

## Query safety

- Bound every query by time, service, event kind, incident, or another selective predicate where possible.
- Always set an explicit limit for interactive and automation queries.
- Prefer typed `--param key=value` parameters supported by the installed CLI.
- Never interpolate untrusted input into LQL or raw SQL strings.
- Do not query or export raw secrets, authorization headers, or unnecessary PII.
- Treat errors containing SQL or raw payloads as sensitive before sharing them.

## Targets and differences

DuckDB and ClickHouse differ in supported functions and type behavior. Compile/check against the target engine before shipping a query. A query accepted by the local Rust CLI is not proof that the Collector's deployed engine, schema, or release accepts it.
