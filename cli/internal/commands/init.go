package commands

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/astraive/loxa-cli/internal/config"
)

func InitCommand(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	valMode := fs.String("validation-mode", "", "validation mode: off, warn, enforce, quarantine")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *valMode != "" {
		cfg.ValidationMode = strings.ToLower(strings.TrimSpace(*valMode))
	}
	target := ".loxa-cli.yaml"
	if fs.NArg() > 0 && fs.Arg(0) != "" {
		target = fs.Arg(0)
	}
	if err := config.Generate(target, cfg); err != nil {
		return err
	}
	fmt.Printf("initialized %s\n", filepath.Clean(target))
	return nil
}