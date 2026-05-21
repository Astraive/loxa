# Security

SDK-side security controls for the LOXA Python SDK. Redaction runs before final schema encoding, ensuring sensitive data never reaches sinks.

## Redactors

### DefaultRedactor

The built-in 14-key safety-net redactor. It automatically redacts values for common sensitive field names:

```
password, secret, token, api_key, apikey, access_token,
refresh_token, authorization, credit_card, credit_card_number,
ssn, social_security, private_key, connection_string
```

Usage:

```python
cfg = loxa.production("my-service").with_redactor(loxa.DefaultRedactor())
```

### RedactKeys

Replace the values of specified keys with `[REDACTED]`:

```python
redactor = loxa.RedactKeys("email", "phone", "address")
cfg = loxa.production("my-service").with_redactor(redactor)
```

### HashKeys

Replace the values of specified keys with their SHA-256 hash. Useful when you need to correlate events without exposing the original value:

```python
redactor = loxa.HashKeys("user.email", "user.phone")
cfg = loxa.production("my-service").with_redactor(redactor)
```

### DropKeys

Remove keys entirely from the event before encoding:

```python
redactor = loxa.DropKeys("internal.debug_info", "raw_request_body")
cfg = loxa.production("my-service").with_redactor(redactor)
```

### MaskKeys

Mask values, showing only a prefix and suffix of characters. Default shows first 2 and last 2 characters:

```python
redactor = loxa.MaskKeys("credit_card", prefix=4, suffix=4)
# "4111222233334444" -> "4111********4444"
```

### RedactPatterns

Redact values that match regular expression patterns:

```python
redactor = loxa.RedactPatterns(r"\b\d{3}-\d{2}-\d{4}\b")
# Redacts SSN-pattern values
```

### ComposeRedactors

Chain multiple redactors together. They run in order:

```python
redactor = loxa.ComposeRedactors(
    loxa.DefaultRedactor(),
    loxa.RedactKeys("email"),
    loxa.HashKeys("user.phone"),
)
cfg = loxa.production("my-service").with_redactor(redactor)
```

## SensitiveString and HashString

Mark individual attributes as sensitive at construction time:

```python
loxa.enrich(ctx,
    loxa.SensitiveString("credit_card", "4111222233334444"),
    loxa.HashString("user.email", "alice@example.com"),
)
```

`SensitiveString` triggers redaction by the configured redactor. `HashString` hashes the value at construction time, before the event reaches the redactor.

## MarkSensitive

Mark any existing attribute as sensitive:

```python
attr = loxa.String("field", "value")
sensitive_attr = loxa.MarkSensitive(attr)
```

## 14-Key Safety Net

The `DefaultRedactor` covers these 14 key patterns by default. This provides a baseline even if no explicit redactor is configured. The patterns match case-insensitively and catch common variations (e.g., `API_KEY`, `api-key`, `ApiKey`).

## Field Size Limits

The SDK enforces limits to prevent oversized events:

- **Max event bytes**: Configurable, default 256 KB.
- **Max attr count**: Configurable, default 128 attributes per event.
- **Max string value length**: Configurable, default 8 KB per string value.

These limits are enforced at emit time. Events exceeding limits are dropped and a delivery failure is reported to the stats handler.

## Duplicate Field Policy

When a canonical field (e.g., `user.id`) is set both by the SDK automatically and by user enrichment, the duplicate policy determines the winner:

| Policy | Behavior |
|--------|----------|
| `canonical_wins` (default) | SDK canonical value takes precedence. |
| `user_wins` | User-provided value takes precedence. |
| `first_wins` | First value set wins. |
| `last_wins` | Last value set wins. |
| `keep_both` | Both values kept (renames one). |
| `error_on_duplicate` | Raises an error. |

## Redaction Execution Order

1. User enrichment (attrs set via `enrich`, `set`, `merge`)
2. Canonical field enforcement (duplicate policy)
3. Redaction (configured redactor runs on all fields)
4. Schema encoding (final JSON/flat/nested output)
5. Sink delivery
