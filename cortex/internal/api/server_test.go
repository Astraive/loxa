package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/middleware"
	"github.com/astraive/loza/cortex/internal/storage"
)

func TestServerHealthAndReadiness(t *testing.T) {
	srv := &Server{config: &config.Config{}, ready: true}

	rec := httptest.NewRecorder()
	srv.Healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected healthz ok, got %d", rec.Code)
	}

	// With nil graph/processor/incidents, readyz should return not_ready
	rec = httptest.NewRecorder()
	srv.Readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected not ready (nil deps), got %d", rec.Code)
	}

	srv.ready = false
	rec = httptest.NewRecorder()
	srv.Readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected not ready, got %d", rec.Code)
	}
}

func TestRouterServesHealthz(t *testing.T) {
	srv := &Server{config: &config.Config{}, ready: true}
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected router healthz ok, got %d", rec.Code)
	}
}

func TestNewServerDoesNotStartUnownedWorkers(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.DuckDB.Path = filepath.Join(t.TempDir(), "cortex.duckdb")
	stor, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer stor.Close()

	before := runtime.NumGoroutine()
	for range 3 {
		_ = NewServer(cfg, stor)
	}
	time.Sleep(20 * time.Millisecond)
	if delta := runtime.NumGoroutine() - before; delta > 2 {
		t.Fatalf("server construction leaked %d background workers", delta)
	}
}

func TestGraphQLHelpers(t *testing.T) {
	if got := removeWhitespace("query {  a \n b }"); got != "query{ab}" {
		t.Fatalf("unexpected whitespace removal: %s", got)
	}
	if !containsOperation("ingestEvent{foo}", "ingestEvent") {
		t.Fatal("expected operation match")
	}
}

func TestContainsWordHandlesInputBoundary(t *testing.T) {
	if !containsWord("query", "query") {
		t.Fatal("expected exact word match")
	}
	if containsWord("somequery", "query") {
		t.Fatal("unexpected suffix match inside identifier")
	}
}

func TestGraphQLRejectsReaderIngestMutations(t *testing.T) {
	ctx := middleware.WithAuthResult(context.Background(), &middleware.AuthResult{
		Authorized: true,
		Role:       "reader",
	})
	tests := []struct {
		name  string
		query string
	}{
		{name: "single event", query: "mutation IngestEvent { ingestEvent { status } }"},
		{name: "batch", query: "mutation IngestBatch { ingestBatch { status } }"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&GraphQLServer{}).executeQuery(ctx, tt.query, map[string]interface{}{})
			if err == nil || !strings.Contains(err.Error(), "writer role required") {
				t.Fatalf("expected writer role error, got %v", err)
			}
		})
	}
}

func TestGraphQLAllowsWriterLevelIngestMutation(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{
			name: "writer",
			ctx: middleware.WithAuthResult(context.Background(), &middleware.AuthResult{
				Authorized: true,
				Role:       "writer",
			}),
		},
		{
			name: "admin",
			ctx: middleware.WithAuthResult(context.Background(), &middleware.AuthResult{
				Authorized: true,
				Role:       "admin",
			}),
		},
		{name: "auth disabled", ctx: context.Background()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&GraphQLServer{}).executeQuery(
				tt.ctx,
				"mutation IngestEvent { ingestEvent { status } }",
				map[string]interface{}{},
			)
			if err == nil || !strings.Contains(err.Error(), "event variables required") {
				t.Fatalf("expected operation validation error, got %v", err)
			}
		})
	}
}
