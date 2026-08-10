import loza


ctx = loza.StartHTTPEvent(None, loza.Params(event="GET /ready", method="GET", path="/ready", kind="http"))
loza.Finish(ctx, "success", loza.Int("status_code", 200))
print(loza.Emit(ctx))

