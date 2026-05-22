package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// KeyRecord holds the stored metadata for an API key.
type KeyRecord struct {
	ID           string
	OrgID        string
	ProjectID    string
	KeyID        string
	SecretHash   []byte
	Kind         KeyKind
	Roles        []Role
	AllowedEnvs     []string
	AllowedServices []string
	AllowedOrigins  []string
	AllowedIPs      []string
	MaxPayloadBytes     int
	MaxRequestsPerMinute int
	MaxEventsPerMinute  int
	SamplingRate        float64
	AllowPII            bool
	AllowAttachments    bool
	RevokedAt       *time.Time
	ExpiresAt       *time.Time
}

// KeyStore is the interface for key storage backends.
type KeyStore interface {
	FindByKeyID(ctx context.Context, keyID string) (*KeyRecord, error)
}

// Middleware returns an http.Handler middleware that validates API keys
// and attaches an AuthContext to the request. It does NOT check permissions
// — that's done per-route via RequirePermission.
//
// Validation flow:
//  1. Parse Authorization header
//  2. ParseKey → prefix, key_id, secret
//  3. Cache lookup by key_id
//  4. If cache miss: KeyStore.FindByKeyID()
//  5. Check revoked / expired
//  6. Verify HMAC-SHA256(incoming secret) == stored hash
//  7. Build AuthContext (org, project, roles, permissions)
//  8. Check X-Loxa-Env against AllowedEnvs
//  9. Check X-Loxa-Service against AllowedServices
//  10. Check Origin against AllowedOrigins (public keys)
//  11. Check remote IP against AllowedIPs
//  12. Wrap body with MaxBytesReader if MaxPayloadBytes > 0
//  13. Apply per-key rate limit
//  14. Attach AuthContext to request context
//  15. Call next handler
func Middleware(store KeyStore, cache *MemoryKeyCache, serverSecret []byte) func(http.Handler) http.Handler {
	rateLimiter := NewKeyRateLimiter()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, errCode, errReason := authenticate(r, store, cache, serverSecret, rateLimiter)
			if ac == nil {
				w.Header().Set("X-Auth-Failure-Reason", errReason)
				w.Header().Set("X-Auth-Failure-Code", errCode)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Wrap body with MaxBytesReader if configured
			if ac.MaxPayloadBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, int64(ac.MaxPayloadBytes))
			}

			ctx := WithAuthContext(r.Context(), ac)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission wraps a handler with a permission check.
// Returns 403 Forbidden if the auth context lacks the required permission.
// RequirePermission wraps a handler with a permission check.
// Returns 403 Forbidden if the auth context lacks the required permission.
func RequirePermission(next http.Handler, perm Permission) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil || !ac.HasPermission(perm) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MakeRouteProtector returns a RouteProtector compatible with httpserver.BuildMux.
// It wraps each route with RequirePermission, using the permission name as a Permission type.
func MakeRouteProtector() func(next http.Handler, perm string) http.Handler {
	return func(next http.Handler, perm string) http.Handler {
		return RequirePermission(next, Permission(perm))
	}
}

func authenticate(r *http.Request, store KeyStore, cache *MemoryKeyCache, serverSecret []byte, rateLimiter *KeyRateLimiter) (*AuthContext, string, string) {
	// 1. Parse Authorization header
	raw := extractBearerToken(r)
	if raw == "" {
		return nil, "missing_token", "missing Authorization header"
	}

	// 2. Parse key
	parsed, err := ParseKey(raw)
	if err != nil {
		return nil, "invalid_key_format", err.Error()
	}

	// Handle local dev keys
	if parsed.Kind == KeyKindLocal {
		return &AuthContext{
			KeyKind:     KeyKindLocal,
			Permissions: ExpandRoles([]Role{RoleIngestServer}),
			Roles:       []Role{RoleIngestServer},
		}, "", ""
	}

	// 3. Cache lookup
	var record *KeyRecord
	if cache != nil {
		if cached, ok := cache.Get(parsed.KeyID); ok {
			record = cached
		}
	}

	// 4. Store lookup on cache miss
	if record == nil {
		var err error
		record, err = store.FindByKeyID(r.Context(), parsed.KeyID)
		if err != nil || record == nil {
			if cache != nil {
				cache.SetNegative(parsed.KeyID)
			}
			return nil, "key_not_found", "invalid api key"
		}
		if cache != nil {
			cache.Set(parsed.KeyID, record, 0) // default TTL
		}
	}

	// 5. Check revoked / expired
	if record.RevokedAt != nil {
		return nil, "key_revoked", "api key has been revoked"
	}
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return nil, "key_expired", "api key has expired"
	}

	// 6. Verify prefix matches
	if string(record.Kind) != string(parsed.Kind) {
		return nil, "key_kind_mismatch", "api key kind mismatch"
	}

	// 7. Verify secret hash
	incomingHash := HashSecret(parsed.Secret, serverSecret)
	if !CompareSecret(incomingHash, record.SecretHash) {
		return nil, "invalid_secret", "invalid api key"
	}

	// 8. Build AuthContext
	permissions := ExpandRoles(record.Roles)
	ac := &AuthContext{
		OrgID:               record.OrgID,
		ProjectID:           record.ProjectID,
		APIKeyID:            record.KeyID,
		KeyKind:             record.Kind,
		Roles:               record.Roles,
		Permissions:         permissions,
		AllowedEnvs:         record.AllowedEnvs,
		AllowedServices:     record.AllowedServices,
		AllowedOrigins:      record.AllowedOrigins,
		AllowedIPs:          record.AllowedIPs,
		MaxPayloadBytes:     record.MaxPayloadBytes,
		MaxRequestsPerMinute: record.MaxRequestsPerMinute,
		MaxEventsPerMinute:  record.MaxEventsPerMinute,
		SamplingRate:        record.SamplingRate,
		AllowPII:            record.AllowPII,
		AllowAttachments:    record.AllowAttachments,
	}

	// Public key strict defaults
	if record.Kind == KeyKindPublic {
		ac.AllowPII = false
		ac.AllowAttachments = false
	}

	// 9. Check env restriction
	if len(ac.AllowedEnvs) > 0 {
		env := r.Header.Get("X-Loxa-Env")
		if !contains(ac.AllowedEnvs, env) {
			return nil, "env_not_allowed", "environment not permitted for this key"
		}
	}

	// 10. Check service restriction
	if len(ac.AllowedServices) > 0 {
		svc := r.Header.Get("X-Loxa-Service")
		if !contains(ac.AllowedServices, svc) {
			return nil, "service_not_allowed", "service not permitted for this key"
		}
	}

	// 11. Check origin restriction (for public keys)
	if len(ac.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if origin != "" && !contains(ac.AllowedOrigins, origin) {
			return nil, "origin_not_allowed", "origin not permitted"
		}
	}

	// 12. Check IP restriction
	if len(ac.AllowedIPs) > 0 {
		remoteIP := extractIP(r)
		if !ipAllowed(remoteIP, ac.AllowedIPs) {
			return nil, "ip_not_allowed", "ip address not permitted"
		}
	}

	// 13. Rate limiting
	if rateLimiter != nil && ac.MaxRequestsPerMinute > 0 {
		if !rateLimiter.AllowRequest(ac.APIKeyID, ac.MaxRequestsPerMinute) {
			return nil, "rate_limited", "request rate limit exceeded"
		}
	}

	return ac, "", ""
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	// Also check X-API-Key header for backward compat
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func extractIP(r *http.Request) string {
	// X-Forwarded-For takes precedence
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func ipAllowed(ip string, allowed []string) bool {
	for _, a := range allowed {
		if a == ip {
			return true
		}
		// Check CIDR
		if strings.Contains(a, "/") {
			_, cidr, err := net.ParseCIDR(a)
			if err == nil && cidr.Contains(net.ParseIP(ip)) {
				return true
			}
		}
	}
	return false
}
