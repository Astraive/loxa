package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loxa/sdks/go"
)

func TestClientCreationAndConfiguration(t *testing.T) {
	sink, store := loxa.MemorySink()
	cfg := loxa.ApplyConfig(
		loxa.Test(),
		loxa.WithService("verification"),
		loxa.WithEnvironment("prod"),
		loxa.WithRelease("1.2.3"),
		loxa.WithNamespace("payments"),
		loxa.WithAPIKey("secret"),
		loxa.WithOtelBridge(true),
		loxa.WithRetry(2),
		loxa.WithTimeout(2*time.Second),
		loxa.WithQueueSize(128),
		loxa.WithSink(sink),
	)

	logger, err := loxa.CreateLoxa(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	if logger.Config().Service != "verification" {
		t.Fatalf("expected service verification, got %q", logger.Config().Service)
	}
	if err := loxa.Configure(cfg); err != nil {
		t.Fatalf("configure default: %v", err)
	}
	aliased, err := loxa.Alias("audit")
	if err != nil {
		t.Fatalf("alias: %v", err)
	}
	if aliased.Config().Alias != "audit" {
		t.Fatalf("expected alias audit, got %q", aliased.Config().Alias)
	}
	if logger.Config().Alias != "" {
		t.Fatalf("expected parent logger alias to remain empty")
	}
	logger.Info("configured logger", loxa.String("family", "config"))
	if err := logger.Flush(context.Background()); err != nil {
		t.Fatalf("flush configured logger: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected configured logger to emit an event")
	}
}
