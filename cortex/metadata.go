// Package loxacortex provides build-time metadata for the loxa-cortex module.
package loxacortex

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed loxa-cortex.yaml
var metadataYAML []byte

// Version is the module version read from loxa-cortex.yaml at build time.
var Version = loadVersion()

func loadVersion() string {
	var m struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(metadataYAML, &m); err != nil || m.Version == "" {
		return "dev"
	}
	return m.Version
}
