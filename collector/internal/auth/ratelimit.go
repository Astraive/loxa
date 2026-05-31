package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// KeyRateLimiter provides per-key rate limiting using token bucket algorithm.
// It supports both request-level (requests/min) and event-level (events/min) limits.
type KeyRateLimiter struct {
	requestLimiters map[string]*rate.Limiter
	eventLimiters   map[string]*rate.Limiter
	lastSeen        map[string]time.Time
	mu              sync.RWMutex
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewKeyRateLimiter creates a new per-key rate limiter.
func NewKeyRateLimiter() *KeyRateLimiter {
	rl := &KeyRateLimiter{
		requestLimiters: make(map[string]*rate.Limiter),
		eventLimiters:   make(map[string]*rate.Limiter),
		lastSeen:        make(map[string]time.Time),
		stopCh:          make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Close stops the background cleanup goroutine.
func (rl *KeyRateLimiter) Close() {
	rl.stopOnce.Do(func() {
		close(rl.stopCh)
	})
}

// cleanupLoop evicts entries not seen in the last 30 minutes.
func (rl *KeyRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute)
			for k, t := range rl.lastSeen {
				if t.Before(cutoff) {
					delete(rl.requestLimiters, k)
					delete(rl.eventLimiters, k)
					delete(rl.lastSeen, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// AllowRequest checks if a request is allowed for the given key.
// rpm is the max requests per minute.
func (rl *KeyRateLimiter) AllowRequest(keyID string, rpm int) bool {
	if rpm <= 0 {
		return true // no limit
	}

	limiter := rl.getOrCreateRequestLimiter(keyID, rpm)
	rl.touchLastSeen(keyID)
	return limiter.Allow()
}

// AllowEvents checks if the given number of events is allowed for the key.
// epm is the max events per minute. count is the number of events in this request.
// Uses ReserveN for atomic batch check — no partial token consumption on rejection.
func (rl *KeyRateLimiter) AllowEvents(keyID string, epm int, count int) bool {
	if epm <= 0 {
		return true // no limit
	}
	if count <= 0 {
		count = 1
	}

	limiter := rl.getOrCreateEventLimiter(keyID, epm)
	rl.touchLastSeen(keyID)

	// Atomic reservation: either all tokens are available or none are consumed.
	r := limiter.ReserveN(time.Now(), count)
	if !r.OK() {
		return false
	}
	if d := r.Delay(); d > 0 {
		r.Cancel() // Cancel the reservation since we're rejecting
		return false
	}
	return true
}

// touchLastSeen updates the last-access time for a key.
func (rl *KeyRateLimiter) touchLastSeen(keyID string) {
	rl.mu.Lock()
	rl.lastSeen[keyID] = time.Now()
	rl.mu.Unlock()
}

func (rl *KeyRateLimiter) getOrCreateRequestLimiter(keyID string, rpm int) *rate.Limiter {
	rl.mu.RLock()
	limiter, ok := rl.requestLimiters[keyID]
	rl.mu.RUnlock()
	if ok {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, ok := rl.requestLimiters[keyID]; ok {
		return limiter
	}

	// Convert rpm to rate per second
	rps := rate.Limit(float64(rpm) / 60.0)
	limiter = rate.NewLimiter(rps, rpm/10+1) // burst = 10% of rpm + 1
	rl.requestLimiters[keyID] = limiter
	rl.lastSeen[keyID] = time.Now()
	return limiter
}

func (rl *KeyRateLimiter) getOrCreateEventLimiter(keyID string, epm int) *rate.Limiter {
	rl.mu.RLock()
	limiter, ok := rl.eventLimiters[keyID]
	rl.mu.RUnlock()
	if ok {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, ok := rl.eventLimiters[keyID]; ok {
		return limiter
	}

	eps := rate.Limit(float64(epm) / 60.0)
	limiter = rate.NewLimiter(eps, epm/10+1)
	rl.eventLimiters[keyID] = limiter
	rl.lastSeen[keyID] = time.Now()
	return limiter
}
