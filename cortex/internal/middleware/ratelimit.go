package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	perAPIKeyRPM   int
	perIPRPM       int
	apiKeys        map[string]*clientLimiter
	ips            map[string]*clientLimiter
	mu             sync.Mutex
	cleanupTick    *time.Ticker
	trustedProxies []*net.IPNet
}

// RateLimiterOption configures the RateLimiter.
type RateLimiterOption func(*RateLimiter)

// WithTrustedProxies sets the list of trusted proxy CIDRs for the rate limiter.
// When set, X-Forwarded-For is trusted from these IPs to determine the real client IP.
func WithTrustedProxies(proxies []*net.IPNet) RateLimiterOption {
	return func(rl *RateLimiter) { rl.trustedProxies = proxies }
}

func NewRateLimiter(perAPIKeyRPM, perIPRPM int, opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		perAPIKeyRPM: perAPIKeyRPM,
		perIPRPM:     perIPRPM,
		apiKeys:      make(map[string]*clientLimiter),
		ips:          make(map[string]*clientLimiter),
		cleanupTick:  time.NewTicker(5 * time.Minute),
	}
	for _, o := range opts {
		o(rl)
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

		// Rate limit by IP — use trusted proxy headers when applicable
		ip := rl.extractClientIP(r)
		ipLimiter := rl.getIPLimiter(ip)
		if !ipLimiter.Allow() {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":"rate limit exceeded","retry_after":1}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractClientIP determines the real client IP, respecting trusted proxies.
func (rl *RateLimiter) extractClientIP(r *http.Request) string {
	ipStr, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ipStr == "" {
		ipStr = r.RemoteAddr
	}

	remoteIP := net.ParseIP(ipStr)
	if remoteIP == nil {
		return ipStr
	}

	// Only trust X-Forwarded-For if RemoteAddr is a trusted proxy
	if len(rl.trustedProxies) > 0 && isTrustedProxyIP(remoteIP, rl.trustedProxies) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for _, part := range parts {
				candidate := net.ParseIP(strings.TrimSpace(part))
				if candidate != nil && !isTrustedProxyIP(candidate, rl.trustedProxies) {
					return candidate.String()
				}
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	return ipStr
}

func isTrustedProxyIP(ip net.IP, trusted []*net.IPNet) bool {
	for _, cidr := range trusted {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
