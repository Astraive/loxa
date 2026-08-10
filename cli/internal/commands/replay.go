package commands

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

func ReplayCommand(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	source := fs.String("source", "", "NDJSON source file path (optional, replays from collector DLQ if not set)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *source != "" {
		fmt.Printf("Replaying from file: %s\n", *source)
		return replayFromFile(cfg, *source)
	}

	fmt.Println("Replaying DLQ events from collector...")
	return replayFromCollector(cfg)
}

func replayFromFile(cfg config.Config, sourcePath string) error {
	data, err := readFromFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	return client.ReplayEvents(cfg.CollectorURL, data)
}

func replayFromCollector(cfg config.Config) error {
	dlqPath := filepath.Clean(cfg.DLQPath)
	data, err := readFromFile(dlqPath)
	if err != nil {
		return fmt.Errorf("failed to read DLQ events from %s: %w", dlqPath, err)
	}
	return client.ReplayEvents(cfg.CollectorURL, data)
}

func readFromFile(path string) ([]byte, error) {
	return readFileBytes(path)
}
