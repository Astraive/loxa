# LOZA release migration checklist

Use this checklist when moving Collector, Cortex, CLI, or SDKs between releases.

## Before the change

- Pin and record the old and target versions, image digests, SDK package versions, and LQL version.
- Read target release notes, migration guide, config schema, and API changes.
- Back up DuckDB, spool, DLQ, quarantine, schema, and Cortex state according to the deployment's recovery plan.
- Export effective configuration without secrets and record credential scopes.
- Confirm the target supports the current event schema and storage format.

## Stage the upgrade

1. Validate target configuration with `loza config validate`.
2. Start the target in an isolated environment with a copy of durable state where possible.
3. Check `/health`, `/readyz`, `/version`, sink health, and storage encryption-key resolution.
4. Send a disposable authenticated event using a scoped write credential.
5. Read it back using a separate scoped read credential.
6. Exercise one query, one tail, one schema validation, and one DLQ/quarantine path if deployed.
7. For Cortex, verify one ingest, reconstruction, graph lookup, and feedback request.

## Cutover

- Stop new writers or drain them according to the release's compatibility guidance.
- Keep the Collector and SDK auth/schema contract compatible during the overlap window.
- Rotate credentials only after the target accepts traffic; revoke old credentials after verification.
- Preserve collector/environment scope in every client and reverse-proxy route.
- Monitor rejection, deduplication, sink errors, retry, queue/spool, readiness, and latency metrics.

## Rollback triggers

Rollback or halt promotion when the target cannot validate existing events, cannot decrypt durable state, loses collector scope, returns unexpected readiness, grows its queue/spool without drain, or changes deletion/export behavior unexpectedly. Preserve diagnostic evidence before destroying the failed instance.

## Do not assume

Do not assume config fields, endpoint aliases, API-key headers, query functions, CLI flags, wire envelopes, or hot-reload semantics are backward-compatible. Verify them against the target release rather than adding compatibility shims in user guidance.
