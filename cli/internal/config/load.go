package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CollectorRepoPath string        `yaml:"collector_repo_path"`
	CortexRepoPath    string        `yaml:"cortex_repo_path"`
	SpecRepoPath      string        `yaml:"spec_repo_path"`
	CollectorURL      string        `yaml:"collector_url"`
	DuckDBPath        string        `yaml:"duckdb_path"`
	SpoolDir          string        `yaml:"spool_dir"`
	SpoolFile         string        `yaml:"spool_file"`
	DLQPath           string        `yaml:"dlq_path"`
	ValidationMode    string        `yaml:"validation_mode"`
	Cortex            *CortexConfig `yaml:"cortex,omitempty"`
}

type CortexConfig struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

func Default() Config {
	cfg, err := loadDefaults()
	if err != nil {
		panic(err)
	}
	return cfg
}

func Load() (Config, error) {
	cfg, err := loadDefaults()
	if err != nil {
		return Config{}, err
	}
	configPath := os.Getenv("LOXA_CLI_CONFIG")
	if configPath == "" {
		configPath = ".loxa-cli.yaml"
	}
	if _, err := os.Stat(configPath); err == nil {
		raw, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return Config{}, readErr
		}
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return Config{}, err
		}
	}
	cfg.CollectorRepoPath = filepath.Clean(cfg.CollectorRepoPath)
	if strings.TrimSpace(cfg.CortexRepoPath) != "" {
		cfg.CortexRepoPath = filepath.Clean(cfg.CortexRepoPath)
	}
	cfg.SpecRepoPath = filepath.Clean(cfg.SpecRepoPath)
	return cfg, Validate(cfg)
}

func loadDefaults() (Config, error) {
	path := findDefaultsFile()
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func findDefaultsFile() string {
	if override := strings.TrimSpace(os.Getenv("LOXA_CLI_DEFAULTS")); override != "" {
		return override
	}

	candidates := []string{
		"loxa-cli.defaults.yaml",
		filepath.Join("..", "loxa-cli.defaults.yaml"),
		filepath.Join("..", "..", "loxa-cli.defaults.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 4; i++ {
			candidate := filepath.Join(dir, "loxa-cli.defaults.yaml")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			dir = filepath.Dir(dir)
		}
	}
	return "loxa-cli.defaults.yaml"
}
