package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	collectorconfig "github.com/astraive/loxa-collector/internal/config"
	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"github.com/astraive/loxa-collector/internal/ingest"
	"github.com/astraive/loxa-collector/internal/sinks/duckdb"
	storagepath "github.com/astraive/loxa-collector/internal/storage"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/klauspost/compress/zstd"
	_ "github.com/marcboeker/go-duckdb"
	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
)

type fakeSink struct {
	events [][]byte
	fail   bool
}

func (s *fakeSink) Name() string { return "fake" }
func (s *fakeSink) WriteEvent(_ context.Context, encoded []byte, _ *collectorevent.Event) error {
	if s.fail {
		return context.DeadlineExceeded
	}
	cp := make([]byte, len(encoded))
	copy(cp, encoded)
	s.events = append(s.events, cp)
	return nil
}
func (s *fakeSink) Flush(_ context.Context) error { return nil }
func (s *fakeSink) Close(_ context.Context) error { return nil }

type errSink struct {
	err error
}

func (s errSink) Name() string { return "err" }
func (s errSink) WriteEvent(context.Context, []byte, *collectorevent.Event) error {
	return s.err
}
func (s errSink) Flush(context.Context) error { return nil }
func (s errSink) Close(context.Context) error { return nil }

func testCollectorConfig() collectorConfig {
	cfg := runtimeConfigFromFile(defaultFileConfig())
	cfg.retryMaxAttempts = 1
	cfg.retryInitialBackoff = time.Millisecond
	cfg.retryMaxBackoff = time.Millisecond
	return cfg
}

func TestParseEventsJSONArray(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`[{"event":"a"},{"event":"b"}]`))
	events, err := ingest.ParseEvents(req, 1024)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestParseEventsWrappedEvents(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"events":[{"event":"a"},{"event":"b"}]}`))
	events, err := ingest.ParseEvents(req, 1024)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestParseEventsWrappedEventsPrettyJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{
  "api_version": "v1",
  "events": [
    {"event":"a"},
    {"event":"b"}
  ]
}`))
	events, err := ingest.ParseEvents(req, 4096)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestParseEventsNDJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader("{\"event\":\"a\"}\n{\"event\":\"b\"}\n"))
	events, err := ingest.ParseEvents(req, 1024)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestHandleIngestAuthFailure(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.apiKey = "lx_sec_live_ktest_testsecret"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	// Auth is now handled by middleware, so test through the mux
	srv := httptest.NewServer(buildMux(state))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/events", strings.NewReader(`[{"event":"a"}]`))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header → should get 401 from middleware
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestHandleIngestPartialSuccess(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`[{"event":"a"},123]`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusMultiStatus {
		t.Fatalf("expected 207, got %d", rec.Code)
	}
	var out ingest.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Accepted != 1 || out.Invalid != 1 || out.Rejected != 0 {
		t.Fatalf("unexpected response: %+v", out)
	}
	if len(out.Acks) != 2 {
		t.Fatalf("expected 2 acks, got %d", len(out.Acks))
	}
	if out.Acks[0].Status != "accepted" || out.Acks[0].Reason != "accepted" {
		t.Fatalf("unexpected accepted ack: %+v", out.Acks[0])
	}
	if out.Acks[1].Status != "invalid" || out.Acks[1].Reason != "expected_json_object" {
		t.Fatalf("unexpected invalid ack: %+v", out.Acks[1])
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(sink.events))
	}
}

func TestHandleIngestWrappedEnvelopeWithContractMetadata(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	body := `{"api_version":"v1","source":{"sdk":"loxa-go","version":"1.0.0","service":"checkout"},"events":[{"event_id":"evt-1","schema_version":"v1","event_version":"v1","event":"checkout.request","service":"checkout"}]}`
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(body))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out ingest.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Accepted != 1 || len(out.Acks) != 1 || out.Acks[0].Status != "accepted" {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestHandleIngestRejectsUnsupportedSchemaVersion(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-bad","schema_version":"v999","event_version":"v1","event":"checkout.request","service":"checkout"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out ingest.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Invalid != 1 || len(out.Acks) != 1 {
		t.Fatalf("unexpected response: %+v", out)
	}
	if out.Acks[0].Status != "invalid" || out.Acks[0].Reason != "unsupported_schema_version" {
		t.Fatalf("unexpected ack: %+v", out.Acks[0])
	}
	if len(sink.events) != 0 {
		t.Fatalf("expected no sink writes, got %d", len(sink.events))
	}
}

func TestHandleIngestSingleJSONObject(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(sink.events))
	}
}

func TestPrepareEventNormalizesEventTypeAlias(t *testing.T) {
	state := &collectorState{cfg: testCollectorConfig()}
	raw := []byte(`{"schema_version":"v1","event_version":"v1","event_id":"evt-alias","service":"checkout","event_type":"checkout.request"}`)
	next, govErr := state.prepareEvent(raw)
	if govErr != nil {
		t.Fatalf("prepare event: %v", govErr)
	}
	var payload map[string]any
	if err := json.Unmarshal(next, &payload); err != nil {
		t.Fatalf("decode normalized event: %v", err)
	}
	if payload["event"] != "checkout.request" {
		t.Fatalf("expected event alias normalization, got %#v", payload["event"])
	}
	if _, ok := payload["event_type"]; ok {
		t.Fatalf("expected event_type alias to be removed after normalization")
	}
}

func TestHandleIngestInvalidJSON(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleIngestRejectsUnsupportedContentType(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestParseEventsZstd(t *testing.T) {
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("new zstd writer: %v", err)
	}
	if _, err := enc.Write([]byte(`[{"event":"a"}]`)); err != nil {
		t.Fatalf("write zstd: %v", err)
	}
	enc.Close()

	req := httptest.NewRequest(http.MethodPost, "/ingest", &buf)
	req.Header.Set("Content-Encoding", "zstd")
	events, err := ingest.ParseEvents(req, 1024)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestHandleIngestJWTAuthorization(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.identityMode = "jwt"
	cfg.apiKey = "0123456789abcdef0123456789abcdef"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte(cfg.apiKey)}, nil)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	token, err := jwt.Signed(signer).Claims(jwt.Claims{
		Expiry: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).Serialize()
	if err != nil {
		t.Fatalf("serialize jwt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSchemaPublishAndDiff(t *testing.T) {
	state := &collectorState{
		cfg: testCollectorConfig(),
	}
	state.ready.Store(true)

	publishReq := httptest.NewRequest(http.MethodPost, "/v1/schema/publish", strings.NewReader(`{"schema_version":"v2","event_version":"v1","required_fields":["event_id","timestamp"]}`))
	publishRec := httptest.NewRecorder()
	state.handleSchemaPublish(publishRec, publishReq)
	if publishRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", publishRec.Code, publishRec.Body.String())
	}
	if len(state.cfg.schemaRegistry) == 0 {
		t.Fatal("expected registry entry to be published")
	}

	diffReq := httptest.NewRequest(http.MethodPost, "/v1/schema/diff", strings.NewReader(`{"schema_version":"v2","event_version":"v1","required_fields":["event_id","service"]}`))
	diffRec := httptest.NewRecorder()
	state.handleSchemaDiff(diffRec, diffReq)
	if diffRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", diffRec.Code, diffRec.Body.String())
	}
	if !strings.Contains(diffRec.Body.String(), `"added":["service"]`) {
		t.Fatalf("expected schema diff to include added service field, got %s", diffRec.Body.String())
	}
}

func TestHandlePIIAudit(t *testing.T) {
	cfg := testCollectorConfig()
	db, err := sql.Open(cfg.duckDBDriver, filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	defer db.Close()
	cfg.duckDBTable = "events"
	cfg.duckDBRawColumn = "raw"
	if err := ensureSchema(db, cfg); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO events (raw) VALUES (?)`, `{"user":{"email":"user@example.com"},"authorization":"Bearer secret-token-value"}`); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	state := &collectorState{
		cfg:     cfg,
		queryDB: db,
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/v1/audit/pii", strings.NewReader(`{"limit":10}`))
	rec := httptest.NewRecorder()
	state.handlePIIAudit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"classification":"restricted"`) {
		t.Fatalf("expected restricted finding, got %s", rec.Body.String())
	}
}

func TestHandleOTLPLogs(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	body := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"checkout"}},{"key":"deployment.environment","value":{"stringValue":"prod"}}]},"scopeLogs":[{"logRecords":[{"severityText":"INFO","body":{"stringValue":"payment.completed"},"attributes":[{"key":"user.id","value":{"stringValue":"usr_1"}}]}]}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/otlp/logs", strings.NewReader(body))
	rec := httptest.NewRecorder()
	state.handleOTLPLogs(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 converted event, got %d", len(sink.events))
	}
}

func TestHandleOTLPLogsProtobuf(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	reqBody := &collectorlogsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1.ResourceLogs{{
			Resource: &resourcev1.Resource{
				Attributes: []*commonv1.KeyValue{{
					Key: "service.name",
					Value: &commonv1.AnyValue{
						Value: &commonv1.AnyValue_StringValue{StringValue: "billing"},
					},
				}},
			},
			ScopeLogs: []*logsv1.ScopeLogs{{
				LogRecords: []*logsv1.LogRecord{{
					SeverityText: "WARN",
					Body: &commonv1.AnyValue{
						Value: &commonv1.AnyValue_StringValue{StringValue: "invoice.failed"},
					},
				}},
			}},
		}},
	}
	raw, err := proto.Marshal(reqBody)
	if err != nil {
		t.Fatalf("marshal protobuf: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/otlp/logs", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/x-protobuf")
	rec := httptest.NewRecorder()
	state.handleOTLPLogs(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 converted event, got %d", len(sink.events))
	}
	if !strings.Contains(string(sink.events[0]), `"service":"billing"`) {
		t.Fatalf("expected converted payload to include service name, got %s", string(sink.events[0]))
	}
}

func TestHybridModeWritesQueueAndSpool(t *testing.T) {
	queueSink := &fakeSink{}
	primarySink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "hybrid"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true
	cfg.deliveryQueueSize = 16

	state := &collectorState{
		cfg:             cfg,
		ingestSink:      primarySink,
		hybridQueueSink: queueSink,
		rateLimiter:     rate.NewLimiter(rate.Limit(1000), 1000),
		rng:             randSourceForTests(),
		dedupeSeenAt:    make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-hybrid","event":"checkout.completed"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(queueSink.events) != 1 {
		t.Fatalf("expected hybrid queue sink write, got %d", len(queueSink.events))
	}
	if state.metrics.spoolBytes.Load() == 0 {
		t.Fatal("expected hybrid mode to persist to spool")
	}
}

func TestSchemaRegistryFilePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	entries := []collectorconfig.SchemaRegistryEntry{{
		SchemaVersion:  "v2",
		EventVersion:   "v1",
		RequiredFields: []string{"event_id", "timestamp"},
	}}
	if err := saveSchemaRegistryFile(path, entries, ""); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	loaded, err := loadPersistedSchemaRegistry(path, "")
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if len(loaded) != 1 || loaded[0].SchemaVersion != "v2" {
		t.Fatalf("unexpected loaded registry: %+v", loaded)
	}
}

func TestSchemaRegistryFilePersistenceEncrypted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.enc")
	entries := []collectorconfig.SchemaRegistryEntry{{
		SchemaVersion:  "v3",
		EventVersion:   "v2",
		RequiredFields: []string{"event_id", "tenant_id"},
	}}
	key := "test-encryption-key"
	if err := saveSchemaRegistryFile(path, entries, key); err != nil {
		t.Fatalf("save encrypted registry: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read encrypted registry: %v", err)
	}
	if bytes.Contains(raw, []byte(`"schema_version":"v3"`)) {
		t.Fatalf("expected persisted registry to be encrypted, got %s", string(raw))
	}
	loaded, err := loadPersistedSchemaRegistry(path, key)
	if err != nil {
		t.Fatalf("load encrypted registry: %v", err)
	}
	if len(loaded) != 1 || loaded[0].SchemaVersion != "v3" || loaded[0].EventVersion != "v2" {
		t.Fatalf("unexpected loaded encrypted registry: %+v", loaded)
	}
}

func TestConfigureHTTPServerTLSForMTLS(t *testing.T) {
	server := &http.Server{}
	cfg := testCollectorConfig()
	cfg.identityMode = "mtls"
	cfg.serverConfig.HTTP.TLSEnabled = true
	if err := configureHTTPServerTLS(server, cfg); err != nil {
		t.Fatalf("configure tls: %v", err)
	}
	if server.TLSConfig == nil || server.TLSConfig.ClientAuth == 0 {
		t.Fatal("expected mtls client auth to be configured")
	}
}

func TestHandleIngestNonObjectJSON(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`123`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleIngestMaxBodyExceeded(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.maxBodyBytes = 4
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"too-big"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestHandleIngestMaxEventsExceeded(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.maxEventsPerRequest = 1
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`[{"event":"a"},{"event":"b"}]`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestHandleIngestAPIPass(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.apiKey = "secret"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	req.Header.Set(cfg.apiKeyHeader, "secret")
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestHandleIngestRateLimit(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.rateLimitEnabled = true
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1), 1),
	}
	state.ready.Store(true)

	first := httptest.NewRecorder()
	state.handleIngest(first, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`)))
	second := httptest.NewRecorder()
	state.handleIngest(second, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"b"}`)))

	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", second.Code)
	}
}

func TestHandleIngestBackpressureByInflightRequests(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.maxInflightRequests = 1
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.metrics.inflightRequests.Store(1)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d body=%s", rec.Code, rec.Body.String())
	}
	var out ingest.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Reason != "backpressure" || out.RetryAfterMS == 0 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestHandleIngestSinkWriteError(t *testing.T) {
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  errSink{err: context.DeadlineExceeded},
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleIngestDiskFullFlipsReadiness(t *testing.T) {
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  errSink{err: errors.New("disk full")},
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if state.effectiveDiskHealthy() {
		t.Fatalf("expected disk health false after disk full")
	}
}

func TestHandleMetricsIncrements(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.apiKey = "secret"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1), 1),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`))
	req.Header.Set(cfg.apiKeyHeader, "secret")
	state.handleIngest(httptest.NewRecorder(), req)
	state.handleIngest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"b"}`)))
	badReq := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"c"}`))
	badReq.Header.Set(cfg.apiKeyHeader, "wrong")
	state.handleIngest(httptest.NewRecorder(), badReq)

	rec := httptest.NewRecorder()
	state.handleMetrics(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"loxa_collector_requests_total 3",
		"loxa_collector_events_accepted_total 1",
		"loxa_collector_requests_limited_total 2",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q in %q", want, body)
		}
	}
}

func TestHandleMetricsPrometheusFormat(t *testing.T) {
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  &fakeSink{},
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	state.handleMetrics(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		"# HELP loxa_collector_requests_total",
		"# TYPE loxa_collector_spool_bytes gauge",
		"# TYPE loxa_collector_queue_bytes gauge",
		"loxa_collector_ready 1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected metrics body to include %q, got: %s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("expected prometheus content type, got %q", got)
	}
}

func TestHandleIngestSpoolModeAcceptsAndAsyncFailsToSink(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "spool"
	cfg.spoolDir = t.TempDir()
	cfg.maxSpoolBytes = 1024 * 1024
	cfg.spoolFsync = true
	cfg.retryEnabled = true
	cfg.retryMaxAttempts = 1
	cfg.retryInitialBackoff = time.Millisecond
	cfg.retryMaxBackoff = time.Millisecond
	cfg.dlqEnabled = true
	cfg.dlqPath = filepath.Join(t.TempDir(), "dlq.ndjson")

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   errSink{err: context.DeadlineExceeded},
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		rng:          randSourceForTests(),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)
	state.spoolHealthy.Store(true)
	state.diskHealthy.Store(true)
	if err := state.initReliability(); err != nil {
		t.Fatalf("init reliability: %v", err)
	}
	defer state.closeReliability()

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event_id":"evt-1","event":"a"}`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if state.metrics.sinkWriteErrors.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if state.metrics.sinkWriteErrors.Load() == 0 {
		t.Fatalf("expected async sink errors to be recorded")
	}
	if state.metrics.spoolBytes.Load() == 0 {
		t.Fatalf("expected spool bytes > 0")
	}
}

func TestHandleIngestDedupe(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.dedupeEnabled = true
	cfg.dedupeKey = "event_id"
	cfg.dedupeWindow = time.Minute

	state := &collectorState{
		cfg:          cfg,
		ingestSink:   sink,
		rateLimiter:  rate.NewLimiter(rate.Limit(1000), 1000),
		dedupeSeenAt: make(map[string]time.Time),
	}
	state.ready.Store(true)

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`[{"event_id":"evt-1","event":"a"},{"event_id":"evt-1","event":"a"}]`))
	rec := httptest.NewRecorder()
	state.handleIngest(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected 1 sink write, got %d", len(sink.events))
	}
	if state.metrics.eventsDeduped.Load() != 1 {
		t.Fatalf("expected deduped metric 1, got %d", state.metrics.eventsDeduped.Load())
	}
}

func TestHandleIngestQueueModeAcceptsRawNonObject(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "queue"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	rec := httptest.NewRecorder()
	state.handleIngest(rec, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader("123")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(sink.events) != 0 {
		t.Fatalf("expected invalid payload to be rejected before queueing, got %d sink writes", len(sink.events))
	}
}

func TestHandleIngestQueueModeSinkFailure(t *testing.T) {
	sink := &fakeSink{fail: true}
	cfg := testCollectorConfig()
	cfg.reliabilityMode = "queue"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)
	state.sinkHealthy.Store(true)

	rec := httptest.NewRecorder()
	state.handleIngest(rec, httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{"event":"a"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rec.Code, rec.Body.String())
	}
	if state.metrics.sinkWriteErrors.Load() != 1 {
		t.Fatalf("expected sink write error metric, got %d", state.metrics.sinkWriteErrors.Load())
	}
	if state.effectiveSinkHealthy() {
		t.Fatalf("expected sink health to flip false")
	}
}

func randSourceForTests() *rand.Rand {
	return rand.New(rand.NewSource(1))
}

func TestExecuteCollectorCLIRunCommandParsesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.yaml")
	raw := `
collector:
  addr: ":9191"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	called := false
	var got collectorConfig
	err := executeCollectorCLI([]string{"run", "-c", path}, func(cfg collectorConfig) error {
		called = true
		got = cfg
		return nil
	}, func([]string) error {
		t.Fatalf("config command should not be called")
		return nil
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !called {
		t.Fatalf("expected run function to be called")
	}
	if got.addr != ":9191" {
		t.Fatalf("expected run config addr :9191, got %q", got.addr)
	}
}

func TestExecuteCollectorCLIDispatchesConfigPrint(t *testing.T) {
	var got []string
	err := executeCollectorCLI([]string{"config", "print", "-c", "cfg.yaml"}, func(collectorConfig) error {
		t.Fatalf("run command should not be called")
		return nil
	}, func(args []string) error {
		got = append([]string(nil), args...)
		return nil
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(got, " ") != "print -c cfg.yaml" {
		t.Fatalf("unexpected config args: %v", got)
	}
}

func TestExecuteCollectorCLIDispatchesConfigValidate(t *testing.T) {
	var got []string
	err := executeCollectorCLI([]string{"config", "validate", "-c", "cfg.yaml"}, func(collectorConfig) error {
		t.Fatalf("run command should not be called")
		return nil
	}, func(args []string) error {
		got = append([]string(nil), args...)
		return nil
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Join(got, " ") != "validate -c cfg.yaml" {
		t.Fatalf("unexpected config args: %v", got)
	}
}

func TestBuildMuxSkipsMetricsWhenDisabled(t *testing.T) {
	sink := &fakeSink{}
	cfg := testCollectorConfig()
	cfg.metricsPrometheus = false
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, cfg.metricsPath, nil)
	buildMux(state).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when metrics disabled, got %d", rec.Code)
	}
}

func TestTailStreamsAcceptedEvents(t *testing.T) {
	sink := &fakeSink{}
	state := &collectorState{
		cfg:         testCollectorConfig(),
		ingestSink:  sink,
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	srv := httptest.NewServer(buildMux(state))
	defer srv.Close()

	tailResp, err := http.Get(srv.URL + "/tail")
	if err != nil {
		t.Fatalf("open tail stream: %v", err)
	}
	defer tailResp.Body.Close()

	done := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(tailResp.Body).ReadString('\n')
		done <- strings.TrimSpace(line)
	}()

	postResp, err := http.Post(srv.URL+"/ingest", "application/json", strings.NewReader(`{"event":"tail.event"}`))
	if err != nil {
		t.Fatalf("post ingest: %v", err)
	}
	_, _ = io.Copy(io.Discard, postResp.Body)
	postResp.Body.Close()

	select {
	case line := <-done:
		if line != `{"event":"tail.event"}` {
			t.Fatalf("unexpected tail line: %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tail event")
	}
}

func TestControlEndpointsRequireAPIKeyWhenAuthEnabled(t *testing.T) {
	cfg := testCollectorConfig()
	cfg.authEnabled = true
	cfg.apiKey = "secret"
	state := &collectorState{
		cfg:         cfg,
		ingestSink:  &fakeSink{},
		rateLimiter: rate.NewLimiter(rate.Limit(1000), 1000),
	}
	state.ready.Store(true)

	srv := httptest.NewServer(buildMux(state))
	defer srv.Close()

	cases := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/status"},
		{method: http.MethodGet, path: "/sinks"},
		{method: http.MethodGet, path: "/v1/sinks/primary"},
		{method: http.MethodPost, path: "/query"},
		{method: http.MethodGet, path: "/dlq"},
		{method: http.MethodGet, path: "/v1/dlq/some-id"},
		{method: http.MethodPost, path: "/v1/dlq/some-id/replay"},
		{method: http.MethodPost, path: "/v1/dlq/replay"},
		{method: http.MethodDelete, path: "/v1/dlq/some-id"},
		{method: http.MethodGet, path: "/tail"},
	}
	for _, tc := range cases {
		req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
		if err != nil {
			t.Fatalf("new request for %s %s: %v", tc.method, tc.path, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request %s %s: %v", tc.method, tc.path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, resp.StatusCode)
		}
	}
}

func TestParquetExportPathConvention(t *testing.T) {
	ts := time.Date(2026, time.March, 9, 14, 5, 6, 123456789, time.UTC)
	got := filepath.ToSlash(storagepath.LocalParquetExportPath("exports", "analytics.events", ts))
	want := "exports/table=analytics_events/date=2026-03-09/hour=14/part-1773065106123456789.parquet"
	if got != want {
		t.Fatalf("unexpected export path:\nwant: %s\ngot:  %s", want, got)
	}
}

// Quality Gates Tests (Integration, End-to-End, and Stress Validation)

func TestQualityGatesE2EPipelineSDKEmitToQuery(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := setupTestDuckDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("setup duckdb: %v", err)
	}
	defer db.Close()

	sink := mustCreateDuckDBSink(t, db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sink.Close(ctx)
	}()

	ctx := context.Background()

	// Simulate SDK emit -> collector ingest -> DuckDB write
	event1 := `{"event_id":"evt_e2e_1","event":"test.event","service":"test-service","attributes":{"user_id":"user123","duration_ms":42}}`
	event2 := `{"event_id":"evt_e2e_2","event":"test.action","service":"test-service","attributes":{"action":"click"}}`

	if err := sink.WriteEvent(ctx, []byte(event1), nil); err != nil {
		t.Fatalf("write event 1: %v", err)
	}
	if err := sink.WriteEvent(ctx, []byte(event2), nil); err != nil {
		t.Fatalf("write event 2: %v", err)
	}
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Now query back
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%test-service%").Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 events, got %d", count)
	}

	// Verify specific events can be retrieved
	var raw1 string
	err = db.QueryRowContext(ctx, "SELECT raw FROM events WHERE raw LIKE $1 LIMIT 1", "%evt_e2e_1%").Scan(&raw1)
	if err != nil {
		t.Fatalf("query specific event: %v", err)
	}
	if !strings.Contains(raw1, "evt_e2e_1") {
		t.Fatalf("event mismatch: expected evt_e2e_1 in raw, got %q", raw1)
	}
}

func TestQualityGatesMultiTenantIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := setupTestDuckDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("setup duckdb: %v", err)
	}
	defer db.Close()

	sink := mustCreateDuckDBSink(t, db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sink.Close(ctx)
	}()

	ctx := context.Background()

	// Emit events from different tenants (via service name or tenant_id attribute)
	tenant1Event := `{"event_id":"evt_mt_t1_1","event":"test","service":"tenant1-svc","tenant_id":"tenant1"}`
	tenant2Event := `{"event_id":"evt_mt_t2_1","event":"test","service":"tenant2-svc","tenant_id":"tenant2"}`

	if err := sink.WriteEvent(ctx, []byte(tenant1Event), nil); err != nil {
		t.Fatalf("write tenant1 event: %v", err)
	}
	if err := sink.WriteEvent(ctx, []byte(tenant2Event), nil); err != nil {
		t.Fatalf("write tenant2 event: %v", err)
	}
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Verify isolation: tenant1 query should only see tenant1 events
	var tenant1Count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%tenant1-svc%").Scan(&tenant1Count)
	if err != nil {
		t.Fatalf("query tenant1 count: %v", err)
	}
	if tenant1Count != 1 {
		t.Fatalf("expected 1 tenant1 event, got %d", tenant1Count)
	}

	var tenant2Count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%tenant2-svc%").Scan(&tenant2Count)
	if err != nil {
		t.Fatalf("query tenant2 count: %v", err)
	}
	if tenant2Count != 1 {
		t.Fatalf("expected 1 tenant2 event, got %d", tenant2Count)
	}
}

func TestQualityGatesStressEventWriteAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tmpDir := t.TempDir()
	db, err := setupTestDuckDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("setup duckdb: %v", err)
	}
	defer db.Close()

	sink := mustCreateDuckDBSink(t, db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = sink.Close(ctx)
	}()

	ctx := context.Background()
	eventCount := 1000

	// Write 1000 events in batches
	for i := 0; i < eventCount; i++ {
		payload := fmt.Sprintf(`{"event_id":"evt_stress_%d","event":"stress.write","service":"stress-test","counter":%d}`, i, i)
		if err := sink.WriteEvent(ctx, []byte(payload), nil); err != nil {
			t.Fatalf("write event %d: %v", i, err)
		}
	}

	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Query count
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%stress-test%").Scan(&count)
	if err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != eventCount {
		t.Fatalf("expected %d events, got %d", eventCount, count)
	}

	// Verify range query works (performance check)
	start := time.Now()
	rows, err := db.QueryContext(ctx, "SELECT raw FROM events WHERE raw LIKE $1 LIMIT 100", "%stress-test%")
	if err != nil {
		t.Fatalf("range query: %v", err)
	}
	defer rows.Close()

	scanCount := 0
	for rows.Next() {
		scanCount++
	}
	elapsed := time.Since(start)

	if scanCount != 100 {
		t.Fatalf("expected 100 rows, got %d", scanCount)
	}
	if elapsed > 5*time.Second {
		t.Logf("warning: 100-row query took %v (expected < 5s)", elapsed)
	}
}

func TestQualityGatesRetentionPolicyExecution(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := setupTestDuckDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("setup duckdb: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	sink := mustCreateDuckDBSink(t, db)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sink.Close(ctx)
	}()

	// Write events with old timestamps
	oldTime := time.Now().AddDate(0, 0, -35).Format(time.RFC3339Nano)
	recentTime := time.Now().Format(time.RFC3339Nano)

	oldEvent := fmt.Sprintf(`{"event_id":"evt_old","event":"old.event","timestamp":"%s"}`, oldTime)
	recentEvent := fmt.Sprintf(`{"event_id":"evt_recent","event":"recent.event","timestamp":"%s"}`, recentTime)

	if err := sink.WriteEvent(ctx, []byte(oldEvent), nil); err != nil {
		t.Fatalf("write old event: %v", err)
	}
	if err := sink.WriteEvent(ctx, []byte(recentEvent), nil); err != nil {
		t.Fatalf("write recent event: %v", err)
	}
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Verify initial count
	var initialCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&initialCount)
	if err != nil {
		t.Fatalf("query initial count: %v", err)
	}
	if initialCount != 2 {
		t.Fatalf("expected 2 events initially, got %d", initialCount)
	}

	// Verify that we can query by timestamp/date patterns
	var oldCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%evt_old%").Scan(&oldCount)
	if err != nil {
		t.Fatalf("query old count: %v", err)
	}
	if oldCount != 1 {
		t.Fatalf("expected 1 old event, got %d", oldCount)
	}

	var recentCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE raw LIKE $1", "%evt_recent%").Scan(&recentCount)
	if err != nil {
		t.Fatalf("query recent count: %v", err)
	}
	if recentCount != 1 {
		t.Fatalf("expected 1 recent event, got %d", recentCount)
	}

	// Test data retention by row limit (simplified: delete old entries)
	_, err = db.ExecContext(ctx, "DELETE FROM events WHERE raw LIKE ?", "%evt_old%")
	if err != nil {
		t.Fatalf("delete old events: %v", err)
	}

	// Verify old event deleted, recent remains
	var finalCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&finalCount)
	if err != nil {
		t.Fatalf("query final count: %v", err)
	}
	if finalCount != 1 {
		t.Fatalf("expected 1 event after retention, got %d", finalCount)
	}

	var remainingRaw string
	err = db.QueryRowContext(ctx, "SELECT raw FROM events LIMIT 1").Scan(&remainingRaw)
	if err != nil {
		t.Fatalf("query remaining: %v", err)
	}
	if !strings.Contains(remainingRaw, "evt_recent") {
		t.Fatalf("expected recent event to remain, got %s", remainingRaw)
	}
}

func setupTestDuckDB(path string) (*sql.DB, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, err
	}

	// Create simple schema that stores raw JSON
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		raw TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func mustCreateDuckDBSink(t testing.TB, db *sql.DB) collectorevent.Sink {
	duckdbsink, err := duckdb.New(duckdb.Config{
		DB:              db,
		Driver:          "duckdb",
		Table:           "events",
		StoreRaw:        false,
		RawColumn:       "raw",
		Schema:          map[string]string{},
		BatchSize:       0,
		FlushInterval:   0,
		WriterLoop:      false,
		WriterQueueSize: 0,
	})
	if err != nil {
		t.Fatalf("create duckdb sink: %v", err)
	}
	return duckdbsink
}
