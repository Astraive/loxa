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
	KeyKindPublic KeyKind = "pub"   // lx_pub_live_kxxx_yyyy
	KeyKindSecret KeyKind = "sec"   // lx_sec_live_kxxx_yyyy
	KeyKindLocal  KeyKind = "local" // lx_local_dev_yyyy (dev only)
)

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
// Format: lx_{kind}_{env}_{key_id}_{secret}
// key_id is "k" + base64 token (no underscore between k and token).
// Examples:
//
//	lx_sec_live_k2M9aQpXy_7QmVxN8pT4zRbK1sYw
//	lx_pub_live_kabc12345_xxxxx
//	lx_local_dev_yyyy
func ParseKey(raw string) (*ParsedKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty api key")
	}

	parts := strings.Split(raw, "_")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid api key format: expected lx_{kind}_{env}_...")
	}

	if parts[0] != "lx" {
		return nil, fmt.Errorf("invalid api key prefix: expected 'lx', got %q", parts[0])
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
		// Local keys: lx_local_dev_{secret} (remaining parts form the secret)
		if len(parts) < 4 {
			return nil, fmt.Errorf("local key missing secret")
		}
		pk.Secret = strings.Join(parts[3:], "_")
		return pk, nil
	}

	// Public/secret keys: lx_{kind}_{env}_{key_id}_{secret}
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

// CompareSecret performs constant-time comparison of two secret hashes.
func CompareSecret(incoming, stored []byte) bool {
	return subtle.ConstantTimeCompare(incoming, stored) == 1
}
