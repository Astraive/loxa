// Package dsn parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
//
// The loza:// URI is the standard connection string for Loza Collector.
// It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
//
// Format:
//
//	loza://[host][:port]/[project]?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
//
// Examples:
//
//	loza://localhost:9308/my-app?env=dev&tls=false
//	loza://collector.example.com/my-app?env=prod&tls=true
//	loza://loza.internal:4318/backend?env=staging&service=auth&transport=otlp
package dsn

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// LozaDSN holds the parsed and resolved values from a loza:// connection URI.
type LozaDSN struct {
	Scheme    string // always "loza"
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

// Parse parses a raw loza:// connection URI into a LozaDSN.
//
// Validation rules:
//   - Scheme must be loza://
//   - Host is required (loza:// or loza:///project are rejected)
//   - Project path is required (loza://host is rejected)
//   - No userinfo allowed (loza://user:pass@host/project is rejected)
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
//   - localhost without explicit port -> 9308
func Parse(raw string) (*LozaDSN, error) {
	if raw == "" {
		return nil, fmt.Errorf("invalid Loza DSN: empty string")
	}

	if !strings.HasPrefix(raw, "loza://") {
		return nil, fmt.Errorf("invalid Loza DSN: scheme must be loza://")
	}

	// Parse as URL. The loza:// scheme is valid for url.Parse.
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid Loza DSN: %w", err)
	}

	// Reject userinfo (API keys must not be in the URL).
	if u.User != nil {
		return nil, fmt.Errorf("invalid Loza DSN: do not put API keys in the URL, use LOZA_API_KEY instead")
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid Loza DSN: host is required")
	}

	portStr := u.Port()

	// Project is the path segment without leading slash.
	project := strings.TrimPrefix(u.Path, "/")
	if project == "" {
		return nil, fmt.Errorf("invalid Loza DSN: project path is required, e.g. loza://host/my-project")
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
			return nil, fmt.Errorf("invalid Loza DSN: tls must be true, false, or auto, got %q", v)
		}
	}

	// ── Port default ─────────────────────────────────────────────────────────
	port := 443
	if !tls {
		port = 80
	}
	if isLocalhost(host) && portStr == "" {
		port = 9308
	}
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid Loza DSN: invalid port %q", portStr)
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
			return nil, fmt.Errorf("invalid Loza DSN: transport must be http, otlp, or grpc, got %q", v)
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

	return &LozaDSN{
		Scheme:    "loza",
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
