package bench

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/auth"
)

var benchServerSecret = []byte("bench-server-secret-key-32bytes-long!")

// benchKeyStore is a minimal in-memory KeyStore for benchmarks.
type benchKeyStore struct {
	record *auth.KeyRecord
}

func (s *benchKeyStore) FindByKeyID(_ context.Context, _ string) (*auth.KeyRecord, error) {
	return s.record, nil
}

func makeBenchKeyRecord() *auth.KeyRecord {
	hash := auth.HashSecret("bench_secret_value", benchServerSecret)
	return &auth.KeyRecord{
		ID:           "rec_bench",
		OrgID:        "org_bench",
		ProjectID:    "proj_bench",
		KeyID:        "kBenchKey",
		SecretHash:   hash,
		Kind:         auth.KeyKindSecret,
		Roles:        []auth.Role{auth.RoleIngestServer},
		AllowedEnvs:     []string{"live", "test"},
		AllowedServices: []string{"bench-service"},
		MaxPayloadBytes:     262144,
		MaxRequestsPerMinute: 999999999,
		MaxEventsPerMinute:  999999999,
	}
}

// BenchmarkAuthParseKey measures API key parsing.
func BenchmarkAuthParseKey(b *testing.B) {
	raw := "lx_sec_live_kBenchKey_bench_secret_value"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := auth.ParseKey(raw)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAuthHMAC measures HMAC-SHA256 secret hashing.
func BenchmarkAuthHMAC(b *testing.B) {
	secret := "bench_secret_value"
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		auth.HashSecret(secret, benchServerSecret)
	}
}

// BenchmarkAuthHMACDirect measures raw HMAC without the auth package.
func BenchmarkAuthHMACDirect(b *testing.B) {
	secret := []byte("bench_secret_value")
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mac := hmac.New(sha256.New, benchServerSecret)
		mac.Write(secret)
		mac.Sum(nil)
	}
}

// BenchmarkAuthMiddlewareHit measures auth middleware with a cache hit.
func BenchmarkAuthMiddlewareHit(b *testing.B) {
	record := makeBenchKeyRecord()
	store := &benchKeyStore{record: record}
	cache := auth.NewMemoryKeyCache(60*time.Second, 10*time.Second)
	cache.Set(record.KeyID, record, 60*time.Second)

	middleware := auth.Middleware(store, cache, benchServerSecret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/events", bytes.NewReader([]byte(`[{"event_name":"test"}]`)))
		req.Header.Set("Authorization", "Bearer lx_sec_live_kBenchKey_bench_secret_value")
		req.Header.Set("X-Loxa-Service", "bench-service")
		req.Header.Set("X-Loxa-Env", "live")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			b.Fatalf("expected 202, got %d", rec.Code)
		}
	}
}

// BenchmarkAuthMiddlewareMiss measures auth middleware with a cache miss (store lookup).
func BenchmarkAuthMiddlewareMiss(b *testing.B) {
	record := makeBenchKeyRecord()
	store := &benchKeyStore{record: record}
	cache := auth.NewMemoryKeyCache(60*time.Second, 10*time.Second) // empty cache

	middleware := auth.Middleware(store, cache, benchServerSecret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Invalidate(record.KeyID) // force miss every time
		req := httptest.NewRequest("POST", "/events", bytes.NewReader([]byte(`[{"event_name":"test"}]`)))
		req.Header.Set("Authorization", "Bearer lx_sec_live_kBenchKey_bench_secret_value")
		req.Header.Set("X-Loxa-Service", "bench-service")
		req.Header.Set("X-Loxa-Env", "live")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			b.Fatalf("expected 202, got %d", rec.Code)
		}
	}
}

// BenchmarkAuthLocalKey measures auth middleware with a local dev key (no store lookup).
func BenchmarkAuthLocalKey(b *testing.B) {
	store := &benchKeyStore{}
	cache := auth.NewMemoryKeyCache(60*time.Second, 10*time.Second)

	middleware := auth.Middleware(store, cache, benchServerSecret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/events", bytes.NewReader([]byte(`[{"event_name":"test"}]`)))
		req.Header.Set("Authorization", "Bearer lx_local_dev_mydevtoken")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			b.Fatalf("expected 202, got %d", rec.Code)
		}
	}
}

// BenchmarkAuthHexEncode measures hex encoding (used in key generation).
func BenchmarkAuthHexEncode(b *testing.B) {
	data := make([]byte, 32)
	for i := range data {
		data[i] = byte(i)
	}
	dst := make([]byte, hex.EncodedLen(len(data)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hex.Encode(dst, data)
	}
}
