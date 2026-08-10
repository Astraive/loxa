package commands

import (
	"context"
	"flag"
	"fmt"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

func DeleteCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected delete target: tenant, user, or event")
	}

	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	reason := fs.String("reason", "", "optional audit reason for the deletion")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) == 0 {
		return fmt.Errorf("expected identifier for delete target %q", args[0])
	}

	var (
		body []byte
		err  error
	)
	switch args[0] {
	case "tenant":
		body, err = client.DeleteEventsByTenant(ctx, cfg.CollectorURL, rest[0], *reason)
	case "user":
		body, err = client.DeleteEventsByUser(ctx, cfg.CollectorURL, rest[0], *reason)
	case "event":
		body, err = client.DeleteEventByID(ctx, cfg.CollectorURL, rest[0], *reason)
	default:
		return fmt.Errorf("unknown delete target: %s", args[0])
	}
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}
