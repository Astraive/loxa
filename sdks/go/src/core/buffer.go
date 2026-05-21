package core

import (
	"context"
	"sync"
	"time"
)

// EventBuffer is an in-memory buffer for events with automatic flushing.
// It implements batching and buffering per Requirement 33.
//
// Requirements: 33.1, 33.2, 33.3, 33.4, 33.5, 33.6, 33.7, 33.8, 33.9, 33.10, 33.11
type EventBuffer struct {
	mu            sync.Mutex
	events        []*Event
	encoded       [][]byte
	batchSize     int
	maxBufferSize int
	flushInterval time.Duration
	flushFunc     func(ctx context.Context, events []*Event, encoded [][]byte) error
	ticker        *time.Ticker
	stopCh        chan struct{}
	closeOnce     sync.Once
	droppedCount  int64
}

// EventBufferConfig configures the event buffer.
type EventBufferConfig struct {
	// BatchSize is the number of events that trigger an automatic flush.
	// Default: 100. Requirement 33.2
	BatchSize int

	// FlushInterval is the maximum time between flushes.
	// Default: 5 seconds. Requirement 33.3
	FlushInterval time.Duration

	// MaxBufferSize is the maximum number of events to buffer.
	// When exceeded, oldest events are dropped. Requirement 33.8
	MaxBufferSize int

	// FlushFunc is called to flush buffered events.
	FlushFunc func(ctx context.Context, events []*Event, encoded [][]byte) error
}

// NewEventBuffer creates a new event buffer with the given configuration.
// Requirement 33.1
func NewEventBuffer(cfg EventBufferConfig) *EventBuffer {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxBufferSize <= 0 {
		cfg.MaxBufferSize = 10000
	}
	if cfg.FlushFunc == nil {
		cfg.FlushFunc = func(ctx context.Context, events []*Event, encoded [][]byte) error {
			return nil
		}
	}

	buf := &EventBuffer{
		events:        make([]*Event, 0, cfg.BatchSize),
		encoded:       make([][]byte, 0, cfg.BatchSize),
		batchSize:     cfg.BatchSize,
		maxBufferSize: cfg.MaxBufferSize,
		flushInterval: cfg.FlushInterval,
		flushFunc:     cfg.FlushFunc,
		ticker:        time.NewTicker(cfg.FlushInterval),
		stopCh:        make(chan struct{}),
	}

	// Start background flush goroutine
	// Requirement 33.5: flush on interval elapsed
	go buf.flushLoop()

	return buf
}

// Add adds an event to the buffer.
// If the buffer reaches batch_size, it flushes automatically.
// If the buffer exceeds max_buffer_size, oldest events are dropped.
// Requirements: 33.4, 33.8, 33.9
func (b *EventBuffer) Add(ctx context.Context, ev *Event, encoded []byte) error {
	b.mu.Lock()
	
	// Check if buffer is at max capacity
	// Requirement 33.9: drop oldest events when buffer exceeds max_buffer_size
	if len(b.events) >= b.maxBufferSize {
		// Drop oldest event (first in slice)
		b.events = b.events[1:]
		b.encoded = b.encoded[1:]
		b.droppedCount++
	}

	// Add event to buffer
	// Requirement 33.11: preserve event order within batches
	b.events = append(b.events, ev)
	b.encoded = append(b.encoded, encoded)

	// Check if we should flush
	// Requirement 33.4: flush when batch_size is reached
	shouldFlush := len(b.events) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		return b.Flush(ctx)
	}

	return nil
}

// Flush immediately flushes all buffered events.
// Requirement 33.6
func (b *EventBuffer) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.events) == 0 {
		b.mu.Unlock()
		return nil
	}

	// Take ownership of current batch
	// Requirement 33.11: preserve event order
	events := b.events
	encoded := b.encoded

	// Reset buffer for new events
	b.events = make([]*Event, 0, b.batchSize)
	b.encoded = make([][]byte, 0, b.batchSize)
	b.mu.Unlock()

	// Flush outside the lock to avoid blocking new events
	return b.flushFunc(ctx, events, encoded)
}

// flushLoop runs in a background goroutine and flushes on interval.
// Requirement 33.5: flush when flush_interval elapses
func (b *EventBuffer) flushLoop() {
	for {
		select {
		case <-b.ticker.C:
			// Flush on interval
			_ = b.Flush(context.Background())
		case <-b.stopCh:
			return
		}
	}
}

// Close stops the buffer and flushes remaining events.
// Requirement 33.7: flush on Shutdown()
func (b *EventBuffer) Close(ctx context.Context) error {
	var err error
	b.closeOnce.Do(func() {
		close(b.stopCh)
		b.ticker.Stop()
		err = b.Flush(ctx)
	})
	return err
}

// Size returns the current number of buffered events.
// Requirement 33.10: expose buffer_size gauge
func (b *EventBuffer) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

// DroppedCount returns the total number of events dropped due to buffer overflow.
// Requirement 33.9: increment events_dropped_total
func (b *EventBuffer) DroppedCount() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.droppedCount
}
