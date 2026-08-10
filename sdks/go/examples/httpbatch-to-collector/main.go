package main

import (
	"context"
	"log"
	"time"

	"github.com/astraive/loza/sdks/go"
	"github.com/astraive/loza/sdks/go/sinks/httpbatch"
)

func main() {
	sink, err := httpbatch.New(httpbatch.Config{
		URL:           "http://127.0.0.1:9308/ingest",
		FlushInterval: 250 * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := loza.Configure(
		loza.Production().
			WithService("checkout").
			WithSink(sink),
	); err != nil {
		log.Fatal(err)
	}
	defer loza.Shutdown(context.Background())

	ctx := loza.StartEvent(context.Background(), loza.Params{
		Event:  "checkout.request",
		Method: "POST",
		Path:   "/checkout",
		Route:  "/checkout",
	})
	defer loza.Emit(ctx)

	loza.Enrich(ctx, loza.String("payment.provider", "stripe"))
	loza.Finish(ctx, "success", loza.Int("status_code", 200))

	time.Sleep(500 * time.Millisecond)
}
