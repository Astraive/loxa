package validation

import (
	"bytes"
	"encoding/json"
)

func IsJSONObject(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 1 && raw[0] == '{' && json.Valid(raw)
}
