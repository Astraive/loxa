# Basic

Minimal LOXA SDK setup:

```go
_ = loxa.Configure(loxa.Production().WithService("checkout"))
defer loxa.Shutdown(context.Background())

ctx := loxa.StartEvent(context.Background(), loxa.Params{Event: "checkout.request"})
defer loxa.Emit(ctx)

loxa.Finish(ctx, "success")
```
