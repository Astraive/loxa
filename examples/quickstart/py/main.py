from loxa import Production, Logger

cfg = Production("quickstart-demo").with_collector_endpoint("http://localhost:9090")
logger = Logger(cfg)

ctx = logger.start_event(event="user.signup", kind="http", service="quickstart-demo")

logger.enrich(ctx, user__email="demo@example.com", user__plan="pro")

logger.finish(ctx, "success")

result = logger.emit(ctx)
print(f"Event emitted: {result}")
