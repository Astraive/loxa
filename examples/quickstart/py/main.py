import loxa

# Default API — configure once, use everywhere
loxa.configure(loxa.Production("quickstart-demo").with_collector_endpoint("http://localhost:9090"))

loxa.info("server started")

ctx = loxa.start_event(event="user.signup", kind="http")
loxa.enrich(ctx, user__email="demo@example.com", user__plan="pro")
loxa.finish(ctx, "success")
result = loxa.emit(ctx)
print(f"Event emitted: {result}")

# Custom instance
logger = loxa.create_loxa(service="checkout-api", collector_endpoint="http://localhost:9090")
logger.info("custom instance ready")

# Alias — same config, different service name
audit = loxa.alias("audit-service")
audit.info("audit trail started")
