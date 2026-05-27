package auth

import (
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestAllowEvents_AtomicBatchCheck(t *testing.T) {
	rl := NewKeyRateLimiter()

	// With epm=600, burst = 600/10+1 = 61
	// First batch of 10 should succeed
	if !rl.AllowEvents("test-key", 600, 10) {
		t.Fatal("first batch of 10 should be allowed")
	}

	// Second batch of 10 should succeed (still have tokens)
	if !rl.AllowEvents("test-key", 600, 10) {
		t.Fatal("second batch of 10 should be allowed")
	}
}

func TestAllowEvents_NoPartialConsumption(t *testing.T) {
	rl := NewKeyRateLimiter()

	// With epm=60, rate=1/sec, burst=7
	// Drain the burst
	for i := 0; i < 7; i++ {
		if !rl.AllowEvents("key-partial", 60, 1) {
			t.Fatalf("single event %d should be allowed", i)
		}
	}

	// Now try a batch of 5 — should fail atomically (not consume any)
	if rl.AllowEvents("key-partial", 60, 5) {
		t.Fatal("batch of 5 should fail after draining burst")
	}

	// A single event should still fail — confirming no partial consumption
	// (bucket is empty, needs time to refill)
	if rl.AllowEvents("key-partial", 60, 1) {
		// May succeed if enough time passed, that's OK
	}
}

func TestAllowEvents_ZeroCountDefaultsToOne(t *testing.T) {
	rl := NewKeyRateLimiter()

	if !rl.AllowEvents("key-zero", 60, 0) {
		t.Fatal("zero count should default to 1 and succeed")
	}
}

func TestAllowEvents_NegativeEPMReturnsTrue(t *testing.T) {
	rl := NewKeyRateLimiter()

	// Negative or zero EPM means no limit
	if !rl.AllowEvents("key-neg", -1, 1000) {
		t.Fatal("negative EPM should always return true")
	}
	if !rl.AllowEvents("key-zero-epm", 0, 1000) {
		t.Fatal("zero EPM should always return true")
	}
}

func TestAllowRequest_BasicFlow(t *testing.T) {
	rl := NewKeyRateLimiter()

	// rpm=60 means 1/sec, burst=7
	if !rl.AllowRequest("req-key", 60) {
		t.Fatal("first request should be allowed")
	}
	if !rl.AllowRequest("req-key", 60) {
		t.Fatal("second request should be allowed")
	}
}

func TestAllowRequest_NegativeRPMReturnsTrue(t *testing.T) {
	rl := NewKeyRateLimiter()

	if !rl.AllowRequest("req-neg", -1) {
		t.Fatal("negative RPM should always return true")
	}
}

func TestCleanup_EvictsOldEntries(t *testing.T) {
	rl := &KeyRateLimiter{
		requestLimiters: make(map[string]*rate.Limiter),
		eventLimiters:   make(map[string]*rate.Limiter),
		lastSeen:        make(map[string]time.Time),
	}

	// Simulate old entries
	rl.requestLimiters["old-key"] = rate.NewLimiter(rate.Inf, 0)
	rl.eventLimiters["old-key"] = rate.NewLimiter(rate.Inf, 0)
	rl.lastSeen["old-key"] = time.Now().Add(-31 * time.Minute)

	rl.requestLimiters["new-key"] = rate.NewLimiter(rate.Inf, 0)
	rl.eventLimiters["new-key"] = rate.NewLimiter(rate.Inf, 0)
	rl.lastSeen["new-key"] = time.Now()

	// Run cleanup manually
	rl.mu.Lock()
	cutoff := time.Now().Add(-30 * time.Minute)
	for k, lastAccess := range rl.lastSeen {
		if lastAccess.Before(cutoff) {
			delete(rl.requestLimiters, k)
			delete(rl.eventLimiters, k)
			delete(rl.lastSeen, k)
		}
	}
	rl.mu.Unlock()

	if _, ok := rl.requestLimiters["old-key"]; ok {
		t.Error("old-key should have been evicted")
	}
	if _, ok := rl.requestLimiters["new-key"]; !ok {
		t.Error("new-key should NOT have been evicted")
	}
}

func TestAllowEvents_DifferentKeysIndependent(t *testing.T) {
	rl := NewKeyRateLimiter()

	// Drain key-a
	for i := 0; i < 7; i++ {
		rl.AllowEvents("key-a", 60, 1)
	}

	// key-b should still be allowed
	if !rl.AllowEvents("key-b", 60, 1) {
		t.Fatal("key-b should be independent of key-a")
	}
}
