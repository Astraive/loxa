package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/astraive/loxa/cli/internal/client"
	"github.com/astraive/loxa/cli/internal/config"
	"github.com/astraive/loxa/cli/internal/output"
)

func DebugCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: loxa debug <event|pipeline|cortex> [args...]")
	}

	switch args[0] {
	case "event":
		return debugEvent(ctx, cfg, args[1:])
	case "pipeline":
		return debugPipeline(ctx, cfg)
	case "cortex":
		return debugCortex(ctx, cfg)
	default:
		return fmt.Errorf("unknown debug subcommand: %s", args[0])
	}
}

func debugEvent(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: loxa debug event <event_id>")
	}
	eventID := args[0]
	// Validate event_id is a safe identifier to prevent SQL injection
	// Allow alphanumerics, underscores, hyphens, and colons (for UUID-style IDs)
	if !isValidEventID(eventID) {
		return fmt.Errorf("invalid event_id: must match [a-zA-Z0-9_:-]+")
	}
	sql := fmt.Sprintf("SELECT * FROM events WHERE event_id = %s LIMIT 1", quoteSQLString(eventID))
	body, err := client.Query(cfg.CollectorURL, "duckdb", sql)
	if err != nil {
		return fmt.Errorf("query event: %w", err)
	}

	if output.ShouldOutputJSON(ctx) {
		fmt.Println(string(body))
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	output.PrintSection("Event: " + eventID)
	if rows, ok := result["rows"].([]any); ok && len(rows) > 0 {
		if row, ok := rows[0].(map[string]any); ok {
			pairs := map[string]string{}
			for k, v := range row {
				pairs[k] = fmt.Sprintf("%v", v)
			}
			output.PrintKeyValue(pairs)
		}
	} else {
		fmt.Println("Event not found")
	}
	return nil
}

func quoteSQLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func debugPipeline(ctx context.Context, cfg config.Config) error {
	output.PrintSection("Collector Pipeline")

	status, err := client.FetchStatus(cfg.CollectorURL)
	if err != nil {
		return fmt.Errorf("fetch status: %w", err)
	}
	var st map[string]any
	if err := json.Unmarshal(status, &st); err != nil {
		return fmt.Errorf("decode status: %w", err)
	}

	pairs := map[string]string{}
	for k, v := range st {
		pairs[k] = fmt.Sprintf("%v", v)
	}
	output.PrintKeyValue(pairs)

	sinks, err := client.FetchSinks(cfg.CollectorURL)
	if err == nil {
		fmt.Println()
		output.PrintSection("Sinks")
		var sk map[string]any
		if err := json.Unmarshal(sinks, &sk); err != nil {
			return fmt.Errorf("decode sinks: %w", err)
		}
		if sinkList, ok := sk["sinks"].([]any); ok {
			rows := [][]string{}
			for _, s := range sinkList {
				if m, ok := s.(map[string]any); ok {
					rows = append(rows, []string{
						fmt.Sprintf("%v", m["name"]),
						fmt.Sprintf("%v", m["type"]),
						fmt.Sprintf("%v", m["health"]),
					})
				}
			}
			output.PrintTable([]string{"Name", "Type", "Health"}, rows)
		}
	}

	fmt.Println()
	fmt.Println(output.Bold("Pipeline Flow:"))
	fmt.Println("  ingest -> validate -> enrich -> dedupe -> privacy -> schema -> [fanout sinks]")

	return nil
}

func debugCortex(ctx context.Context, cfg config.Config) error {
	cortexURL := getCortexURL(cfg)
	output.PrintSection("Cortex Diagnostics")

	if err := client.CheckCortexHealth(cortexURL); err != nil {
		fmt.Printf("  Health: %s\n", output.Error("FAIL"))
	} else {
		fmt.Printf("  Health: %s\n", output.Success("OK"))
	}

	if err := client.CheckCortexReady(cortexURL); err != nil {
		fmt.Printf("  Ready:  %s\n", output.Error("FAIL"))
	} else {
		fmt.Printf("  Ready:  %s\n", output.Success("OK"))
	}

	metrics, err := client.FetchCortexMetrics(cortexURL)
	if err == nil {
		fmt.Println()
		output.PrintSection("Key Metrics")
		lines := strings.Split(string(metrics), "\n")
		count := 0
		for _, line := range lines {
			if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
				continue
			}
			if strings.Contains(line, "cortex_") {
				fmt.Printf("  %s\n", line)
				count++
				if count >= 20 {
					fmt.Println("  ... (truncated)")
					break
				}
			}
		}
	}

	fmt.Println()
	fmt.Println(output.Bold("Subsystems:"))
	fmt.Println("  Storage:  DuckDB or PostgreSQL")
	fmt.Println("  Matcher:  Go (default) or Rust FFI")
	fmt.Println("  Learner:  Continuous remediation learning")
	fmt.Println("  Memory:   Signature evolution and decay")

	return nil
}
