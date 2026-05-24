package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestBasicLoggingAndEventFacades(t *testing.T) {
	sink, store := loxa.MemorySink()
	if err := loxa.Configure(loxa.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	loxa.Notice("notice event", loxa.String("family", "logs"))
	loxa.Track("checkout.page_view", loxa.String("page", "/checkout"))
	loxa.Audit("user.login", loxa.UserID("u_123"))
	loxa.Security("auth.failure", loxa.ErrorCode("AUTH_BAD_PASSWORD"))
	loxa.Metric("latency", 12.5, loxa.String("unit", "ms"))
	loxa.Count("requests", 4)
	loxa.Gauge("cpu", 0.72)
	loxa.Histogram("payload.bytes", 512)
	loxa.Breadcrumb("nav.click", loxa.String("button", "submit"))

	if err := loxa.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() < 9 {
		t.Fatalf("expected 9 emitted facade events, got %d", store.Len())
	}
}
