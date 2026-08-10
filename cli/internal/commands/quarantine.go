package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
	"github.com/astraive/loza/cli/internal/output"
)

func QuarantineCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand: list, replay, delete")
	}

	switch args[0] {
	case "list":
		body, err := client.FetchQuarantine(cfg.CollectorURL)
		if err != nil {
			return err
		}
		if output.ShouldOutputJSON(ctx) {
			fmt.Println(string(body))
			return nil
		}
		var result map[string]any
		if json.Unmarshal(body, &result) == nil {
			if events, ok := result["events"].([]any); ok {
				rows := [][]string{}
				for _, e := range events {
					if m, ok := e.(map[string]any); ok {
						rows = append(rows, []string{
							fmt.Sprintf("%v", m["id"]),
							fmt.Sprintf("%v", m["event_id"]),
							fmt.Sprintf("%v", m["reason"]),
							fmt.Sprintf("%v", m["service"]),
							fmt.Sprintf("%v", m["timestamp"]),
						})
					}
				}
				output.PrintSection("Quarantined Events")
				output.PrintTable([]string{"ID", "Event ID", "Reason", "Service", "Timestamp"}, rows)
				return nil
			}
		}
		fmt.Println(string(body))
		return nil
	case "replay":
		if len(args) < 2 {
			return fmt.Errorf("usage: loza quarantine replay <id>")
		}
		body, err := client.ReplayQuarantineItem(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: loza quarantine delete <id>")
		}
		body, err := client.DeleteQuarantineItem(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	default:
		return fmt.Errorf("unknown quarantine subcommand: %s", args[0])
	}
}
