package lqlclient

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestParseLozaDSNRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{"empty", ""},
		{"wrong scheme", "https://example.com/demo"},
		{"missing host", "loza:///demo"},
		{"missing collector", "loza://example.com"},
		{"duplicate collector slash", "loza://example.com//demo"},
		{"credentials need password", "loza://user@example.com/demo"},
		{"username colon", "loza://user%3Aname:secret@example.com/demo"},
		{"username whitespace", "loza://user%20name:secret@example.com/demo"},
		{"invalid tls", "loza://example.com/demo?tls=maybe"},
		{"invalid transport", "loza://example.com/demo?transport=tcp"},
		{"invalid port text", "loza://example.com:abc/demo"},
		{"port zero", "loza://example.com:0/demo"},
		{"port too large", "loza://example.com:65536/demo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseLozaDSN(tc.dsn); err == nil {
				t.Fatalf("parseLozaDSN(%q) unexpectedly succeeded", tc.dsn)
			}
		})
	}
}

func TestParseLozaDSNResolvesDefaultsAndOverrides(t *testing.T) {
	cases := []struct {
		name     string
		dsn      string
		baseURL  string
		username string
		password string
		env      string
		service  string
	}{
		{
			name: "secure default",
			dsn: "loza://user:secret@example.com/demo",
			baseURL: "https://example.com:443",
			username: "user", password: "secret", env: "default",
		},
		{
			name: "public key with empty password",
			dsn: "loza://lx_pub_key:@example.com/demo?tls=false&transport=grpc&env=prod&service=api",
			baseURL: "http://example.com:80",
			username: "lx_pub_key", env: "prod", service: "api",
		},
		{
			name: "localhost auto port",
			dsn: "loza://localhost/demo",
			baseURL: "http://localhost:9308",
			env: "default",
		},
		{
			name: "explicit tls port and IPv6",
			dsn: "loza://user:secret@[::1]:9443/demo?tls=true&transport=otlp",
			baseURL: "https://[::1]:9443",
			username: "user", password: "secret", env: "default",
		},
		{
			name: "localhost secure override",
			dsn: "loza://localhost:9443/demo?tls=true",
			baseURL: "https://localhost:9443",
			env: "default",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLozaDSN(tc.dsn)
			if err != nil {
				t.Fatal(err)
			}
			if got.BaseURL != tc.baseURL || got.Username != tc.username || got.Password != tc.password || got.Env != tc.env || got.Service != tc.service {
				t.Fatalf("parseLozaDSN(%q) = %#v", tc.dsn, got)
			}
		})
	}
}

func TestNewResolvesEnvironmentAndPrecedence(t *testing.T) {
	t.Setenv("LOZA_DSN", "loza://dsn-user:dsn-pass@example.com/dsn-collector?env=dsn-env&service=dsn-service")
	t.Setenv("LOZA_API_KEY", "environment-key")
	t.Setenv("LOZA_USERNAME", "environment-user")
	t.Setenv("LOZA_PASSWORD", "environment-pass")

	client, err := New(ConnectionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "https://example.com:443" || client.collector != "dsn-collector" || client.apiKey != "environment-key" || client.username != "dsn-user" || client.password != "dsn-pass" || client.env != "dsn-env" || client.service != "dsn-service" {
		t.Fatalf("environment config = %#v", client)
	}
	if client.timeout != 30*time.Second || client.maxResponseBytes != 8<<20 || client.httpClient == nil {
		t.Fatalf("defaults = %#v", client)
	}

	explicitClient, err := New(ConnectionConfig{
		DSN: "loza://dsn-user:dsn-pass@example.com/dsn-collector?env=dsn-env&service=dsn-service",
		Endpoint: "http://localhost:8080/",
		Collector: "explicit",
		APIKey: "explicit-key",
		Username: "explicit-user",
		Password: "explicit-pass",
		Env: "explicit-env",
		Service: "explicit-service",
		Timeout: time.Second,
		MaxResponseBytes: 42,
		HTTPClient: &http.Client{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicitClient.endpoint != "http://localhost:8080" || explicitClient.collector != "explicit" || explicitClient.apiKey != "explicit-key" || explicitClient.username != "explicit-user" || explicitClient.password != "explicit-pass" || explicitClient.env != "explicit-env" || explicitClient.service != "explicit-service" || explicitClient.timeout != time.Second || explicitClient.maxResponseBytes != 42 {
		t.Fatalf("explicit config = %#v", explicitClient)
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		config ConnectionConfig
	}{
		{"invalid DSN", ConnectionConfig{DSN: "not-a-dsn"}},
		{"missing endpoint", ConnectionConfig{Collector: "demo"}},
		{"endpoint userinfo", ConnectionConfig{Endpoint: "https://user:pass@example.com", Collector: "demo"}},
		{"endpoint missing host", ConnectionConfig{Endpoint: "https:///path", Collector: "demo"}},
		{"endpoint unsupported scheme", ConnectionConfig{Endpoint: "ftp://example.com", Collector: "demo"}},
		{"invalid collector", ConnectionConfig{Endpoint: "https://example.com", Collector: "bad/collector"}},
		{"basic password required", ConnectionConfig{Endpoint: "https://example.com", Collector: "demo", Username: "private"}},
		{"basic auth requires TLS", ConnectionConfig{Endpoint: "http://example.com", Collector: "demo", Username: "private", Password: "secret"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.config)
			if err == nil || ErrorCategoryOf(err) != ErrorInvalidConfiguration {
				t.Fatalf("New(%s) error = %v, category = %q", tc.name, err, ErrorCategoryOf(err))
			}
		})
	}
}

func TestNewAcceptsPublicBasicUserAndLocalHTTP(t *testing.T) {
	client, err := New(ConnectionConfig{
		Endpoint: "http://127.0.0.1:9308/",
		Collector: "demo_1",
		Username: "lx_pub_key",
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.endpoint != "http://127.0.0.1:9308" || client.username != "lx_pub_key" || client.password != "" {
		t.Fatalf("client = %#v", client)
	}
}

func TestNewUsesPasswordAndUsernameEnvironmentWhenNotProvidedByDSN(t *testing.T) {
	t.Setenv("LOZA_DSN", "loza://example.com/demo")
	t.Setenv("LOZA_USERNAME", "env-user")
	t.Setenv("LOZA_PASSWORD", "env-pass")
	client, err := New(ConnectionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if client.username != "env-user" || client.password != "env-pass" {
		t.Fatalf("credentials = %q/%q", client.username, client.password)
	}
}

func TestErrorHelpers(t *testing.T) {
	var nilError *QueryError
	if nilError.Error() != "" {
		t.Fatalf("nil QueryError Error = %q", nilError.Error())
	}
	cause := errors.New("cause")
	queryErr := &QueryError{Category: ErrorExecution, Message: "failed", Cause: cause}
	if queryErr.Error() != "failed" || !errors.Is(queryErr, cause) || ErrorCategoryOf(queryErr) != ErrorExecution {
		t.Fatalf("query error helpers failed: %v/%v/%q", queryErr.Error(), errors.Is(queryErr, cause), ErrorCategoryOf(queryErr))
	}
	wrapped := errors.Join(errors.New("wrapper"), queryErr)
	if ErrorCategoryOf(wrapped) != ErrorExecution {
		t.Fatalf("wrapped category = %q", ErrorCategoryOf(wrapped))
	}
	if ErrorCategoryOf(errors.New("plain")) != ErrorTransport {
		t.Fatalf("plain category = %q", ErrorCategoryOf(errors.New("plain")))
	}
}

func TestParseLozaDSNAcceptsAllTransportValues(t *testing.T) {
	for _, transport := range []string{"", "http", "otlp", "grpc"} {
		t.Run(transport, func(t *testing.T) {
			dsn := "loza://example.com/demo"
			if transport != "" {
				dsn += "?transport=" + transport
			}
			if _, err := parseLozaDSN(dsn); err != nil {
				t.Fatal(err)
			}
		})
	}
}
