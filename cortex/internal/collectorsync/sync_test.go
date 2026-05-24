package collectorsync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/collectorbridge"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

type fakeBatchProcessor struct {
	mu        sync.Mutex
	batches   [][]string
	err       error
	onProcess func()
}

func (f *fakeBatchProcessor) ProcessBatch(_ context.Context, events []*models.Event) error {
	if f.err != nil {
		return f.err
	}
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	f.mu.Lock()
	f.batches = append(f.batches, ids)
	f.mu.Unlock()
	if f.onProcess != nil {
		f.onProcess()
	}
	return nil
}

func (f *fakeBatchProcessor) FlattenedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, batch := range f.batches {
		ids = append(ids, batch...)
	}
	return ids
}

func TestRunSourceOfTruthSyncProcessesPollThenTail(t *testing.T) {
	t.Helper()
	pollEvent := map[string]any{
		"id":         "evt-1",
		"timestamp":  "2026-01-01T00:00:00Z",
		"kind":       "log",
		"service":    "checkout",
		"provenance": "collector",
	}
	tailEvent := map[string]any{
		"id":         "evt-2",
		"timestamp":  "2026-01-01T00:00:01Z",
		"kind":       "log",
		"service":    "checkout",
		"provenance": "collector",
	}

	var queryCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/query":
			call := queryCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if call == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"rows": []map[string]any{{"raw": pollEvent}},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"rows": []map[string]any{}})
		case "/tail":
			w.Header().Set("Content-Type", "application/x-ndjson")
			flusher, _ := w.(http.Flusher)
			_ = json.NewEncoder(w).Encode(tailEvent)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "collector.cursor")
	cfg := config.CollectorConfig{
		URL:                  server.URL,
		SourceOfTruth:        true,
		BatchSize:            10,
		TailEnabled:          true,
		TailBufferSize:       8,
		TailBatchSize:        1,
		TailFlushInterval:    10 * time.Millisecond,
		TailReconnectBackoff: 10 * time.Millisecond,
		QueryTable:           "events",
		RawColumn:            "raw",
		TimestampColumn:      "timestamp",
		CursorPath:           cursorPath,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processor := &fakeBatchProcessor{}
	processor.onProcess = func() {
		if len(processor.FlattenedIDs()) >= 2 {
			cancel()
		}
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		RunSourceOfTruthSync(ctx, cfg, processor)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for source-of-truth sync")
	}

	ids := processor.FlattenedIDs()
	if len(ids) < 2 || ids[0] != "evt-1" || ids[1] != "evt-2" {
		t.Fatalf("expected poll and tail events in order, got %v", ids)
	}

	client := collectorbridge.NewClient(cfg)
	cursor, err := client.LoadCursor()
	if err != nil {
		t.Fatalf("load cursor: %v", err)
	}
	if cursor.EventID != "evt-2" {
		t.Fatalf("expected cursor to advance to tail event, got %+v", cursor)
	}
}

func TestRunPollCatchupDoesNotAdvanceCursorOnFailure(t *testing.T) {
	t.Helper()
	event := map[string]any{
		"id":         "evt-1",
		"timestamp":  "2026-01-01T00:00:00Z",
		"kind":       "log",
		"service":    "checkout",
		"provenance": "collector",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/query" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rows": []map[string]any{{"raw": event}},
		})
	}))
	defer server.Close()

	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "collector.cursor")
	cfg := config.CollectorConfig{
		URL:             server.URL,
		SourceOfTruth:   true,
		BatchSize:       10,
		QueryTable:      "events",
		RawColumn:       "raw",
		TimestampColumn: "timestamp",
		CursorPath:      cursorPath,
	}

	client := collectorbridge.NewClient(cfg)
	state := &cursorState{}
	proc := &fakeBatchProcessor{err: errors.New("boom")}

	err := runPollCatchup(context.Background(), cfg, client, proc, state)
	if err == nil {
		t.Fatal("expected poll catch-up to fail")
	}
	if _, statErr := os.Stat(cursorPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no persisted cursor on failure, got stat err=%v", statErr)
	}
	if !state.Current().Timestamp.IsZero() || state.Current().EventID != "" {
		t.Fatalf("expected in-memory cursor to remain zero, got %+v", state.Current())
	}
}
