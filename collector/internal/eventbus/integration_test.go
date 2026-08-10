package eventbus_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/astraive/loza/collector/internal/eventbus"
	_ "github.com/astraive/loza/collector/internal/eventbus/kafka"
	_ "github.com/astraive/loza/collector/internal/eventbus/memory"
	_ "github.com/astraive/loza/collector/internal/eventbus/nats"
	_ "github.com/astraive/loza/collector/internal/eventbus/redis"
)

// redisAddr returns the Redis address from env or default.
func redisAddr() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:6379"
}

// natsAddr returns the NATS address from env or default.
func natsAddr() string {
	if addr := os.Getenv("NATS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:4222"
}

// kafkaBrokers returns the Kafka brokers from env or default.
func kafkaBrokers() []string {
	if addr := os.Getenv("KAFKA_BROKERS"); addr != "" {
		return []string{addr}
	}
	return []string{"127.0.0.1:9092"}
}

// TestMemoryPublishSubscribe tests the memory backend publish-subscribe cycle.
func TestMemoryPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	bus, err := eventbus.New(ctx, eventbus.Config{
		Type:  "memory",
		Topic: "test.memory.pubsub",
	})
	if err != nil {
		t.Fatalf("create bus: %v", err)
	}
	defer bus.Close(ctx)

	received := make(chan eventbus.Envelope, 1)
	handler := func(_ context.Context, msg eventbus.Message) error {
		received <- msg.Envelope()
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, "test.memory.pubsub", "test-group", handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env := eventbus.Envelope{
		ID:        "mem-1",
		Event:     "test.event",
		Timestamp: time.Now(),
		Body:      []byte(`{"test":"memory"}`),
	}
	if err := bus.Publish(ctx, "test.memory.pubsub", []eventbus.Envelope{env}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != "mem-1" {
			t.Errorf("expected ID mem-1, got %s", got.ID)
		}
		if got.Event != "test.event" {
			t.Errorf("expected Event test.event, got %s", got.Event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestRedisPublishSubscribe tests the Redis backend publish-subscribe cycle.
func TestRedisPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
	topic := "test.redis.pubsub." + uniqueID
	group := "test-redis-group." + uniqueID

	bus, err := eventbus.New(ctx, eventbus.Config{
		Type:          "redis",
		Topic:         topic,
		ConsumerGroup: group,
		Redis: eventbus.RedisConfig{
			Addr:  redisAddr(),
			Stream: topic,
			Group:  group,
		},
	})
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	defer bus.Close(ctx)

	received := make(chan eventbus.Envelope, 1)
	handler := func(_ context.Context, msg eventbus.Message) error {
		received <- msg.Envelope()
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, topic, group, handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env := eventbus.Envelope{
		ID:        "redis-1",
		Event:     "test.event",
		Timestamp: time.Now(),
		Body:      []byte(`{"test":"redis"}`),
	}
	if err := bus.Publish(ctx, topic, []eventbus.Envelope{env}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != "redis-1" {
			t.Errorf("expected ID redis-1, got %s", got.ID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestNATSPublishSubscribe tests the NATS backend publish-subscribe cycle.
func TestNATSPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
	topic := "test.nats.pubsub." + uniqueID

	bus, err := eventbus.New(ctx, eventbus.Config{
		Type:  "nats",
		Topic: topic,
		NATS: eventbus.NATSConfig{
			URL: "nats://" + natsAddr(),
		},
	})
	if err != nil {
		t.Skipf("nats not available: %v", err)
	}
	defer bus.Close(ctx)

	received := make(chan eventbus.Envelope, 1)
	handler := func(_ context.Context, msg eventbus.Message) error {
		received <- msg.Envelope()
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, topic, "test-group", handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env := eventbus.Envelope{
		ID:        "nats-1",
		Event:     "test.event",
		Timestamp: time.Now(),
		Body:      []byte(`{"test":"nats"}`),
	}
	if err := bus.Publish(ctx, topic, []eventbus.Envelope{env}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != "nats-1" {
			t.Errorf("expected ID nats-1, got %s", got.ID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestKafkaPublishSubscribe tests the Kafka backend publish-subscribe cycle.
func TestKafkaPublishSubscribe(t *testing.T) {
	ctx := context.Background()
	uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
	topic := "test.kafka.pubsub." + uniqueID
	group := "test-kafka-group." + uniqueID

	bus, err := eventbus.New(ctx, eventbus.Config{
		Type:          "kafka",
		Topic:         topic,
		ConsumerGroup: group,
		Kafka: eventbus.KafkaConfig{
			Brokers:       kafkaBrokers(),
			Topic:         topic,
			ConsumerGroup: group,
			Acks:          "all",
		},
	})
	if err != nil {
		t.Skipf("kafka not available: %v", err)
	}
	defer bus.Close(ctx)

	// Give Kafka a moment to stabilize after topic creation
	time.Sleep(500 * time.Millisecond)

	received := make(chan eventbus.Envelope, 1)
	handler := func(_ context.Context, msg eventbus.Message) error {
		received <- msg.Envelope()
		return msg.Ack(ctx)
	}

	if err := bus.Subscribe(ctx, topic, group, handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	env := eventbus.Envelope{
		ID:        "kafka-1",
		Event:     "test.event",
		Timestamp: time.Now(),
		Body:      []byte(`{"test":"kafka"}`),
	}
	if err := bus.Publish(ctx, topic, []eventbus.Envelope{env}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != "kafka-1" {
			t.Errorf("expected ID kafka-1, got %s", got.ID)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestAllBackendsHealth checks that all backends report healthy.
func TestAllBackendsHealth(t *testing.T) {
	ctx := context.Background()

	backends := []struct {
		name string
		cfg  eventbus.Config
	}{
		{"memory", eventbus.Config{Type: "memory", Topic: "health.mem"}},
		{"redis", eventbus.Config{
			Type:  "redis",
			Topic: "health.redis",
			Redis: eventbus.RedisConfig{Addr: redisAddr()},
		}},
		{"nats", eventbus.Config{
			Type:  "nats",
			Topic: "health.nats",
			NATS:  eventbus.NATSConfig{URL: "nats://" + natsAddr()},
		}},
		{"kafka", eventbus.Config{
			Type:  "kafka",
			Topic: "health.kafka",
			Kafka: eventbus.KafkaConfig{
				Brokers: kafkaBrokers(),
				Topic:   "health.kafka",
			},
		}},
	}

	for _, bc := range backends {
		t.Run(bc.name, func(t *testing.T) {
			bus, err := eventbus.New(ctx, bc.cfg)
			if err != nil {
				t.Skipf("%s not available: %v", bc.name, err)
			}
			defer bus.Close(ctx)

			h := bus.Health(ctx)
			if !h.OK {
				t.Errorf("%s health check failed: %s", bc.name, h.Detail)
			}
		})
	}
}
