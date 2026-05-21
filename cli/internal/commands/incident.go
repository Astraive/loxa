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

func IncidentCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("incident", flag.ContinueOnError)
	mode := fs.String("mode", "fast", "reconstruction mode: fast or deep")
	depth := fs.Int("depth", 3, "graph depth")
	format := fs.String("format", "", "output format: json or text")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("usage: loxa incident <id> [--mode fast|deep] [--depth N]")
	}
	incidentID := fs.Arg(0)

	cortexURL := getCortexURL(cfg)

	recon, err := client.ReconstructCortexIncident(ctx, cortexURL, incidentID, *mode, 0)
	if err != nil {
		return fmt.Errorf("reconstruct: %w", err)
	}

	if *format == "json" || output.ShouldOutputJSON(ctx) {
		fmt.Println(string(recon))
		return nil
	}

	var result map[string]any
	if err := json.Unmarshal(recon, &result); err != nil {
		fmt.Println(string(recon))
		return nil
	}

	output.PrintSection("Incident: " + incidentID)

	pairs := map[string]string{
		"Incident ID":    fmt.Sprintf("%v", result["incident_id"]),
		"Timestamp":      fmt.Sprintf("%v", result["timestamp"]),
		"Confidence":     fmt.Sprintf("%v", result["confidence"]),
		"Primary Svc":    fmt.Sprintf("%v", result["primary_service"]),
		"Severity":       fmt.Sprintf("%v", result["severity"]),
	}
	output.PrintKeyValue(pairs)

	if services, ok := result["related_services"].([]any); ok && len(services) > 0 {
		fmt.Println()
		fmt.Println(output.Bold("Related Services:"))
		for _, s := range services {
			fmt.Printf("  - %v\n", s)
		}
	}

	if chain, ok := result["causal_chain"].([]any); ok && len(chain) > 0 {
		fmt.Println()
		fmt.Println(output.Bold("Causal Chain:"))
		for i, c := range chain {
			if m, ok := c.(map[string]any); ok {
				fmt.Printf("  %d. %v (%v)\n", i+1, m["event"], m["timestamp"])
			}
		}
	}

	if symptoms, ok := result["symptoms"].([]any); ok && len(symptoms) > 0 {
		fmt.Println()
		fmt.Println(output.Bold("Symptoms:"))
		for _, s := range symptoms {
			if m, ok := s.(map[string]any); ok {
				fmt.Printf("  - %v\n", m["description"])
			}
		}
	}

	if remediations, ok := result["suggested_remediations"].([]any); ok && len(remediations) > 0 {
		fmt.Println()
		fmt.Println(output.Bold("Suggested Remediations:"))
		for _, r := range remediations {
			if m, ok := r.(map[string]any); ok {
				fmt.Printf("  - %v\n", m["action"])
			}
		}
	}

	graph, err := client.FetchCortexIncidentGraph(ctx, cortexURL, incidentID, *depth)
	if err == nil {
		var g map[string]any
		if json.Unmarshal(graph, &g) == nil {
			fmt.Println()
			fmt.Println(output.Bold("Dependency Graph:"))
			fmt.Println(output.RenderASCIIGraph(g))
		}
	}

	return nil
}
