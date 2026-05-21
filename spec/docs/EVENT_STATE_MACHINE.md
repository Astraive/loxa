# Event State Machine

Every SDK MUST implement the same local lifecycle state machine for each
`EventContext`.

```text
created -> active -> finished -> emitting -> emitted
created/active -> failed_validation
emitting -> delivery_failed
emitted is terminal
```

Rules:

- `Enrich`, `Set`, `Add`, `Merge`, `Delete`, and `Checkpoint` are allowed only before the event is closed. SDKs MUST reject mutation after `emitting`, `emitted`, `failed_validation`, or `delivery_failed`.
- `Finish` is allowed once.
- `FinishError` is allowed once.
- `Emit` is allowed once per `EventContext`.
- `Emit` after `emitted` MUST return `DuplicateEmitError`.
- `Finish` after `emitted` MUST return `EventClosedError`.
- Validation failure MUST NOT mark the event emitted.
- Delivery accepted by the configured local sink or collector transport marks the event `emitted`.
- Delivery failure after the local emit attempt marks the event `delivery_failed`.

SDKs MAY keep deprecated boolean helpers such as `is_emitted`, but those helpers
MUST be derived from this state machine.

