# Custom Schema Example

```go
cfg := loxa.Production().
	WithService("checkout").
	WithSchema(loxa.CustomSchema(func(v loxa.EventView) map[string]any {
		return map[string]any{
			"ts":         v.Timestamp(),
			"event_name": v.Event(),
			"service":    v.Service(),
			"status":     v.StatusCode(),
			"outcome":    v.Outcome(),
			"attrs":      v.Attrs(),
		}
	}))
```

Sample output:

```json
{
  "ts": "2026-05-11T10:10:42Z",
  "event_name": "checkout.request",
  "service": "checkout",
  "status": 200,
  "outcome": "success",
  "attrs": {
    "user.id": "u-1",
    "payment.provider": "stripe"
  }
}
```
