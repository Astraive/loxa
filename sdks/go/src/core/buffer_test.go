package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestEventBuffer_BatchSizeFlush tests that buffer flushes when batch_size is reached.
// Requirement 33.4
func TestEventBuffer_BatchSizeFlush(t *testing.T) {
	var flushCount atomic.Int32
	var flushedEvents atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     10,
		FlushInterval: time.Hour, // Long interval to avoid time-based flushes
		MaxBufferSize: 100,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			flushedEvents.Add(int32(len(events)))
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 9 events - should not flush
	for i := 0; i < 9; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Give a moment for any potential flush
	time.Sleep(50 * time.Millisecond)

	if flushCount.Load() != 0 {
		t.Errorf("Expected no flush with 9 events, got %d flushes", flushCount.Load())
	}

	// Add 10th event - should trigger flush
	ev := &Event{EventID: NewUUIDv7()}
	err := buf.Add(ctx, ev, []byte("test"))
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Give a moment for flush to complete
	time.Sleep(50 * time.Millisecond)

	if flushCount.Load() != 1 {
		t.Errorf("Expected 1 flush after 10 events, got %d", flushCount.Load())
	}

	if flushedEvents.Load() != 10 {
		t.Errorf("Expected 10 flushed events, got %d", flushedEvents.Load())
	}
}

// TestEventBuffer_IntervalFlush tests that buffer flushes when flush_interval elapses.
// Requirement 33.5
func TestEventBuffer_IntervalFlush(t *testing.T) {
	var flushCount atomic.Int32
	var flushedEvents atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     100,                // Large batch size to avoid batch-based flush
		FlushInterval: 100 * time.Millisecond, // Short interval for testing
		MaxBufferSize: 1000,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			flushedEvents.Add(int32(len(events)))
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 5 events
	for i := 0; i < 5; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Wait for interval to elapse
	time.Sleep(150 * time.Millisecond)

	if flushCount.Load() < 1 {
		t.Errorf("Expected at least 1 flush after interval, got %d", flushCount.Load())
	}

	if flushedEvents.Load() != 5 {
		t.Errorf("Expected 5 flushed events, got %d", flushedEvents.Load())
	}
}

// TestEventBuffer_ManualFlush tests that Flush() immediately flushes buffered events.
// Requirement 33.6
func TestEventBuffer_ManualFlush(t *testing.T) {
	var flushCount atomic.Int32
	var flushedEvents atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
		MaxBufferSize: 1000,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			flushedEvents.Add(int32(len(events)))
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 3 events
	for i := 0; i < 3; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Manually flush
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if flushCount.Load() != 1 {
		t.Errorf("Expected 1 flush, got %d", flushCount.Load())
	}

	if flushedEvents.Load() != 3 {
		t.Errorf("Expected 3 flushed events, got %d", flushedEvents.Load())
	}

	// Buffer should be empty now
	if buf.Size() != 0 {
		t.Errorf("Expected empty buffer after flush, got size %d", buf.Size())
	}
}

// TestEventBuffer_CloseFlush tests that Close() flushes remaining events.
// Requirement 33.7
func TestEventBuffer_CloseFlush(t *testing.T) {
	var flushCount atomic.Int32
	var flushedEvents atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
		MaxBufferSize: 1000,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			flushedEvents.Add(int32(len(events)))
			return nil
		},
	})

	ctx := context.Background()

	// Add 7 events
	for i := 0; i < 7; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Close should flush
	err := buf.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if flushCount.Load() != 1 {
		t.Errorf("Expected 1 flush on close, got %d", flushCount.Load())
	}

	if flushedEvents.Load() != 7 {
		t.Errorf("Expected 7 flushed events, got %d", flushedEvents.Load())
	}
}

// TestEventBuffer_OverflowDropsOldest tests that buffer drops oldest events when max_buffer_size is exceeded.
// Requirement 33.9
func TestEventBuffer_OverflowDropsOldest(t *testing.T) {
	var mu sync.Mutex
	var flushedEventIDs []string

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
		MaxBufferSize: 10, // Small buffer to test overflow
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			mu.Lock()
			defer mu.Unlock()
			for _, ev := range events {
				flushedEventIDs = append(flushedEventIDs, ev.EventID)
			}
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 15 events (5 more than max)
	eventIDs := make([]string, 15)
	for i := 0; i < 15; i++ {
		id := NewUUIDv7()
		eventIDs[i] = id
		ev := &Event{EventID: id}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Check dropped count
	if buf.DroppedCount() != 5 {
		t.Errorf("Expected 5 dropped events, got %d", buf.DroppedCount())
	}

	// Buffer should contain only the last 10 events
	if buf.Size() != 10 {
		t.Errorf("Expected buffer size 10, got %d", buf.Size())
	}

	// Flush and verify we got the last 10 events (not the first 5)
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(flushedEventIDs) != 10 {
		t.Fatalf("Expected 10 flushed events, got %d", len(flushedEventIDs))
	}

	// Verify the flushed events are the last 10 (indices 5-14)
	for i, id := range flushedEventIDs {
		expectedID := eventIDs[i+5]
		if id != expectedID {
			t.Errorf("Event %d: expected ID %s, got %s", i, expectedID, id)
		}
	}
}

// TestEventBuffer_EventOrderPreserved tests that event order is preserved within batches.
// Requirement 33.11
func TestEventBuffer_EventOrderPreserved(t *testing.T) {
	var mu sync.Mutex
	var flushedEventIDs []string

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     5,
		FlushInterval: time.Hour,
		MaxBufferSize: 100,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			mu.Lock()
			defer mu.Unlock()
			for _, ev := range events {
				flushedEventIDs = append(flushedEventIDs, ev.EventID)
			}
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 12 events (will trigger 2 flushes of 5, leaving 2 buffered)
	eventIDs := make([]string, 12)
	for i := 0; i < 12; i++ {
		id := NewUUIDv7()
		eventIDs[i] = id
		ev := &Event{EventID: id}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		// Small delay to ensure IDs are generated in order
		time.Sleep(time.Millisecond)
	}

	// Flush remaining events
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify all 12 events were flushed in order
	if len(flushedEventIDs) != 12 {
		t.Fatalf("Expected 12 flushed events, got %d", len(flushedEventIDs))
	}

	for i, id := range flushedEventIDs {
		if id != eventIDs[i] {
			t.Errorf("Event %d: expected ID %s, got %s (order not preserved)", i, eventIDs[i], id)
		}
	}
}

// TestEventBuffer_ConcurrentAdds tests that buffer handles concurrent adds correctly.
func TestEventBuffer_ConcurrentAdds(t *testing.T) {
	var flushCount atomic.Int32
	var flushedEvents atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     50,
		FlushInterval: 100 * time.Millisecond,
		MaxBufferSize: 1000,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			flushedEvents.Add(int32(len(events)))
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()
	const numGoroutines = 10
	const eventsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Add events concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				ev := &Event{EventID: NewUUIDv7()}
				_ = buf.Add(ctx, ev, []byte("test"))
			}
		}()
	}

	wg.Wait()

	// Flush remaining events
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// Wait a bit for any pending flushes
	time.Sleep(50 * time.Millisecond)

	totalEvents := numGoroutines * eventsPerGoroutine
	if int(flushedEvents.Load()) != totalEvents {
		t.Errorf("Expected %d total flushed events, got %d", totalEvents, flushedEvents.Load())
	}
}

// TestEventBuffer_EmptyFlush tests that flushing an empty buffer is a no-op.
func TestEventBuffer_EmptyFlush(t *testing.T) {
	var flushCount atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     10,
		FlushInterval: time.Hour,
		MaxBufferSize: 100,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Flush empty buffer
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if flushCount.Load() != 0 {
		t.Errorf("Expected no flush for empty buffer, got %d flushes", flushCount.Load())
	}
}

// TestEventBuffer_Size tests the Size() method.
// Requirement 33.10
func TestEventBuffer_Size(t *testing.T) {
	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     100,
		FlushInterval: time.Hour,
		MaxBufferSize: 1000,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	if buf.Size() != 0 {
		t.Errorf("Expected initial size 0, got %d", buf.Size())
	}

	// Add 5 events
	for i := 0; i < 5; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	if buf.Size() != 5 {
		t.Errorf("Expected size 5, got %d", buf.Size())
	}

	// Flush
	err := buf.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if buf.Size() != 0 {
		t.Errorf("Expected size 0 after flush, got %d", buf.Size())
	}
}

// TestEventBuffer_MultipleFlushes tests multiple flush cycles.
func TestEventBuffer_MultipleFlushes(t *testing.T) {
	var flushCount atomic.Int32
	var totalFlushed atomic.Int32

	buf := NewEventBuffer(EventBufferConfig{
		BatchSize:     5,
		FlushInterval: time.Hour,
		MaxBufferSize: 100,
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			flushCount.Add(1)
			totalFlushed.Add(int32(len(events)))
			return nil
		},
	})
	defer buf.Close(context.Background())

	ctx := context.Background()

	// Add 15 events (should trigger 3 flushes)
	for i := 0; i < 15; i++ {
		ev := &Event{EventID: NewUUIDv7()}
		err := buf.Add(ctx, ev, []byte("test"))
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Give time for flushes to complete
	time.Sleep(50 * time.Millisecond)

	if flushCount.Load() != 3 {
		t.Errorf("Expected 3 flushes, got %d", flushCount.Load())
	}

	if totalFlushed.Load() != 15 {
		t.Errorf("Expected 15 total flushed events, got %d", totalFlushed.Load())
	}
}

// TestEventBuffer_DefaultConfig tests that default configuration values are applied.
func TestEventBuffer_DefaultConfig(t *testing.T) {
	buf := NewEventBuffer(EventBufferConfig{
		FlushFunc: func(ctx context.Context, events []*Event, encoded [][]byte) error {
			return nil
		},
	})
	defer buf.Close(context.Background())

	if buf.batchSize != 100 {
		t.Errorf("Expected default batch size 100, got %d", buf.batchSize)
	}

	if buf.flushInterval != 5*time.Second {
		t.Errorf("Expected default flush interval 5s, got %s", buf.flushInterval)
	}

	if buf.maxBufferSize != 10000 {
		t.Errorf("Expected default max buffer size 10000, got %d", buf.maxBufferSize)
	}
}
