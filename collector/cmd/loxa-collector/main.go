package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/astraive/loxa-collector/internal/version"
	// Register eventbus backends
	_ "github.com/astraive/loxa-collector/internal/eventbus/kafka"
	_ "github.com/astraive/loxa-collector/internal/eventbus/memory"
	_ "github.com/astraive/loxa-collector/internal/eventbus/nats"
	_ "github.com/astraive/loxa-collector/internal/eventbus/redis"
)

var collectorVersion = version.CollectorVersion()

func main() {
	if err := executeCollectorCLI(os.Args[1:], runCollector, configCommand); err != nil {
		log.Fatalf("collector: %v", err)
	}
}

func executeCollectorCLI(args []string, runFn func(collectorConfig) error, configFn func([]string) error) error {
	if len(args) > 0 {
		switch args[0] {
		case "config":
			if err := configFn(args[1:]); err != nil {
				return fmt.Errorf("config: %w", err)
			}
			return nil
		case "run":
			args = args[1:]
		case "version", "-v", "--version":
			fmt.Println("loxa-collector version", collectorVersion)
			return nil
		default:
			if !strings.HasPrefix(args[0], "-") {
				return fmt.Errorf("unknown command %q", args[0])
			}
		}
	}

	cfg, err := loadCollectorConfigFromArgs(args)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := runFn(cfg); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}
