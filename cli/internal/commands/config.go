package commands

import (
	"context"
	"fmt"

	"github.com/astraive/loxa-cli/internal/config"
)

func ConfigCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected 'print' or 'validate' subcommand")
	}
	switch args[0] {
	case "print", "validate":
		return runCollectorConfigCommand(ctx, cfg, append([]string{args[0]}, args[1:]...))
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}
