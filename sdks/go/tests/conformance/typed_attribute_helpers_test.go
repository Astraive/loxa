package conformance

import (
	"testing"

	"github.com/astraive/loxa/sdks/go"
)

func TestTypedAttributeHelpers(t *testing.T) {
	attrCases := []struct {
		name string
		attr loxa.Attr
		key  string
	}{
		{"string", loxa.String("service.name", "checkout"), "service.name"},
		{"int", loxa.Int("attempt", 2), "attempt"},
		{"float", loxa.Float("latency_ms", 12.5), "latency_ms"},
		{"bool", loxa.Bool("cache.hit", true), "cache.hit"},
		{"json", loxa.JSON("payload", map[string]any{"ok": true}), "payload"},
		{"money", loxa.Money("cart.total", 4999, "USD"), "cart.total"},
		{"percent", loxa.Percent("cpu", 87.5), "cpu"},
		{"bytes", loxa.Bytes("payload.size", 2048), "payload.size"},
		{"http status", loxa.HTTPStatus(200), "status_code"},
		{"error code", loxa.ErrorCode("checkout.declined"), "error_code"},
		{"bucket", loxa.Bucket("user.tier", "pro"), "bucket"},
		{"masked", loxa.Masked("card", "4111111111111111"), "card"},
		{"url", loxa.URL("url", "https://example.com"), "url"},
		{"email hash", loxa.EmailHash("email.hash", "User@Example.com"), "email.hash"},
		{"ip hash", loxa.IPHash("ip.hash", "127.0.0.1"), "ip.hash"},
	}

	for _, tc := range attrCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.attr.Key != tc.key {
				t.Fatalf("expected key %q, got %q", tc.key, tc.attr.Key)
			}
		})
	}
}
