package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/astraive/loxa/cli/internal/client"
	"github.com/astraive/loxa/cli/internal/config"
	"github.com/astraive/loxa/cli/internal/output"
)

func StatusCommand(ctx context.Context, cfg config.Config, args []string) error {
	body, err := client.FetchStatus(cfg.CollectorURL)
	if err != nil {
		return fmt.Errorf("fetch status: %w", err)
	}

	if output.ShouldOutputJSON(ctx) {
		fmt.Println(string(body))
		return nil
	}

	var status map[string]any
	if err := json.Unmarshal(body, &status); err != nil {
		fmt.Println(string(body))
		return nil
	}

	output.PrintSection("Collector Status")
	pairs := map[string]string{}
	for k, v := range status {
		pairs[k] = fmt.Sprintf("%v", v)
	}
	output.PrintKeyValue(pairs)
	return nil
}
