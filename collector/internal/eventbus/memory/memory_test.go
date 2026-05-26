package memory

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
)

func TestBusContract(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{
		Type:   "memory",
		Topic:  "test",
		Memory: eventbus.MemoryConfig{BufferSize: 100},
	})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	defer bus.Close(context.Background())

	var received atomic.Int32
	handler := func(_ context.Context, msg eventbus.Message) error {
		received.Add(1)
		return msg.Ack(context.Background())
	}

	ctx := context.Background()
	if err := bus.Subscribe(ctx, "test-topic", "group-1", handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	envelopes := []eventbus.Envelope{
		{ID: "1", Event: "test.event", Timestamp: time.Now(), Body: []byte(`{"a":1}`)},
		{ID: "2", Event: "test.event", Timestamp: time.Now(), Body: []byte(`{"a":2}`)},
		{ID: "3", Event: "test.event", Timestamp: time.Now(), Body: []byte(`{"a":3}`)},
	}

	if err := bus.Publish(ctx, "test-topic", envelopes); err != nil {
		t.Fatalf("publish: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if got := received.Load(); got != 3 {
		t.Fatalf("expected 3 messages, got %d", got)
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	defer bus.Close(context.Background())

	if err := bus.Publish(context.Background(), "no-topic", []eventbus.Envelope{
		{ID: "1", Event: "test", Timestamp: time.Now(), Body: []byte(`{}`)},
	}); err != nil {
		t.Fatalf("publish to empty topic should not error: %v", err)
	}
}

func TestHealth(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}

	h := bus.Health(context.Background())
	if !h.OK {
		t.Fatal("expected healthy bus")
	}

	bus.Close(context.Background())
	h = bus.Health(context.Background())
	if h.OK {
		t.Fatal("expected unhealthy after close")
	}
}

func TestDLQ(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{
		DLQTopic: "dlq",
	})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	defer bus.Close(context.Background())

	memBus := bus.(*Bus)
	env := eventbus.Envelope{ID: "dlq-1", Event: "failed", Timestamp: time.Now(), Body: []byte(`{}`), Headers: map[string]string{}}
	if err := memBus.PublishDLQ(context.Background(), env, fmt.Errorf("test error")); err != nil {
		t.Fatalf("publish dlq: %v", err)
	}

	msgs := memBus.DLQMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 DLQ message, got %d", len(msgs))
	}
	if msgs[0].ID != "dlq-1" {
		t.Fatalf("expected DLQ message ID dlq-1, got %s", msgs[0].ID)
	}
}

func TestPublishAfterClose(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	bus.Close(context.Background())

	err = bus.Publish(context.Background(), "t", []eventbus.Envelope{{ID: "x"}})
	if err != eventbus.ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got %v", err)
	}
}

func TestSubscribeAfterClose(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{})
	if err != nil {
		t.Fatalf("new bus: %v", err)
	}
	bus.Close(context.Background())

	handler := func(_ context.Context, _ eventbus.Message) error { return nil }
	err = bus.Subscribe(context.Background(), "t", "g", handler)
	if err != eventbus.ErrBusClosed {
		t.Errorf("expected ErrBusClosed, got %v", err)
	}
}
