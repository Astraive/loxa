package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

// KeyKind represents the type of API key.
type KeyKind string

const (
	KeyKindPublic KeyKind = "pub"   // lz_pub_live_kxxx_yyyy
	KeyKindSecret KeyKind = "sec"   // lz_sec_live_kxxx_yyyy
	KeyKindLocal  KeyKind = "local" // lz_local_dev_yyyy (dev only)
	KeyKindToken  KeyKind = "token" // opaque Bearer token, stored by HMAC-derived ID
)

const (
	publicAccessIDPrefix   = "lz_pub_"
	minPublicAccessIDToken = 32
	maxPublicAccessIDToken = 128
)

// IsPublicAccessID reports whether id is a valid opaque public Basic
// credential. The random component is intentionally constrained to URL-safe
// characters and a minimum entropy-bearing length; callers must never include
// an invalid value in an error because the entire ID is a bearer capability.
func IsPublicAccessID(id string) bool {
	if !strings.HasPrefix(id, publicAccessIDPrefix) {
		return false
	}
	token := id[len(publicAccessIDPrefix):]
	if len(token) < minPublicAccessIDToken || len(token) > maxPublicAccessIDToken {
		return false
	}
	for i := range len(token) {
		c := token[i]
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ParsedKey holds the components of a parsed LOZA API key.
type ParsedKey struct {
	Raw    string
	Kind   KeyKind
	Env    string // "live", "test", "dev"
	KeyID  string // e.g. "k2M9aQpXy" (no underscore between prefix and token)
	Secret string // e.g. "7QmVxN8pT4zRbK1sYw"
}

// ParseKey parses a LOZA API key into its components.
//
// Format: lz_{kind}_{env}_{key_id}_{secret}
// key_id is "k" + base64 token (no underscore between k and token).
// Examples:
//
//	lz_sec_live_k2M9aQpXy_7QmVxN8pT4zRbK1sYw
//	lz_pub_live_kabc12345_xxxxx
//	lz_local_dev_yyyy
func ParseKey(raw string) (*ParsedKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty api key")
	}

	parts := strings.Split(raw, "_")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid api key format: expected lz_{kind}_{env}_...")
	}

	if parts[0] != "lz" {
		return nil, fmt.Errorf("invalid api key prefix: expected 'lz', got %q", parts[0])
	}

	kind := KeyKind(parts[1])
	switch kind {
	case KeyKindPublic, KeyKindSecret, KeyKindLocal:
		// valid
	default:
		return nil, fmt.Errorf("invalid api key kind: %q", parts[1])
	}

	env := parts[2]
	switch env {
	case "live", "test", "dev":
		// valid
	default:
		return nil, fmt.Errorf("invalid api key env: %q", parts[2])
	}

	pk := &ParsedKey{
		Raw:  raw,
		Kind: kind,
		Env:  env,
	}

	if kind == KeyKindLocal {
		// Local keys: lz_local_dev_{secret} (remaining parts form the secret)
		if len(parts) < 4 {
			return nil, fmt.Errorf("local key missing secret")
		}
		pk.Secret = strings.Join(parts[3:], "_")
		return pk, nil
	}

	// Public/secret keys: lz_{kind}_{env}_{key_id}_{secret}
	// key_id is always a single segment: "k" + base64 token (e.g., "k2M9aQpXy")
	// secret is everything after key_id
	if len(parts) < 5 {
		return nil, fmt.Errorf("api key missing key_id or secret")
	}

	pk.KeyID = parts[3]
	pk.Secret = strings.Join(parts[4:], "_")

	if pk.KeyID == "" {
		return nil, fmt.Errorf("empty key_id")
	}
	if pk.Secret == "" {
		return nil, fmt.Errorf("empty secret")
	}

	return pk, nil
}

// HashSecret computes HMAC-SHA256 of the secret using the server secret.
func HashSecret(secret string, serverSecret []byte) []byte {
	mac := hmac.New(sha256.New, serverSecret)
	mac.Write([]byte(secret))
	return mac.Sum(nil)
}

// TokenLookupID derives the opaque store identifier for a bearer token. The
// token itself is never used as a map key or logged; the server secret makes
// the identifier unguessable and prevents offline enumeration.
func TokenLookupID(token string, serverSecret []byte) string {
	return "tok_" + fmt.Sprintf("%x", HashSecret(token, serverSecret))
}

// CompareSecret performs constant-time comparison of two secret hashes.
func CompareSecret(incoming, stored []byte) bool {
	return subtle.ConstantTimeCompare(incoming, stored) == 1
}
