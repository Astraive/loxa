package main

import (
	"context"
	"log"
	"time"

	"github.com/astraive/loxa/sdks/go"
	"github.com/astraive/loxa/sdks/go/sinks/httpbatch"
)

func main() {
	sink, err := httpbatch.New(httpbatch.Config{
		URL:           "http://127.0.0.1:9090/ingest",
		FlushInterval: 250 * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := loxa.Configure(
		loxa.Production().
			WithService("checkout").
			WithSink(sink),
	); err != nil {
		log.Fatal(err)
	}
	defer loxa.Shutdown(context.Background())

	ctx := loxa.StartEvent(context.Background(), loxa.Params{
		Event:  "checkout.request",
		Method: "POST",
		Path:   "/checkout",
		Route:  "/checkout",
	})
	defer loxa.Emit(ctx)

	loxa.Enrich(ctx, loxa.String("payment.provider", "stripe"))
	loxa.Finish(ctx, "success", loxa.Int("status_code", 200))

	time.Sleep(500 * time.Millisecond)
}
