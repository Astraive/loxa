# Config

Collector config precedence:

1. defaults
2. YAML config file
3. environment variables
4. flags

Core sections:

- `collector`
- `auth` (see [Authentication](../../docs/authentication.md) and [Authorization](../../docs/authorization.md))
- `rate_limit`
- `routes`
- `storage`
- `duckdb`
- `kafka`
- `worker`
- `reliability`
- `retry`
- `dead_letter`
- `fanout`
- `dedupe`
- `schema_governance`

Queue + Redis dedupe example:

```yaml
reliability:
  mode: queue
kafka:
  brokers: ["127.0.0.1:9092"]
  topic: loxa-events
  acks: all
  enable_idempotence: true
dedupe:
  enabled: true
  key: event_id
  window: 24h
  backend: redis
  redis_addr: 127.0.0.1:6379
```
