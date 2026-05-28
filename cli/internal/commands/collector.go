package commands

import (
	"context"
	"fmt"

	"github.com/astraive/loxa/cli/internal/client"
	"github.com/astraive/loxa/cli/internal/config"
)

func CollectorCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected collector subcommand (e.g. 'run')")
	}

	switch args[0] {
	case "run":
		fmt.Println("Starting collector...")
		return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, append([]string{"run"}, args[1:]...))
	case "version":
		return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, []string{"version"})
	case "config":
		return runCollectorConfig(ctx, cfg, args[1:])
	default:
		return fmt.Errorf("unknown collector subcommand: %s", args[0])
	}
}

func runCollectorConfig(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected 'print' or 'validate'")
	}
	switch args[0] {
	case "print":
		return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, []string{"config", "print"})
	case "validate":
		return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, []string{"config", "validate"})
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}
