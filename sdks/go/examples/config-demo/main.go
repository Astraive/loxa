package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/astraive/loxa/sdks/go"
)

func main() {
	fmt.Println("=== LOXA SDK Configuration Demo ===")
	fmt.Println()

	// Example 1: Configuration via code (highest precedence)
	fmt.Println("1. Configuration via code:")
	cfg1 := loxa.Dev()
	cfg1 = loxa.ApplyConfig(cfg1,
		loxa.WithService("my-service"),
		loxa.WithVersion("0.2.3"),
		loxa.WithEnvironment("production"),
		loxa.WithCollectorURL("http://localhost:8080"),
		loxa.WithTenantID("tenant-123"),
		loxa.WithBatchSize(200),
		loxa.WithFlushInterval(10*time.Second),
		loxa.WithMaxRetries(5),
		loxa.WithTimeout(30*time.Second),
	)
	fmt.Printf("  Service: %s\n", cfg1.Service)
	fmt.Printf("  Version: %s\n", cfg1.Version)
	fmt.Printf("  Environment: %s\n", cfg1.Environment)
	fmt.Printf("  CollectorURL: %s\n", cfg1.CollectorURL)
	fmt.Printf("  TenantID: %s\n", cfg1.TenantID)
	fmt.Printf("  BatchSize: %d\n", cfg1.BatchSize)
	fmt.Printf("  FlushInterval: %s\n", cfg1.FlushInterval)
	fmt.Printf("  MaxRetries: %d\n", cfg1.MaxRetries)
	fmt.Printf("  Timeout: %s\n\n", cfg1.Timeout)

	// Example 2: Configuration via environment variables
	fmt.Println("2. Configuration via environment variables:")
	os.Setenv("LOXA_SERVICE_NAME", "env-service")
	os.Setenv("LOXA_SERVICE_VERSION", "2.0.0")
	os.Setenv("LOXA_ENVIRONMENT", "staging")
	os.Setenv("LOXA_COLLECTOR_URL", "http://env-collector:8080")
	os.Setenv("LOXA_TENANT_ID", "tenant-env")
	os.Setenv("LOXA_FLUSH_INTERVAL", "15s")
	
	cfg2 := loxa.Dev()
	cfg2 = loxa.LoadFromEnv(cfg2)
	fmt.Printf("  Service: %s\n", cfg2.Service)
	fmt.Printf("  Version: %s\n", cfg2.Version)
	fmt.Printf("  Environment: %s\n", cfg2.Environment)
	fmt.Printf("  CollectorURL: %s\n", cfg2.CollectorURL)
	fmt.Printf("  TenantID: %s\n", cfg2.TenantID)
	fmt.Printf("  FlushInterval: %s\n\n", cfg2.FlushInterval)

	// Example 3: Configuration precedence (code > env > defaults)
	fmt.Println("3. Configuration precedence (code > env > defaults):")
	cfg3 := loxa.Dev() // defaults
	cfg3 = loxa.LoadFromEnv(cfg3) // apply env vars
	cfg3 = loxa.ApplyConfig(cfg3, // apply code config (highest precedence)
		loxa.WithService("code-service"),
		loxa.WithCollectorURL("http://code-collector:8080"),
	)
	fmt.Printf("  Service: %s (from code, overrides env)\n", cfg3.Service)
	fmt.Printf("  Version: %s (from env)\n", cfg3.Version)
	fmt.Printf("  CollectorURL: %s (from code, overrides env)\n", cfg3.CollectorURL)
	fmt.Printf("  TenantID: %s (from env)\n", cfg3.TenantID)
	fmt.Printf("  FlushInterval: %s (from env)\n\n", cfg3.FlushInterval)

	// Example 4: Configuration validation
	fmt.Println("4. Configuration validation:")
	invalidCfg := loxa.Config{
		BatchSize: -1, // invalid
	}
	if err := invalidCfg.Validate(); err != nil {
		fmt.Printf("  ✓ Validation caught error: %v\n", err)
	}

	validCfg := loxa.Dev()
	if err := validCfg.Validate(); err != nil {
		fmt.Printf("  ✗ Unexpected validation error: %v\n", err)
	} else {
		fmt.Printf("  ✓ Valid configuration passed validation\n")
	}
	fmt.Println()

	// Example 5: Using the configured logger
	fmt.Println("5. Using the configured logger:")
	logger, err := loxa.New(cfg1)
	if err != nil {
		fmt.Printf("  Error creating logger: %v\n", err)
		return
	}
	defer logger.Shutdown(context.Background())

	logger.Info("Configuration demo completed successfully")
	fmt.Println("  ✓ Logger created and event emitted")
	fmt.Println()

	// Example 6: Flush and Shutdown
	fmt.Println("6. Flush and Shutdown:")
	if err := logger.Flush(context.Background()); err != nil {
		fmt.Printf("  Error flushing: %v\n", err)
	} else {
		fmt.Println("  ✓ Flush completed successfully")
	}

	if err := logger.Shutdown(context.Background()); err != nil {
		fmt.Printf("  Error shutting down: %v\n", err)
	} else {
		fmt.Println("  ✓ Shutdown completed successfully")
	}

	fmt.Println("\n=== Demo Complete ===")
}
