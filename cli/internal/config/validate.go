package config

import (
	"fmt"
	"strings"
)

func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.CollectorRepoPath) == "" {
		return fmt.Errorf("collector_repo_path must not be empty")
	}
	if strings.TrimSpace(cfg.SpecRepoPath) == "" {
		return fmt.Errorf("spec_repo_path must not be empty")
	}
	if strings.TrimSpace(cfg.CollectorURL) == "" {
		return fmt.Errorf("collector_url must not be empty")
	}
	if cfg.Cortex != nil && strings.TrimSpace(cfg.Cortex.URL) == "" && strings.TrimSpace(cfg.CortexRepoPath) == "" {
		return fmt.Errorf("cortex.url or cortex_repo_path must be configured when cortex is enabled")
	}
	return nil
}
