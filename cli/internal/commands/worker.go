package commands

import (
	"context"
	"fmt"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

func WorkerCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected worker subcommand (e.g. 'run')")
	}

	switch args[0] {
	case "run":
		fmt.Println("Starting worker...")
		return client.RunWorkerCommand(ctx, cfg.CollectorRepoPath, append([]string{"run"}, args[1:]...))
	case "version":
		return client.RunWorkerCommand(ctx, cfg.CollectorRepoPath, []string{"version"})
	default:
		return fmt.Errorf("unknown worker subcommand: %s", args[0])
	}
}
