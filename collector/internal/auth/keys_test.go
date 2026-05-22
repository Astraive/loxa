package auth

import (
	"testing"
)

func TestParseKey_SecretKey(t *testing.T) {
	pk, err := ParseKey("lx_sec_live_k_2M9aQp_7QmVxN8pT4zRbK1sYw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pk.Kind != KeyKindSecret {
		t.Errorf("kind = %q, want %q", pk.Kind, KeyKindSecret)
	}
	if pk.Env != "live" {
		t.Errorf("env = %q, want %q", pk.Env, "live")
	}
	if pk.KeyID != "k" {
		t.Errorf("keyID = %q, want %q", pk.KeyID, "k")
	}
	if pk.Secret != "2M9aQp_7QmVxN8pT4zRbK1sYw" {
		t.Errorf("secret = %q, want %q", pk.Secret, "2M9aQp_7QmVxN8pT4zRbK1sYw")
	}
}

func TestParseKey_PublicKey(t *testing.T) {
	pk, err := ParseKey("lx_pub_live_kabc123_xxxxx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pk.Kind != KeyKindPublic {
		t.Errorf("kind = %q, want %q", pk.Kind, KeyKindPublic)
	}
	if pk.Env != "live" {
		t.Errorf("env = %q, want %q", pk.Env, "live")
	}
	if pk.KeyID != "kabc123" {
		t.Errorf("keyID = %q, want %q", pk.KeyID, "kabc123")
	}
}

func TestParseKey_LocalKey(t *testing.T) {
	pk, err := ParseKey("lx_local_dev_mydevtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pk.Kind != KeyKindLocal {
		t.Errorf("kind = %q, want %q", pk.Kind, KeyKindLocal)
	}
	if pk.Env != "dev" {
		t.Errorf("env = %q, want %q", pk.Env, "dev")
	}
	if pk.Secret != "mydevtoken" {
		t.Errorf("secret = %q, want %q", pk.Secret, "mydevtoken")
	}
}

func TestParseKey_TestEnv(t *testing.T) {
	pk, err := ParseKey("lx_sec_test_k_test123_secretvalue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pk.Env != "test" {
		t.Errorf("env = %q, want %q", pk.Env, "test")
	}
}

func TestParseKey_InvalidPrefix(t *testing.T) {
	_, err := ParseKey("xx_sec_live_k_xxx_yyy")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestParseKey_InvalidKind(t *testing.T) {
	_, err := ParseKey("lx_bad_live_k_xxx_yyy")
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestParseKey_InvalidEnv(t *testing.T) {
	_, err := ParseKey("lx_sec_staging_k_xxx_yyy")
	if err == nil {
		t.Fatal("expected error for invalid env")
	}
}

func TestParseKey_EmptyString(t *testing.T) {
	_, err := ParseKey("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestParseKey_MissingSecret(t *testing.T) {
	// "lx_sec_live_k_xxx" has 5 parts: lx, sec, live, k, xxx
	// This is valid (keyID=k, secret=xxx). Only < 5 parts is an error.
	_, err := ParseKey("lx_sec_live_k")
	if err == nil {
		t.Fatal("expected error for missing key_id and secret")
	}
}

func TestParseKey_TrimmedWhitespace(t *testing.T) {
	pk, err := ParseKey("  lx_sec_live_k_xxx_yyy  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// keyID = "k", secret = "xxx_yyy" (joined from remaining parts)
	if pk.Secret != "xxx_yyy" {
		t.Errorf("secret = %q, want %q", pk.Secret, "xxx_yyy")
	}
	if pk.KeyID != "k" {
		t.Errorf("keyID = %q, want %q", pk.KeyID, "k")
	}
}

func TestHashSecret_ConsistentOutput(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	secret := "my-test-secret"

	hash1 := HashSecret(secret, serverSecret)
	hash2 := HashSecret(secret, serverSecret)

	if len(hash1) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash1))
	}

	for i := range hash1 {
		if hash1[i] != hash2[i] {
			t.Fatalf("hash mismatch at byte %d", i)
		}
	}
}

func TestHashSecret_DifferentInputs(t *testing.T) {
	serverSecret := []byte("test-server-secret")

	hash1 := HashSecret("secret1", serverSecret)
	hash2 := HashSecret("secret2", serverSecret)

	same := true
	for i := range hash1 {
		if hash1[i] != hash2[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("expected different hashes for different inputs")
	}
}

func TestCompareSecret_Match(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	secret := "my-test-secret"

	hash := HashSecret(secret, serverSecret)
	if !CompareSecret(hash, hash) {
		t.Fatal("expected CompareSecret to return true for matching hashes")
	}
}

func TestCompareSecret_Mismatch(t *testing.T) {
	serverSecret := []byte("test-server-secret")

	hash1 := HashSecret("secret1", serverSecret)
	hash2 := HashSecret("secret2", serverSecret)

	if CompareSecret(hash1, hash2) {
		t.Fatal("expected CompareSecret to return false for non-matching hashes")
	}
}
