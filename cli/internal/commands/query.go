package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

func QueryCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	engine := fs.String("engine", "duckdb", "query engine")
	sqlQuery := fs.String("q", "", "SQL query")
	format := fs.String("format", "", "output format: table, json, csv")
	rowLimit := fs.Int("limit", 0, "limit rows (0 = unlimited)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sqlQuery == "" {
		return fmt.Errorf("-q flag is required")
	}

	if *rowLimit > 0 && !strings.Contains(strings.ToUpper(*sqlQuery), "LIMIT") {
		*sqlQuery = fmt.Sprintf("%s LIMIT %d", *sqlQuery, *rowLimit)
	}

	result, err := client.Query(cfg.CollectorURL, *engine, *sqlQuery)
	if err != nil {
		return err
	}

	outFmt := *format
	if outFmt == "" {
		outFmt = output.GetFormat(ctx)
	}

	switch outFmt {
	case "json":
		fmt.Println(string(result))
		return nil
	case "csv":
		return printCSV(result)
	default:
		return printTable(result)
	}
}

func printCSV(result []byte) error {
	var data map[string]any
	if err := json.Unmarshal(result, &data); err != nil {
		fmt.Println(string(result))
		return nil
	}
	cols, _ := data["columns"].([]any)
	rows, _ := data["rows"].([]any)
	headers := []string{}
	for _, c := range cols {
		headers = append(headers, fmt.Sprintf("%v", c))
	}
	fmt.Println(strings.Join(headers, ","))
	for _, r := range rows {
		cells := extractRow(r, cols)
		fmt.Println(strings.Join(cells, ","))
	}
	return nil
}

func printTable(result []byte) error {
	var data map[string]any
	if err := json.Unmarshal(result, &data); err != nil {
		fmt.Println(string(result))
		return nil
	}
	cols, _ := data["columns"].([]any)
	rows, _ := data["rows"].([]any)
	headers := []string{}
	for _, c := range cols {
		headers = append(headers, fmt.Sprintf("%v", c))
	}
	tableRows := [][]string{}
	for _, r := range rows {
		tableRows = append(tableRows, extractRow(r, cols))
	}
	output.PrintTable(headers, tableRows)
	return nil
}

func extractRow(r any, cols []any) []string {
	if row, ok := r.([]any); ok {
		cells := []string{}
		for _, c := range row {
			cells = append(cells, fmt.Sprintf("%v", c))
		}
		return cells
	}
	if row, ok := r.(map[string]any); ok {
		cells := []string{}
		for _, c := range cols {
			key := fmt.Sprintf("%v", c)
			cells = append(cells, fmt.Sprintf("%v", row[key]))
		}
		return cells
	}
	return []string{fmt.Sprintf("%v", r)}
}
