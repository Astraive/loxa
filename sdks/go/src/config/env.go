package config

import "github.com/astraive/loxa/sdks/go/src/core"

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv(cfg Config) Config { return core.LoadFromEnv(cfg) }
