package main

import (
	"context"
	"testing"
	"time"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/models"
)

type noopBatchProcessor struct{}

func (noopBatchProcessor) ProcessBatch(context.Context, []*models.Event) error { return nil }

func TestStartCollectorSyncReturnsWaitableCompletion(t *testing.T) {
	cfg := config.Default()
	cfg.Collector.SourceOfTruth = true
	cfg.Collector.Mode = "pull"

	ctx, cancel := context.WithCancel(context.Background())
	done := startCollectorSync(ctx, cfg.Collector, noopBatchProcessor{})
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("collector sync did not stop after cancellation")
	}
}
