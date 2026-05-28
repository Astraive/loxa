// Package loxacollector provides build-time metadata for the loxa-collector module.
package loxacollector

import (
	_ "embed"
	"gopkg.in/yaml.v3"
)

//go:embed loxa.yaml
var metadataYAML []byte

// Version is the module version read from loxa.yaml at build time.
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
