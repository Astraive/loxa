# Tenant And Service Identity

Canonical identity fields:

- `service.name`
- `service.version`
- `deployment.environment`
- `deployment.region`
- `tenant.id`
- `workspace.id`
- `organization.id`
- `source.sdk.name`
- `source.sdk.version`
- `source.instance_id`

Collector identity sources:

- Payload identity from the event or ingest envelope.
- API key binding.
- mTLS certificate identity.
- JWT/OIDC claims.

Security rule:

In production, authenticated identity wins over payload identity unless payload
identity override is explicitly allowed. A service MUST NOT be able to spoof
another service by setting `service.name` in the payload.

