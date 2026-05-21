package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// ParseNDJSON parses newline-delimited JSON payloads into ordered objects.
func ParseNDJSON(data []byte) ([]map[string]any, error) {
	return ParseNDJSONReader(bytes.NewReader(data))
}

// ParseNDJSONReader parses newline-delimited JSON from r.
func ParseNDJSONReader(r io.Reader) ([]map[string]any, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var out []map[string]any
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("utils: parse ndjson line %d: %w", line, err)
		}
		out = append(out, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("utils: read ndjson: %w", err)
	}
	return out, nil
}
