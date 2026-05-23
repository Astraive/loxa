package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
)


func runCollectorConfigCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected collector config subcommand")
	}
	cmdArgs := append([]string{"config"}, args...)
	return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, cmdArgs)
}


func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
