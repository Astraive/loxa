package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/middleware"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/processor"
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

func TestIngestEventClassifiesStorageFailuresAsServerErrors(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.DuckDB.Path = filepath.Join(t.TempDir(), "cortex.duckdb")
	stor, err := storage.NewStorage(cfg)
	if err != nil {
		t.Fatal(err)
	}
	eventProcessor := processor.NewEventProcessor(stor.Events(), stor.Topology(), stor.Graph())
	if err := stor.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(models.Event{
		ID: "evt-storage", Timestamp: time.Now(), Service: "api",
		Kind: models.EventKindLog, Provenance: "loza",
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	(&Server{processor: eventProcessor}).IngestEvent(
		rec,
		httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body)),
	)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected storage failure to return 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIngestBatchClassifiesValidationFailuresAsClientErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Server{processor: processor.NewEventProcessor(nil, nil, nil)}).IngestBatch(
		rec,
		httptest.NewRequest(http.MethodPost, "/events/batch", strings.NewReader(`{"events":[{}]}`)),
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected validation failure to return 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

type graphQLSignatureStore struct {
	signatures map[string]*models.IncidentSignature
}

func (*graphQLSignatureStore) Save(context.Context, *models.IncidentSignature) error { return nil }
func (s *graphQLSignatureStore) Get(_ context.Context, id string) (*models.IncidentSignature, error) {
	signature := s.signatures[id]
	if signature == nil {
		return nil, errors.New("signature not found")
	}
	return signature, nil
}
func (s *graphQLSignatureStore) List(context.Context, int) ([]*models.IncidentSignature, error) {
	result := make([]*models.IncidentSignature, 0, len(s.signatures))
	for _, signature := range s.signatures {
		result = append(result, signature)
	}
	return result, nil
}
func (*graphQLSignatureStore) FindSimilar(context.Context, *models.IncidentSignature, int) ([]*models.SimilarIncident, error) {
	return nil, nil
}
func (*graphQLSignatureStore) FindByBehavioralHash(context.Context, string) ([]*models.IncidentSignature, error) {
	return nil, nil
}
func (*graphQLSignatureStore) UpdateDecay(context.Context, string, float64) error { return nil }
func (*graphQLSignatureStore) ArchiveStale(context.Context, float64) (int, error) {
	return 0, nil
}
func (*graphQLSignatureStore) UpdateLastMatched(context.Context, string) error { return nil }

func testGraphQLServer(t *testing.T) *GraphQLServer {
	t.Helper()
	server := &GraphQLServer{
		signatures: &graphQLSignatureStore{signatures: map[string]*models.IncidentSignature{
			"sig-1": {SignatureID: "sig-1", Shape: "first"},
			"sig-2": {SignatureID: "sig-2", Shape: "second"},
		}},
	}
	if err := server.initSchema(); err != nil {
		t.Fatal(err)
	}
	return server
}

func TestGraphQLExecutesNamedOperationAliasesAndSelectionSets(t *testing.T) {
	result := testGraphQLServer(t).executeQuery(
		context.Background(),
		`query First { ignored: signature(id: "sig-1") { signature_id shape } }
		 query Second { chosen: signature(id: "sig-2") { shape } }`,
		nil,
		"Second",
	)
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected GraphQL errors: %+v", result.Errors)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected GraphQL data: %#v", result.Data)
	}
	chosen, ok := data["chosen"].(map[string]interface{})
	if !ok || chosen["shape"] != "second" {
		t.Fatalf("unexpected aliased selection: %#v", data)
	}
	if _, exists := chosen["signature_id"]; exists {
		t.Fatalf("executor returned an unrequested field: %#v", chosen)
	}
	if _, exists := data["ignored"]; exists {
		t.Fatalf("executor ran an unselected operation: %#v", data)
	}
}

func TestGraphQLDoesNotDispatchFromStringArguments(t *testing.T) {
	result := testGraphQLServer(t).executeQuery(
		context.Background(),
		`query { signature(id: "ingestEvent") { shape } }`,
		nil,
		"",
	)
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, "not found") {
		t.Fatalf("expected signature lookup error, got %+v", result.Errors)
	}
}

func TestGraphQLRejectsReaderIngestMutations(t *testing.T) {
	ctx := middleware.WithAuthResult(context.Background(), &middleware.AuthResult{
		Authorized: true,
		Role:       "reader",
	})
	result := testGraphQLServer(t).executeQuery(
		ctx,
		`mutation { ingestEvent(event: {id: "evt", service: "api", kind: "log"}) { status } }`,
		nil,
		"",
	)
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[0].Message, "writer role required") {
		t.Fatalf("expected writer authorization error, got %+v", result.Errors)
	}
}
