package main

import (
	"context"
	"fmt"

	loxa "github.com/astraive/loxa-go"
)

func main() {
	cfg := loxa.Production("quickstart-demo").WithCollectorEndpoint("http://localhost:9090")
	logger, err := loxa.New(cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Shutdown(context.Background())

	ctx := logger.StartEvent(context.Background(), loxa.Params{
		Event:   "user.signup",
		Kind:    "http",
		Service: "quickstart-demo",
	})

	logger.Enrich(ctx, loxa.String("user.email", "demo@example.com"))
	logger.Enrich(ctx, loxa.String("user.plan", "pro"))

	logger.Finish(ctx, "success")

	if err := logger.Emit(ctx); err != nil {
		fmt.Printf("emit error: %v\n", err)
		return
	}

	fmt.Println("Event emitted successfully")
}
