package main

import (
	"encoding/json"
	"fmt"
	"os"

	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
	"github.com/astraive/loxa/cli/internal/cli"
	"github.com/astraive/loxa/cli/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if ve, ok := err.(speccontract.ValidationErrors); ok {
			raw, _ := json.Marshal(ve)
			fmt.Fprintln(os.Stderr, string(raw))
		} else {
			fmt.Fprintf(os.Stderr, "loxa: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	return cli.Run(args)
}

func printUsage() {
	fmt.Printf("LOXA CLI v%s\n", version.Version)
	fmt.Println("\nCommands:")
	fmt.Println("  init         Initialize LOXA config")
	fmt.Println("  dev          Start development server (collector + cortex)")
	fmt.Println("  config       Configuration management")
	fmt.Println("  collector    Manage collector binary")
	fmt.Println("  worker       Manage worker binary")
	fmt.Println("  cortex       Cortex operational memory commands")
	fmt.Println("  emit         Emit events to collector")
	fmt.Println("  query        Query events via SQL")
	fmt.Println("  tail         Stream events from collector")
	fmt.Println("  watch        Live event viewer with filters")
	fmt.Println("  status       Show collector status")
	fmt.Println("  sinks        Inspect sink health")
	fmt.Println("  dlq          Dead letter queue management")
	fmt.Println("  quarantine   Manage quarantined events")
	fmt.Println("  keys         API key management")
	fmt.Println("  schema       Schema validation and governance")
	fmt.Println("  delete       GDPR event deletion")
	fmt.Println("  audit        PII audit scanning")
	fmt.Println("  export       Export events to file")
	fmt.Println("  replay       Replay events from file")
	fmt.Println("  incident     View incident details (cortex)")
	fmt.Println("  graph        Service/incident graphs (cortex)")
	fmt.Println("  signatures   Incident signatures (cortex)")
	fmt.Println("  debug        Debug and diagnostics")
	fmt.Println("  bench        Load generation benchmark")
	fmt.Println("  deploy       Deployment asset management")
	fmt.Println("  dashboard    Grafana dashboard management")
	fmt.Println("  doctor       Health checks on all components")
	fmt.Println("\nFlags: --verbose, --output (json/table/text)")
	fmt.Println("Use 'loxa <command> --help' for details")
}
