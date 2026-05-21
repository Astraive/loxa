# CORTEX Security

## Authentication

- API Key: Bearer token in Authorization header
- mTLS: Optional client certificate validation
- OAuth 2.0: Service-to-service flow

## Authorization

- RBAC: Role-based access control
  - `admin`: Full access
  - `ingest`: Write events, read own service data
  - `viewer`: Read-only access
  - `analyst`: Full read, reconstruct, feedback

- Scoped to:
  - Services: Can only ingest events for authorized services
  - Incident access: Can only view incidents within org/team

## PII Redaction

Events may contain personally identifiable information (PII). Apply redaction rules:

```
Patterns to redact:
- Emails: user@example.com
- Phone: +1-555-1234
- SSN: xxx-xx-xxxx
- Credit cards: ****-****-****-1234
- API keys: sk_live_xxx...
- User IDs in logs: user#12345

Store original in secure field (encrypted at rest)
```

## Encryption

- At Rest: AES-256 for sensitive fields (raw event payload)
- In Transit: TLS 1.3 minimum
- Keys: Managed by KMS (AWS KMS or HashiCorp Vault)

## Audit Logging

- All ingest operations logged: timestamp, source, service, event_count
- All reconstructions logged: who, when, incident_id, result
- All feedback logged: who, when, incident_id, change

## Data Isolation

- Multi-tenant:  Events/graphs/feedback isolated by organization
- Service-level: Users can't access events from unauthorized services
- Encryption keys: Per-tenant key management

## Rate Limiting

- Per API key: 10k events/sec (configurable)
- Per organization: 100k events/sec
- Reconstruction: 10 per minute per org

## Incident Response

- Incident incident involves customer data exposure
- Steps:
  1. Identify affected incidents
  2. Redact sensitive fields from public reports
  3. Retain audit log for investigation
  4. Notify org admins within 24h

## Compliance

- SOC 2 Type II
- GDPR: Right to be forgotten (redact/delete customer data)
- HIPAA: Configurable encryption and audit logging
- Separate deployments per compliance zone
