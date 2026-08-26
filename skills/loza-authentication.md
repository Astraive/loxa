# LOZA authentication and authorization

LOZA supports scoped API keys and opaque bearer tokens through `Authorization: Bearer`. Credentialed DSNs may use HTTP Basic authentication. Use the mechanism documented by the installed release; do not infer a custom header.

## Bearer API keys

The common key shape is:

```text
lz_{kind}_{env}_{key_id}_{secret}
```

- `sec`: backend/server credential.
- `pub`: browser/client credential with origin and reduced-permission restrictions.
- `local`: local development only; never use in production.

A bearer request typically includes the collector and environment scopes:

```bash
curl --fail-with-body -X POST "$COLLECTOR_URL/collectors/checkout/events" \
  -H "Authorization: Bearer ${LOZA_INGEST_TOKEN}" \
  -H "X-Loza-Env: prod" \
  -H 'Content-Type: application/json' \
  --data '[{"event":"checkout.request","kind":"http","service":"checkout","outcome":"success"}]'
```

The exact path, environment header, token shape, and accepted permissions are release-specific. Use `loza config print`, the installed API reference, and a disposable credential to verify them.

## Credentialed DSNs

A credentialed DSN places the Collector `key_id` and secret in URL userinfo and is converted to Basic auth by the SDK. Reserved characters in userinfo must be percent-encoded. DSN passwords must never appear in resolved URLs, debug output, logs, shell history, or committed configuration. Plain HTTP with credentials should be rejected except for explicitly local/insecure development.

## Scope and roles

Grant the minimum exact permission needed:

- `events:write` for ingestion;
- `events:read` for query, status, or tail access;
- `events:delete` for destructive deletion workflows;
- `schema:write` or `project:admin` for schema/admin operations;
- separate permissions for logs or other event families when supported.

Constrain credentials by collector, environment, service, origin/source IP, payload size, request rate, and event rate where supported. Public credentials require an origin allowlist and must not receive administrative roles. Never use a browser key for a server or query credential.

## Credential precedence

For SDK credentials, the usual precedence is explicit code credentials, explicitly supplied DSN credentials, then environment credentials such as `LOZA_API_KEY` or `LOZA_DSN`. Confirm the installed SDK's release behavior before relying on precedence during rotation.

## Secret-handling rules

- Never send credentials in query parameters or event bodies.
- Never log `Authorization`, DSN userinfo, or raw tokens.
- Keep bearer and Basic credentials on TLS or a private authenticated channel.
- Rotate keys with an overlap window only when both old and new scopes are known.
- Use separate Collector auth and storage-encryption secrets.
- Validate both rejection without credentials and acceptance with a disposable least-privilege credential.
