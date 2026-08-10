package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

func DashboardCommand(cfg config.Config, args []string) error {
	if len(args) == 0 || args[0] != "install" {
		return fmt.Errorf("expected 'install' subcommand")
	}

	// Create Grafana dashboard config from template
	dashboardPath := filepath.Join(".", ".loza-dashboard")
	if err := os.MkdirAll(dashboardPath, 0o755); err != nil {
		return fmt.Errorf("create dashboard dir: %w", err)
	}

	fmt.Println("Installing Grafana dashboard templates...")

	// Verify collector is accessible
	if err := client.CheckHealth(cfg.CollectorURL); err != nil {
		fmt.Printf("Warning: collector not reachable at %s: %v\n", cfg.CollectorURL, err)
	}

	collectorDashboard := filepath.Join(
		cfg.CollectorRepoPath,
		"deploy",
		"observability",
		"grafana",
		"provisioning",
		"dashboards",
		"loza-collector.json",
	)
	if _, err := os.Stat(collectorDashboard); err == nil {
		if err := copyDeployAsset(filepath.Dir(collectorDashboard), dashboardPath); err != nil {
			return fmt.Errorf("copy dashboard assets: %w", err)
		}
		fmt.Printf("Dashboard assets copied to: %s\n", dashboardPath)
		return nil
	}
	dashboardJSON := generateDashboardConfig(cfg)
	dashboardFile := filepath.Join(dashboardPath, "loza-dashboard.json")
	if err := os.WriteFile(dashboardFile, []byte(dashboardJSON), 0o644); err != nil {
		return fmt.Errorf("write dashboard config: %w", err)
	}
	fmt.Printf("Dashboard config written to: %s\n", dashboardFile)
	return nil
}

func generateDashboardConfig(cfg config.Config) string {
	return `{
  "dashboard": {
    "title": "LOZA Collector",
    "tags": ["loza", "collector"],
    "timezone": "browser",
    "panels": [
      {
        "title": "Requests Total",
        "targets": [{"expr": "loza_collector_requests_total"}]
      },
      {
        "title": "Events Accepted",
        "targets": [{"expr": "loza_collector_events_accepted_total"}]
      },
      {
        "title": "Sink Health",
        "targets": [{"expr": "loza_collector_sink_health"}]
      }
    ]
  }
}`
}
