package commands

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
var validEventID = regexp.MustCompile(`^[a-zA-Z0-9_:-]+$`)

// isValidIdentifier checks if a string is a safe SQL identifier (letters, digits, underscores; must start with letter or underscore).
func isValidIdentifier(s string) bool {
	return validIdentifier.MatchString(s)
}

// isValidEventID checks if a string is a safe event ID (letters, digits, underscores, hyphens, colons).
func isValidEventID(s string) bool {
	return validEventID.MatchString(s)
}


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
