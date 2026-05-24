# Canonical Duplicate Field Policy

**Version**: 0.0.2  
**Date**: May 15, 2026  
**Status**: Production - All SDKs MUST implement this policy

---

## Overview

Duplicate fields occur when a custom attribute uses the same name as a canonical top-level field. This document defines:
1. Which fields are canonical (reserved)
2. How SDKs MUST handle duplicates
3. What behavior to enforce across Go/Python/Rust
4. Required test cases

---

## Canonical (Reserved) Top-Level Fields

The following field names are reserved and MUST NOT be overridden by custom attributes:

### Metadata Fields (Always Present)
- `schema_version` - Always "v1"
- `event_version` - Always "v1"
- `timestamp` - ISO 8601 creation time
- `event_id` - UUIDv7 or equivalent

### Event Description Fields
- `service` - Service identifier
- `event` - Event name
- `kind` - Event kind (http, async, batch, cron, error)
- `level` - Log level (info, warn, error, debug, trace)
- `outcome` - Outcome (success, error, partial, abandoned, retried)

### Timing & Correlation Fields
- `duration_ms` - Event duration in milliseconds
- `request_id` - Request/operation ID
- `trace_id` - Distributed trace ID
- `span_id` - Span identifier

### HTTP Fields (if kind="http")
- `method` - HTTP method
- `path` - Request path
- `route` - Route pattern
- `status_code` - HTTP status code

### Context Objects (top-level)
- `http` - HTTP context object (nested)
- `user` - User object (nested)
- `tenant` - Tenant object (nested)

### Deprecated Fields (for backward compatibility)
- `environment` (deprecated: use deployment.environment)

---

## Duplicate Field Policies

Each SDK MUST support these policies via configuration:

### Policy 1: CanonicalWins (Recommended Default)
```
When a custom attribute duplicates a canonical field:
1. Keep the canonical value
2. Drop the conflicting attribute
3. Log a warning
4. REASON: Canonical fields always take precedence, protect against accidental override
```

**Behavior**:
```
Canonical: service="api"
Custom: attrs["service"]="api-custom"
Result: service="api", attrs["service"] REMOVED
```

**Test Case**:
```go
// Go example
ctx := logger.StartEvent(Params{Service: "api"})
logger.Enrich(ctx, String("service", "api-custom"))
event := logger.Emit(ctx)
assert event.Service == "api"
assert !event.HasAttr("service") // Dropped
```

### Policy 2: AttrWins (Permissive, Not Recommended)
```
When a custom attribute duplicates a canonical field:
1. Use the custom value (override canonical)
2. Remove the duplicate from attrs
3. Log a warning (strong warning)
4. REASON: Allow SDK-level configuration overrides, but warn loudly
```

**Behavior**:
```
Canonical: service="api"
Custom: attrs["service"]="api-custom"
Result: service="api-custom", attrs["service"] REMOVED
```

**Warning Message**:
```
WARNING: Custom attribute 'service' overrides canonical field. 
This may cause data corruption. Use CanonicalWins policy instead.
```

### Policy 3: Error (Strictest)
```
When a custom attribute duplicates a canonical field:
1. Reject the event with ErrDuplicateField
2. Call StatsHandler.OnError(err)
3. Do NOT emit the event
4. REASON: Fail-safe for strict environments
```

**Behavior**:
```
Canonical: service="api"
Custom: attrs["service"]="api-custom"
Result: ErrDuplicateField returned to caller
```

---

## Nested Field Rules

### HTTP Object
Nested fields like `http.method`, `http.path` are NOT considered duplicates of top-level fields when used correctly.

✅ ALLOWED:
```json
{
  "method": "POST",
  "http": {
    "method": "POST",
    "path": "/api/users"
  }
}
```

❌ NOT ALLOWED (duplicate in custom attrs):
```json
{
  "method": "POST",
  "attrs": {
    "method": "GET"
  }
}
```

### User/Tenant Objects
Similar rules apply to nested user and tenant objects.

---

## Configuration

### Go SDK
```go
loxa.Configure(loxa.Production().
    WithService("api").
    WithDuplicateFieldPolicy(loxa.CanonicalWins))
```

### Python SDK
```python
client = CollectorClient(service="api", sink=sink,
    duplicate_field_policy="canonical_wins")
```

### Rust SDK
```rust
Config::production("api")
    .with_duplicate_field_policy(DuplicateFieldPolicy::CanonicalWins)
```

### CLI/Collector
```yaml
# loxa-collector.yaml
SDK:
  DuplicateFieldPolicy: canonical_wins  # canonical_wins | attr_wins | error
```

---

## Default Behavior (All SDKs)

| SDK | Default Policy | Minimum Version |
|-----|---|---|
| Go | CanonicalWins | 0.0.2 |
| Python | CanonicalWins | 0.0.2 |
| Rust | CanonicalWins | 0.0.2 |
| Collector | CanonicalWins | 0.0.2 |

**All SDKs MUST default to CanonicalWins for production safety.**

---

## Detection Algorithm

All SDKs MUST implement this algorithm:

```
func CheckForDuplicates(canonical *Event, attrs map[string]interface{}) error {
    reservedKeys := [
        "schema_version", "event_version", "timestamp", "event_id",
        "service", "event", "kind", "level", "outcome",
        "duration_ms", "request_id", "trace_id", "span_id",
        "method", "path", "route", "status_code",
        "http", "user", "tenant"
    ]
    
    for key := range attrs {
        if contains(reservedKeys, key) {
            return ErrDuplicateField{Key: key}
        }
    }
    return nil
}
```

---

## Required Test Coverage

Every SDK MUST pass these test cases:

### Test 1: CanonicalWins Policy
- [ ] Duplicate "service" field is dropped
- [ ] Duplicate "status_code" is dropped
- [ ] Duplicate "trace_id" is dropped
- [ ] Canonical value is preserved
- [ ] Warning is logged

### Test 2: AttrWins Policy
- [ ] Duplicate "service" field overrides canonical
- [ ] Duplicate "status_code" field overrides canonical
- [ ] Warning (or error) is logged
- [ ] Custom value takes precedence

### Test 3: Error Policy
- [ ] Duplicate "service" field causes ErrDuplicateField
- [ ] Event is NOT emitted
- [ ] Error is returned to caller
- [ ] StatsHandler.OnError called

### Test 4: Nested Fields (No Duplicates)
- [ ] http.method DOES NOT conflict with top-level method
- [ ] user.id DOES NOT conflict with top-level user
- [ ] Nested objects allowed

### Test 5: Valid Custom Attributes
- [ ] Custom "user_id" attr allowed (not "user")
- [ ] Custom "request_metadata" allowed (not "request_id")
- [ ] Custom "trace_metadata" allowed (not "trace_id")

### Test 6: Enrich Parity
- [ ] Enrich(ctx, String("service", "override")) follows policy
- [ ] Enrich(ctx, String("custom", "value")) always allowed

---

## Collector Enforcement

The collector MUST:
1. Accept events from SDKs (SDKs already enforce policy)
2. Validate against schema in validation processor
3. Log warnings if duplicate fields detected in collector config
4. Support all 3 policies in collector config (for flexibility)

---

## Migration Path (v1.x)

| Version | Behavior |
|---------|----------|
| 0.0.2 | CanonicalWins default, support all 3 policies |

---

## Examples

### Example 1: Checkout Service (Correct Usage)
```go
ctx := logger.StartEvent(context.Background(), Params{
    Event:   "checkout.completed",
    Service: "checkout",
})
logger.Enrich(ctx,
    String("user_id", "u-123"),      // ✅ Custom attr, no conflict
    String("cart_id", "cart-456"),   // ✅ Custom attr, no conflict
    Int("total_cents", 9999),        // ✅ Custom attr, no conflict
)
logger.Finish(ctx, "success", Int("status_code", 200))
logger.Emit(ctx)
```

### Example 2: Accidental Duplicate (CanonicalWins Policy)
```go
ctx := logger.StartEvent(context.Background(), Params{
    Event:   "payment.charged",
    Service: "payment",
})
logger.Enrich(ctx,
    String("service", "my-override"),  // ❌ Duplicate! Will be dropped.
    Int("status_code", 500),           // ❌ Duplicate! Will be dropped.
)
logger.Finish(ctx, "error")
logger.Emit(ctx)
// Result: service="payment" (canonical wins), status_code NOT in event
```

### Example 3: Strict Mode (Error Policy)
```go
client, _ := loxa.New(loxa.Config{
    Service: "api",
    Sink:    sink,
    DuplicateFieldPolicy: loxa.ErrorPolicy,
})

ctx := logger.StartEvent(Params{Event: "test"})
logger.Enrich(ctx, String("trace_id", "override"))  // ❌ Error!
err := logger.Emit(ctx)
// Error: ErrDuplicateField{Key: "trace_id"}
// Event NOT emitted
```

---

## FAQ

**Q: Why protect canonical fields?**  
A: Canonical fields are required by schema and processors. Allowing overrides risks data corruption and schema violations.

**Q: What about valid user overrides?**  
A: Use non-conflicting keys like "user_id_override" instead of "user_id".

**Q: Can I have http.method and method?**  
A: Yes. `method` is canonical, but `http.method` (nested) is separate. Only custom attrs collide.

**Q: What if I need maximum backward compatibility?**  
A: Use AttrWins policy (not recommended), but understand the risks.

**Q: When will strict Error policy be default?**  
A: v0.0.1 defaults to CanonicalWins.

---

**Last Updated**: May 15, 2026  
**Status**: Production - Enforced across all SDKs  
**All SDKs MUST implement this policy for v0.0.1 release.**
