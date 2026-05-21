package commands

import (
	"fmt"
	"path/filepath"

	"github.com/astraive/loxa-cli/internal/config"
)

func InitCommand(cfg config.Config, args []string) error {
	target := ".loxa-cli.yaml"
	if len(args) > 0 && args[0] != "" {
		target = args[0]
	}
	if err := config.Generate(target, cfg); err != nil {
		return err
	}
	fmt.Printf("initialized %s\n", filepath.Clean(target))
	return nil
}