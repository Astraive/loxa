import loxa


ctx = loxa.StartHTTPEvent(None, loxa.Params(event="GET /ready", method="GET", path="/ready", kind="http"))
loxa.Finish(ctx, "success", loxa.Int("status_code", 200))
print(loxa.Emit(ctx))

