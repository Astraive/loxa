package commands

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/loadgen"
	"github.com/astraive/loxa-cli/internal/output"
)

func BenchCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	url := fs.String("url", cfg.CollectorURL, "collector URL")
	rate := fs.Int("rate", 100, "events per second")
	duration := fs.Duration("duration", 10*time.Second, "benchmark duration")
	size := fs.Int("size", 1024, "payload size in bytes")
	batchSize := fs.Int("batch-size", 10, "events per batch")
	if err := fs.Parse(args); err != nil {
		return err
	}

	totalEvents := *rate * int(duration.Seconds())
	if totalEvents <= 0 {
		totalEvents = *rate
	}
	workers := 4
	if *rate < workers {
		workers = max(*rate, 1)
	}

	output.PrintSection("Load Benchmark")
	fmt.Printf("  Target:      %s\n", *url)
	fmt.Printf("  Rate:        %d events/sec\n", *rate)
	fmt.Printf("  Duration:    %v\n", *duration)
	fmt.Printf("  Total:       %d events\n", totalEvents)
	fmt.Printf("  Workers:     %d\n", workers)
	fmt.Printf("  Batch size:  %d\n", *batchSize)
	fmt.Printf("  Payload:     %d bytes\n", *size)
	fmt.Println()

	runnerCfg := loadgen.RunnerConfig{
		URL:       *url,
		Events:    totalEvents,
		Workers:   workers,
		BatchSize: *batchSize,
		BodySize:  *size,
	}

	report, err := loadgen.Run(ctx, runnerCfg)
	if err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}
	report.Print()
	return nil
}
