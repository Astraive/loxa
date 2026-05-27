// Package dsn parses loxa:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
//
// The loxa:// URI is the standard connection string for Loxa Collector.
// It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
//
// Format:
//
//	loxa://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
//
// Examples:
//
//	loxa://localhost:8080/my-app?env=dev&tls=false
//	loxa://collector.example.com/my-app?env=prod&tls=true
//	loxa://loxa.internal:4318/backend?env=staging&service=auth&transport=otlp
package dsn

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// LoxaDSN holds the parsed and resolved values from a loxa:// connection URI.
type LoxaDSN struct {
	Scheme    string // always "loxa"
	Host      string // hostname (no port)
	Port      int    // resolved port number
	Project   string // path segment (project name)
	Env       string // environment name (default: "default")
	Service   string // optional service name
	TLS       bool   // whether to use HTTPS
	Transport string // "http", "otlp", or "grpc" (default: "http")
	BaseURL   string // resolved http(s)://host:port
	EventsURL string // base + /events
	BatchURL  string // base + /events/batch
	OTLPURL   string // base + /otlp/logs
	TailWSURL string // ws(s)://host:port/tail
}

// Parse parses a raw loxa:// connection URI into a LoxaDSN.
//
// Validation rules:
//   - Scheme must be loxa://
//   - Host is required (loxa:// or loxa:///project are rejected)
//   - Project path is required (loxa://host is rejected)
//   - No userinfo allowed (loxa://user:pass@host/project is rejected)
//   - tls must be "true", "false", or "auto"
//   - transport must be "http", "otlp", or "grpc"
//   - Port must be 1-65535 if specified
//
// TLS defaults:
//   - localhost/127.0.0.1/::1 -> false
//   - everything else -> true
//
// Port defaults:
//   - tls=true -> 443
//   - tls=false -> 80
//   - localhost without explicit port -> 8080
func Parse(raw string) (*LoxaDSN, error) {
	if raw == "" {
		return nil, fmt.Errorf("invalid Loxa DSN: empty string")
	}

	if !strings.HasPrefix(raw, "loxa://") {
		return nil, fmt.Errorf("invalid Loxa DSN: scheme must be loxa://")
	}

	// Parse as URL. The loxa:// scheme is valid for url.Parse.
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid Loxa DSN: %w", err)
	}

	// Reject userinfo (API keys must not be in the URL).
	if u.User != nil {
		return nil, fmt.Errorf("invalid Loxa DSN: do not put API keys in the URL, use LOXA_API_KEY instead")
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid Loxa DSN: host is required")
	}

	portStr := u.Port()

	// Project is the path segment without leading slash.
	project := strings.TrimPrefix(u.Path, "/")
	if project == "" {
		return nil, fmt.Errorf("invalid Loxa DSN: project path is required, e.g. loxa://host/my-project")
	}

	q := u.Query()

	// ── TLS default ──────────────────────────────────────────────────────────
	tls := true
	if isLocalhost(host) {
		tls = false
	}
	if v := q.Get("tls"); v != "" {
		switch v {
		case "true":
			tls = true
		case "false":
			tls = false
		case "auto":
			// keep the computed default
		default:
			return nil, fmt.Errorf("invalid Loxa DSN: tls must be true, false, or auto, got %q", v)
		}
	}

	// ── Port default ─────────────────────────────────────────────────────────
	port := 443
	if !tls {
		port = 80
	}
	if host == "localhost" && portStr == "" {
		port = 8080
	}
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid Loxa DSN: invalid port %q", portStr)
		}
		port = p
	}

	// ── Transport ────────────────────────────────────────────────────────────
	transport := "http"
	if v := q.Get("transport"); v != "" {
		switch v {
		case "http", "otlp", "grpc":
			transport = v
		default:
			return nil, fmt.Errorf("invalid Loxa DSN: transport must be http, otlp, or grpc, got %q", v)
		}
	}

	// ── Env ──────────────────────────────────────────────────────────────────
	env := q.Get("env")
	if env == "" {
		env = "default"
	}

	service := q.Get("service")

	// ── Build resolved URLs ──────────────────────────────────────────────────
	scheme := "https"
	wsScheme := "wss"
	if !tls {
		scheme = "http"
		wsScheme = "ws"
	}

	// IPv6 addresses must be bracketed in URLs per RFC 2732/3986.
	hostPart := host
	if strings.Contains(host, ":") {
		hostPart = "[" + host + "]"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, hostPart, port)

	return &LoxaDSN{
		Scheme:    "loxa",
		Host:      host,
		Port:      port,
		Project:   project,
		Env:       env,
		Service:   service,
		TLS:       tls,
		Transport: transport,
		BaseURL:   baseURL,
		EventsURL: baseURL + "/events",
		BatchURL:  baseURL + "/events/batch",
		OTLPURL:   baseURL + "/otlp/logs",
		TailWSURL: fmt.Sprintf("%s://%s:%d/tail", wsScheme, hostPart, port),
	}, nil
}

// isLocalhost returns true for localhost, 127.0.0.1, or ::1.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
