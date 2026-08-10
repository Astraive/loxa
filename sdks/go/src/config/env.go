package config

import "github.com/astraive/loza/sdks/go/src/core"

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv(cfg Config) Config { return core.LoadFromEnv(cfg) }
