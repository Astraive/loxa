package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
	_ "github.com/marcboeker/go-duckdb"
)

func DoctorCommand(ctx context.Context, cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	checkMetrics := fs.Bool("metrics", true, "check metrics endpoint")
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Println("Running LOXA health checks...")
	fmt.Println()

	checks := []struct {
		name string
		fn   func() error
	}{
		{"collector repo", func() error {
			_, err := os.Stat(cfg.CollectorRepoPath)
			return err
		}},
		{"spec repo", func() error {
			_, err := os.Stat(cfg.SpecRepoPath)
			return err
		}},
		{"collector health", func() error {
			return client.CheckHealth(cfg.CollectorURL)
		}},
		{"collector ready", func() error {
			return client.CheckReady(cfg.CollectorURL)
		}},
		{"collector status", func() error {
			_, err := client.FetchStatus(cfg.CollectorURL)
			return err
		}},
		{"test event accepted", func() error {
			event := map[string]any{
				"schema_version": speccontract.LOXASpecVersion,
				"event_version":  speccontract.LOXAEventVersion,
				"event_id":       fmt.Sprintf("evt_doctor_%d", time.Now().UnixNano()),
				"timestamp":      time.Now().UTC().Format(time.RFC3339Nano),
				"service":        "loxa-doctor",
				"event":          "doctor.check",
				"kind":           "cli",
				"level":          "info",
				"event_state":    "finished",
			}
			rawEvent, err := json.Marshal(event)
			if err != nil {
				return err
			}
			raw, err := speccontract.MarshalIngestEnvelope("loxa-cli", "0.2.0", "loxa-doctor", []json.RawMessage{rawEvent})
			if err != nil {
				return err
			}
			return client.PostIngest(cfg.CollectorURL, "application/json", raw)
		}},
		{"duckdb", func() error {
			db, err := sql.Open("duckdb", cfg.DuckDBPath)
			if err != nil {
				return err
			}
			defer db.Close()
			return db.Ping()
		}},
	}

	if cfg.Cortex != nil || cfg.CortexRepoPath != "" {
		checks = append(checks,
			struct {
				name string
				fn   func() error
			}{
				name: "cortex repo",
				fn: func() error {
					if cfg.CortexRepoPath == "" {
						return nil
					}
					_, err := os.Stat(cfg.CortexRepoPath)
					return err
				},
			},
			struct {
				name string
				fn   func() error
			}{
				name: "cortex health",
				fn: func() error {
					return client.CheckCortexHealth(getCortexURL(cfg))
				},
			},
			struct {
				name string
				fn   func() error
			}{
				name: "cortex ready",
				fn: func() error {
					return client.CheckCortexReady(getCortexURL(cfg))
				},
			},
		)
	}

	if *checkMetrics {
		checks = append(checks, struct {
			name string
			fn   func() error
		}{
			name: "metrics",
			fn: func() error {
				_, err := client.FetchMetrics(cfg.CollectorURL)
				return err
			},
		})
		if cfg.Cortex != nil || cfg.CortexRepoPath != "" {
			checks = append(checks, struct {
				name string
				fn   func() error
			}{
				name: "cortex metrics",
				fn: func() error {
					_, err := client.FetchCortexMetrics(getCortexURL(cfg))
					return err
				},
			})
		}
	}

	allPassed := true
	for _, check := range checks {
		if err := check.fn(); err != nil {
			fmt.Printf("  FAIL  %s: %v\n", check.name, err)
			allPassed = false
			continue
		}
		fmt.Printf("  OK    %s\n", check.name)
	}

	fmt.Println()
	if allPassed {
		fmt.Println("All checks passed.")
		return nil
	}
	return fmt.Errorf("some checks failed")
}
