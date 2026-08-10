// Package memory implements an in-process event bus using buffered channels.
// Suitable for local dev, testing, and single-process deployments.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/astraive/loza/collector/internal/eventbus"
)

func init() {
	eventbus.Register("memory", New)
}

// Bus implements eventbus.Bus using in-process buffered channels.
type Bus struct {
	mu      sync.RWMutex
	topics  map[string]*topic
	dlq     []eventbus.Envelope
	dlqMu   sync.Mutex
	closed  bool
	cfg     eventbus.Config
}

type topic struct {
	name    string
	ch       chan eventbus.Envelope
	mu       sync.RWMutex
	handlers []handlerEntry
	started  bool
}

type handlerEntry struct {
	group   string
	handler eventbus.Handler
}

// Message implements eventbus.Message for the memory adapter.
type Message struct {
	id       string
	topic    string
	envelope eventbus.Envelope
}

func (m *Message) ID() string                   { return m.id }
func (m *Message) Topic() string                { return m.topic }
func (m *Message) Envelope() eventbus.Envelope  { return m.envelope }

func (m *Message) Ack(_ context.Context) error  { return nil }
func (m *Message) Nack(_ context.Context, _ error) error { return nil }

// New creates a new memory bus.
func New(_ context.Context, cfg eventbus.Config) (eventbus.Bus, error) {
	return &Bus{
		topics: make(map[string]*topic),
		cfg:    cfg,
	}, nil
}

func (b *Bus) bufferSize() int {
	if b.cfg.Memory.BufferSize > 0 {
		return b.cfg.Memory.BufferSize
	}
	return 10000
}

func (b *Bus) getOrCreateTopic(name string) *topic {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.topics[name]
	if !ok {
		t = &topic{
			name: name,
			ch:   make(chan eventbus.Envelope, b.bufferSize()),
		}
		b.topics[name] = t
	}
	return t
}

// Publish sends envelopes to the in-process channel for the given topic.
func (b *Bus) Publish(ctx context.Context, topicName string, events []eventbus.Envelope) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return eventbus.ErrBusClosed
	}
	b.mu.RUnlock()

	t := b.getOrCreateTopic(topicName)

	for _, env := range events {
		select {
		case t.ch <- env:
		case <-ctx.Done():
			return ctx.Err()
		default:
			return fmt.Errorf("memory bus: topic %q buffer full", topicName)
		}
	}

	return nil
}

// Subscribe registers a handler for the given topic and consumer group.
func (b *Bus) Subscribe(_ context.Context, topicName string, group string, handler eventbus.Handler) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return eventbus.ErrBusClosed
	}
	b.mu.RUnlock()

	t := b.getOrCreateTopic(topicName)

	t.mu.Lock()
	// Check for duplicate group subscription
	for _, h := range t.handlers {
		if h.group == group {
			t.mu.Unlock()
			return fmt.Errorf("memory bus: group %q already subscribed to topic %q", group, topicName)
		}
	}
	t.handlers = append(t.handlers, handlerEntry{group: group, handler: handler})

	// Start consumer goroutine if not already started
	if !t.started {
		t.started = true
		go b.consumeLoop(t)
	}
	t.mu.Unlock()

	return nil
}

func (b *Bus) consumeLoop(t *topic) {
	for env := range t.ch {
		t.mu.RLock()
		handlers := make([]handlerEntry, len(t.handlers))
		copy(handlers, t.handlers)
		t.mu.RUnlock()

		for _, h := range handlers {
			msg := &Message{
				id:       env.ID,
				topic:    t.name,
				envelope: env,
			}
			if err := h.handler(context.Background(), msg); err != nil {
				b.dlqMu.Lock()
				env.Attempts++
				if env.Headers == nil {
					env.Headers = make(map[string]string)
				}
				env.Headers["dlq_reason"] = err.Error()
				b.dlq = append(b.dlq, env)
				b.dlqMu.Unlock()
			}
		}
	}
}

// Close shuts down the memory bus.
func (b *Bus) Close(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, t := range b.topics {
		close(t.ch)
	}
	return nil
}

// Health returns the health status.
func (b *Bus) Health(_ context.Context) eventbus.Health {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return eventbus.Health{OK: false, Detail: "bus closed"}
	}
	return eventbus.Health{OK: true, Detail: "memory"}
}

// PublishDLQ sends an envelope to the in-process DLQ.
func (b *Bus) PublishDLQ(_ context.Context, original eventbus.Envelope, reason error) error {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	original.Attempts++
	if original.Headers == nil {
		original.Headers = make(map[string]string)
	}
	original.Headers["dlq_reason"] = reason.Error()
	b.dlq = append(b.dlq, original)
	return nil
}

// DLQMessages returns the current DLQ messages (for testing/debugging).
func (b *Bus) DLQMessages() []eventbus.Envelope {
	b.dlqMu.Lock()
	defer b.dlqMu.Unlock()
	out := make([]eventbus.Envelope, len(b.dlq))
	copy(out, b.dlq)
	return out
}

var _ eventbus.Bus = (*Bus)(nil)
var _ eventbus.DeadLetterPublisher = (*Bus)(nil)

// MarshalEnvelope serializes an Envelope to JSON bytes.
func MarshalEnvelope(env eventbus.Envelope) ([]byte, error) {
	return json.Marshal(env)
}

// UnmarshalEnvelope deserializes an Envelope from JSON bytes.
func UnmarshalEnvelope(data []byte) (eventbus.Envelope, error) {
	var env eventbus.Envelope
	err := json.Unmarshal(data, &env)
	return env, err
}
