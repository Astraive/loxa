package dsn

import (
	"fmt"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		valid     bool
		username  string
		password  string
		host      string
		port      int
		project   string
		env       string
		service   string
		tls       bool
		transport string
		baseURL   string
		eventsURL string
		batchURL  string
		otlpURL   string
		tailWSURL string
	}{
		// ── Valid cases ───────────────────────────────────────────────────────
		{
			name:      "localhost dev with explicit port and tls=false",
			input:     "loza://localhost:9308/demo?tls=false",
			valid:     true,
			host:      "localhost",
			port:      9308,
			project:   "demo",
			env:       "default",
			tls:       false,
			transport: "http",
			baseURL:   "http://localhost:9308",
			eventsURL: "http://localhost:9308/events",
			batchURL:  "http://localhost:9308/events/batch",
			otlpURL:   "http://localhost:9308/otlp/logs",
			tailWSURL: "ws://localhost:9308/tail",
		},
		{
			name:      "localhost default port 9308",
			input:     "loza://localhost/demo?tls=false",
			valid:     true,
			host:      "localhost",
			port:      9308,
			project:   "demo",
			tls:       false,
			transport: "http",
			baseURL:   "http://localhost:9308",
			eventsURL: "http://localhost:9308/events",
			tailWSURL: "ws://localhost:9308/tail",
		},
		{
			name:      "prod default tls=true",
			input:     "loza://collector.example.com/demo",
			valid:     true,
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			env:       "default",
			tls:       true,
			transport: "http",
			baseURL:   "https://collector.example.com:443",
			eventsURL: "https://collector.example.com:443/events",
			batchURL:  "https://collector.example.com:443/events/batch",
			otlpURL:   "https://collector.example.com:443/otlp/logs",
			tailWSURL: "wss://collector.example.com:443/tail",
		},
		{
			name:      "custom env and service",
			input:     "loza://collector.example.com/demo?env=prod&service=api",
			valid:     true,
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			env:       "prod",
			service:   "api",
			tls:       true,
			transport: "http",
			baseURL:   "https://collector.example.com:443",
		},
		{
			name:      "otlp transport",
			input:     "loza://collector.example.com/demo?transport=otlp",
			valid:     true,
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			tls:       true,
			transport: "otlp",
			baseURL:   "https://collector.example.com:443",
		},
		{
			name:      "grpc transport",
			input:     "loza://collector.example.com/demo?transport=grpc",
			valid:     true,
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			tls:       true,
			transport: "grpc",
			baseURL:   "https://collector.example.com:443",
			eventsURL: "https://collector.example.com:443/events",
			batchURL:  "https://collector.example.com:443/events/batch",
			otlpURL:   "https://collector.example.com:443/otlp/logs",
			tailWSURL: "wss://collector.example.com:443/tail",
		},
		{
			name:    "127.0.0.1 defaults to tls=false and port 9308",
			input:   "loza://127.0.0.1/demo",
			valid:   true,
			host:    "127.0.0.1",
			port:    9308,
			project: "demo",
			tls:     false,
			baseURL: "http://127.0.0.1:9308",
		},
		{
			name:    "::1 defaults to tls=false and port 9308 (brackets required)",
			input:   "loza://[::1]/demo",
			valid:   true,
			host:    "::1",
			port:    9308,
			project: "demo",
			tls:     false,
			baseURL: "http://[::1]:9308",
		},
		{
			name:    "tls=auto keeps localhost default",
			input:   "loza://localhost/demo?tls=auto",
			valid:   true,
			host:    "localhost",
			port:    9308,
			project: "demo",
			tls:     false,
			baseURL: "http://localhost:9308",
		},
		{
			name:    "tls=auto keeps remote default",
			input:   "loza://collector.example.com/demo?tls=auto",
			valid:   true,
			host:    "collector.example.com",
			port:    443,
			project: "demo",
			tls:     true,
			baseURL: "https://collector.example.com:443",
		},
		{
			name:    "explicit tls=true on localhost",
			input:   "loza://localhost:8443/demo?tls=true",
			valid:   true,
			host:    "localhost",
			port:    8443,
			project: "demo",
			tls:     true,
			baseURL: "https://localhost:8443",
		},
		{
			name:      "explicit port 4318 with otlp",
			input:     "loza://collector.example.com:4318/backend?env=staging&service=auth&transport=otlp",
			valid:     true,
			host:      "collector.example.com",
			port:      4318,
			project:   "backend",
			env:       "staging",
			service:   "auth",
			tls:       true,
			transport: "otlp",
			baseURL:   "https://collector.example.com:4318",
		},
		{
			name:      "credentialed DSN with percent-encoded password",
			input:     "loza://key-id:s%40cret%3Avalue%2Fpart@collector.example.com/demo",
			valid:     true,
			username:  "key-id",
			password:  "s@cret:value/part",
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			env:       "default",
			tls:       true,
			transport: "http",
			baseURL:   "https://collector.example.com:443",
		},
		{
			name:      "credentialed DSN with percent-encoded username",
			input:     "loza://key%2Did:secret@collector.example.com/demo?env=prod",
			valid:     true,
			username:  "key-id",
			password:  "secret",
			host:      "collector.example.com",
			port:      443,
			project:   "demo",
			env:       "prod",
			tls:       true,
			transport: "http",
			baseURL:   "https://collector.example.com:443",
		},

		// ── Invalid credential cases ─────────────────────────────────────────
		{
			name:  "reject empty username",
			input: "loza://:secret@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject empty password",
			input: "loza://key:@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject userinfo without password",
			input: "loza://key@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject username containing encoded colon",
			input: "loza://key%3Aid:secret@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject username containing encoded whitespace",
			input: "loza://key%20id:secret@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject unencoded reserved password character",
			input: "loza://key:secret:part@collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject malformed percent escape in credentials",
			input: "loza://key:secret%ZZ@collector.example.com/demo",
			valid: false,
		},

		// ── Invalid cases ────────────────────────────────────────────────────

		{
			name:  "reject empty string",
			input: "",
			valid: false,
		},
		{
			name:  "reject wrong scheme https",
			input: "https://collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject wrong scheme http",
			input: "http://collector.example.com/demo",
			valid: false,
		},
		{
			name:  "reject no host",
			input: "loza://",
			valid: false,
		},
		{
			name:  "reject triple-slash no host",
			input: "loza:///project",
			valid: false,
		},
		{
			name:  "reject no project",
			input: "loza://collector.example.com",
			valid: false,
		},
		{
			name:  "reject empty project",
			input: "loza://collector.example.com/",
			valid: false,
		},
		{
			name:  "reject invalid tls value",
			input: "loza://collector.example.com/demo?tls=maybe",
			valid: false,
		},
		{
			name:  "reject invalid transport value",
			input: "loza://collector.example.com/demo?transport=random",
			valid: false,
		},
		{
			name:  "reject port 0",
			input: "loza://collector.example.com:0/demo",
			valid: false,
		},
		{
			name:  "reject port above 65535",
			input: "loza://collector.example.com:99999/demo",
			valid: false,
		},
		{
			name:  "reject non-numeric port",
			input: "loza://collector.example.com:abc/demo",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, err := Parse(tt.input)

			if !tt.valid {
				if err == nil {
					t.Fatalf("expected error for %q, got valid DSN: %+v", tt.input, dsn)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected valid DSN for %q, got error: %v", tt.input, err)
			}

			if dsn.Scheme != "loza" {
				t.Errorf("Scheme = %q, want %q", dsn.Scheme, "loza")
			}
			if dsn.Host != tt.host {
				t.Errorf("Host = %q, want %q", dsn.Host, tt.host)
			}
			if dsn.Port != tt.port {
				t.Errorf("Port = %d, want %d", dsn.Port, tt.port)
			}
			if dsn.Project != tt.project {
				t.Errorf("Project = %q, want %q", dsn.Project, tt.project)
			}
			if dsn.Username != tt.username {
				t.Errorf("Username = %q, want %q", dsn.Username, tt.username)
			}
			if dsn.Password != tt.password {
				t.Errorf("Password = %q, want %q", dsn.Password, tt.password)
			}
			if dsn.TLS != tt.tls {
				t.Errorf("TLS = %v, want %v", dsn.TLS, tt.tls)
			}
			if tt.transport != "" && dsn.Transport != tt.transport {
				t.Errorf("Transport = %q, want %q", dsn.Transport, tt.transport)
			}
			if dsn.BaseURL != tt.baseURL {
				t.Errorf("BaseURL = %q, want %q", dsn.BaseURL, tt.baseURL)
			}
			if tt.eventsURL != "" && dsn.EventsURL != tt.eventsURL {
				t.Errorf("EventsURL = %q, want %q", dsn.EventsURL, tt.eventsURL)
			}
			if tt.batchURL != "" && dsn.BatchURL != tt.batchURL {
				t.Errorf("BatchURL = %q, want %q", dsn.BatchURL, tt.batchURL)
			}
			if tt.otlpURL != "" && dsn.OTLPURL != tt.otlpURL {
				t.Errorf("OTLPURL = %q, want %q", dsn.OTLPURL, tt.otlpURL)
			}
			if tt.tailWSURL != "" && dsn.TailWSURL != tt.tailWSURL {
				t.Errorf("TailWSURL = %q, want %q", dsn.TailWSURL, tt.tailWSURL)
			}

			// Env defaults to "default" when not specified
			if tt.env != "" && dsn.Env != tt.env {
				t.Errorf("Env = %q, want %q", dsn.Env, tt.env)
			}
			if tt.service != "" && dsn.Service != tt.service {
				t.Errorf("Service = %q, want %q", dsn.Service, tt.service)
			}
		})
	}
}

func TestParseCredentialErrorsDoNotEchoPassword(t *testing.T) {
	const password = "super-secret"
	_, err := Parse("loza://key:" + password + ":part@collector.example.com/demo")
	if err == nil {
		t.Fatal("expected an error for an unencoded reserved password character")
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("credential error echoed password: %q", err)
	}
}

func TestParseStringRedactsCredentials(t *testing.T) {
	dsn, err := Parse("loza://key-id:s%40cret%3Avalue@collector.example.com/demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		rendered := fmt.Sprintf(format, dsn)
		if strings.Contains(rendered, "s@cret:value") || strings.Contains(rendered, "key-id") {
			t.Fatalf("format %q exposed credentials: %s", format, rendered)
		}
	}
}

func TestParseEnvDefault(t *testing.T) {
	dsn, err := Parse("loza://localhost/demo?tls=false")
	if err != nil {
		t.Fatal(err)
	}
	if dsn.Env != "default" {
		t.Errorf("Env = %q, want %q", dsn.Env, "default")
	}
}

func TestParseSchemeAlwaysLoza(t *testing.T) {
	dsn, err := Parse("loza://localhost/demo?tls=false")
	if err != nil {
		t.Fatal(err)
	}
	if dsn.Scheme != "loza" {
		t.Errorf("Scheme = %q, want %q", dsn.Scheme, "loza")
	}
}
