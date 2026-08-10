package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/astraive/loza/cli/internal/client"
	"github.com/astraive/loza/cli/internal/config"
)

func DeployCommand(cfg config.Config, args []string) error {
	if len(args) == 0 {
		fmt.Println("Deploy starts the local collector process from the configured collector repo.")
		return client.RunCollectorCommand(context.Background(), cfg.CollectorRepoPath, []string{"run"})
	}

	switch args[0] {
	case "up":
		fmt.Println("Starting collector...")
		return client.RunCollectorCommand(context.Background(), cfg.CollectorRepoPath, []string{"run"})
	case "down":
		return fmt.Errorf("deploy down is not implemented; stop the local collector process directly")
	case "status":
		fmt.Println("Checking collector health...")
		return client.CheckHealth(cfg.CollectorURL)
	case "docker", "compose", "k8s", "helm":
		return deployAssets(cfg, args[0], args[1:])
	default:
		return fmt.Errorf("unknown deploy subcommand: %s", args[0])
	}
}

func deployAssets(cfg config.Config, kind string, args []string) error {
	fs := flag.NewFlagSet("deploy "+kind, flag.ContinueOnError)
	outDir := fs.String("out", filepath.Join(".", ".loza-deploy", kind), "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	deployRoot := filepath.Join(cfg.CollectorRepoPath, "deploy")
	var src string
	switch kind {
	case "docker":
		src = filepath.Join(deployRoot, "docker")
	case "compose":
		src = filepath.Join(deployRoot, "docker-compose.yml")
	case "k8s":
		src = filepath.Join(deployRoot, "loza-collector.yaml")
	case "helm":
		src = filepath.Join(deployRoot, "helm", "loza")
	default:
		return fmt.Errorf("unsupported deploy asset kind: %s", kind)
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("deploy assets not found at %s: %w", src, err)
	}
	if err := copyDeployAsset(src, *outDir); err != nil {
		return err
	}
	fmt.Printf("Deploy assets copied to: %s\n", filepath.Clean(*outDir))
	return nil
}

func copyDeployAsset(src, outDir string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		base := filepath.Base(src)
		dstRoot := filepath.Join(outDir, base)
		return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			dst := filepath.Join(dstRoot, rel)
			if info.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			return copyFile(path, dst)
		})
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	return copyFile(src, filepath.Join(outDir, filepath.Base(src)))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
