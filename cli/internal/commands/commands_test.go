package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/astraive/loxa-cli/internal/config"
)

func TestEmitSamplePrintsEnvelope(t *testing.T) {
	cfg := config.Config{CollectorURL: "http://127.0.0.1:9090"}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	err := EmitCommand(cfg, []string{"sample", "--service", "checkout", "--event", "payment.completed", "--print"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("emit sample: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, `"event": "payment.completed"`) {
		t.Fatalf("expected printed sample event, got %s", out)
	}
}

func TestEmitSampleWritesOutputFile(t *testing.T) {
	cfg := config.Config{CollectorURL: "http://127.0.0.1:9090"}
	path := filepath.Join(t.TempDir(), "sample.json")
	if err := EmitCommand(cfg, []string{"sample", "--output", path, "--print"}); err != nil {
		t.Fatalf("emit sample: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(raw), `"api_version": "v1"`) {
		t.Fatalf("unexpected output file: %s", string(raw))
	}
}

func TestDLQListHitsCollectorAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/dlq" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"events":[],"count":0}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := DLQCommand(context.Background(), cfg, []string{"list"}); err != nil {
		t.Fatalf("dlq list: %v", err)
	}
}

func TestEmitSampleSendsAPIKeyFromEnv(t *testing.T) {
	t.Setenv("LOXA_API_KEY", "test-key")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := EmitCommand(cfg, []string{"sample", "--service", "checkout", "--event", "payment.completed"}); err != nil {
		t.Fatalf("emit sample: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("expected Authorization: Bearer header, got %q", gotAuth)
	}
}

func TestSchemaListHitsCollectorAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/schema" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"registry":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := SchemaCommand(cfg, []string{"list"}); err != nil {
		t.Fatalf("schema list: %v", err)
	}
}

func TestAuditPIIHitsCollectorAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/audit/pii" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"count":0,"findings":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := AuditCommand(context.Background(), cfg, []string{"pii"}); err != nil {
		t.Fatalf("audit pii: %v", err)
	}
}

func TestDeployComposeCopiesCollectorAssets(t *testing.T) {
	collectorRepo := filepath.Join(t.TempDir(), "collector")
	deployDir := filepath.Join(collectorRepo, "deploy")
	if err := os.MkdirAll(deployDir, 0o755); err != nil {
		t.Fatalf("mkdir deploy dir: %v", err)
	}
	src := filepath.Join(deployDir, "docker-compose.yml")
	if err := os.WriteFile(src, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose asset: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "out")
	cfg := config.Config{CollectorRepoPath: collectorRepo}
	if err := DeployCommand(cfg, []string{"compose", "--out", outDir}); err != nil {
		t.Fatalf("deploy compose: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "docker-compose.yml")); err != nil {
		t.Fatalf("expected copied compose file: %v", err)
	}
}

func TestDashboardInstallCopiesCollectorAssets(t *testing.T) {
	collectorRepo := filepath.Join(t.TempDir(), "collector")
	dashboardDir := filepath.Join(collectorRepo, "deploy", "observability", "grafana", "provisioning", "dashboards")
	if err := os.MkdirAll(dashboardDir, 0o755); err != nil {
		t.Fatalf("mkdir dashboard dir: %v", err)
	}
	src := filepath.Join(dashboardDir, "loxa-collector.json")
	if err := os.WriteFile(src, []byte(`{"title":"LOXA Collector"}`), 0o644); err != nil {
		t.Fatalf("write dashboard asset: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(wd) }()

	cfg := config.Config{CollectorRepoPath: collectorRepo, CollectorURL: "http://127.0.0.1:65535"}
	if err := DashboardCommand(cfg, []string{"install"}); err != nil {
		t.Fatalf("dashboard install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".loxa-dashboard", "dashboards", "loxa-collector.json")); err != nil {
		t.Fatalf("expected copied dashboard asset: %v", err)
	}
}

func TestDoctorChecksCortexWhenConfigured(t *testing.T) {
	collectorRepo := t.TempDir()
	specRepo := t.TempDir()
	cortexRepo := t.TempDir()

	collectorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/ready":
			w.WriteHeader(http.StatusOK)
		case "/v1/status":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/events":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted","accepted":1}`))
		case "/metrics":
			_, _ = w.Write([]byte("loxa_collector_events_accepted_total 1\n"))
		default:
			t.Fatalf("unexpected collector path %s", r.URL.Path)
		}
	}))
	defer collectorSrv.Close()

	cortexSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			w.WriteHeader(http.StatusOK)
		case "/metrics":
			_, _ = w.Write([]byte("loxa_cortex_events_ingested_total 1\n"))
		default:
			t.Fatalf("unexpected cortex path %s", r.URL.Path)
		}
	}))
	defer cortexSrv.Close()

	dbPath := filepath.Join(t.TempDir(), "doctor.db")
	cfg := config.Config{
		CollectorRepoPath: collectorRepo,
		CortexRepoPath:    cortexRepo,
		SpecRepoPath:      specRepo,
		CollectorURL:      collectorSrv.URL,
		DuckDBPath:        dbPath,
		Cortex:            &config.CortexConfig{URL: cortexSrv.URL},
	}
	if err := DoctorCommand(context.Background(), cfg, nil); err != nil {
		t.Fatalf("doctor: %v", err)
	}
}

func TestRunCortexServerRequiresConfiguredRepo(t *testing.T) {
	cfg := config.Config{}
	err := RunCortexServer(context.Background(), cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "cortex_repo_path") {
		t.Fatalf("expected cortex repo path error, got %v", err)
	}
}

func TestRunCortexReconstructHitsSharedClientPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/reconstruct" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"incident_id":"inc_1","timestamp":"2026-05-17T00:00:00Z","confidence":0.9,"causal_chain":[],"similar_incidents":[],"symptoms":[],"suggested_actions":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{Cortex: &config.CortexConfig{URL: srv.URL}}
	if err := RunCortexReconstruct(context.Background(), cfg, []string{"--incident", "inc_1"}); err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
}

func TestRunCortexGraphHitsSharedClientPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/graph/service/checkout") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"nodes":[{"id":"n1"}],"edges":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{Cortex: &config.CortexConfig{URL: srv.URL}}
	if err := RunCortexGraph(context.Background(), cfg, []string{"--service", "checkout"}); err != nil {
		t.Fatalf("graph: %v", err)
	}
}

func TestStatusHitsCollectorAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok","uptime":"5m","events_accepted":100}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := StatusCommand(context.Background(), cfg, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
}

func TestSinksListHitsCollectorAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sinks" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"sinks":[{"name":"duckdb","type":"primary","health":"ok"}]}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := SinksCommand(context.Background(), cfg, []string{"list"}); err != nil {
		t.Fatalf("sinks list: %v", err)
	}
}

func TestGraphServiceHitsCortexAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/graph/service/") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"nodes":[{"id":"n1","label":"checkout","type":"service"}],"edges":[]}`))
	}))
	defer srv.Close()

	cfg := config.Config{Cortex: &config.CortexConfig{URL: srv.URL}}
	if err := GraphCommand(context.Background(), cfg, []string{"service", "checkout"}); err != nil {
		t.Fatalf("graph: %v", err)
	}
}

func TestDebugEventQueriesCollector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/query" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"columns":["event_id","event"],"rows":[["evt_1","test.event"]]}`))
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := DebugCommand(context.Background(), cfg, []string{"event", "evt_1"}); err != nil {
		t.Fatalf("debug event: %v", err)
	}
}

func TestDebugPipelineShowsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/status":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/sinks":
			_, _ = w.Write([]byte(`{"sinks":[]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cfg := config.Config{CollectorURL: srv.URL}
	if err := DebugCommand(context.Background(), cfg, []string{"pipeline"}); err != nil {
		t.Fatalf("debug pipeline: %v", err)
	}
}

func TestEmitWithCustomKindAndOutcome(t *testing.T) {
	cfg := config.Config{CollectorURL: "http://127.0.0.1:9090"}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	err := EmitCommand(cfg, []string{"sample", "--kind", "http", "--outcome", "error", "--level", "warn", "--print"})
	_ = w.Close()
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	out := string(buf[:n])
	if !strings.Contains(out, `"kind": "http"`) {
		t.Fatalf("expected kind=http in output, got %s", out)
	}
	if !strings.Contains(out, `"outcome": "error"`) {
		t.Fatalf("expected outcome=error in output, got %s", out)
	}
}
