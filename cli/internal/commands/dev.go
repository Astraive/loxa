package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/astraive/loxa/cli/internal/config"
)

func DevCommand(ctx context.Context, cfg config.Config, _ []string) error {
	collectorBin := filepath.Join(cfg.CollectorRepoPath, "cmd", "loxa-collector", "main.go")
	if _, err := os.Stat(collectorBin); err != nil {
		return fmt.Errorf("collector not found at %s", cfg.CollectorRepoPath)
	}

	collectorCmd := exec.CommandContext(ctx, "go", "run", collectorBin, "-c", configPath(cfg, "loxa.local.yaml"))
	collectorCmd.Stdout = os.Stdout
	collectorCmd.Stderr = os.Stderr
	collectorCmd.Dir = cfg.CollectorRepoPath

	if strings.TrimSpace(cfg.CortexRepoPath) == "" {
		fmt.Println("Starting collector...")
		return collectorCmd.Run()
	}

	cortexMain := filepath.Join(cfg.CortexRepoPath, "cmd", "server", "main.go")
	if _, err := os.Stat(cortexMain); err != nil {
		fmt.Println("Starting collector...")
		return collectorCmd.Run()
	}

	fmt.Println("Starting collector...")
	if err := collectorCmd.Start(); err != nil {
		return fmt.Errorf("start collector: %w", err)
	}
	defer func() {
		if collectorCmd.Process != nil {
			_ = collectorCmd.Process.Kill()
			_, _ = collectorCmd.Process.Wait()
		}
	}()

	// Wait for collector to be ready before starting cortex
	collectorURL := "http://localhost:9308/health"
	fmt.Println("Waiting for collector to be ready...")
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get(collectorURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if i == 29 {
			return fmt.Errorf("collector did not become ready within 30s")
		}
	}

	fmt.Println("Starting cortex...")
	cortexCmd := exec.CommandContext(ctx, "go", "run", cortexMain, "--config", filepath.Join(cfg.CortexRepoPath, "configs", "loxa-cortex.defaults.yaml"))
	cortexCmd.Stdout = os.Stdout
	cortexCmd.Stderr = os.Stderr
	cortexCmd.Dir = cfg.CortexRepoPath
	if err := cortexCmd.Run(); err != nil {
		return fmt.Errorf("run cortex: %w", err)
	}
	return collectorCmd.Wait()
}

func configPath(cfg config.Config, name string) string {
	return filepath.Join(cfg.CollectorRepoPath, "configs", name)
}
