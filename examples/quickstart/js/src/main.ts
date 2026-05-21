import { Logger, production } from "loxa-js";

const logger = new Logger(production("quickstart-demo").withCollectorEndpoint("http://localhost:9090"));

const ctx = logger.startEvent({
  event: "user.signup",
  kind: "http",
  service: "quickstart-demo",
});

logger.enrich(ctx, { "user.email": "demo@example.com", "user.plan": "pro" });

logger.finish(ctx, "success");

const result = logger.emit(ctx);
console.log(`Event emitted: ${result}`);
