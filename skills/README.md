# LOZA skills knowledge base

These Markdown files are release-user guidance for agents working with an installed LOZA release. They are not implementation instructions for changing the repository.

## Load order

1. Read `loza.md` to choose the component and establish the release boundary.
2. Read the focused file for the task:
   - `loza-getting-started.md` — first installation and first event.
   - `loza-configuration.md` — config discovery, precedence, and durable runtime settings.
   - `loza-authentication.md` — API keys, bearer tokens, DSNs, RBAC, and scope.
   - `loza-sdk.md` — cross-language instrumentation and lifecycle semantics.
   - `loza-cli.md` — released CLI commands and safe command usage.
   - `loza-lql.md` — bounded LQL queries and query safety.
   - `loza-operations.md` — health, readiness, sinks, spool, DLQ, quarantine, replay, and deletion.
   - `loza-troubleshooting.md` — evidence-first diagnosis order.
   - `loza-architecture.md` — component boundaries, data flow, and sink/reliability semantics.
   - `loza-glossary.md` — shared LOZA terminology.
   - `loza-migration.md` — release upgrade and rollback checklist.
3. Read `loza-collector.md` or `loza-cortex.md` for component-specific deployment and API details.

## Authority and freshness

Prefer the installed release's `README`, `--help`, config schema, `/version`, and release notes over repository source, roadmap documents, or examples copied from an older release. If a route, field, flag, or SDK helper is not confirmed for the installed artifact, label it as release-dependent instead of inventing a fallback.

The repository's source documentation remains the source material for these notes:

- `loza/docs/getting-started.md`
- `loza/docs/configuration.md`
- `loza/docs/authentication.md`
- `loza/docs/authorization.md`
- `loza/docs/instrumentation.md`
- `loza/docs/collector-operations.md`
- `loza/docs/RELEASE_GUIDE.md`

Never place real credentials, raw event payloads, or customer PII in examples or generated answers.
