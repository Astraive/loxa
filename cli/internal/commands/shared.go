package commands

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/astraive/loxa-cli/internal/client"
	"github.com/astraive/loxa-cli/internal/config"
)

func newCancelableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigCh)
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		cancel()
		signal.Stop(sigCh)
	}
}

func tailFile(ctx context.Context, path string) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" {
					fmt.Println(line)
				}
			}
			_ = f.Close()
		}
	}
}

func collectorConfigPath(cfg config.Config, name string) string {
	return filepath.Join(cfg.CollectorRepoPath, "configs", name)
}

func runCollectorConfigCommand(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected collector config subcommand")
	}
	cmdArgs := append([]string{"config"}, args...)
	return client.RunCollectorCommand(ctx, cfg.CollectorRepoPath, cmdArgs)
}

func parseCommonOutput(fs *flag.FlagSet, path *string, defaultValue, usage string) {
	fs.StringVar(path, "o", defaultValue, usage)
}

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
