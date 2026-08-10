package commands

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/astraive/loza/cli/internal/config"
)

func ConfigCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	valMode := fs.String("validation-mode", "", "validation mode: off, warn, enforce, quarantine")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *valMode != "" {
		cfg.ValidationMode = strings.ToLower(strings.TrimSpace(*valMode))
	}
	subArgs := fs.Args()
	if len(subArgs) == 0 {
		return fmt.Errorf("expected 'print' or 'validate' subcommand")
	}
	switch subArgs[0] {
	case "print", "validate":
		collectorArgs := []string{subArgs[0]}
		if *valMode != "" {
			collectorArgs = append(collectorArgs, "--validation-mode", *valMode)
		}
		collectorArgs = append(collectorArgs, subArgs[1:]...)
		return runCollectorConfigCommand(ctx, cfg, collectorArgs)
	default:
		return fmt.Errorf("unknown config subcommand: %s", subArgs[0])
	}
}
