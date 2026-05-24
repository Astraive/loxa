package projection

import (
	"testing"
)

var benchEvent = []byte(`{"event":{"name":"checkout_payment_failed","type":"payment_flow","category":"checkout","action":"pay","level":"error","timestamp":"2026-05-21T10:42:18.731Z","status_code":402,"outcome":"failed","ids":{"request_id":"req_9f8c2a41","trace_id":"trc_7b21d90f4a8c","span_id":"spn_checkout_payment_001","correlation_id":"corr_user456_cart789","session_id":"sess_7xk29"},"service":{"name":"checkout-service","version":"2.4.1","environment":"production","region":"ap-south-1","host":"checkout-prod-17"},"http":{"method":"POST","route":"/api/checkout/pay","path":"/api/checkout/pay","status_code":402,"duration_ms":8611},"actor":{"user_id":"user_456","country":"IN"},"payment":{"provider":"stripe","method":"card","amount_cents":1399900}}}`)

var benchSchema = map[string]string{
	"event_id":       "event_id",
	"timestamp":      "timestamp",
	"event_name":     "event.name",
	"level":          "event.level",
	"status_code":    "event.status_code",
	"outcome":        "event.outcome",
	"service":        "event.service.name",
	"region":         "event.service.region",
	"trace_id":       "event.ids.trace_id",
	"http_status":    "event.http.status_code",
	"duration_ms":    "event.http.duration_ms",
	"payment_provider": "event.payment.provider",
}

// BenchmarkProjectValues measures the core enrich operation:
// decode JSON + extract canonical columns from nested paths.
// Target: >1M ops/sec (<1µs per operation).
func BenchmarkProjectValues(b *testing.B) {
	cols := SortedColumns(benchSchema)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ProjectValues(benchEvent, benchSchema, cols)
	}
}

// BenchmarkDecodeObject measures JSON decode only.
func BenchmarkDecodeObject(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeObject(benchEvent)
	}
}

// BenchmarkExtractPath measures a single nested path extraction.
func BenchmarkExtractPath(b *testing.B) {
	doc, _ := DecodeObject(benchEvent)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractPath(doc, "event.payment.provider")
	}
}
