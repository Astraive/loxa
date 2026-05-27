package core

import (
	"os"
	"testing"
)

func TestWithDSN(t *testing.T) {
	tests := []struct {
		name          string
		dsn           string
		wantURL       string
		wantEnv       string
		wantService   string
		wantInsecure  bool
		wantParseFail bool
	}{
		{
			name:         "localhost dev",
			dsn:          "loxa://localhost:8080/demo?env=dev&tls=false",
			wantURL:      "http://localhost:8080",
			wantEnv:      "dev",
			wantInsecure: true,
		},
		{
			name:         "prod default tls",
			dsn:          "loxa://collector.example.com/demo?env=prod",
			wantURL:      "https://collector.example.com:443",
			wantEnv:      "prod",
			wantInsecure: false,
		},
		{
			name:         "with service",
			dsn:          "loxa://collector.example.com/demo?env=staging&service=auth",
			wantURL:      "https://collector.example.com:443",
			wantEnv:      "staging",
			wantService:  "auth",
			wantInsecure: false,
		},
		{
			name:          "invalid DSN",
			dsn:           "https://collector.example.com/demo",
			wantParseFail: true,
		},
		{
			name:          "missing project",
			dsn:           "loxa://collector.example.com",
			wantParseFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ApplyConfig(Config{}, WithDSN(tt.dsn))

			if tt.wantParseFail {
				if cfg.CollectorURL != "" {
					t.Fatalf("expected empty CollectorURL for invalid DSN, got %q", cfg.CollectorURL)
				}
				return
			}

			if cfg.CollectorURL != tt.wantURL {
				t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, tt.wantURL)
			}
			if cfg.Environment != tt.wantEnv {
				t.Errorf("Environment = %q, want %q", cfg.Environment, tt.wantEnv)
			}
			if tt.wantService != "" && cfg.Service != tt.wantService {
				t.Errorf("Service = %q, want %q", cfg.Service, tt.wantService)
			}
			if cfg.Insecure != tt.wantInsecure {
				t.Errorf("Insecure = %v, want %v", cfg.Insecure, tt.wantInsecure)
			}
		})
	}
}

func TestLoadFromEnv_DSN(t *testing.T) {
	// Save and restore env vars
	saved := map[string]string{
		"LOXA_DSN":           os.Getenv("LOXA_DSN"),
		"LOXA_COLLECTOR_URL": os.Getenv("LOXA_COLLECTOR_URL"),
		"LOXA_ENVIRONMENT":   os.Getenv("LOXA_ENVIRONMENT"),
		"LOXA_SERVICE_NAME":  os.Getenv("LOXA_SERVICE_NAME"),
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	t.Run("DSN sets CollectorURL and Environment", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://collector.example.com/myapp?env=staging")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_ENVIRONMENT")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.CollectorURL != "https://collector.example.com:443" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "https://collector.example.com:443")
		}
		if cfg.Environment != "staging" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "staging")
		}
	})

	t.Run("LOXA_COLLECTOR_URL overrides DSN", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://collector.example.com/myapp?env=staging")
		os.Setenv("LOXA_COLLECTOR_URL", "http://override:9090")
		os.Unsetenv("LOXA_ENVIRONMENT")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.CollectorURL != "http://override:9090" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "http://override:9090")
		}
		// Environment from DSN should still be present
		if cfg.Environment != "staging" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "staging")
		}
	})

	t.Run("LOXA_ENVIRONMENT overrides DSN env", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://collector.example.com/myapp?env=staging")
		os.Setenv("LOXA_ENVIRONMENT", "production")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.Environment != "production" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "production")
		}
	})

	t.Run("DSN with service sets Service", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://collector.example.com/myapp?env=prod&service=payments")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_ENVIRONMENT")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.Service != "payments" {
			t.Errorf("Service = %q, want %q", cfg.Service, "payments")
		}
	})

	t.Run("LOXA_SERVICE_NAME overrides DSN service", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://collector.example.com/myapp?env=prod&service=payments")
		os.Setenv("LOXA_SERVICE_NAME", "billing")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_ENVIRONMENT")

		cfg := LoadFromEnv(Config{})
		if cfg.Service != "billing" {
			t.Errorf("Service = %q, want %q", cfg.Service, "billing")
		}
	})

	t.Run("localhost DSN sets Insecure", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "loxa://localhost:8080/demo?tls=false")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_ENVIRONMENT")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if !cfg.Insecure {
			t.Errorf("Insecure = false, want true for localhost DSN")
		}
		if cfg.CollectorURL != "http://localhost:8080" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "http://localhost:8080")
		}
	})

	t.Run("invalid DSN is silently ignored", func(t *testing.T) {
		os.Setenv("LOXA_DSN", "https://not-a-loxa-dsn")
		os.Unsetenv("LOXA_COLLECTOR_URL")
		os.Unsetenv("LOXA_ENVIRONMENT")
		os.Unsetenv("LOXA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		// Invalid DSN should not set anything
		if cfg.CollectorURL != "" {
			t.Errorf("CollectorURL should be empty for invalid DSN, got %q", cfg.CollectorURL)
		}
	})
}
