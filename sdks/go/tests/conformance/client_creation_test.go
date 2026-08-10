package conformance

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loza/sdks/go"
)

func TestClientCreationAndConfiguration(t *testing.T) {
	sink, store := loza.MemorySink()
	cfg := loza.ApplyConfig(
		loza.Test(),
		loza.WithService("verification"),
		loza.WithEnvironment("prod"),
		loza.WithRelease("1.2.3"),
		loza.WithNamespace("payments"),
		loza.WithAPIKey("secret"),
		loza.WithOtelBridge(true),
		loza.WithRetry(2),
		loza.WithTimeout(2*time.Second),
		loza.WithQueueSize(128),
		loza.WithSink(sink),
	)

	logger, err := loza.CreateLoza(cfg)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	if logger.Config().Service != "verification" {
		t.Fatalf("expected service verification, got %q", logger.Config().Service)
	}
	if err := loza.Configure(cfg); err != nil {
		t.Fatalf("configure default: %v", err)
	}
	aliased, err := loza.Alias("audit")
	if err != nil {
		t.Fatalf("alias: %v", err)
	}
	if aliased.Config().Alias != "audit" {
		t.Fatalf("expected alias audit, got %q", aliased.Config().Alias)
	}
	if logger.Config().Alias != "" {
		t.Fatalf("expected parent logger alias to remain empty")
	}
	logger.Info("configured logger", loza.String("family", "config"))
	if err := logger.Flush(context.Background()); err != nil {
		t.Fatalf("flush configured logger: %v", err)
	}
	if store.Len() == 0 {
		t.Fatalf("expected configured logger to emit an event")
	}
}
