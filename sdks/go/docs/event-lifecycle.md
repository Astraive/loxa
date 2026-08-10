# Event Lifecycle

Recommended order:

`StartEvent -> Append/Enrich/Set/Merge/Delete -> Checkpoint -> Finish/FinishError -> Emit`

Typical usage:

```go
ctx := loza.StartEvent(context.Background(), loza.Params{Event: "checkout.request"})
defer loza.Emit(ctx)

loza.Enrich(ctx, loza.String("payment.provider", "stripe"))
loza.Checkpoint(ctx, "payment_started")
loza.Finish(ctx, "success", loza.Int("status_code", 200))
```
