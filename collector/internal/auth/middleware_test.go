package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testKeyStore implements KeyStore for testing.
type testKeyStore struct {
	keys map[string]*KeyRecord
}

func (s *testKeyStore) FindByKeyID(_ context.Context, keyID string) (*KeyRecord, error) {
	return s.keys[keyID], nil
}

func newTestStore(serverSecret []byte) *testKeyStore {
	secHash := HashSecret("testsecret", serverSecret)
	pubHash := HashSecret("pubsecret", serverSecret)
	return &testKeyStore{
		keys: map[string]*KeyRecord{
			"ksec1": {
				ID:                   "id_sec1",
				OrgID:                "org_1",
				ProjectID:            "proj_1",
				KeyID:                "ksec1",
				SecretHash:           secHash,
				Kind:                 KeyKindSecret,
				Roles:                []Role{RoleIngestServer},
				AllowedEnvs:          []string{"prod", "staging"},
				AllowedServices:      []string{"checkout-api"},
				MaxPayloadBytes:      262144,
				MaxRequestsPerMinute: 1000,
			},
			"kpub1": {
				ID:                   "id_pub1",
				OrgID:                "org_1",
				ProjectID:            "proj_1",
				KeyID:                "kpub1",
				SecretHash:           pubHash,
				Kind:                 KeyKindPublic,
				Roles:                []Role{RoleIngestPublic},
				AllowedEnvs:          []string{"prod"},
				AllowedOrigins:       []string{"https://app.example.com"},
				MaxPayloadBytes:      65536,
				MaxRequestsPerMinute: 100,
			},
			"krevoked": {
				ID:         "id_revoked",
				KeyID:      "krevoked",
				SecretHash: secHash,
				Kind:       KeyKindSecret,
				Roles:      []Role{RoleIngestServer},
				RevokedAt:  timePtr(time.Now().Add(-1 * time.Hour)),
			},
			"kexpired": {
				ID:         "id_expired",
				KeyID:      "kexpired",
				SecretHash: secHash,
				Kind:       KeyKindSecret,
				Roles:      []Role{RoleIngestServer},
				ExpiresAt:  timePtr(time.Now().Add(-1 * time.Hour)),
			},
		},
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func TestMiddleware_ValidSecretKey(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil {
			t.Fatal("expected auth context")
		}
		if ac.OrgID != "org_1" {
			t.Errorf("orgID = %q, want %q", ac.OrgID, "org_1")
		}
		if !ac.HasPermission(PermEventsWrite) {
			t.Error("expected events:write permission")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_ksec1_testsecret")
	req.Header.Set("X-Loza-Env", "prod")
	req.Header.Set("X-Loza-Service", "checkout-api")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_WebSocketSubprotocolCredentialAndScope(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	handler := Middleware(store, cache, serverSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Loza-Env"); got != "prod" {
			t.Errorf("X-Loza-Env = %q, want prod", got)
		}
		if got := r.Header.Get("X-Loza-Service"); got != "checkout-api" {
			t.Errorf("X-Loza-Service = %q, want checkout-api", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	credential := "lz_sec_live_ksec1_testsecret"
	protocol := WebSocketAuthProtocolPrefix + base64.RawURLEncoding.EncodeToString([]byte(credential))
	req := httptest.NewRequest("GET", "/collectors/orders/ws/tail?environment=prod&service=checkout-api", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Protocol", "loza.tail.v1, "+protocol)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_ValidBasicAuth(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil {
			t.Fatal("expected auth context")
		}
		if ac.APIKeyID != "ksec1" {
			t.Errorf("APIKeyID = %q, want %q", ac.APIKeyID, "ksec1")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.SetBasicAuth("ksec1", "testsecret")
	req.Header.Set("X-Loza-Env", "prod")
	req.Header.Set("X-Loza-Service", "checkout-api")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_BasicAuthWrongPassword(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.SetBasicAuth("ksec1", "wrong-password")
	req.Header.Set("X-Loza-Env", "prod")
	req.Header.Set("X-Loza-Service", "checkout-api")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_BasicAuthUnknownUsername(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.SetBasicAuth("unknown-user", "testsecret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_ValidPublicKey(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil {
			t.Fatal("expected auth context")
		}
		if ac.KeyKind != KeyKindPublic {
			t.Errorf("keyKind = %q, want %q", ac.KeyKind, KeyKindPublic)
		}
		if !ac.HasPermission(PermEventsWrite) {
			t.Error("expected events:write permission for public key")
		}
		if ac.HasPermission(PermLogsWrite) {
			t.Error("public key should NOT have logs:write")
		}
		if ac.AllowPII {
			t.Error("public key should NOT allow PII")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_pub_live_kpub1_pubsecret")
	req.Header.Set("X-Loza-Env", "prod")
	req.Header.Set("Origin", "https://app.example.com") // must match AllowedOrigins in test store
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if rec.Header().Get("X-Auth-Failure-Code") != "unauthorized" || rec.Header().Get("X-Auth-Failure-Reason") != "authentication required" {
		t.Errorf("unexpected generic auth failure headers: %#v", rec.Header())
	}
	if got := rec.Body.String(); got != "{\"error\":\"unauthorized\"}\n" {
		t.Errorf("body = %q, want unauthorized JSON", got)
	}
}

func TestMiddleware_InvalidKey(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_k_nonexistent_secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_RevokedKey(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_krevoked_testsecret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// X-Auth-Failure-Code is no longer sent to clients (logged server-side only).
}

func TestMiddleware_ExpiredKey(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_kexpired_testsecret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// X-Auth-Failure-Code is no longer sent to clients (logged server-side only).
}

func TestMiddleware_WrongEnv(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_ksec1_testsecret")
	req.Header.Set("X-Loza-Env", "dev") // not in allowed_envs
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// X-Auth-Failure-Code is no longer sent to clients (logged server-side only).
}

func TestMiddleware_WrongService(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	store := newTestStore(serverSecret)
	cache := NewMemoryKeyCache(10*time.Second, 5*time.Second)
	defer cache.Close()

	mw := Middleware(store, cache, serverSecret)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/events", nil)
	req.Header.Set("Authorization", "Bearer lz_sec_live_ksec1_testsecret")
	req.Header.Set("X-Loza-Env", "prod")
	req.Header.Set("X-Loza-Service", "wrong-service") // not in allowed_services
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	// X-Auth-Failure-Code is no longer sent to clients (logged server-side only).
}

func TestRequirePermission_Allowed(t *testing.T) {
	handler := RequirePermission(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), PermEventsWrite)

	req := httptest.NewRequest("POST", "/events", nil)
	ac := &AuthContext{
		Permissions: map[Permission]bool{PermEventsWrite: true},
	}
	req = req.WithContext(WithAuthContext(req.Context(), ac))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRequirePermission_Denied(t *testing.T) {
	handler := RequirePermission(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}), PermEventsWrite)

	req := httptest.NewRequest("POST", "/events", nil)
	ac := &AuthContext{
		Permissions: map[Permission]bool{PermLogsWrite: true}, // no events:write
	}
	req = req.WithContext(WithAuthContext(req.Context(), ac))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequirePermission_NoAuthContext(t *testing.T) {
	handler := RequirePermission(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}), PermEventsWrite)

	req := httptest.NewRequest("POST", "/events", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestMiddleware_PrivateTokenCarriesRBACPermissions(t *testing.T) {
	serverSecret := []byte("test-server-secret")
	token := "lxt_opaque_private_token_for_admin"
	tokenID := TokenLookupID(token, serverSecret)
	store := &testKeyStore{keys: map[string]*KeyRecord{
		tokenID: {
			ID:         tokenID,
			KeyID:      tokenID,
			SecretHash: HashSecret(token, serverSecret),
			Kind:       KeyKindToken,
			Mode:       ModePrivate,
			Roles:      []Role{RoleAdmin},
		},
	}}
	cache := NewMemoryKeyCache(time.Minute, time.Second)
	defer cache.Close()

	handler := Middleware(store, cache, serverSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !GetAuthContext(r.Context()).HasPermission(PermLogsDelete) {
			t.Fatal("admin token must carry logs:delete")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodDelete, "/logs/1", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
