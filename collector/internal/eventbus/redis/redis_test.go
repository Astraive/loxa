package redis

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
)

func TestRedisRequiresConnection(t *testing.T) {
	_, err := New(context.Background(), eventbus.Config{
		Type: "redis",
		Redis: eventbus.RedisConfig{
			Addr: "127.0.0.1:6379",
		},
	})
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
}

func TestRedisBusContract(t *testing.T) {
	bus, err := New(context.Background(), eventbus.Config{
		Type:         "redis",
		Topic:        "loxa.test.events",
		ConsumerGroup: "loxa-test-group",
		Redis: eventbus.RedisConfig{
			Addr:   "127.0.0.1:6379",
			Stream: "loxa.test.events",
			Group:  "loxa-test-group",
		},
	})
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer bus.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	handler := func(_ context.Context, msg eventbus.Message) error {
		close(done)
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, "loxa.test.events", "loxa-test-group", handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	envelopes := []eventbus.Envelope{
		{ID: "redis-1", Event: "test.event", Timestamp: time.Now(), Body: []byte(`{"x":1}`)},
	}
	if err := bus.Publish(ctx, "loxa.test.events", envelopes); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}
