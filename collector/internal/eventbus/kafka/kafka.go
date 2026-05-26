// Package kafka implements the Kafka event bus adapter.
// This is the enterprise-grade adapter for high-throughput, long-retention deployments.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

func init() {
	eventbus.Register("kafka", New)
}

type kafkaBus struct {
	client *kgo.Client
	cfg    eventbus.KafkaConfig
	dlqCfg string
	closed atomic.Bool
	mu     sync.Mutex
	subs   []context.CancelFunc
}

// New creates a new Kafka event bus.
// Auto-creates topics via the admin API (idempotent).
func New(_ context.Context, cfg eventbus.Config) (eventbus.Bus, error) {
	kc := cfg.Kafka
	if len(kc.Brokers) == 0 {
		return nil, fmt.Errorf("eventbus/kafka: brokers must not be empty")
	}
	if kc.Topic == "" {
		kc.Topic = cfg.Topic
	}
	if kc.Topic == "" {
		kc.Topic = "loxa.events.raw"
	}
	if kc.ConsumerGroup == "" {
		kc.ConsumerGroup = cfg.ConsumerGroup
	}
	if kc.ConsumerGroup == "" {
		kc.ConsumerGroup = "loxa-worker"
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(kc.Brokers...),
		kgo.DefaultProduceTopic(kc.Topic),
		kgo.RecordPartitioner(kgo.RoundRobinPartitioner()),
	}

	if !kc.EnableIdempotence {
		opts = append(opts, kgo.DisableIdempotentWrite())
	}

	switch kc.Compression {
	case "snappy":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.SnappyCompression()))
	case "lz4":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.Lz4Compression()))
	case "zstd":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.ZstdCompression()))
	case "gzip":
		opts = append(opts, kgo.ProducerBatchCompression(kgo.GzipCompression()))
	default:
		opts = append(opts, kgo.ProducerBatchCompression(kgo.NoCompression()))
	}

	switch kc.Acks {
	case "0":
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()), kgo.DisableIdempotentWrite())
	case "1":
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()))
	default:
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("eventbus/kafka: client: %w", err)
	}

	// Verify connectivity and create topics
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adm := kadm.NewClient(client)

	topicNames := []string{kc.Topic}
	if cfg.DLQTopic != "" {
		topicNames = append(topicNames, cfg.DLQTopic)
	}
	resps, createErr := adm.CreateTopics(ctx, 1, 1, nil, topicNames...)
	if createErr != nil {
		client.Close()
		return nil, fmt.Errorf("eventbus/kafka: create topics: %w", createErr)
	}
	for _, resp := range resps.Sorted() {
		if resp.Err != nil {
			// kadm.CreateTopics ignores topics that already exist at the Go level,
			// but returns per-topic errors. Ignore TOPIC_ALREADY_EXISTS (code 36).
			errStr := resp.Err.Error()
			if !strings.Contains(errStr, "TOPIC_ALREADY_EXISTS") {
				client.Close()
				return nil, fmt.Errorf("eventbus/kafka: create topic %q: %w", resp.Topic, resp.Err)
			}
		}
	}

	return &kafkaBus{
		client: client,
		cfg:    kc,
		dlqCfg: cfg.DLQTopic,
	}, nil
}

func (b *kafkaBus) Publish(ctx context.Context, topic string, events []eventbus.Envelope) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	produceTopic := topic
	if produceTopic == "" {
		produceTopic = b.cfg.Topic
	}

	var wg sync.WaitGroup
	var firstErr atomic.Value

	for _, env := range events {
		data, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("eventbus/kafka: marshal: %w", err)
		}
		wg.Add(1)
		record := &kgo.Record{
			Topic: produceTopic,
			Key:   []byte(env.ID),
			Value: data,
		}
		b.client.Produce(ctx, record, func(r *kgo.Record, err error) {
			defer wg.Done()
			if err != nil {
				firstErr.CompareAndSwap(nil, err)
			}
		})
	}

	wg.Wait()
	if v := firstErr.Load(); v != nil {
		return fmt.Errorf("eventbus/kafka: produce: %w", v.(error))
	}
	return nil
}

func (b *kafkaBus) Subscribe(ctx context.Context, topic string, group string, handler eventbus.Handler) error {
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}

	consumeTopic := topic
	if consumeTopic == "" {
		consumeTopic = b.cfg.Topic
	}
	consumerGroup := group
	if consumerGroup == "" {
		consumerGroup = b.cfg.ConsumerGroup
	}

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(b.cfg.Brokers...),
		kgo.ConsumeTopics(consumeTopic),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return fmt.Errorf("eventbus/kafka: consumer: %w", err)
	}

	subCtx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.subs = append(b.subs, cancel)
	b.mu.Unlock()

	go func() {
		defer consumer.Close()
		for {
			if subCtx.Err() != nil {
				return
			}
			fetches := consumer.PollFetches(subCtx)
			if subCtx.Err() != nil {
				return
			}
			fetches.EachRecord(func(rec *kgo.Record) {
				var env eventbus.Envelope
				if err := json.Unmarshal(rec.Value, &env); err != nil {
					return
				}
				msg := &message{
					id:       env.ID,
					topic:    rec.Topic,
					envelope: env,
					consumer: consumer,
					rec:      rec,
				}
				if err := handler(subCtx, msg); err != nil {
					return
				}
				consumer.CommitRecords(subCtx, rec)
			})
		}
	}()
	return nil
}

func (b *kafkaBus) Close(_ context.Context) error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	b.mu.Lock()
	for _, cancel := range b.subs {
		cancel()
	}
	b.mu.Unlock()
	b.client.Close()
	return nil
}

func (b *kafkaBus) Health(_ context.Context) eventbus.Health {
	if b.closed.Load() {
		return eventbus.Health{OK: false, Detail: "closed"}
	}
	return eventbus.Health{OK: true}
}

func (b *kafkaBus) PublishDLQ(ctx context.Context, original eventbus.Envelope, reason error) error {
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

type message struct {
	id       string
	topic    string
	envelope eventbus.Envelope
	consumer *kgo.Client
	rec      *kgo.Record
}

func (m *message) ID() string                   { return m.id }
func (m *message) Topic() string                { return m.topic }
func (m *message) Envelope() eventbus.Envelope  { return m.envelope }
func (m *message) Ack(ctx context.Context) error { return m.consumer.CommitRecords(ctx, m.rec) }
func (m *message) Nack(_ context.Context, _ error) error { return nil }

var _ eventbus.Bus = (*kafkaBus)(nil)
var _ eventbus.DeadLetterPublisher = (*kafkaBus)(nil)
