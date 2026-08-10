import loza

ctx = loza.StartEvent(None, loza.Params(event="example.basic", service="example"))
loza.Finish(ctx, "success")
print(loza.Emit(ctx))
