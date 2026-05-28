package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	collectorevent "github.com/astraive/loxa/collector/internal/event"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Config controls Kafka sink behavior.
type Config struct {
	Brokers []string
	Topic   string
	Acks    string
	// RequestTimeout bounds producer requests.
	RequestTimeout time.Duration
	// EnableIdempotence controls idempotent producer semantics.
	EnableIdempotence bool
	// MaxRetries bounds producer retry attempts.
	MaxRetries int
	// RetryBackoff controls retry delay between transient failures.
	RetryBackoff time.Duration
	// TLSConfig enables TLS for Kafka broker connections when provided.
	TLSConfig *tls.Config
}

type sink struct {
	client *kgo.Client
	topic  string
}

// New creates a Kafka sink.
func New(cfg Config) (collectorevent.Sink, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: Brokers is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka: Topic is required")
	}

	acks := strings.ToLower(strings.TrimSpace(cfg.Acks))
	var requiredAcks kgo.Acks
	switch acks {
	case "", "all":
		requiredAcks = kgo.AllISRAcks()
	case "1":
		requiredAcks = kgo.LeaderAck()
	case "0":
		requiredAcks = kgo.NoAck()
	default:
		return nil, fmt.Errorf("kafka: invalid acks %q (expected 0, 1, all)", cfg.Acks)
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(requiredAcks),
	}
	if cfg.TLSConfig != nil {
		opts = append(opts, kgo.DialTLSConfig(cfg.TLSConfig.Clone()))
	}
	if cfg.RequestTimeout > 0 {
		opts = append(opts, kgo.ProduceRequestTimeout(cfg.RequestTimeout))
	}
	if cfg.MaxRetries > 0 {
		opts = append(opts, kgo.RecordRetries(cfg.MaxRetries), kgo.RequestRetries(cfg.MaxRetries))
	}
	if cfg.RetryBackoff > 0 {
		backoff := cfg.RetryBackoff
		opts = append(opts, kgo.RetryBackoffFn(func(int) time.Duration {
			return backoff
		}))
	}
	if !cfg.EnableIdempotence {
		opts = append(opts, kgo.DisableIdempotentWrite())
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return &sink{client: client, topic: cfg.Topic}, nil
}

func (s *sink) Name() string { return "kafka" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, _ *collectorevent.Event) error {
	rec := &kgo.Record{Topic: s.topic, Value: encoded}
	res := s.client.ProduceSync(ctx, rec)
	return res.FirstErr()
}

func (s *sink) Flush(ctx context.Context) error {
	return s.client.Flush(ctx)
}

func (s *sink) Close(_ context.Context) error {
	s.client.Close()
	return nil
}
