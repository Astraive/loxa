package auth

import (
	"sync"
	"time"
)

type cacheEntry struct {
	record   *KeyRecord
	expires  time.Time
	negative bool // cache miss entry (short TTL)
}

// MemoryKeyCache is a TTL-based in-memory cache for API key lookups.
type MemoryKeyCache struct {
	entries          map[string]*cacheEntry
	mu               sync.RWMutex
	defaultTTL       time.Duration
	negativeTTL      time.Duration
	cleanupInterval  time.Duration
	stopCleanup      chan struct{}
	cleanupWG        sync.WaitGroup
}

// NewMemoryKeyCache creates a new cache with the given TTLs.
// Positive entries: defaultTTL. Negative entries (misses): negativeTTL.
func NewMemoryKeyCache(defaultTTL, negativeTTL time.Duration) *MemoryKeyCache {
	c := &MemoryKeyCache{
		entries:         make(map[string]*cacheEntry),
		defaultTTL:      defaultTTL,
		negativeTTL:     negativeTTL,
		cleanupInterval: defaultTTL,
		stopCleanup:     make(chan struct{}),
	}
	c.cleanupWG.Add(1)
	go c.cleanupLoop()
	return c
}

// Get retrieves a key record from the cache.
// Returns (record, true) on positive hit, (nil, false) on miss or negative cache hit.
func (c *MemoryKeyCache) Get(keyID string) (*KeyRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[keyID]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		return nil, false // expired
	}
	if entry.negative {
		return nil, false // negative cache hit
	}
	return entry.record, true
}

// Set stores a key record in the cache.
func (c *MemoryKeyCache) Set(keyID string, record *KeyRecord, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	c.entries[keyID] = &cacheEntry{
		record:  record,
		expires: time.Now().Add(ttl),
	}
}

// SetNegative stores a negative cache entry (key not found).
func (c *MemoryKeyCache) SetNegative(keyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[keyID] = &cacheEntry{
		expires:  time.Now().Add(c.negativeTTL),
		negative: true,
	}
}

// Invalidate removes a key from the cache.
func (c *MemoryKeyCache) Invalidate(keyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, keyID)
}

// Close stops the background cleanup goroutine and waits for it to exit.
func (c *MemoryKeyCache) Close() {
	close(c.stopCleanup)
	c.cleanupWG.Wait()
}

func (c *MemoryKeyCache) cleanupLoop() {
	defer c.cleanupWG.Done()
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

func (c *MemoryKeyCache) evictExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, v := range c.entries {
		if now.After(v.expires) {
			delete(c.entries, k)
		}
	}
}
