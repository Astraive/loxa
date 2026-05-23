package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	"github.com/astraive/loxa-cli/internal/output"
)

func TailCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("tail", flag.ContinueOnError)
	kindFilter := fs.String("kind", "", "filter by event kind")
	serviceFilter := fs.String("service", "", "filter by service name")
	levelFilter := fs.String("level", "", "filter by level")
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
		return fmt.Errorf("tail stream failed: %w", err)
	}
	defer conn.Close()

	output.PrintSection("Tailing events... (Ctrl+C to stop)")
	streamLines(ctx, conn)
	return nil
}

func streamLines(ctx context.Context, body io.Reader) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
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
					fmt.Printf("%s %s %s %s", output.Dim(ts), output.Error(level), output.Bold(svc), evt)
				case "warn":
					fmt.Printf("%s %s %s %s", output.Dim(ts), output.Warning(level), output.Bold(svc), evt)
				default:
					fmt.Printf("%s %s %s %s", output.Dim(ts), output.Info(level), output.Bold(svc), evt)
				}
				if rel, ok := ev["release"]; ok {
					fmt.Printf(" release=%v", rel)
				}
				if tf, ok := ev["trace_flags"]; ok {
					fmt.Printf(" trace_flags=%v", tf)
				}
				if errs, ok := ev["errors"]; ok {
					if b, e := json.Marshal(errs); e == nil {
						fmt.Printf(" errors=%s", string(b))
					} else {
						fmt.Printf(" errors=%v", errs)
					}
				}
				fmt.Println()
			} else {
				fmt.Println(line)
			}
		}
	}
}
