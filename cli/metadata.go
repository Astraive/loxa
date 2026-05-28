// Package loxacli provides build-time metadata for the loxa-cli module.
package loxacli

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed loxa-cli.yaml
var metadataYAML []byte

// Version is the module version read from loxa-cli.yaml at build time.
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
