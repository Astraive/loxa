# LOZA CLI user reference

Use the released `loza` binary that matches the Collector/Cortex release. Start with `loza --help` and `loza <command> --help`; flags and experimental commands can change between releases.

## Stable command groups

```text
loza version
loza maturity
loza init
loza config print|validate
loza emit sample
loza query
loza tail
loza watch
loza status
loza sinks list|show|test
loza dlq list|show|delete|replay|replay-all
loza quarantine list|replay|delete
loza replay
loza delete tenant|user|event
loza audit pii
loza schema validate|fetch|list|diff|publish
loza export
loza keys create|revoke|rotate
loza incident
loza graph service|incident
loza signatures
loza doctor
```

Cortex-specific groups may include `loza cortex run|ingest|reconstruct|similar|remediation|feedback|graph`. Treat `deploy`, `dashboard`, `debug`, `bench`, and other marked experimental surfaces as release-dependent.

## Safe query workflow

```bash
loza status
loza query --engine duckdb \
  --format table \
  --limit 100 \
  --param service=checkout \
  -q 'from events where service = $service and level = "error" limit 100'
```

Use a time window, service/event filter, and explicit limit. Prefer typed `--param key=value` values where supported. Use `--raw-sql` only when the selected engine and installed release explicitly support it; never concatenate untrusted user input into SQL.

## Tail and watch

```bash
loza tail --service checkout --level error
loza watch --service checkout --kind http
```

Scope live streams with service, kind, level, trace, incident, cursor, or a bounded limit where supported. Stop an intentionally unbounded stream when diagnosis is complete.

## Destructive and administrative commands

`loza delete`, key rotation/revocation, schema publication, export, DLQ deletion, and quarantine deletion are privileged operations. Preview scope, use a separate admin credential, supply an audit reason where supported, and verify the Collector/environment before execution. Do not print credentials or raw sensitive event bodies into terminal captures.

## Output modes

Use `--output=json` or `--format json` for automation only after checking the installed command's syntax. Keep machine-readable output free of tokens and redact raw payloads before forwarding results to tickets or chat.
