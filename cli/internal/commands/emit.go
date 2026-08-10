package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
	speccontract "github.com/astraive/loza/spec/generated/go/contract"
	"github.com/astraive/loza/cli/internal/version"
)

func EmitCommand(cfg config.Config, args []string) error {
	if len(args) == 0 || args[0] != "sample" {
		return fmt.Errorf("expected subcommand: sample")
	}

	fs := flag.NewFlagSet("emit sample", flag.ContinueOnError)
	service := fs.String("service", "loza-cli", "service name")
	eventName := fs.String("event", "sample.event", "event name")
	kind := fs.String("kind", "cli", "event kind (http, db, rpc, cli, etc.)")
	outcome := fs.String("outcome", "success", "event outcome (success, error, timeout)")
	level := fs.String("level", "info", "event level (debug, info, warn, error)")
	attrsJSON := fs.String("attrs", "", "JSON string of custom attributes")
	printOnly := fs.Bool("print", false, "print sample without sending")
	outputPath := fs.String("output", "", "write sample envelope to a file")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	attrs := map[string]any{"sample": true}
	if *attrsJSON != "" {
		if err := json.Unmarshal([]byte(*attrsJSON), &attrs); err != nil {
			return fmt.Errorf("invalid --attrs JSON: %w", err)
		}
	}

	event := map[string]any{
		"schema_version": speccontract.LOZASpecVersion,
		"event_version":  speccontract.LOZAEventVersion,
		"event_id":       fmt.Sprintf("evt_sample_%d", time.Now().UnixNano()),
		"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
		"service":        *service,
		"event":          *eventName,
		"kind":           *kind,
		"level":          *level,
		"outcome":        *outcome,
		"event_state":    "finished",
		"attrs":          attrs,
	}
	eventRaw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	compactRaw, err := speccontract.MarshalIngestEnvelope("loza-cli", version.Version, *service, []json.RawMessage{eventRaw})
	if err != nil {
		return err
	}
	var pretty any
	if err := json.Unmarshal(compactRaw, &pretty); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return err
	}

	if *outputPath != "" {
		if err := os.WriteFile(*outputPath, raw, 0o600); err != nil {
			return err
		}
		fmt.Printf("wrote sample envelope to %s\n", *outputPath)
	}
	if *printOnly {
		fmt.Println(string(raw))
		return nil
	}
	if err := client.PostIngest(cfg.CollectorURL, "application/json", raw); err != nil {
		return err
	}
	fmt.Printf("sent sample event %s for service %s (kind=%s outcome=%s level=%s)\n", *eventName, *service, *kind, *outcome, *level)
	return nil
}
