package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
	"github.com/astraive/loza/cli/internal/output"
)

func parseQueryParameter(raw string) (string, any, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", nil, fmt.Errorf("query parameter must be key=value")
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if value == "null" {
		return key, nil, nil
	}
	if value == "true" || value == "false" {
		return key, value == "true", nil
	}
	if integer, err := strconv.ParseInt(value, 10, 64); err == nil {
		return key, integer, nil
	}
	if decimal, err := strconv.ParseFloat(value, 64); err == nil {
		return key, decimal, nil
	}
	return key, value, nil
}

func QueryCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	engine := fs.String("engine", "duckdb", "query engine (used only with --raw-sql)")
	lqlQuery := fs.String("q", "", "LQL query")
	rawSQL := fs.Bool("raw-sql", false, "send -q as explicit raw SQL to /query")
	format := fs.String("format", "", "output format: table, json, csv")
	rowLimit := fs.Int("limit", 0, "limit rows (0 = collector default)")
	parameters := map[string]any{}
	fs.Func("param", "typed LQL parameter (key=value; repeatable)", func(raw string) error {
		key, value, err := parseQueryParameter(raw)
		if err != nil {
			return err
		}
		parameters[key] = value
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *lqlQuery == "" {
		return fmt.Errorf("-q flag is required")
	}

	var (
		result []byte
		err    error
	)
	if *rawSQL {
		result, err = client.Query(cfg.CollectorURL, *engine, *lqlQuery)
	} else {
		result, err = client.QueryLQL(cfg.CollectorURL, *lqlQuery, parameters, *rowLimit)
	}
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
			cells = append(cells, formatCell(c))
		}
		return cells
	}
	if row, ok := r.(map[string]any); ok {
		cells := []string{}
		for _, c := range cols {
			key := fmt.Sprintf("%v", c)
			cells = append(cells, formatCell(row[key]))
		}
		return cells
	}
	return []string{formatCell(r)}
}

func formatCell(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case map[string]any, []any:
		b, err := json.Marshal(val)
		if err == nil {
			return string(b)
		}
	}
	return fmt.Sprintf("%v", v)
}
