package auth

import (
	"testing"
	"time"
)

func TestMemoryKeyCache_SetAndGet(t *testing.T) {
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	record := &KeyRecord{
		KeyID:  "k_test123",
		OrgID:  "org_1",
		Kind:   KeyKindSecret,
		Roles:  []Role{RoleIngestServer},
	}

	cache.Set("k_test123", record, 0)

	got, ok := cache.Get("k_test123")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.KeyID != "k_test123" {
		t.Errorf("keyID = %q, want %q", got.KeyID, "k_test123")
	}
	if got.OrgID != "org_1" {
		t.Errorf("orgID = %q, want %q", got.OrgID, "org_1")
	}
}

func TestMemoryKeyCache_Miss(t *testing.T) {
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestMemoryKeyCache_Expiration(t *testing.T) {
	cache := NewMemoryKeyCache(50*time.Millisecond, 10*time.Millisecond)
	defer cache.Close()

	record := &KeyRecord{KeyID: "k_expire"}
	cache.Set("k_expire", record, 50*time.Millisecond)

	// Should hit immediately
	_, ok := cache.Get("k_expire")
	if !ok {
		t.Fatal("expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	_, ok = cache.Get("k_expire")
	if ok {
		t.Fatal("expected cache miss after expiration")
	}
}

func TestMemoryKeyCache_NegativeEntry(t *testing.T) {
	cache := NewMemoryKeyCache(10*time.Second, 50*time.Millisecond)
	defer cache.Close()

	cache.SetNegative("k_missing")

	// Negative entry should return miss
	_, ok := cache.Get("k_missing")
	if ok {
		t.Fatal("expected miss for negative cache entry")
	}
}

func TestMemoryKeyCache_Invalidate(t *testing.T) {
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	record := &KeyRecord{KeyID: "k_invalidate"}
	cache.Set("k_invalidate", record, 0)

	_, ok := cache.Get("k_invalidate")
	if !ok {
		t.Fatal("expected cache hit before invalidation")
	}

	cache.Invalidate("k_invalidate")

	_, ok = cache.Get("k_invalidate")
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestMemoryKeyCache_Overwrite(t *testing.T) {
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	cache.Set("k_overwrite", &KeyRecord{KeyID: "k_overwrite", OrgID: "org_1"}, 0)
	cache.Set("k_overwrite", &KeyRecord{KeyID: "k_overwrite", OrgID: "org_2"}, 0)

	got, ok := cache.Get("k_overwrite")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.OrgID != "org_2" {
		t.Errorf("orgID = %q, want %q", got.OrgID, "org_2")
	}
}
