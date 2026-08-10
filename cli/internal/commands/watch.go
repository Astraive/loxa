package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
	"github.com/astraive/loza/cli/internal/output"
)

func WatchCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	kindFilter := fs.String("kind", "", "filter by event kind")
	serviceFilter := fs.String("service", "", "filter by service name")
	levelFilter := fs.String("level", "", "filter by level (debug, info, warn, error)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	filters := map[string]string{}
	if *kindFilter != "" {
		filters["kind"] = *kindFilter
	}
	if *serviceFilter != "" {
		filters["service"] = *serviceFilter
	}
	if *levelFilter != "" {
		filters["level"] = *levelFilter
	}

	conn, err := client.WatchStream(ctx, cfg.CollectorURL, filters)
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}
	defer conn.Close()

	fmt.Println(output.Bold("Watching events... (Ctrl+C to stop)"))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) == nil {
			level := fmt.Sprintf("%v", ev["level"])
			ts := fmt.Sprintf("%v", ev["timestamp"])
			svc := fmt.Sprintf("%v", ev["service"])
			evt := fmt.Sprintf("%v", ev["event"])
			switch level {
			case "error":
				fmt.Printf("%s %s %s %s\n", output.Dim(ts), output.Error(level), output.Bold(svc), evt)
			case "warn":
				fmt.Printf("%s %s %s %s\n", output.Dim(ts), output.Warning(level), output.Bold(svc), evt)
			default:
				fmt.Printf("%s %s %s %s\n", output.Dim(ts), output.Info(level), output.Bold(svc), evt)
			}
		} else {
			fmt.Println(line)
		}
	}
	return scanner.Err()
}
