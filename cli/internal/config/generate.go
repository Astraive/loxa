package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Generate(path string, cfg Config) error {
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}
