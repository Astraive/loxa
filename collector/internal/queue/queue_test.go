package queue

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskSinkEncryptsPayloadAtRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.ndjson")
	sink, err := NewDiskSink(DiskSinkConfig{
		Path:       path,
		EncryptKey: "queue-secret",
	})
	if err != nil {
		t.Fatalf("new disk sink: %v", err)
	}
	defer func() {
		_ = sink.Close(context.Background())
	}()

	payload := []byte(`{"event_id":"evt-1","message":"secret"}`)
	if err := sink.WriteEvent(context.Background(), payload, nil); err != nil {
		t.Fatalf("write event: %v", err)
	}
	if err := sink.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sink file: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("expected persisted queue payload")
	}
	if strings.Contains(string(raw), `"message":"secret"`) || strings.Contains(string(raw), `"event_id":"evt-1"`) {
		t.Fatalf("expected encrypted payload, got plaintext %q", string(raw))
	}
}
