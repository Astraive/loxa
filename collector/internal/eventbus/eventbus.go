// Package eventbus provides a pluggable event bus abstraction for the Loxa collector.
// It decouples ingest from delivery, allowing queue mode to use memory, Redis, NATS, or Kafka.
package eventbus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
)

// Bus is the core event bus interface. All adapters implement this.
// It is batch-first because Loxa ingestion already supports batches.
type Bus interface {
	// Publish sends a batch of envelopes to the given topic.
	Publish(ctx context.Context, topic string, events []Envelope) error

	// Subscribe starts consuming from the given topic in the specified consumer group.
	// The handler is called for each message. The implementation should handle
	// redelivery/nack semantics internally.
	Subscribe(ctx context.Context, topic string, group string, handler Handler) error

	// Close shuts down the bus, flushing any pending messages.
	Close(ctx context.Context) error

	// Health returns the current health status of the bus.
	Health(ctx context.Context) Health
}

// Handler processes a single message from the bus.
type Handler func(ctx context.Context, msg Message) error

// Message is a consumed message from the bus. It wraps an Envelope
// and provides Ack/Nack for consumer-driven acknowledgment.
type Message interface {
	ID() string
	Topic() string
	Envelope() Envelope
	Ack(ctx context.Context) error
	Nack(ctx context.Context, err error) error
}

// Health represents the health status of an event bus adapter.
type Health struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// DeadLetterPublisher is an optional interface for adapters that support DLQ.
type DeadLetterPublisher interface {
	PublishDLQ(ctx context.Context, original Envelope, reason error) error
}

// SinkAdapter wraps a Bus as an event.Sink for use with the collector's processor.
// This allows the processor to write events through the eventbus.
type SinkAdapter struct {
	bus   Bus
	topic string
}

// NewSinkAdapter creates a new Bus-to-Sink adapter.
func NewSinkAdapter(bus Bus, topic string) *SinkAdapter {
	return &SinkAdapter{bus: bus, topic: topic}
}

func (s *SinkAdapter) Name() string { return "eventbus" }

// WriteEvent publishes a single event through the bus.
func (s *SinkAdapter) WriteEvent(ctx context.Context, encoded []byte, ev *collectorevent.Event) error {
	env := Envelope{
		ID:        generateID(),
		Timestamp: time.Now(),
		Body:      encoded,
		Headers:   make(map[string]string),
	}
	return s.bus.Publish(ctx, s.topic, []Envelope{env})
}

// Flush is a no-op for the eventbus sink (publish is immediate).
func (s *SinkAdapter) Flush(_ context.Context) error { return nil }

// Close delegates to the bus.
func (s *SinkAdapter) Close(ctx context.Context) error { return s.bus.Close(ctx) }

// WriteBatch publishes a batch of events through the bus.
func (s *SinkAdapter) WriteBatch(ctx context.Context, events [][]byte) error {
	envs := make([]Envelope, len(events))
	for i, body := range events {
		envs[i] = Envelope{
			ID:        generateID(),
			Timestamp: time.Now(),
			Body:      body,
			Headers:   make(map[string]string),
		}
	}
	return s.bus.Publish(ctx, s.topic, envs)
}

// MarshalEnvelope serializes an Envelope to JSON bytes.
func MarshalEnvelope(env Envelope) ([]byte, error) {
	return json.Marshal(env)
}

// UnmarshalEnvelope deserializes an Envelope from JSON bytes.
func UnmarshalEnvelope(data []byte) (Envelope, error) {
	var env Envelope
	err := json.Unmarshal(data, &env)
	return env, err
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
