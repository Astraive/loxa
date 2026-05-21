package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

func SignaturesCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("signatures", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "max signatures to list")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cortexURL := getCortexURL(cfg)
	data, err := client.FetchSignatures(ctx, cortexURL, *limit)
	if err != nil {
		return fmt.Errorf("fetch signatures: %w", err)
	}

	if output.ShouldOutputJSON(ctx) {
		fmt.Println(string(data))
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}

	sigs, ok := result["data"].(map[string]any)["signatures"].([]any)
	if !ok {
		fmt.Println(string(data))
		return nil
	}

	rows := [][]string{}
	for _, s := range sigs {
		if m, ok := s.(map[string]any); ok {
			rows = append(rows, []string{
				fmt.Sprintf("%v", m["id"]),
				fmt.Sprintf("%v", m["pattern"]),
				fmt.Sprintf("%v", m["count"]),
				fmt.Sprintf("%v", m["lastSeen"]),
			})
		}
	}
	output.PrintSection("Incident Signatures")
	output.PrintTable([]string{"ID", "Pattern", "Count", "Last Seen"}, rows)
	return nil
}
