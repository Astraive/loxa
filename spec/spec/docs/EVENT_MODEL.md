# Event Model

Canonical LOXA event fields:

- `event_id`
- `timestamp`
- `schema_version`
- `event_version`
- `service`
- `service.name`
- `service.version`
- `deployment.environment`
- `deployment.region`
- `version`
- `environment`
- `event`
- `level`
- `outcome`
- `duration_ms`
- `request_id`
- `trace_id`
- `span_id`
- `tenant.id`
- `workspace.id`
- `organization.id`
- `source.sdk.name`
- `source.sdk.version`
- `source.instance_id`

Structured domains may include:

- `http.*`
- `error.*`
- custom attributes

The lifecycle is governed by `EVENT_STATE_MACHINE.md`. `emitted` is terminal,
validation failure must not mark an event emitted, and delivery acceptance is
the point where `event_state` becomes `emitted`.

Strict validation requires:

- `event_id`
- `timestamp`
- `service`
- `event`

Collectors may validate canonical field types, but they should remain permissive for additional custom attributes.
