package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

// CortexCommand handles loxa cortex subcommands
func CortexCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a cortex subcommand")
	}

	switch args[0] {
	case "run":
		return RunCortexServer(ctx, cfg, args[1:])
	case "ingest":
		return RunCortexIngest(ctx, cfg, args[1:])
	case "reconstruct":
		return RunCortexReconstruct(ctx, cfg, args[1:])
	case "similar":
		return RunCortexSimilar(ctx, cfg, args[1:])
	case "remediation":
		return RunCortexRemediation(ctx, cfg, args[1:])
	case "feedback":
		return RunCortexFeedback(ctx, cfg, args[1:])
	case "graph":
		return RunCortexGraph(ctx, cfg, args[1:])
	case "replay":
		return RunCortexReplay(ctx, cfg, args[1:])
	default:
		return fmt.Errorf("unknown cortex command: %s", args[0])
	}
}

func getCortexURL(cfg config.Config) string {
	if cfg.Cortex != nil && cfg.Cortex.URL != "" {
		return strings.TrimRight(cfg.Cortex.URL, "/")
	}
	url := os.Getenv("LOXA_CORTEX_URL")
	if url != "" {
		return strings.TrimRight(url, "/")
	}
	return "http://localhost:9100"
}

func RunCortexServer(ctx context.Context, cfg config.Config, args []string) error {
	output.PrintSection("Starting Cortex Server")
	if strings.TrimSpace(cfg.CortexRepoPath) == "" {
		return fmt.Errorf("cortex_repo_path is not configured")
	}
	mainPath := filepath.Join(cfg.CortexRepoPath, "cmd", "server", "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		return fmt.Errorf("cortex server not found at %s", cfg.CortexRepoPath)
	}
	cmdArgs := args
	hasConfig := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" || args[i] == "-config" {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		cmdArgs = append([]string{"--config", filepath.Join(cfg.CortexRepoPath, "configs", "loxa-cortex.defaults.yaml")}, cmdArgs...)
	}
	return client.RunCortexCommand(ctx, cfg.CortexRepoPath, cmdArgs)
}

func RunCortexIngest(ctx context.Context, cfg config.Config, args []string) error {
	var file, url string
	for i := 0; i < len(args); i++ {
		if args[i] == "--file" && i+1 < len(args) {
			file = args[i+1]
			i++
		} else if args[i] == "--url" && i+1 < len(args) {
			url = args[i+1]
			i++
		}
	}

	if file == "" && url == "" {
		return fmt.Errorf("specify --file <path> or --url <url>")
	}

	cortexURL := getCortexURL(cfg)

	var events []map[string]interface{}
	if file != "" {
		events = parseNDJSONFile(file)
	}

	if url != "" {
		events = fetchEventsFromURL(url)
	}

	if len(events) == 0 {
		return fmt.Errorf("no events to ingest")
	}
	if err := client.IngestCortexBatch(ctx, cortexURL, events); err != nil {
		return fmt.Errorf("ingest failed: %w", err)
	}

	fmt.Printf("Ingested %d events\n", len(events))
	return nil
}

func parseNDJSONFile(path string) []map[string]interface{} {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open file: %v\n", err)
		return nil
	}
	defer f.Close()

	var events []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events
}

func fetchEventsFromURL(url string) []map[string]interface{} {
	resp, err := http.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var events []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil
	}
	return events
}

func RunCortexReconstruct(ctx context.Context, cfg config.Config, args []string) error {
	var incidentID, mode string
	for i := 0; i < len(args); i++ {
		if args[i] == "--incident" && i+1 < len(args) {
			incidentID = args[i+1]
			i++
		} else if args[i] == "--mode" && i+1 < len(args) {
			mode = args[i+1]
			i++
		}
	}

	if incidentID == "" {
		return fmt.Errorf("specify --incident <id>")
	}

	if mode == "" {
		mode = "fast"
	}

	cortexURL := getCortexURL(cfg)
	var context map[string]interface{}
	raw, err := client.ReconstructCortexIncident(ctx, cortexURL, incidentID, mode, 0)
	if err != nil {
		return fmt.Errorf("reconstruct failed: %w", err)
	}
	if err := json.Unmarshal(raw, &context); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	outputIncidentContext(context)
	return nil
}

func outputIncidentContext(ctx map[string]interface{}) {
	output.PrintSection("Incident Context")

	fmt.Printf("Incident ID: %s\n", ctx["incident_id"])
	fmt.Printf("Timestamp: %s\n", ctx["timestamp"])
	fmt.Printf("Confidence: %.2f\n", ctx["confidence"])

	if causalChain, ok := ctx["causal_chain"].([]interface{}); ok && len(causalChain) > 0 {
		fmt.Println("\nCausal Chain:")
		for i, e := range causalChain {
			if event, ok := e.(map[string]interface{}); ok {
				fmt.Printf("  %d. [%s] %s @ %s\n", i+1, event["kind"], event["service"], event["timestamp"])
			}
		}
	}

	if similarIncidents, ok := ctx["similar_incidents"].([]interface{}); ok && len(similarIncidents) > 0 {
		fmt.Println("\nSimilar Incidents:")
		for i, s := range similarIncidents {
			if sim, ok := s.(map[string]interface{}); ok {
				fmt.Printf("  %d. %s (similarity: %.2f)\n", i+1, sim["incident_id"], sim["similarity"])
			}
		}
	}

	if symptoms, ok := ctx["symptoms"].([]interface{}); ok && len(symptoms) > 0 {
		fmt.Println("\nSymptoms:")
		for _, s := range symptoms {
			if symptom, ok := s.(map[string]interface{}); ok {
				fmt.Printf("  - %s: %s\n", symptom["type"], symptom["description"])
			}
		}
	}

	if actions, ok := ctx["suggested_actions"].([]interface{}); ok && len(actions) > 0 {
		fmt.Println("\nSuggested Actions:")
		for _, a := range actions {
			if action, ok := a.(map[string]interface{}); ok {
				fmt.Printf("  - %s (success rate: %.1f%%)\n", action["action"], action["success_rate"].(float64)*100)
			}
		}
	}
}

func RunCortexSimilar(ctx context.Context, cfg config.Config, args []string) error {
	var incidentID string
	limit := 5

	for i := 0; i < len(args); i++ {
		if args[i] == "--incident" && i+1 < len(args) {
			incidentID = args[i+1]
			i++
		} else if args[i] == "--limit" && i+1 < len(args) {
			if _, err := fmt.Sscanf(args[i+1], "%d", &limit); err != nil {
				return fmt.Errorf("invalid limit: %w", err)
			}
			i++
		}
	}

	if incidentID == "" {
		return fmt.Errorf("specify --incident <id>")
	}

	cortexURL := getCortexURL(cfg)
	var context map[string]interface{}
	raw, err := client.ReconstructCortexIncident(ctx, cortexURL, incidentID, "", limit)
	if err != nil {
		return fmt.Errorf("similar failed: %w", err)
	}
	if err := json.Unmarshal(raw, &context); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if similar, ok := context["similar_incidents"].([]interface{}); ok {
		fmt.Printf("Found %d similar incidents:\n", len(similar))
		for i, s := range similar {
			if sim, ok := s.(map[string]interface{}); ok {
				fmt.Printf("  %d. %s [similarity: %.2f]\n", i+1, sim["incident_id"], sim["similarity"])
			}
		}
	}

	return nil
}

func RunCortexRemediation(ctx context.Context, cfg config.Config, args []string) error {
	var incidentID, action, operator string

	for i := 0; i < len(args); i++ {
		if args[i] == "--incident" && i+1 < len(args) {
			incidentID = args[i+1]
			i++
		} else if args[i] == "--action" && i+1 < len(args) {
			action = args[i+1]
			i++
		} else if args[i] == "--operator" && i+1 < len(args) {
			operator = args[i+1]
			i++
		}
	}

	if incidentID == "" || action == "" {
		return fmt.Errorf("specify --incident <id> --action <action>")
	}

	remediation := map[string]interface{}{
		"remediation_id": fmt.Sprintf("rem-%d", time.Now().Unix()),
		"incident_id":    incidentID,
		"action":         action,
		"timestamp":      time.Now().Format(time.RFC3339),
		"operator":       operator,
	}

	cortexURL := getCortexURL(cfg)
	payload, _ := json.Marshal(remediation)
	if _, err := client.RecordCortexRemediation(ctx, cortexURL, payload); err != nil {
		return fmt.Errorf("record remediation failed: %w", err)
	}

	fmt.Printf("Recorded remediation: %s for incident %s\n", action, incidentID)
	return nil
}

func RunCortexFeedback(ctx context.Context, cfg config.Config, args []string) error {
	var remediationID, outcome string
	var timeToResolve int64

	for i := 0; i < len(args); i++ {
		if args[i] == "--remediation" && i+1 < len(args) {
			remediationID = args[i+1]
			i++
		} else if args[i] == "--outcome" && i+1 < len(args) {
			outcome = args[i+1]
			i++
		} else if args[i] == "--time-to-resolve" && i+1 < len(args) {
			if _, err := fmt.Sscanf(args[i+1], "%d", &timeToResolve); err != nil {
				return fmt.Errorf("invalid time-to-resolve: %w", err)
			}
			i++
		}
	}

	if remediationID == "" || outcome == "" {
		return fmt.Errorf("specify --remediation <id> --outcome <outcome>")
	}

	feedback := map[string]interface{}{
		"feedback_id":             fmt.Sprintf("fb-%d", time.Now().Unix()),
		"remediation_id":          remediationID,
		"outcome":                 outcome,
		"time_to_resolve_seconds": timeToResolve,
		"timestamp":               time.Now().Format(time.RFC3339),
	}

	cortexURL := getCortexURL(cfg)
	payload, _ := json.Marshal(feedback)
	if _, err := client.RecordCortexFeedback(ctx, cortexURL, payload); err != nil {
		return fmt.Errorf("record feedback failed: %w", err)
	}

	fmt.Printf("Recorded feedback for remediation: %s, outcome: %s\n", remediationID, outcome)
	return nil
}

func RunCortexGraph(ctx context.Context, cfg config.Config, args []string) error {
	var service, incidentID string
	depth := 3

	for i := 0; i < len(args); i++ {
		if args[i] == "--service" && i+1 < len(args) {
			service = args[i+1]
			i++
		} else if args[i] == "--incident" && i+1 < len(args) {
			incidentID = args[i+1]
			i++
		} else if args[i] == "--depth" && i+1 < len(args) {
			if _, err := fmt.Sscanf(args[i+1], "%d", &depth); err != nil {
				return fmt.Errorf("invalid depth: %w", err)
			}
			i++
		}
	}

	if service == "" && incidentID == "" {
		return fmt.Errorf("specify --service <name> or --incident <id>")
	}

	cortexURL := getCortexURL(cfg)
	var raw []byte
	var err error
	if service != "" {
		raw, err = client.FetchCortexServiceGraph(ctx, cortexURL, service, depth)
	} else {
		raw, err = client.FetchCortexIncidentGraph(ctx, cortexURL, incidentID, depth)
	}
	if err != nil {
		return fmt.Errorf("fetch graph failed: %w", err)
	}

	var graph map[string]interface{}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return fmt.Errorf("decode graph: %w", err)
	}

	nodes := graph["nodes"].([]interface{})
	edges := graph["edges"].([]interface{})

	fmt.Printf("Graph: %d nodes, %d edges\n", len(nodes), len(edges))
	return nil
}

func RunCortexReplay(ctx context.Context, cfg config.Config, args []string) error {
	var collectorURL, since string

	for i := 0; i < len(args); i++ {
		if args[i] == "--collector-url" && i+1 < len(args) {
			collectorURL = args[i+1]
			i++
		} else if args[i] == "--since" && i+1 < len(args) {
			since = args[i+1]
			i++
		}
	}

	if collectorURL == "" {
		collectorURL = "http://localhost:8081"
	}
	if since == "" {
		since = "1h"
	}

	fmt.Printf("Replaying events from %s since %s\n", collectorURL, since)
	return nil
}
