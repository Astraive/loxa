package commands

import (
	"flag"
	"fmt"
	"strings"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
)

func ExportCommand(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	engine := fs.String("engine", "duckdb", "query engine")
	outputPath := fs.String("o", "export.ndjson", "output file path")
	format := fs.String("format", "ndjson", "output format (ndjson or parquet)")
	table := fs.String("table", "events", "table name to query")
	if err := fs.Parse(args); err != nil {
		return err
	}

	safeTable := strings.TrimSpace(*table)
	if safeTable == "" {
		safeTable = "events"
	}
	if !isValidIdentifier(safeTable) {
		return fmt.Errorf("invalid table name: must start with letter/underscore, contain only alphanumeric/underscore")
	}
	query := fmt.Sprintf("SELECT * FROM %s", safeTable)

	result, err := client.Query(cfg.CollectorURL, *engine, query)
	if err != nil {
		return err
	}

	return writeExport(*outputPath, result, *format)
}

func writeExport(path string, data []byte, format string) error {
	switch format {
	case "ndjson":
		return writeFile(path, data)
	case "parquet":
		return fmt.Errorf("parquet export is not implemented; use --format ndjson")
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

func writeFile(path string, data []byte) error {
	return writeFileBytes(path, data)
}
