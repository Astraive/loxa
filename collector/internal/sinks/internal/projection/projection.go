package projection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// DecodeObject decodes encoded JSON into a map object.
func DecodeObject(encoded []byte) (map[string]any, error) {
	raw := bytes.TrimSpace(encoded)
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// ExtractPath reads a dot-path value from object map.
func ExtractPath(doc map[string]any, path string) (any, bool) {
	segments, err := parsePath(path)
	if err != nil {
		return nil, false
	}
	cur := any(doc)
	for _, seg := range segments {
		if seg.isIndex {
			vv, ok := cur.([]any)
			if !ok {
				return nil, false
			}
			if seg.index < 0 || seg.index >= len(vv) {
				return nil, false
			}
			cur = vv[seg.index]
			continue
		}
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := mm[seg.key]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// ValidatePath validates projection path syntax.
func ValidatePath(path string) error {
	_, err := parsePath(path)
	return err
}

type pathSegment struct {
	key     string
	index   int
	isIndex bool
}

func parsePath(path string) ([]pathSegment, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is empty")
	}
	segments := make([]pathSegment, 0, 4)
	i := 0
	for i < len(path) {
		switch path[i] {
		case '.':
			return nil, fmt.Errorf("invalid path %q: empty segment", path)
		case '[':
			end := strings.IndexByte(path[i:], ']')
			if end <= 1 {
				return nil, fmt.Errorf("invalid path %q: malformed index", path)
			}
			end += i
			n, err := strconv.Atoi(path[i+1 : end])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid path %q: malformed index", path)
			}
			segments = append(segments, pathSegment{index: n, isIndex: true})
			i = end + 1
			if i < len(path) && path[i] == '.' {
				i++
			}
		default:
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' && path[i] != ']' {
				i++
			}
			key := strings.TrimSpace(path[start:i])
			if key == "" {
				return nil, fmt.Errorf("invalid path %q: empty segment", path)
			}
			segments = append(segments, pathSegment{key: key})
			if i < len(path) && path[i] == '.' {
				i++
				if i >= len(path) {
					return nil, fmt.Errorf("invalid path %q: trailing dot", path)
				}
			}
			if i < len(path) && path[i] == ']' {
				return nil, fmt.Errorf("invalid path %q: unexpected ']'", path)
			}
		}
	}
	return segments, nil
}

func normalizeValue(v any) any {
	switch vv := v.(type) {
	case map[string]any, []any:
		b, err := json.Marshal(vv)
		if err != nil {
			return fmt.Sprintf("%v", vv)
		}
		return string(b)
	default:
		return vv
	}
}

// SortedColumns returns deterministic schema output columns.
func SortedColumns(schema map[string]string) []string {
	keys := make([]string, 0, len(schema))
	for k := range schema {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ProjectValues projects column values from encoded JSON using schema map.
func ProjectValues(encoded []byte, schema map[string]string, columns []string) ([]any, error) {
	doc, err := DecodeObject(encoded)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(columns))
	for _, col := range columns {
		path := schema[col]
		v, ok := ExtractPath(doc, path)
		if !ok {
			out = append(out, nil)
			continue
		}
		out = append(out, normalizeValue(v))
	}
	return out, nil
}

// ProjectEncoded projects encoded JSON into a new JSON object using schema map.
func ProjectEncoded(encoded []byte, schema map[string]string, columns []string) ([]byte, error) {
	doc, err := DecodeObject(encoded)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(columns))
	for _, col := range columns {
		path := schema[col]
		v, ok := ExtractPath(doc, path)
		if !ok {
			continue
		}
		out[col] = v
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b, nil
}
