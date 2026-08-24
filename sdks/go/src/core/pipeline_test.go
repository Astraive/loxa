package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type nopSinkWriter struct{}

func (nopSinkWriter) WriteEvent(context.Context, []byte, *Event) error { return nil }
func (nopSinkWriter) Flush(context.Context) error                      { return nil }
func (nopSinkWriter) Close(context.Context) error                      { return nil }

type failingSinkWriter struct{ err error }

func (s failingSinkWriter) WriteEvent(context.Context, []byte, *Event) error { return s.err }
func (failingSinkWriter) Flush(context.Context) error                        { return nil }
func (failingSinkWriter) Close(context.Context) error                        { return nil }

type gateSinkWriter struct {
	entered chan string
	gate    chan struct{}
	mu      sync.Mutex
	writes  []string
}

func newGateSinkWriter() *gateSinkWriter {
	return &gateSinkWriter{
		entered: make(chan string, 8),
		gate:    make(chan struct{}, 8),
	}
}

func (s *gateSinkWriter) WriteEvent(_ context.Context, encoded []byte, _ *Event) error {
	select {
	case s.entered <- string(encoded):
	default:
	}
	<-s.gate
	s.mu.Lock()
	s.writes = append(s.writes, string(encoded))
	s.mu.Unlock()
	return nil
}

func (s *gateSinkWriter) Flush(context.Context) error { return nil }
func (s *gateSinkWriter) Close(context.Context) error { return nil }

func (s *gateSinkWriter) snapshotWrites() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.writes))
	copy(out, s.writes)
	return out
}

func TestPipelineEnqueueAfterShutdown(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		QueueSize:     8,
		Workers:       1,
		FlushInterval: time.Second,
		Sinks:         []SinkWriter{nopSinkWriter{}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	_, err := p.Enqueue(PipelineItem{Encoded: []byte(`{}`)})
	if !errors.Is(err, ErrPipelineClosed) {
		t.Fatalf("expected ErrPipelineClosed, got %v", err)
	}
}

func TestPipelineWorkerHandlesClosedQueue(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		QueueSize:     1,
		Workers:       1,
		FlushInterval: time.Second,
		Sinks:         []SinkWriter{nopSinkWriter{}},
	})

	close(p.queue)
	p.stopOnce.Do(func() { close(p.done) })

	waitCh := make(chan struct{})
	go func() {
		p.workerWG.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("worker did not exit after queue close")
	}
}

func TestP0PipelineDropOldestPreservesNewest(t *testing.T) {
	sink := newGateSinkWriter()
	p := NewPipeline(PipelineConfig{
		QueueSize:     1,
		Workers:       1,
		FlushInterval: time.Hour,
		Backpressure:  DropOldest,
		Sinks:         []SinkWriter{sink},
	})

	if ok, err := p.Enqueue(PipelineItem{Encoded: []byte("one")}); err != nil || !ok {
		t.Fatalf("enqueue one failed: ok=%v err=%v", ok, err)
	}

	select {
	case got := <-sink.entered:
		if got != "one" {
			t.Fatalf("expected first write to be one, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("worker did not start first write")
	}

	if ok, err := p.Enqueue(PipelineItem{Encoded: []byte("two")}); err != nil || !ok {
		t.Fatalf("enqueue two failed: ok=%v err=%v", ok, err)
	}
	if ok, err := p.Enqueue(PipelineItem{Encoded: []byte("three")}); err != nil || !ok {
		t.Fatalf("enqueue three failed: ok=%v err=%v", ok, err)
	}

	for i := 0; i < 3; i++ {
		sink.gate <- struct{}{}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	writes := sink.snapshotWrites()
	if len(writes) != 2 {
		t.Fatalf("expected 2 writes after drop-oldest, got %v", writes)
	}
	if writes[0] != "one" || writes[1] != "three" {
		t.Fatalf("expected writes [one three], got %v", writes)
	}
}

func TestPipelineWorkerReportsWriteErrors(t *testing.T) {
	writeErr := errors.New("sink write failed")
	errCh := make(chan error, 4)
	p := NewPipeline(PipelineConfig{
		QueueSize:     2,
		Workers:       1,
		FlushInterval: time.Hour,
		Sinks:         []SinkWriter{failingSinkWriter{err: writeErr}},
		OnError: func(err error) {
			errCh <- err
		},
	})

	ok, err := p.Enqueue(PipelineItem{Encoded: []byte("event")})
	if err != nil || !ok {
		t.Fatalf("enqueue failed: ok=%v err=%v", ok, err)
	}

	select {
	case got := <-errCh:
		if !errors.Is(got, writeErr) {
			t.Fatalf("expected write error, got %v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected worker write error callback")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestPipelineConcurrentEnqueueAndFlushDoesNotLoseDrainWakeup(t *testing.T) {
	p := NewPipeline(PipelineConfig{
		QueueSize:     1,
		Workers:       4,
		FlushInterval: time.Hour,
		Backpressure:  Block,
		Sinks:         []SinkWriter{nopSinkWriter{}},
	})

	for round := range 100 {
		var producers sync.WaitGroup
		for range 32 {
			producers.Add(1)
			go func() {
				defer producers.Done()
				ok, err := p.Enqueue(PipelineItem{Encoded: []byte(`{"event":"race"}`)})
				if err != nil || !ok {
					t.Errorf("enqueue failed: ok=%v err=%v", ok, err)
				}
			}()
		}
		producers.Wait()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := p.Flush(ctx); err != nil {
			cancel()
			t.Fatalf("flush round %d failed: %v (pending=%d)", round, err, p.pending.Load())
		}
		cancel()
		if pending := p.pending.Load(); pending != 0 {
			t.Fatalf("flush round %d left pending=%d", round, pending)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}
