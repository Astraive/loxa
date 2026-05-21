package jsonenc

import "unicode/utf8"

// AppendEscapedString appends a JSON-encoded string (with quotes) to dst.
// It handles all JSON required escapes without reflect.
func AppendEscapedString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b < utf8.RuneSelf {
			if htmlSafeSet[b] {
				i++
				continue
			}
			if start < i {
				dst = append(dst, s[start:i]...)
			}
			dst = appendEscapedByte(dst, b)
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8: replace with unicode replacement character.
			if start < i {
				dst = append(dst, s[start:i]...)
			}
			dst = append(dst, `\ufffd`...)
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		dst = append(dst, s[start:]...)
	}
	dst = append(dst, '"')
	return dst
}

func appendEscapedByte(dst []byte, b byte) []byte {
	switch b {
	case '"':
		return append(dst, '\\', '"')
	case '\\':
		return append(dst, '\\', '\\')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	default:
		// Control character: encode as \u00XX
		return append(dst, '\\', 'u', '0', '0', hexChar(b>>4), hexChar(b&0xf))
	}
}

func hexChar(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

// htmlSafeSet marks bytes that do NOT need escaping inside a JSON string.
// All control chars (< 0x20), '"', '\', and bytes >= 0x80 (handled separately)
// are false; everything else is true (safe to include as-is).
var htmlSafeSet = buildHTMLSafeSet()

func buildHTMLSafeSet() [utf8.RuneSelf]bool {
	var t [utf8.RuneSelf]bool
	for i := range t {
		// safe if printable ASCII, not control, not special JSON char
		t[i] = i >= 0x20 && i != '"' && i != '\\'
	}
	return t
}
