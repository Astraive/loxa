package kafka

import (
	"testing"
	"time"
)

func TestNewRejectsInvalidAcks(t *testing.T) {
	_, err := New(Config{
		Brokers: []string{"127.0.0.1:9092"},
		Topic:   "events",
		Acks:    "invalid",
	})
	if err == nil {
		t.Fatal("expected invalid acks error")
	}
}

func TestNewAcceptsProducerOptions(t *testing.T) {
	sink, err := New(Config{
		Brokers:           []string{"127.0.0.1:9092"},
		Topic:             "events",
		Acks:              "all",
		RequestTimeout:    2 * time.Second,
		EnableIdempotence: true,
		MaxRetries:        3,
		RetryBackoff:      10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new kafka sink: %v", err)
	}
	if err := sink.Close(nil); err != nil {
		t.Fatalf("close kafka sink: %v", err)
	}
}
