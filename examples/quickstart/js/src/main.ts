import { configure, production, startEvent, enrich, finish, emit, info, createLoza, alias } from "loza";

// Default API — configure once, use everywhere
configure(production("quickstart-demo").withCollectorEndpoint("http://localhost:9308"));

info("server started");

const ctx = startEvent({ event: "user.signup", kind: "http" });
enrich(ctx, { "user.email": "demo@example.com", "user.plan": "pro" });
finish(ctx, "success");
const result = await emit(ctx);
console.log(`Event emitted: ${result}`);

// Custom instance
const logger = createLoza({ service: "checkout-api", collectorUrl: "http://localhost:9308" });
logger.info("custom instance ready");

// Alias — same config, different service name
const audit = alias("audit-service");
audit.info("audit trail started");
