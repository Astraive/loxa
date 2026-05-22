import { configure, production, startEvent, enrich, finish, emit, info, createLoxa, alias } from "loxa-js";

// Default API — configure once, use everywhere
configure(production("quickstart-demo").withCollectorEndpoint("http://localhost:9090"));

info("server started");

const ctx = startEvent({ event: "user.signup", kind: "http" });
enrich(ctx, { "user.email": "demo@example.com", "user.plan": "pro" });
finish(ctx, "success");
const result = await emit(ctx);
console.log(`Event emitted: ${result}`);

// Custom instance
const logger = createLoxa({ service: "checkout-api", collectorUrl: "http://localhost:9090" });
logger.info("custom instance ready");

// Alias — same config, different service name
const audit = alias("audit-service");
audit.info("audit trail started");
