import loxa

ctx = loxa.StartEvent(None, loxa.Params(event="example.basic", service="example"))
loxa.Finish(ctx, "success")
print(loxa.Emit(ctx))
