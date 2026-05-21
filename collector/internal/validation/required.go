package validation

import (
	"encoding/json"
	"strings"
)

func ExtractStringPath(raw []byte, path string) (string, bool) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", false
	}
	current := data
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := obj[part]
		if !ok {
			return "", false
		}
		current = next
	}
	value, ok := current.(string)
	return value, ok
}
