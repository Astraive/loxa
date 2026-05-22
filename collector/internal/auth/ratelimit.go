package auth

import (
	"sync"

	"golang.org/x/time/rate"
)

// KeyRateLimiter provides per-key rate limiting using token bucket algorithm.
// It supports both request-level (requests/min) and event-level (events/min) limits.
type KeyRateLimiter struct {
	requestLimiters map[string]*rate.Limiter
	eventLimiters   map[string]*rate.Limiter
	mu              sync.RWMutex
}

// NewKeyRateLimiter creates a new per-key rate limiter.
func NewKeyRateLimiter() *KeyRateLimiter {
	return &KeyRateLimiter{
		requestLimiters: make(map[string]*rate.Limiter),
		eventLimiters:   make(map[string]*rate.Limiter),
	}
}

// AllowRequest checks if a request is allowed for the given key.
// rpm is the max requests per minute.
func (rl *KeyRateLimiter) AllowRequest(keyID string, rpm int) bool {
	if rpm <= 0 {
		return true // no limit
	}

	limiter := rl.getOrCreateRequestLimiter(keyID, rpm)
	return limiter.Allow()
}

// AllowEvents checks if the given number of events is allowed for the key.
// epm is the max events per minute.
func (rl *KeyRateLimiter) AllowEvents(keyID string, epm int) bool {
	if epm <= 0 {
		return true // no limit
	}

	limiter := rl.getOrCreateEventLimiter(keyID, epm)
	return limiter.Allow()
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
	return limiter
}
