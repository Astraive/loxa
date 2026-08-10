# Basic

Minimal LOZA SDK setup:

```go
_ = loza.Configure(loza.Production().WithService("checkout"))
defer loza.Shutdown(context.Background())

ctx := loza.StartEvent(context.Background(), loza.Params{Event: "checkout.request"})
defer loza.Emit(ctx)

loza.Finish(ctx, "success")
```
