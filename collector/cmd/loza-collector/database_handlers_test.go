package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/astraive/loza/collector/internal/database"
	publichttp "github.com/astraive/loza/collector/server/http"
)

type fakeDatabaseConnection struct {
	info database.Metadata
}

func (f *fakeDatabaseConnection) Backend() string             { return f.info.Backend }
func (f *fakeDatabaseConnection) Metadata() database.Metadata { return f.info }
func (f *fakeDatabaseConnection) Query(context.Context, string, ...any) (database.Result, error) {
	return database.Result{Columns: []string{"value"}, Rows: [][]any{{"ok"}}}, nil
}
func (f *fakeDatabaseConnection) Ping(context.Context) error  { return nil }
func (f *fakeDatabaseConnection) Close(context.Context) error { return nil }

type fakeLQLCompiler struct{}

func (fakeLQLCompiler) Compile(context.Context, LQLCompileRequest) (ParameterizedPlan, error) {
	return ParameterizedPlan{SQL: "SELECT value", Args: []any{}}, nil
}
func (fakeLQLCompiler) Close(context.Context) error { return nil }
func TestDatabaseConnectionsMetadataIsSanitized(t *testing.T) {
	connection := &fakeDatabaseConnection{info: database.Metadata{
		Name: "analytics", Backend: "postgres", Host: "db.internal", Port: 5432,
		Database: "loza", Table: "events", Enabled: true, Capabilities: []string{"query"},
	}}
	state := &collectorState{

		cfg:                 collectorConfig{authEnabled: false},
		databaseConnections: map[string]database.Connection{"analytics": connection},
		databaseMetadata:    map[string]database.Metadata{"analytics": connection.Metadata()},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/collectors/acme/database/connections", nil)
	request = request.WithContext(publichttp.WithAuthorizedCollector(request.Context(), "acme", "prod"))
	state.HandleDatabaseConnections(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metadata status = %d", recorder.Code)
	}
	body := recorder.Body.String()
	if body == "" || containsSecretField(body) {
		t.Fatalf("metadata response leaked secret fields: %s", body)
	}
}

func TestDatabaseQueryUsesNamedConnection(t *testing.T) {
	connection := &fakeDatabaseConnection{info: database.Metadata{Name: "analytics", Backend: "postgres"}}
	state := &collectorState{
		cfg: collectorConfig{
			authEnabled:         false,
			databaseConnections: []databaseConnectionConfig{{name: "analytics", queryTimeout: time.Second}},
		},
		databaseConnections: map[string]database.Connection{"analytics": connection},
		lqlCompiler:         fakeLQLCompiler{},
	}
	request := httptest.NewRequest(http.MethodPost, "/collectors/acme/database/query", strings.NewReader(`{"connection":"analytics","query":"from events | take 1","limit":1}`))
	request = request.WithContext(publichttp.WithAuthorizedCollector(request.Context(), "acme", "prod"))
	recorder := httptest.NewRecorder()
	state.HandleDatabaseQuery(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"backend":"postgres"`) {
		t.Fatalf("named query failed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func containsSecretField(body string) bool {
	return stringContains(body, "password") || stringContains(body, "username") || stringContains(body, "dsn")
}

func stringContains(body, needle string) bool {
	for i := 0; i+len(needle) <= len(body); i++ {
		if body[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
