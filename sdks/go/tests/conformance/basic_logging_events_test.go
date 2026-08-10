package conformance

import (
	"context"
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestBasicLoggingAndEventFacades(t *testing.T) {
	sink, store := loza.MemorySink()
	if err := loza.Configure(loza.Test().WithSink(sink)); err != nil {
		t.Fatalf("configure: %v", err)
	}

	loza.Notice("notice event", loza.String("family", "logs"))
	loza.Track("checkout.page_view", loza.String("page", "/checkout"))
	loza.Audit("user.login", loza.UserID("u_123"))
	loza.Security("auth.failure", loza.ErrorCode("AUTH_BAD_PASSWORD"))
	loza.Metric("latency", 12.5, loza.String("unit", "ms"))
	loza.Count("requests", 4)
	loza.Gauge("cpu", 0.72)
	loza.Histogram("payload.bytes", 512)
	loza.Breadcrumb("nav.click", loza.String("button", "submit"))

	if err := loza.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if store.Len() < 9 {
		t.Fatalf("expected 9 emitted facade events, got %d", store.Len())
	}
}
