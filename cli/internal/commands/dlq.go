package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

func DLQCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand: list, show, delete, replay, replay-all")
	}

	switch args[0] {
	case "list":
		body, err := client.FetchDLQ(cfg.CollectorURL)
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
							fmt.Sprintf("%v", m["dlq_id"]),
							fmt.Sprintf("%v", m["event_id"]),
							fmt.Sprintf("%v", m["error"]),
							fmt.Sprintf("%v", m["service"]),
						})
					}
				}
				output.PrintSection("Dead Letter Queue")
				output.PrintTable([]string{"DLQ ID", "Event ID", "Error", "Service"}, rows)
				return nil
			}
		}
		fmt.Println(string(body))
		return nil
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: loxa dlq show <id>")
		}
		body, err := client.FetchDLQItem(cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("usage: loxa dlq delete <id>")
		}
		body, err := client.DeleteDLQItem(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "replay":
		if len(args) < 2 {
			return fmt.Errorf("usage: loxa dlq replay <id>")
		}
		body, err := client.ReplayDLQItem(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "replay-all":
		body, err := client.ReplayDLQ(ctx, cfg.CollectorURL)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	default:
		return fmt.Errorf("unknown dlq subcommand: %s", args[0])
	}
}
