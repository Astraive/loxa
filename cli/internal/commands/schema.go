package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/schema"
)

func SchemaCommand(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand: validate, fetch, list, diff, or publish")
	}

	switch args[0] {
	case "validate":
		fs := flag.NewFlagSet("schema validate", flag.ContinueOnError)
		strict := fs.Bool("strict", false, "enable strict validation (default: loose)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		remaining := fs.Args()
		if len(remaining) == 0 {
			if err := schema.ValidateSpecAssets(cfg.SpecRepoPath); err != nil {
				return err
			}
			fmt.Println("Spec assets validated successfully")
			return nil
		}
		path := remaining[0]
		if err := schema.ValidateEventFileStrict(path, *strict); err != nil {
			return err
		}
		fmt.Println("Event validated successfully")
		return nil
	case "fetch":
		return schema.FetchSpec(cfg.SpecRepoPath)
	case "list":
		body, err := client.FetchSchema(cfg.CollectorURL)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	case "diff":
		return runSchemaMutation(context.Background(), cfg, "/schema/diff", args[1:])
	case "publish":
		return runSchemaMutation(context.Background(), cfg, "/schema/publish", args[1:])
	case "blueprint":
		return runBlueprintCommand(context.Background(), cfg, args[1:])
	default:
		return fmt.Errorf("unknown schema subcommand: %s (expected: validate, fetch, list, diff, publish, blueprint)", args[0])
	}
}

func runBlueprintCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected blueprint subcommand: add, list")
	}

	switch args[0] {
	case "add":
		return runBlueprintAdd(ctx, cfg, args[1:])
	case "list":
		return runBlueprintList(ctx, cfg)
	default:
		return fmt.Errorf("unknown blueprint subcommand: %s (expected: add, list)", args[0])
	}
}

func runBlueprintAdd(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("blueprint add", flag.ContinueOnError)
	name := fs.String("name", "", "blueprint name")
	columns := fs.String("columns", "", "columns as name:type:path (comma-separated)")
	file := fs.String("file", "", "JSON file containing blueprint")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		body, err := client.PublishBlueprint(ctx, cfg.CollectorURL, data)
		if err != nil {
			return err
		}
		fmt.Println(string(body))
		return nil
	}

	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if *columns == "" {
		return fmt.Errorf("--columns is required (format: name:type:path,name:type:path)")
	}

	blueprint := map[string]any{
		"name":    *name,
		"columns": map[string]any{},
	}
	cols := blueprint["columns"].(map[string]any)
	for _, col := range strings.Split(*columns, ",") {
		parts := strings.SplitN(strings.TrimSpace(col), ":", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid column format: %s (expected name:type:path)", col)
		}
		cols[parts[0]] = map[string]string{
			"duckdb_type": parts[1],
			"json_path":   parts[2],
		}
	}

	data, err := json.Marshal(blueprint)
	if err != nil {
		return err
	}
	body, err := client.PublishBlueprint(ctx, cfg.CollectorURL, data)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func runBlueprintList(ctx context.Context, cfg config.Config) error {
	body, err := client.ListBlueprints(ctx, cfg.CollectorURL)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func runSchemaMutation(ctx context.Context, cfg config.Config, path string, args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	schemaVersion := fs.String("schema-version", "", "schema version")
	eventVersion := fs.String("event-version", "", "event version")
	required := fs.String("required", "", "comma-separated required fields")
	file := fs.String("file", "", "JSON file containing schema payload")
	if err := fs.Parse(args); err != nil {
		return err
	}

	payload, err := schemaMutationPayload(*schemaVersion, *eventVersion, *required, *file)
	if err != nil {
		return err
	}

	var body []byte
	switch path {
	case "/schema/diff":
		body, err = client.DiffSchema(ctx, cfg.CollectorURL, payload)
	case "/schema/publish":
		body, err = client.PublishSchema(ctx, cfg.CollectorURL, payload)
	default:
		return fmt.Errorf("unsupported schema operation")
	}
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func schemaMutationPayload(schemaVersion, eventVersion, requiredCSV, file string) ([]byte, error) {
	if strings.TrimSpace(file) != "" {
		return os.ReadFile(file)
	}
	fields := make([]string, 0)
	for _, part := range strings.Split(requiredCSV, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			fields = append(fields, trimmed)
		}
	}
	raw, err := json.Marshal(map[string]any{
		"schema_version":  strings.TrimSpace(schemaVersion),
		"event_version":   strings.TrimSpace(eventVersion),
		"required_fields": fields,
	})
	if err != nil {
		return nil, err
	}
	return raw, nil
}
