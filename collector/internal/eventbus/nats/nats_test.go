package nats

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
)

// Tests require a running NATS server. Use build tag for CI.
// go test -tags=integration ./internal/eventbus/nats/...

func TestNATSRequiresConnection(t *testing.T) {
	_, err := New(context.Background(), eventbus.Config{
		Type: "nats",
		NATS: eventbus.NATSConfig{
			URL: "nats://127.0.0.1:4222",
		},
	})
	// Should fail if no NATS running — that's expected in unit tests
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}
}

func TestNATSBusContract(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{
		Type:     "nats",
		Topic:    "loxa.test.events",
		DLQTopic: "loxa.test.dlq",
		NATS: eventbus.NATSConfig{
			URL:     "nats://127.0.0.1:4222",
			Stream:  "LOXA_TEST",
			Subject: "loxa.test.events",
			Durable: "loxa-test-worker",
		},
	})
	if err != nil {
		t.Skipf("NATS not available: %v", err)
	}
	defer bus.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	handler := func(_ context.Context, msg eventbus.Message) error {
		close(done)
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, "loxa.test.events", "loxa-test-worker", handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	envelopes := []eventbus.Envelope{
		{ID: "nats-1", Event: "test.event", Timestamp: time.Now(), Body: []byte(`{"x":1}`)},
	}
	if err := bus.Publish(ctx, "loxa.test.events", envelopes); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
		// success
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}
