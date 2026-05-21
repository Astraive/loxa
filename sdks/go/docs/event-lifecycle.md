# Event Lifecycle

Recommended order:

`StartEvent -> Append/Enrich/Set/Merge/Delete -> Checkpoint -> Finish/FinishError -> Emit`

Typical usage:

```go
ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "checkout.request"})
defer loxa.Emit(ctx)

loxa.Enrich(ctx, loxa.String("payment.provider", "stripe"))
loxa.Checkpoint(ctx, "payment_started")
loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
```
