//go:build integration

package collectorsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
)

func TestRunSourceOfTruthSyncAgainstLiveCollector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live collector integration test in short mode")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	collectorDir := filepath.Join(repoRoot, "collector")
	binDir := t.TempDir()
	collectorBin := filepath.Join(binDir, "loza-collector-test.exe")
	buildCmd := exec.Command("go", "build", "-o", collectorBin, "./cmd/loza-collector")
	buildCmd.Dir = collectorDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("build live collector binary: %v", err)
	}

	port := freePort(t)
	duckdbPath := filepath.Join(t.TempDir(), "collector-live.duckdb")
	cmd := exec.Command(collectorBin, "run", "-c", "loza-collector.defaults.yaml", "--addr", fmt.Sprintf("127.0.0.1:%d", port), "--duckdb-path", duckdbPath)
	cmd.Dir = collectorDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start live collector: %v", err)
	}
	defer func() {
		cancel()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitForCollector(t, baseURL+"/health")

	postCollectorEvent(t, baseURL, map[string]any{
		"id":         "evt-pre",
		"timestamp":  "2026-01-01T00:00:00Z",
		"kind":       "log",
		"service":    "checkout",
		"provenance": "collector",
	})

	cfg := config.CollectorConfig{
		URL:                  baseURL,
		SourceOfTruth:        true,
		BatchSize:            10,
		TailTransport:        "websocket",
		TailEnabled:          true,
		TailBufferSize:       16,
		TailBatchSize:        1,
		TailFlushInterval:    20 * time.Millisecond,
		TailReconnectBackoff: 20 * time.Millisecond,
		QueryTable:           "events",
		RawColumn:            "raw",
		TimestampColumn:      "timestamp",
		CursorPath:           filepath.Join(t.TempDir(), "collector.cursor"),
	}

	processor := &fakeBatchProcessor{}
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()
	processor.onProcess = func() {
		ids := processor.FlattenedIDs()
		if len(ids) >= 2 {
			syncCancel()
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		RunSourceOfTruthSync(syncCtx, cfg, processor)
	}()

	postCollectorEvent(t, baseURL, map[string]any{
		"id":         "evt-live",
		"timestamp":  "2026-01-01T00:00:01Z",
		"kind":       "log",
		"service":    "checkout",
		"provenance": "collector",
	})

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for live collector sync")
	}

	ids := processor.FlattenedIDs()
	if len(ids) < 2 || ids[0] != "evt-pre" || ids[1] != "evt-live" {
		t.Fatalf("expected catch-up then live tail events, got %v", ids)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForCollector(t *testing.T, healthURL string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("collector did not become healthy: %s", healthURL)
}

func postCollectorEvent(t *testing.T, baseURL string, event map[string]any) {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	resp, err := http.Post(baseURL+"/events", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post collector event: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected ingest status: %d", resp.StatusCode)
	}
}
