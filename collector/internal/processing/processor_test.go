package processing

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
)

type fakeSink struct {
	mu     sync.Mutex
	writes int
	errs   []error
}

func (s *fakeSink) Name() string { return "fake" }

func (s *fakeSink) WriteEvent(context.Context, []byte, *collectorevent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.writes
	s.writes++
	if idx < len(s.errs) {
		return s.errs[idx]
	}
	return nil
}

func (s *fakeSink) Flush(context.Context) error { return nil }
func (s *fakeSink) Close(context.Context) error { return nil }

func (s *fakeSink) Writes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

func TestProcessorInvalidEventWritesDLQ(t *testing.T) {
	dlqPath := filepath.Join(t.TempDir(), "dlq.ndjson")
	p, err := New(Config{
		DLQEnabled:          true,
		DLQPath:             dlqPath,
		ValidateJSONObjects: true,
	}, &fakeSink{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = p.Close() }()

	res := p.Process(context.Background(), []byte("123"))
	if !res.Invalid {
		t.Fatalf("expected invalid result")
	}
	if !errors.Is(res.Err, ErrInvalidEvent) {
		t.Fatalf("expected ErrInvalidEvent, got %v", res.Err)
	}

	raw, err := os.ReadFile(dlqPath)
	if err != nil {
		t.Fatalf("read dlq: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one dlq entry, got %d", len(lines))
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("decode dlq entry: %v", err)
	}
	if !strings.Contains(entry["error"].(string), "invalid event payload") {
		t.Fatalf("unexpected dlq error: %v", entry["error"])
	}
}

func TestProcessorRetryAndDLQOnPrimaryFailure(t *testing.T) {
	dlqPath := filepath.Join(t.TempDir(), "dlq.ndjson")
	primary := &fakeSink{errs: []error{errors.New("primary failed"), errors.New("primary failed")}}
	p, err := New(Config{
		DeliveryPolicy:      "require_primary",
		RetryEnabled:        true,
		RetryMaxAttempts:    2,
		RetryInitialBackoff: time.Millisecond,
		RetryMaxBackoff:     time.Millisecond,
		DLQEnabled:          true,
		DLQPath:             dlqPath,
		DLQOnPrimaryFail:    true,
		ValidateJSONObjects: true,
	}, primary, nil, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = p.Close() }()

	res := p.Process(context.Background(), []byte(`{"event_id":"evt-1","event":"a"}`))
	if res.Err == nil {
		t.Fatalf("expected processing error")
	}
	if got := primary.Writes(); got != 2 {
		t.Fatalf("expected 2 retries, got %d", got)
	}

	raw, err := os.ReadFile(dlqPath)
	if err != nil {
		t.Fatalf("read dlq: %v", err)
	}
	if !strings.Contains(string(raw), "primary failed") {
		t.Fatalf("expected dlq to contain primary failure, got %s", string(raw))
	}
}

func TestProcessorDedupeSkipsSecondWrite(t *testing.T) {
	primary := &fakeSink{}
	p, err := New(Config{
		DedupeEnabled:       true,
		DedupeKey:           "event_id",
		DedupeWindow:        time.Minute,
		ValidateJSONObjects: true,
	}, primary, nil, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = p.Close() }()

	first := p.Process(context.Background(), []byte(`{"event_id":"evt-1","event":"a"}`))
	second := p.Process(context.Background(), []byte(`{"event_id":"evt-1","event":"a"}`))
	if !first.Accepted || first.Deduped {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if !second.Accepted || !second.Deduped {
		t.Fatalf("expected second event to dedupe, got %+v", second)
	}
	if got := primary.Writes(); got != 1 {
		t.Fatalf("expected single sink write, got %d", got)
	}
}

func TestProcessorBestEffortUsesSecondarySuccess(t *testing.T) {
	primary := &fakeSink{errs: []error{errors.New("primary failed")}}
	secondary := &fakeSink{}
	p, err := New(Config{
		DeliveryPolicy:      "best_effort",
		ValidateJSONObjects: true,
	}, primary, []NamedSink{{Name: "secondary", Sink: secondary}}, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = p.Close() }()

	res := p.Process(context.Background(), []byte(`{"event":"a"}`))
	if res.Err != nil {
		t.Fatalf("expected best effort to succeed, got %v", res.Err)
	}
	if !res.Accepted {
		t.Fatalf("expected accepted result")
	}
	if res.Outcome.FailureCount() != 1 {
		t.Fatalf("expected one tracked failure, got %d", res.Outcome.FailureCount())
	}
}
