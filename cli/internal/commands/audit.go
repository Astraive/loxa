package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
)

func AuditCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected audit subcommand: pii")
	}
	switch args[0] {
	case "pii":
		fs := flag.NewFlagSet("audit pii", flag.ContinueOnError)
		limit := fs.Int("limit", 500, "maximum events to scan")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]int{"limit": *limit})
		if err != nil {
			return err
		}
		body, err := client.AuditPII(ctx, cfg.CollectorURL, payload)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	default:
		return fmt.Errorf("unknown audit subcommand: %s", args[0])
	}
}
