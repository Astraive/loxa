package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

// parsedCIDRNet holds a pre-parsed CIDR to avoid repeated net.ParseCIDR calls.
type parsedCIDRNet struct {
	ipNet *net.IPNet
	err   error
}

// cidrCache caches parsed CIDRs so they are only parsed once per unique string.
var cidrCache sync.Map // map[string]*parsedCIDRNet

// KeyRecord holds the stored metadata for an API key.
type KeyRecord struct {
	ID                   string
	OrgID                string
	ProjectID            string
	KeyID                string
	SecretHash           []byte
	Kind                 KeyKind
	Roles                []Role
	AllowedEnvs          []string
	AllowedServices      []string
	AllowedOrigins       []string
	AllowedIPs           []string
	MaxPayloadBytes      int
	MaxRequestsPerMinute int
	MaxEventsPerMinute   int
	SamplingRate         float64
	AllowPII             bool
	AllowAttachments     bool
	RevokedAt            *time.Time
	ExpiresAt            *time.Time
}

// KeyStore is the interface for key storage backends.
type KeyStore interface {
	FindByKeyID(ctx context.Context, keyID string) (*KeyRecord, error)
}

// MiddlewareOption configures the auth middleware.
type MiddlewareOption func(*middlewareConfig)

type middlewareConfig struct {
	allowLocalDevKeys bool
	trustedProxies    []*net.IPNet
	rateLimiter       *KeyRateLimiter
}

// WithAllowLocalDevKeys controls whether lx_local_dev_* keys are accepted.
// Default is false (local dev keys rejected), which is safe for production.
func WithAllowLocalDevKeys(v bool) MiddlewareOption {
	return func(c *middlewareConfig) { c.allowLocalDevKeys = v }
}

// WithTrustedProxies sets the list of trusted proxy CIDRs.
// X-Forwarded-For is only trusted from these IPs.
func WithTrustedProxies(proxies []*net.IPNet) MiddlewareOption {
	return func(c *middlewareConfig) { c.trustedProxies = proxies }
}

// WithRateLimiter sets an external rate limiter to be shared with handlers.
func WithRateLimiter(rl *KeyRateLimiter) MiddlewareOption {
	return func(c *middlewareConfig) { c.rateLimiter = rl }
}

// Middleware returns an http.Handler middleware that validates API keys
// and attaches an AuthContext to the request. It does NOT check permissions
// — that's done per-route via RequirePermission.
//
// Validation flow:
//  1. Parse Authorization header (Basic, Bearer, or X-API-Key)
//  2. Map Basic username to key_id, or ParseKey for API-key credentials
//  3. Cache lookup by key_id
//  4. If cache miss: KeyStore.FindByKeyID()
//  5. Check revoked / expired
//  6. Verify HMAC-SHA256(incoming secret) == stored hash
//  7. Build AuthContext (org, project, roles, permissions)
//  8. Check X-Loza-Env against AllowedEnvs
//  9. Check X-Loza-Service against AllowedServices
//  10. Check Origin against AllowedOrigins (public keys)
//  11. Check remote IP against AllowedIPs
//  12. Wrap body with MaxBytesReader if MaxPayloadBytes > 0
//  13. Apply per-key rate limit
//  14. Attach AuthContext to request context
//  15. Call next handler
func Middleware(store KeyStore, cache *MemoryKeyCache, serverSecret []byte, opts ...MiddlewareOption) func(http.Handler) http.Handler {
	var mc middlewareConfig
	for _, o := range opts {
		o(&mc)
	}
	rateLimiter := mc.rateLimiter
	if rateLimiter == nil {
		rateLimiter = NewKeyRateLimiter()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ac, errCode, _ := authenticate(r, store, cache, serverSecret, rateLimiter, mc.allowLocalDevKeys, mc.trustedProxies)
			if ac == nil {
				slog.Warn("auth_failure", "code", errCode, "path", r.URL.Path, "remote", r.RemoteAddr)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Auth-Failure-Code", "unauthorized")
				w.Header().Set("X-Auth-Failure-Reason", "authentication required")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
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

func authenticate(r *http.Request, store KeyStore, cache *MemoryKeyCache, serverSecret []byte, rateLimiter *KeyRateLimiter, allowLocalDevKeys bool, trustedProxies []*net.IPNet) (*AuthContext, string, string) {
	// 1. Parse Authorization header. Basic credentials use the username as
	// the configured key ID and the password as the HMAC input. Keep the
	// existing Bearer/X-API-Key extraction path unchanged.
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	var keyID, secret string
	var keyKind KeyKind
	isBasic := strings.HasPrefix(strings.ToLower(authHeader), "basic ")
	if isBasic {
		username, password, ok := r.BasicAuth()
		if !ok || !validBasicUsername(username) || password == "" {
			return nil, "invalid_basic_auth", "invalid basic credentials"
		}
		keyID = username
		secret = password
	} else {
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
			if !allowLocalDevKeys {
				slog.Warn("local dev key rejected (allow_local_dev_keys=false)")
				return nil, "local_dev_disabled", "local dev keys are not allowed in production mode"
			}
			return &AuthContext{
				KeyKind:     KeyKindLocal,
				Permissions: ExpandRoles([]Role{RoleIngestServer}),
				Roles:       []Role{RoleIngestServer},
			}, "", ""
		}
		keyID = parsed.KeyID
		secret = parsed.Secret
		keyKind = parsed.Kind
	}

	// 3. Cache lookup
	var record *KeyRecord
	if cache != nil {
		if cached, ok := cache.Get(keyID); ok {
			record = cached
		}
	}

	// 4. Store lookup on cache miss
	if record == nil {
		var err error
		record, err = store.FindByKeyID(r.Context(), keyID)
		if err != nil || record == nil {
			if cache != nil {
				cache.SetNegative(keyID)
			}
			return nil, "key_not_found", "invalid api key"
		}
		if cache != nil {
			cache.Set(keyID, record, 0) // default TTL
		}
	}

	// Basic authentication only accepts configured non-local keys. Existing
	// API-key authentication retains its parsed key-kind check below.
	if isBasic && record.Kind == KeyKindLocal {
		return nil, "key_kind_mismatch", "api key kind mismatch"
	}

	// 5. Check revoked / expired
	if record.RevokedAt != nil {
		return nil, "key_revoked", "api key has been revoked"
	}
	if record.ExpiresAt != nil && time.Now().After(*record.ExpiresAt) {
		return nil, "key_expired", "api key has expired"
	}

	// 6. Verify prefix matches for API-key credentials.
	if !isBasic && string(record.Kind) != string(keyKind) {
		return nil, "key_kind_mismatch", "api key kind mismatch"
	}

	// 7. Verify secret hash
	incomingHash := HashSecret(secret, serverSecret)
	if !CompareSecret(incomingHash, record.SecretHash) {
		return nil, "invalid_secret", "invalid api key"
	}

	// 8. Build AuthContext
	permissions := ExpandRoles(record.Roles)
	ac := &AuthContext{
		OrgID:                record.OrgID,
		ProjectID:            record.ProjectID,
		APIKeyID:             record.KeyID,
		KeyKind:              record.Kind,
		Roles:                record.Roles,
		Permissions:          permissions,
		AllowedEnvs:          record.AllowedEnvs,
		AllowedServices:      record.AllowedServices,
		AllowedOrigins:       record.AllowedOrigins,
		AllowedIPs:           record.AllowedIPs,
		MaxPayloadBytes:      record.MaxPayloadBytes,
		MaxRequestsPerMinute: record.MaxRequestsPerMinute,
		MaxEventsPerMinute:   record.MaxEventsPerMinute,
		SamplingRate:         record.SamplingRate,
		AllowPII:             record.AllowPII,
		AllowAttachments:     record.AllowAttachments,
	}

	// Public key strict defaults
	if record.Kind == KeyKindPublic {
		ac.AllowPII = false
		ac.AllowAttachments = false
	}

	// 9. Check env restriction
	if len(ac.AllowedEnvs) > 0 {
		env := r.Header.Get("X-Loza-Env")
		if !contains(ac.AllowedEnvs, env) {
			return nil, "env_not_allowed", "environment not permitted for this key"
		}
	}

	// 10. Check service restriction
	if len(ac.AllowedServices) > 0 {
		svc := r.Header.Get("X-Loza-Service")
		if !contains(ac.AllowedServices, svc) {
			return nil, "service_not_allowed", "service not permitted for this key"
		}
	}

	// 11. Check origin restriction (for public keys)
	// When AllowedOrigins is configured, empty Origin headers are rejected.
	// Browsers always send Origin on cross-origin requests; an empty Origin
	// indicates a non-browser client attempting to bypass origin restrictions.
	if len(ac.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return nil, "origin_required", "origin header required when allowed_origins is configured"
		}
		if !contains(ac.AllowedOrigins, origin) {
			return nil, "origin_not_allowed", "origin not permitted"
		}
	}

	// 12. Check IP restriction
	if len(ac.AllowedIPs) > 0 {
		remoteIP := extractIP(r, trustedProxies)
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

func validBasicUsername(username string) bool {
	if username == "" || strings.TrimSpace(username) != username {
		return false
	}
	for _, r := range username {
		if r == ':' || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	// Also check X-API-Key header for backward compat
	return strings.TrimSpace(r.Header.Get("X-API-Key"))
}

func extractIP(r *http.Request, trustedProxies []*net.IPNet) string {
	remoteAddr := r.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	remoteIP := net.ParseIP(host)

	// Only trust X-Forwarded-For if the direct client is a trusted proxy
	if remoteIP != nil && isTrustedProxy(remoteIP, trustedProxies) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for _, part := range parts {
				ip := net.ParseIP(strings.TrimSpace(part))
				if ip != nil && !isTrustedProxy(ip, trustedProxies) {
					return ip.String()
				}
			}
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	return host
}

func isTrustedProxy(ip net.IP, trustedProxies []*net.IPNet) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	for _, cidr := range trustedProxies {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// ipAllowed checks if ip is in the allowed list (exact match or CIDR).
// CIDRs are parsed once and cached to avoid per-request parsing overhead.
func ipAllowed(ip string, allowed []string) bool {
	parsedIP := net.ParseIP(ip)
	for _, a := range allowed {
		if a == ip {
			return true
		}
		if strings.Contains(a, "/") {
			cached, ok := cidrCache.Load(a)
			if !ok {
				_, cidr, parseErr := net.ParseCIDR(a)
				cached, _ = cidrCache.LoadOrStore(a, &parsedCIDRNet{ipNet: cidr, err: parseErr})
			}
			parsed := cached.(*parsedCIDRNet)
			if parsed.err == nil && parsed.ipNet.Contains(parsedIP) {
				return true
			}
		}
	}
	return false
}
