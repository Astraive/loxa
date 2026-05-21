package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
	processing "github.com/astraive/loxa-collector/internal/processing"
)

type testSink struct {
	mu     sync.Mutex
	writes int
	err    error
}

func (s *testSink) Name() string { return "test" }

func (s *testSink) WriteEvent(context.Context, []byte, *collectorevent.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes++
	return s.err
}

func (s *testSink) Flush(context.Context) error { return nil }
func (s *testSink) Close(context.Context) error { return nil }

func TestHandleRecordCommitsWhenProcessingFails(t *testing.T) {
	sink := &testSink{err: errors.New("primary failed")}
	processor, err := processing.New(processing.Config{
		DeliveryPolicy:      "require_primary",
		RetryEnabled:        true,
		RetryMaxAttempts:    1,
		RetryInitialBackoff: time.Millisecond,
		RetryMaxBackoff:     time.Millisecond,
		ValidateJSONObjects: true,
	}, sink, nil, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = processor.Close() }()

	state := &workerState{processor: processor}
	committed := 0
	state.handleRecord(context.Background(), queueRecord{
		value: []byte(`{"event":"a"}`),
		commit: func(context.Context) error {
			committed++
			return nil
		},
	})
	if committed != 1 {
		t.Fatalf("expected commit on failure, got %d", committed)
	}
}

func TestHandleRecordCommitsInvalidEvent(t *testing.T) {
	processor, err := processing.New(processing.Config{
		ValidateJSONObjects: true,
	}, &testSink{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}
	defer func() { _ = processor.Close() }()

	state := &workerState{processor: processor}
	committed := 0
	state.handleRecord(context.Background(), queueRecord{
		value: []byte(`123`),
		commit: func(context.Context) error {
			committed++
			return nil
		},
	})
	if committed != 1 {
		t.Fatalf("expected commit for invalid record, got %d", committed)
	}
}

func TestLoadWorkerConfigFromArgsQueueOverrides(t *testing.T) {
	t.Setenv("COLLECTOR_KAFKA_BROKERS", "k1:9092,k2:9092")
	t.Setenv("COLLECTOR_KAFKA_TOPIC", "topic-a")
	t.Setenv("LOXA_WORKER_CONSUMER_GROUP", "group-a")
	t.Setenv("LOXA_WORKER_POLL_TIMEOUT", "4s")

	cfg, err := loadWorkerConfigFromArgs(nil)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.kafkaBrokers) != 2 || cfg.kafkaBrokers[0] != "k1:9092" {
		t.Fatalf("unexpected brokers: %#v", cfg.kafkaBrokers)
	}
	if cfg.kafkaTopic != "topic-a" {
		t.Fatalf("unexpected topic: %q", cfg.kafkaTopic)
	}
	if cfg.workerConsumerGroup != "group-a" {
		t.Fatalf("unexpected consumer group: %q", cfg.workerConsumerGroup)
	}
	if cfg.workerPollTimeout != 4*time.Second {
		t.Fatalf("unexpected poll timeout: %s", cfg.workerPollTimeout)
	}
}
