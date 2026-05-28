// Package nats implements the NATS JetStream event bus adapter.
// This is the recommended production default for Loxa.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astraive/loxa/collector/internal/eventbus"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func init() {
	eventbus.Register("nats", New)
}

type natsBus struct {
	conn    *nats.Conn
	js      jetstream.JetStream
	cfg     eventbus.NATSConfig
	dlqCfg  string
	closed  atomic.Bool
	mu      sync.Mutex
	subs    []*jetstream.Consumer
	cancel  context.CancelFunc
}

// New creates a new NATS JetStream event bus.
func New(ctx context.Context, cfg eventbus.Config) (eventbus.Bus, error) {
	nc := cfg.NATS
	if nc.URL == "" {
		nc.URL = nats.DefaultURL
	}
	if nc.Stream == "" {
		nc.Stream = "LOXA"
	}
	if nc.Subject == "" {
		nc.Subject = cfg.Topic
	}
	if nc.Subject == "" {
		nc.Subject = "loxa.events.raw"
	}
	if nc.Durable == "" {
		nc.Durable = cfg.ConsumerGroup
	}
	if nc.Durable == "" {
		nc.Durable = "loxa-worker"
	}

	opts := []nats.Option{nats.Name("loxa-collector")}
	if nc.Username != "" && nc.Password != "" {
		opts = append(opts, nats.UserInfo(nc.Username, nc.Password))
	}

	conn, err := nats.Connect(nc.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("eventbus/nats: connect: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("eventbus/nats: jetstream: %w", err)
	}

	// Ensure stream exists
	subjects := []string{nc.Subject}
	if cfg.DLQTopic != "" {
		subjects = append(subjects, cfg.DLQTopic)
	}
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      nc.Stream,
		Subjects:  subjects,
		Retention: jetstream.WorkQueuePolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    7 * 24 * time.Hour, // 7 days retention
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("eventbus/nats: create stream: %w", err)
	}

	busCtx, cancel := context.WithCancel(context.Background())
	_ = busCtx // used for consumer lifecycle

	return &natsBus{
		conn:   conn,
		js:     js,
		cfg:    nc,
		dlqCfg: cfg.DLQTopic,
		cancel: cancel,
	}, nil
}

func (b *natsBus) Publish(ctx context.Context, topic string, events []eventbus.Envelope) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	subject := topic
	if subject == "" {
		subject = b.cfg.Subject
	}

	for _, env := range events {
		data, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("eventbus/nats: marshal envelope: %w", err)
		}
		_, err = b.js.Publish(ctx, subject, data)
		if err != nil {
			return fmt.Errorf("eventbus/nats: publish: %w", err)
		}
	}
	return nil
}

func (b *natsBus) Subscribe(ctx context.Context, topic string, group string, handler eventbus.Handler) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	subject := topic
	if subject == "" {
		subject = b.cfg.Subject
	}
	durable := group
	if durable == "" {
		durable = b.cfg.Durable
	}

	consumer, err := b.js.CreateOrUpdateConsumer(ctx, b.cfg.Stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
		AckWait:       30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("eventbus/nats: create consumer: %w", err)
	}

	b.mu.Lock()
	b.subs = append(b.subs, &consumer)
	b.mu.Unlock()

	iter, err := consumer.Messages(jetstream.PullMaxMessages(10))
	if err != nil {
		return fmt.Errorf("eventbus/nats: messages: %w", err)
	}

	go func() {
		for {
			msg, err := iter.Next()
			if err != nil {
				if b.closed.Load() || ctx.Err() != nil {
					return
				}
				continue
			}

			var env eventbus.Envelope
			if err := json.Unmarshal(msg.Data(), &env); err != nil {
				_ = msg.Nak()
				continue
			}

			busMsg := &natsMessage{
				id:   env.ID,
				topic: subject,
				env:  env,
				msg:  msg,
			}
			if err := handler(ctx, busMsg); err != nil {
				_ = msg.Nak()
			} else {
				_ = msg.Ack()
			}
		}
	}()

	return nil
}

func (b *natsBus) Close(_ context.Context) error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	b.cancel()
	if b.conn != nil {
		b.conn.Close()
	}
	return nil
}

func (b *natsBus) Health(_ context.Context) eventbus.Health {
	if b.closed.Load() {
		return eventbus.Health{OK: false, Detail: "closed"}
	}
	if !b.conn.IsConnected() {
		return eventbus.Health{OK: false, Detail: "disconnected"}
	}
	return eventbus.Health{OK: true}
}

func (b *natsBus) PublishDLQ(ctx context.Context, original eventbus.Envelope, reason error) error {
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

type natsMessage struct {
	id    string
	topic string
	env   eventbus.Envelope
	msg   jetstream.Msg
}

func (m *natsMessage) ID() string                   { return m.id }
func (m *natsMessage) Topic() string                 { return m.topic }
func (m *natsMessage) Envelope() eventbus.Envelope   { return m.env }
func (m *natsMessage) Ack(_ context.Context) error   { return m.msg.Ack() }
func (m *natsMessage) Nack(_ context.Context, _ error) error { return m.msg.Nak() }
