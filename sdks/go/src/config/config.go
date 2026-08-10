package config

import (
	"github.com/astraive/loza/sdks/go/src/core"
)

// Config holds all configuration for a Logger instance.
type Config = core.Config

// ConfigOption is a functional option for configuring a Logger.
type ConfigOption = core.ConfigOption

// SecurityConfig controls field-level security policies.
type SecurityConfig = core.SecurityConfig

// New creates a Logger from cfg, validating and applying defaults.
func New(cfg Config) (*core.Logger, error) { return core.New(cfg) }

// ApplyConfig applies a set of ConfigOptions to a Config.
func ApplyConfig(cfg Config, options ...ConfigOption) Config {
	return core.ApplyConfig(cfg, options...)
}
