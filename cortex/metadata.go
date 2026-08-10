// Package lozacortex provides build-time metadata for the loza-cortex module.
package lozacortex

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed loza-cortex.yaml
var metadataYAML []byte

// Version is the module version read from loza-cortex.yaml at build time.
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
