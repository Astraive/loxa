package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsThenUserConfig(t *testing.T) {
	dir := t.TempDir()
	defaultsPath := filepath.Join(dir, "loza-cli.defaults.yaml")
	userPath := filepath.Join(dir, ".loza-cli.yaml")

	if err := os.WriteFile(defaultsPath, []byte(""+
		"collector_repo_path: defaults-collector\n"+
		"spec_repo_path: defaults-spec\n"+
		"collector_url: http://defaults:9308\n"+
		"duckdb_path: defaults.db\n"+
		"spool_dir: defaults-spool\n"+
		"spool_file: defaults.ndjson\n"+
		"dlq_path: defaults-dlq.ndjson\n"), 0o600); err != nil {
		t.Fatalf("write defaults: %v", err)
	}
	if err := os.WriteFile(userPath, []byte("collector_url: http://user:9308\n"), 0o600); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	t.Setenv("LOZA_CLI_DEFAULTS", defaultsPath)
	t.Setenv("LOZA_CLI_CONFIG", userPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CollectorURL != "http://user:9308" {
		t.Fatalf("collector url = %q, want user override", cfg.CollectorURL)
	}
	if cfg.DuckDBPath != "defaults.db" {
		t.Fatalf("duckdb path = %q, want defaults fallback", cfg.DuckDBPath)
	}
}
