package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

func KeysCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand: create, revoke, rotate")
	}

	switch args[0] {
	case "create":
		payload := []byte(`{}`)
		if len(args) > 1 {
			raw := []byte(args[1])
			if !json.Valid(raw) {
				return fmt.Errorf("invalid JSON payload: %s", args[1])
			}
			payload = raw
		}
		body, err := client.CreateAPIKey(ctx, cfg.CollectorURL, payload)
		if err != nil {
			return err
		}
		if output.ShouldOutputJSON(ctx) {
			fmt.Println(string(body))
			return nil
		}
		var result map[string]any
		if json.Unmarshal(body, &result) == nil {
			output.PrintSection("Created API Key")
			output.PrintKeyValue(map[string]string{
				"ID":     fmt.Sprintf("%v", result["id"]),
				"Key":    fmt.Sprintf("%v", result["key"]),
				"Status": fmt.Sprintf("%v", result["status"]),
			})
			return nil
		}
		fmt.Println(string(body))
		return nil
	case "revoke":
		if len(args) < 2 {
			return fmt.Errorf("usage: loxa keys revoke <id>")
		}
		body, err := client.RevokeAPIKey(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "rotate":
		if len(args) < 2 {
			return fmt.Errorf("usage: loxa keys rotate <id>")
		}
		body, err := client.RotateAPIKey(ctx, cfg.CollectorURL, args[1])
		if err != nil {
			return err
		}
		if output.ShouldOutputJSON(ctx) {
			fmt.Println(string(body))
			return nil
		}
		var result map[string]any
		if json.Unmarshal(body, &result) == nil {
			output.PrintSection("Rotated API Key")
			output.PrintKeyValue(map[string]string{
				"ID":     fmt.Sprintf("%v", result["id"]),
				"Key":    fmt.Sprintf("%v", result["new_key"]),
				"Status": fmt.Sprintf("%v", result["status"]),
			})
			return nil
		}
		fmt.Println(string(body))
		return nil
	default:
		return fmt.Errorf("unknown keys subcommand: %s", args[0])
	}
}
