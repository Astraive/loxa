package main

import (
	"context"
	"fmt"

	loza "github.com/astraive/loza/sdks/go"
)

func main() {
	ctx := context.Background()

	// Default API — configure once, use everywhere
	loza.Configure(loza.ApplyConfig(loza.Production(),
		loza.WithService("quickstart-demo"),
		loza.WithCollectorEndpoint("http://localhost:9308"),
	))
	defer loza.Shutdown(ctx)

	loza.Info("server started")

	ev := loza.StartEvent(ctx, loza.Params{Event: "user.signup", Kind: "http"})
	loza.Enrich(ev, loza.String("user.email", "demo@example.com"))
	loza.Enrich(ev, loza.String("user.plan", "pro"))
	loza.Finish(ev, "success")
	if err := loza.Emit(ev); err != nil {
		fmt.Printf("emit error: %v\n", err)
		return
	}
	fmt.Println("Event emitted successfully")

	// Custom instance
	logger, _ := loza.New(loza.ApplyConfig(loza.Config{},
		loza.WithService("checkout-api"),
		loza.WithCollectorEndpoint("http://localhost:9308"),
	))
	logger.Info("custom instance ready")

	// Alias — same config, different service name
	audit, _ := loza.Alias("audit-service")
	audit.Info("audit trail started")
}
