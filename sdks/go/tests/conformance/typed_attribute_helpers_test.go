package conformance

import (
	"testing"

	"github.com/astraive/loza/sdks/go"
)

func TestTypedAttributeHelpers(t *testing.T) {
	attrCases := []struct {
		name string
		attr loza.Attr
		key  string
	}{
		{"string", loza.String("service.name", "checkout"), "service.name"},
		{"int", loza.Int("attempt", 2), "attempt"},
		{"float", loza.Float("latency_ms", 12.5), "latency_ms"},
		{"bool", loza.Bool("cache.hit", true), "cache.hit"},
		{"json", loza.JSON("payload", map[string]any{"ok": true}), "payload"},
		{"money", loza.Money("cart.total", 4999, "USD"), "cart.total"},
		{"percent", loza.Percent("cpu", 87.5), "cpu"},
		{"bytes", loza.Bytes("payload.size", 2048), "payload.size"},
		{"http status", loza.HTTPStatus(200), "status_code"},
		{"error code", loza.ErrorCode("checkout.declined"), "error_code"},
		{"bucket", loza.Bucket("user.tier", "pro"), "bucket"},
		{"masked", loza.Masked("card", "4111111111111111"), "card"},
		{"url", loza.URL("url", "https://example.com"), "url"},
		{"email hash", loza.EmailHash("email.hash", "User@Example.com"), "email.hash"},
		{"ip hash", loza.IPHash("ip.hash", "127.0.0.1"), "ip.hash"},
	}

	for _, tc := range attrCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.attr.Key != tc.key {
				t.Fatalf("expected key %q, got %q", tc.key, tc.attr.Key)
			}
		})
	}
}
