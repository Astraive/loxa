package processing

import "testing"

func TestExtractTenantID(t *testing.T) {
	cases := []struct{
		name string
		raw string
		exp string
		ok bool
	}{
		{"top-level", `{"tenant_id":"tenant-123"}`, "tenant-123", true},
		{"nested", `{"tenant":{"id":"tenant-456"}}`, "tenant-456", true},
		{"missing", `{"service":"checkout"}`, "", false},
		{"empty", `{"tenant_id":""}`, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, ok := ExtractTenantID([]byte(c.raw))
			if ok != c.ok || id != c.exp {
				t.Fatalf("case %s: expected (%v,%s), got (%v,%s)", c.name, c.ok, c.exp, ok, id)
			}
		})
	}
}
