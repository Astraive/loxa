import loza

# Default API — configure once, use everywhere
loza.configure(loza.production("quickstart-demo").with_collector_endpoint("http://localhost:9308"))

loza.info("server started")

ctx = loza.start_event(loza.Params(event="user.signup", kind="http"))
loza.enrich(ctx, loza.String("user.email", "demo@example.com"), loza.String("user.plan", "pro"))
loza.finish(ctx, "success")
result = loza.emit(ctx)
print(f"Event emitted: {result}")

# Custom instance
logger = loza.create_loza(service="checkout-api", collector_endpoint="http://localhost:9308")
logger.info("custom instance ready")

# Alias — same config, different service name
audit = loza.alias("audit-service")
audit.info("audit trail started")
