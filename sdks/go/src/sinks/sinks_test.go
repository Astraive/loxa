package sinks

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStandardSinkConstructorsAndLifecycle(t *testing.T) {
	ctx := context.Background()
	eventBytes := []byte(`{"event":"test"}` + "\n")
	if got := StdoutSink().Name(); got == "" {
		t.Fatal("stdout sink name is empty")
	}
	if got := StderrSink().Name(); got == "" {
		t.Fatal("stderr sink name is empty")
	}

	memory, store := MemorySink()
	if memory.Name() != "memory" {
		t.Fatalf("memory name = %q", memory.Name())
	}
	if err := memory.WriteEvent(ctx, eventBytes, nil); err != nil {
		t.Fatalf("memory WriteEvent: %v", err)
	}
	if store.Len() != 1 || len(store.Raw()) != 1 {
		t.Fatalf("memory store = len %d raw %d", store.Len(), len(store.Raw()))
	}
	if err := memory.Flush(ctx); err != nil {
		t.Fatalf("memory Flush: %v", err)
	}
	store.Clear()
	if store.Len() != 0 {
		t.Fatal("memory Clear did not remove events")
	}
	if err := memory.Close(ctx); err != nil {
		t.Fatalf("memory Close: %v", err)
	}
	if err := NoopSink().WriteEvent(ctx, eventBytes, nil); err != nil {
		t.Fatalf("noop WriteEvent: %v", err)
	}
}

func TestFileAndRotatingSinkConstructorsWriteEvents(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "events.ndjson")
	file, err := FileSink(path)
	if err != nil {
		t.Fatalf("FileSink: %v", err)
	}
	if err := file.WriteEvent(ctx, []byte("file\n"), nil); err != nil {
		t.Fatalf("file WriteEvent: %v", err)
	}
	if err := file.Flush(ctx); err != nil {
		t.Fatalf("file Flush: %v", err)
	}
	if err := file.Close(ctx); err != nil {
		t.Fatalf("file Close: %v", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "file\n" {
		t.Fatalf("file contents = %q, read error = %v", got, err)
	}

	rotating, err := RotatingFileSink(RotatingFileConfig{Path: filepath.Join(dir, "rotating.ndjson"), MaxBytes: 32, MaxAge: time.Hour})
	if err != nil {
		t.Fatalf("RotatingFileSink: %v", err)
	}
	if err := rotating.WriteEvent(ctx, []byte("rotating\n"), nil); err != nil {
		t.Fatalf("rotating WriteEvent: %v", err)
	}
	if err := rotating.Flush(ctx); err != nil {
		t.Fatalf("rotating Flush: %v", err)
	}
	if err := rotating.Close(ctx); err != nil {
		t.Fatalf("rotating Close: %v", err)
	}
}
