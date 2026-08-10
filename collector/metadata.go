// Package lozacollector provides build-time metadata for the loza-collector module.
package lozacollector

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed loza.yaml
var metadataYAML []byte

// Version is the module version read from loza.yaml at build time.
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
