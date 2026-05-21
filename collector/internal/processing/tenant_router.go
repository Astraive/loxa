package processing

import (
	"encoding/json"
)

// ExtractTenantID attempts to extract a tenant identifier from an event payload.
// It supports top-level "tenant_id" and nested "tenant.id" paths.
func ExtractTenantID(raw []byte) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}
	if v, ok := data["tenant_id"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}
	if t, ok := data["tenant"]; ok {
		if m, ok := t.(map[string]any); ok {
			if id, ok := m["id"]; ok {
				if s, ok := id.(string); ok && s != "" {
					return s, true
				}
			}
		}
	}
	return "", false
}
