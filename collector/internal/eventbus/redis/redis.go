// Package redis implements the Redis Streams event bus adapter.
// Good for simple self-hosted deployments with Docker Compose.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
	"github.com/redis/go-redis/v9"
)

func init() {
	eventbus.Register("redis", New)
}

type redisBus struct {
	client  *redis.Client
	cfg     eventbus.RedisConfig
	dlqCfg  string
	closed  atomic.Bool
	mu      sync.Mutex
	subs    []context.CancelFunc
}

// New creates a new Redis Streams event bus.
func New(_ context.Context, cfg eventbus.Config) (eventbus.Bus, error) {
	rc := cfg.Redis
	if rc.Addr == "" {
		rc.Addr = "127.0.0.1:6379"
	}
	if rc.Stream == "" {
		rc.Stream = cfg.Topic
	}
	if rc.Stream == "" {
		rc.Stream = "loxa.events.raw"
	}
	if rc.Group == "" {
		rc.Group = cfg.ConsumerGroup
	}
	if rc.Group == "" {
		rc.Group = "loxa-worker"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     rc.Addr,
		Password: rc.Password,
		DB:       rc.DB,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("eventbus/redis: ping: %w", err)
	}

	// Ensure consumer group exists
	err := client.XGroupCreateMkStream(ctx, rc.Stream, rc.Group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		client.Close()
		return nil, fmt.Errorf("eventbus/redis: create group: %w", err)
	}

	return &redisBus{
		client: client,
		cfg:    rc,
		dlqCfg: cfg.DLQTopic,
	}, nil
}

func (b *redisBus) Publish(ctx context.Context, topic string, events []eventbus.Envelope) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	stream := topic
	if stream == "" {
		stream = b.cfg.Stream
	}

	maxLen := b.cfg.MaxLen
	if maxLen <= 0 {
		maxLen = 1000000
	}

	for _, env := range events {
		data, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("eventbus/redis: marshal: %w", err)
		}
		args := &redis.XAddArgs{
			Stream: stream,
			MaxLen: maxLen,
			Approx: true,
			Values: map[string]any{"body": data},
		}
		if err := b.client.XAdd(ctx, args).Err(); err != nil {
			return fmt.Errorf("eventbus/redis: xadd: %w", err)
		}
	}
	return nil
}

func (b *redisBus) Subscribe(ctx context.Context, topic string, group string, handler eventbus.Handler) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	stream := topic
	if stream == "" {
		stream = b.cfg.Stream
	}
	consumerGroup := group
	if consumerGroup == "" {
		consumerGroup = b.cfg.Group
	}

	consumerName := fmt.Sprintf("consumer-%d", time.Now().UnixNano())

	// Create consumer group if it doesn't exist (MkStream creates the stream too)
	if err := b.client.XGroupCreateMkStream(ctx, stream, consumerGroup, "0").Err(); err != nil {
		// BUSYGROUP means the group already exists, which is fine
		if !isBusyGroupErr(err) {
			return fmt.Errorf("eventbus/redis: create group: %w", err)
		}
	}

	subCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.subs = append(b.subs, cancel)
	b.mu.Unlock()

	go b.consume(subCtx, stream, consumerGroup, consumerName, handler)
	return nil
}

func isBusyGroupErr(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists" ||
		(len(err.Error()) > 20 && err.Error()[:20] == "BUSYGROUP Consumer G"))
}

func (b *redisBus) consume(ctx context.Context, stream, group, consumer string, handler eventbus.Handler) {
	for {
		if ctx.Err() != nil {
			return
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if err == redis.Nil {
				continue
			}
			time.Sleep(time.Second)
			continue
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				bodyRaw, ok := msg.Values["body"]
				if !ok {
					_ = b.client.XAck(ctx, stream, group, msg.ID)
					continue
				}
				body, ok := bodyRaw.(string)
				if !ok {
					_ = b.client.XAck(ctx, stream, group, msg.ID)
					continue
				}

				var env eventbus.Envelope
				if err := json.Unmarshal([]byte(body), &env); err != nil {
					_ = b.client.XAck(ctx, stream, group, msg.ID)
					continue
				}

				busMsg := &redisMessage{
					id:       env.ID,
					topic:    stream,
					env:      env,
					client:   b.client,
					stream:   stream,
					group:    group,
					streamID: msg.ID,
				}
				if err := handler(ctx, busMsg); err != nil {
					// Nak: don't ack, message will be redelivered
					continue
				}
				_ = b.client.XAck(ctx, stream, group, msg.ID)
			}
		}
	}
}

func (b *redisBus) Close(_ context.Context) error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	b.mu.Lock()
	for _, cancel := range b.subs {
		cancel()
	}
	b.mu.Unlock()
	return b.client.Close()
}

func (b *redisBus) Health(ctx context.Context) eventbus.Health {
	if b.closed.Load() {
		return eventbus.Health{OK: false, Detail: "closed"}
	}
	if err := b.client.Ping(ctx).Err(); err != nil {
		return eventbus.Health{OK: false, Detail: err.Error()}
	}
	return eventbus.Health{OK: true}
}

func (b *redisBus) PublishDLQ(ctx context.Context, original eventbus.Envelope, reason error) error {
	dlqTopic := b.dlqCfg
	if dlqTopic == "" {
		dlqTopic = "loxa.events.dlq"
	}
	if original.Headers == nil {
		original.Headers = make(map[string]string)
	}
	original.Headers["dlq_reason"] = reason.Error()
	original.Headers["dlq_time"] = time.Now().UTC().Format(time.RFC3339Nano)
	return b.Publish(ctx, dlqTopic, []eventbus.Envelope{original})
}

type redisMessage struct {
	id       string
	topic    string
	env      eventbus.Envelope
	client   *redis.Client
	stream   string
	group    string
	streamID string
}

func (m *redisMessage) ID() string                   { return m.id }
func (m *redisMessage) Topic() string                 { return m.topic }
func (m *redisMessage) Envelope() eventbus.Envelope   { return m.env }
func (m *redisMessage) Ack(ctx context.Context) error { return m.client.XAck(ctx, m.stream, m.group, m.streamID).Err() }
func (m *redisMessage) Nack(_ context.Context, _ error) error { return nil } // Redis Streams redeliver on no-ack
