package main

import (
	"context"
	"fmt"

	loxa "github.com/astraive/loxa/sdks/go"
)

func main() {
	ctx := context.Background()

	// Default API — configure once, use everywhere
	loxa.Configure(loxa.Production("quickstart-demo").WithCollectorEndpoint("http://localhost:9090"))
	defer loxa.Shutdown(ctx)

	loxa.Info("server started")

	ev := loxa.StartEvent(ctx, loxa.Params{Event: "user.signup", Kind: "http"})
	loxa.Enrich(ev, loxa.String("user.email", "demo@example.com"))
	loxa.Enrich(ev, loxa.String("user.plan", "pro"))
	loxa.Finish(ev, "success")
	if err := loxa.Emit(ev); err != nil {
		fmt.Printf("emit error: %v\n", err)
		return
	}
	fmt.Println("Event emitted successfully")

	// Custom instance
	logger, _ := loxa.New(loxa.Config{Service: "checkout-api", CollectorURL: "http://localhost:9090"})
	logger.Info(ctx, "custom instance ready")

	// Alias — same config, different service name
	audit, _ := loxa.Alias("audit-service")
	audit.Info(ctx, "audit trail started")
}
