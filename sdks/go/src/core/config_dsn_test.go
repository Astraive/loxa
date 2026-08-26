package core

import (
	"os"
	"strings"
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
		wantCollector string
		wantParseFail bool
	}{
		{
			name:          "localhost dev",
			dsn:           "loza://localhost:9308/demo?env=dev&tls=false",
			wantURL:       "http://localhost:9308",
			wantEnv:       "dev",
			wantInsecure:  true,
			wantCollector: "demo",
		},
		{
			name:          "prod default tls",
			dsn:           "loza://collector.example.com/demo?env=prod",
			wantURL:       "https://collector.example.com:443",
			wantEnv:       "prod",
			wantInsecure:  false,
			wantCollector: "demo",
		},
		{
			name:          "with service",
			dsn:           "loza://collector.example.com/demo?env=staging&service=auth",
			wantURL:       "https://collector.example.com:443",
			wantEnv:       "staging",
			wantService:   "auth",
			wantInsecure:  false,
			wantCollector: "demo",
		},
		{
			name:          "invalid DSN",
			dsn:           "https://collector.example.com/demo",
			wantParseFail: true,
		},
		{
			name:          "missing project",
			dsn:           "loza://collector.example.com",
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
			if cfg.CollectorName != tt.wantCollector {
				t.Errorf("CollectorName = %q, want %q", cfg.CollectorName, tt.wantCollector)
			}
		})
	}
}

func TestLoadFromEnv_DSN(t *testing.T) {
	// Save and restore env vars
	saved := map[string]string{
		"LOZA_DSN":           os.Getenv("LOZA_DSN"),
		"LOZA_COLLECTOR_URL": os.Getenv("LOZA_COLLECTOR_URL"),
		"LOZA_ENVIRONMENT":   os.Getenv("LOZA_ENVIRONMENT"),
		"LOZA_SERVICE_NAME":  os.Getenv("LOZA_SERVICE_NAME"),
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
		os.Setenv("LOZA_DSN", "loza://collector.example.com/myapp?env=staging")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_ENVIRONMENT")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.CollectorURL != "https://collector.example.com:443" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "https://collector.example.com:443")
		}
		if cfg.Environment != "staging" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "staging")
		}
	})

	t.Run("LOZA_COLLECTOR_URL overrides DSN", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "loza://collector.example.com/myapp?env=staging")
		os.Setenv("LOZA_COLLECTOR_URL", "http://override:9308")
		os.Unsetenv("LOZA_ENVIRONMENT")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.CollectorURL != "http://override:9308" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "http://override:9308")
		}
		// Environment from DSN should still be present
		if cfg.Environment != "staging" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "staging")
		}
	})

	t.Run("LOZA_ENVIRONMENT overrides DSN env", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "loza://collector.example.com/myapp?env=staging")
		os.Setenv("LOZA_ENVIRONMENT", "production")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.Environment != "production" {
			t.Errorf("Environment = %q, want %q", cfg.Environment, "production")
		}
	})

	t.Run("DSN with service sets Service", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "loza://collector.example.com/myapp?env=prod&service=payments")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_ENVIRONMENT")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if cfg.Service != "payments" {
			t.Errorf("Service = %q, want %q", cfg.Service, "payments")
		}
	})

	t.Run("LOZA_SERVICE_NAME overrides DSN service", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "loza://collector.example.com/myapp?env=prod&service=payments")
		os.Setenv("LOZA_SERVICE_NAME", "billing")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_ENVIRONMENT")

		cfg := LoadFromEnv(Config{})
		if cfg.Service != "billing" {
			t.Errorf("Service = %q, want %q", cfg.Service, "billing")
		}
	})

	t.Run("localhost DSN sets Insecure", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "loza://localhost:9308/demo?tls=false")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_ENVIRONMENT")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		if !cfg.Insecure {
			t.Errorf("Insecure = false, want true for localhost DSN")
		}
		if cfg.CollectorURL != "http://localhost:9308" {
			t.Errorf("CollectorURL = %q, want %q", cfg.CollectorURL, "http://localhost:9308")
		}
	})

	t.Run("invalid DSN is silently ignored", func(t *testing.T) {
		os.Setenv("LOZA_DSN", "https://not-a-loza-dsn")
		os.Unsetenv("LOZA_COLLECTOR_URL")
		os.Unsetenv("LOZA_ENVIRONMENT")
		os.Unsetenv("LOZA_SERVICE_NAME")

		cfg := LoadFromEnv(Config{})
		// Invalid DSN should not set anything
		if cfg.CollectorURL != "" {
			t.Errorf("CollectorURL should be empty for invalid DSN, got %q", cfg.CollectorURL)
		}
	})
}

func TestCredentialedDSNMappingAndPrecedence(t *testing.T) {
	t.Setenv("LOZA_DSN", "loza://dsn-user:dsn%2Fsecret@collector.example.com/project?env=prod")
	t.Setenv("LOZA_API_KEY", "")

	cfg := LoadFromEnv(Config{})
	if cfg.DSNUsername != "dsn-user" || cfg.DSNPassword != "dsn/secret" {
		t.Fatalf("DSN credentials = %q/%q, want decoded userinfo", cfg.DSNUsername, cfg.DSNPassword)
	}
	if strings.Contains(cfg.CollectorURL, "dsn-user") || strings.Contains(cfg.CollectorURL, "secret") {
		t.Fatalf("CollectorURL contains DSN credentials: %q", cfg.CollectorURL)
	}

	code := ApplyConfig(cfg, WithDSN("loza://code-user:code-secret@collector.example.com/project?env=prod"), WithAPIKey("api-key"))
	if code.DSNUsername != "code-user" || code.DSNPassword != "code-secret" {
		t.Fatalf("code DSN credentials = %q/%q", code.DSNUsername, code.DSNPassword)
	}
	if code.APIKey != "api-key" {
		t.Fatalf("APIKey = %q, want explicit code API key", code.APIKey)
	}
}

func TestPublicDSNUsesScopedEndpointAndEmptyBasicPassword(t *testing.T) {
	const capability = "lz_pub_6DJvd3D0izOaQx3n5BhKqN"
	cfg := ApplyConfig(
		Config{},
		WithDSN("loza://"+capability+":@collector.example.com/public-collector?env=prod"),
		WithAPIKey("api-key"),
	)

	if cfg.CollectorName != "public-collector" {
		t.Fatalf("CollectorName = %q, want public-collector", cfg.CollectorName)
	}
	if cfg.DSNUsername != capability || cfg.DSNPassword != "" {
		t.Fatalf("DSN credentials = %q/%q, want public capability and empty password", cfg.DSNUsername, cfg.DSNPassword)
	}
	if strings.Contains(cfg.CollectorURL, capability) {
		t.Fatalf("CollectorURL leaked public capability: %q", cfg.CollectorURL)
	}
	if cfg.APIKey != "api-key" {
		t.Fatalf("API key precedence was not retained")
	}
}

func TestCredentialedDSNRejectsRemotePlaintext(t *testing.T) {
	cfg := ApplyConfig(
		Config{},
		WithDSN("loza://dsn-user:dsn-secret@collector.example.com/project?tls=false"),
	)
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected remote plaintext Basic-auth DSN to be rejected")
	}
}
