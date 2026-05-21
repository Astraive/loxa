package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
)

func TestAuthMiddlewareDisabledPassesThrough(t *testing.T) {
	auth := NewAuth(&config.AuthenticationConfig{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected pass-through, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsAndAccepts(t *testing.T) {
	auth := NewAuth(&config.AuthenticationConfig{
		Enabled: true,
		APIKeys: []config.APIKey{{Name: "key", Key: "secret", Role: "writer"}},
	})

	t.Run("missing credentials", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized, got %d", rec.Code)
		}
	})

	t.Run("valid bearer token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret")
		auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if result := GetAuthResult(r.Context()); result == nil || result.KeyName != "key" {
				t.Fatalf("expected auth result in context, got %+v", result)
			}
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected ok, got %d", rec.Code)
		}
	})
}

func TestRequireRole(t *testing.T) {
	auth := NewAuth(&config.AuthenticationConfig{
		Enabled: true,
		APIKeys: []config.APIKey{{Name: "reader", Key: "r", Role: "reader"}},
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "r")

	rec := httptest.NewRecorder()
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.RequireRole("writer")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, r)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", rec.Code)
	}
}
