package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
)

type Auth struct {
	cfg       *config.AuthenticationConfig
	keyCache map[string]*config.APIKey
	once    sync.Once
}

func NewAuth(cfg *config.AuthenticationConfig) *Auth {
	return &Auth{cfg: cfg}
}

func (a *Auth) init() {
	if a.keyCache == nil && len(a.cfg.APIKeys) > 0 {
		a.keyCache = make(map[string]*config.APIKey, len(a.cfg.APIKeys))
		for i := range a.cfg.APIKeys {
			key := &a.cfg.APIKeys[i]
			a.keyCache[key.Key] = key
		}
	}
}

type AuthResult struct {
	Authorized  bool
	KeyName    string
	Role      string
	Failure   string
	FailureCode string
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cfg == nil || !a.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		result := a.authenticate(r)
		if !result.Authorized {
			a.logAuthFailure(r, result.KeyName, result.Failure, result.FailureCode)
			w.Header().Set("X-Auth-Failure-Reason", result.Failure)
			w.Header().Set("X-Auth-Failure-Code", result.FailureCode)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = WithAuthResult(ctx, result)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) authenticate(r *http.Request) *AuthResult {
	a.once.Do(a.init)

	result := &AuthResult{Authorized: false}

	apiKeyHeader := "X-API-Key"

	providedKey := strings.TrimSpace(r.Header.Get(apiKeyHeader))
	if providedKey == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			providedKey = strings.TrimSpace(authHeader[len("Bearer "):])
		}
	}

	if providedKey == "" {
		result.Failure = "missing credentials"
		result.FailureCode = "missing"
		return result
	}

	subtleKey := []byte(providedKey)
	for _, key := range a.cfg.APIKeys {
		if subtle.ConstantTimeCompare(subtleKey, []byte(key.Key)) == 1 {
			result.Authorized = true
			result.KeyName = key.Name
			result.Role = key.Role
			return result
		}
	}

	result.Failure = "invalid credentials"
	result.FailureCode = "invalid"
	return result
}

func (a *Auth) authorizeRole(r *http.Request, requiredRole string) bool {
	result := GetAuthResult(r.Context())
	if result == nil {
		return false
	}

	roleHierarchy := map[string]int{
		"reader": 1,
		"writer": 2,
		"admin":  3,
	}

	requiredLevel := roleHierarchy[requiredRole]
	currentLevel := roleHierarchy[result.Role]

	return currentLevel >= requiredLevel
}

func (a *Auth) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg == nil || !a.cfg.Enabled {
			next(w, r)
			return
		}

		result := a.authenticate(r)
		if !result.Authorized {
			a.logAuthFailure(r, result.KeyName, result.Failure, result.FailureCode)
			w.Header().Set("X-Auth-Failure-Reason", result.Failure)
			w.Header().Set("X-Auth-Failure-Code", result.FailureCode)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = WithAuthResult(ctx, result)
		next(w, r.WithContext(ctx))
	}
}

func (a *Auth) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if a.cfg == nil || !a.cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			if !a.authorizeRole(r, role) {
				w.Header().Set("X-Auth-Failure-Reason", "insufficient role")
				w.Header().Set("X-Auth-Failure-Code", "unauthorized_role")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const authResultKey contextKey = "auth_result"

func WithAuthResult(ctx context.Context, result *AuthResult) context.Context {
	return context.WithValue(ctx, authResultKey, result)
}

func GetAuthResult(ctx context.Context) *AuthResult {
	if v := ctx.Value(authResultKey); v != nil {
		if result, ok := v.(*AuthResult); ok {
			return result
		}
	}
	return nil
}

type AuthMiddleware struct {
	auth *Auth
}

func NewAuthMiddleware(cfg config.AuthenticationConfig) *AuthMiddleware {
	return &AuthMiddleware{auth: NewAuth(&cfg)}
}

func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	if m.auth == nil {
		return next
	}
	return m.auth.Middleware(next)
}

func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	if m.auth == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.auth.RequireRole(role)
}

func logAuthJSON(level, event string, fields map[string]interface{}) {
	output := map[string]interface{}{
		"level":   level,
		"event":   event,
		"context": fields,
	}
	if data, err := json.Marshal(output); err == nil {
		println(string(data))
	}
}

func (a *Auth) logAuthFailure(r *http.Request, keyName, reason, code string) {
	logAuthJSON("warn", "auth_failure", map[string]interface{}{
		"key_name":      keyName,
		"path":         r.URL.Path,
		"method":       r.Method,
		"remote_addr":  r.RemoteAddr,
		"reason":       reason,
		"failure_code": code,
	})
}