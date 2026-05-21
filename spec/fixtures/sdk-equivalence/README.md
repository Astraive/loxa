# SDK Equivalence Fixture

This fixture defines a canonical event that all 3 SDKs (Go, Python, Rust) must produce.

## Canonical Event

All SDKs must produce an event with these exact field values when configured with:
- Service: `checkout`
- Event: `payment.completed`
- Kind: `event`
- Level: `info`
- Outcome: `success`

## Required Fields (order-independent)

```json
{
  "schema_version": "v1",
  "event_version": "v1",
  "service": "checkout",
  "event": "payment.completed",
  "kind": "event",
  "level": "info",
  "outcome": "success"
}
```

## Assertions

1. All required fields must be present
2. Field values must match exactly
3. `schema_version` and `event_version` must be `"v1"`
4. No SDK-specific fields should leak into the canonical shape
5. `attrs` should contain SDK-provided attributes
