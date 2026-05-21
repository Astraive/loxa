package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	perAPIKeyRPM int
	perIPRPM     int
	apiKeys      map[string]*clientLimiter
	ips          map[string]*clientLimiter
	mu           sync.Mutex
	cleanupTick  *time.Ticker
}

func NewRateLimiter(perAPIKeyRPM, perIPRPM int) *RateLimiter {
	rl := &RateLimiter{
		perAPIKeyRPM: perAPIKeyRPM,
		perIPRPM:     perIPRPM,
		apiKeys:      make(map[string]*clientLimiter),
		ips:          make(map[string]*clientLimiter),
		cleanupTick:  time.NewTicker(5 * time.Minute),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getAPIKeyLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cl, exists := rl.apiKeys[key]
	if !exists {
		rps := rate.Limit(float64(rl.perAPIKeyRPM) / 60.0)
		cl = &clientLimiter{
			limiter:  rate.NewLimiter(rps, rl.perAPIKeyRPM/10+1),
			lastSeen: time.Now(),
		}
		rl.apiKeys[key] = cl
	}
	cl.lastSeen = time.Now()
	return cl.limiter
}

func (rl *RateLimiter) getIPLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cl, exists := rl.ips[ip]
	if !exists {
		rps := rate.Limit(float64(rl.perIPRPM) / 60.0)
		cl = &clientLimiter{
			limiter:  rate.NewLimiter(rps, rl.perIPRPM/10+1),
			lastSeen: time.Now(),
		}
		rl.ips[ip] = cl
	}
	cl.lastSeen = time.Now()
	return cl.limiter
}

func (rl *RateLimiter) cleanup() {
	for range rl.cleanupTick.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for key, cl := range rl.apiKeys {
			if cl.lastSeen.Before(cutoff) {
				delete(rl.apiKeys, key)
			}
		}
		for ip, cl := range rl.ips {
			if cl.lastSeen.Before(cutoff) {
				delete(rl.ips, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rate limit by API key first
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.Header.Get("Authorization")
		}
		if apiKey != "" {
			limiter := rl.getAPIKeyLimiter(apiKey)
			if !limiter.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, `{"error":"rate limit exceeded","retry_after":1}`, http.StatusTooManyRequests)
				return
			}
		}

		// Rate limit by IP
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		ipLimiter := rl.getIPLimiter(ip)
		if !ipLimiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded","retry_after":1}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
