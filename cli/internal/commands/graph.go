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

func GraphCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	depth := fs.Int("depth", 3, "graph depth")
	format := fs.String("format", "ascii", "output format: json or ascii")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: loxa graph <service|incident> <name|id> [--depth N]")
	}

	graphType := fs.Arg(0)
	graphID := fs.Arg(1)
	cortexURL := getCortexURL(cfg)

	var data []byte
	var err error
	switch graphType {
	case "service":
		data, err = client.FetchCortexServiceGraph(ctx, cortexURL, graphID, *depth)
	case "incident":
		data, err = client.FetchCortexIncidentGraph(ctx, cortexURL, graphID, *depth)
	default:
		return fmt.Errorf("unknown graph type: %s (use 'service' or 'incident')", graphType)
	}
	if err != nil {
		return fmt.Errorf("fetch graph: %w", err)
	}

	if *format == "json" || output.ShouldOutputJSON(ctx) {
		fmt.Println(string(data))
		return nil
	}

	var graph map[string]any
	if err := json.Unmarshal(data, &graph); err != nil {
		fmt.Println(string(data))
		return nil
	}

	output.PrintSection(fmt.Sprintf("Graph: %s %s (depth=%d)", graphType, graphID, *depth))
	fmt.Println(output.RenderASCIIGraph(graph))
	return nil
}
