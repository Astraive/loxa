// Package dsn parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
//
// The loza:// URI is the standard connection string for Loza Collector.
// It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
//
// Format:
//
//	loza://[username:password@][host][:port]/[project]?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
//
// Userinfo credentials are percent-decoded and exposed in Username/Password.
// They are never included in resolved endpoint URLs.
//
// Examples:
//
//	loza://localhost:9308/demo?env=dev&tls=false
//	loza://key-id:s%40cret@collector.example.com/my-app?env=prod
//	loza://collector.example.com/my-app?env=prod&tls=true
//	loza://loza.internal:4318/backend?env=staging&service=auth&transport=otlp
package dsn

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// LozaDSN holds the parsed and resolved values from a loza:// connection URI.
type LozaDSN struct {
	Scheme    string // always "loza"
	Username  string // percent-decoded Collector key ID (empty without userinfo)
	Password  string // percent-decoded Collector key secret (empty without userinfo)
	Host      string // hostname (no port)
	Port      int    // resolved port number
	Project   string // path segment (project name)
	Env       string // environment name (default: "default")
	Service   string // optional service name
	TLS       bool   // whether to use HTTPS
	Transport string // "http", "otlp", or "grpc" (default: "http")
	BaseURL   string // resolved http(s)://host:port; never includes credentials
	EventsURL string // base + /events
	BatchURL  string // base + /events/batch
	OTLPURL   string // base + /otlp/logs
	TailWSURL string // ws(s)://host:port/tail
}

// String returns a credential-free representation suitable for logs.
func (d LozaDSN) String() string {
	return d.BaseURL
}

// GoString returns a credential-free representation for %#v formatting.
func (d LozaDSN) GoString() string {
	return fmt.Sprintf("dsn.LozaDSN{BaseURL:%q}", d.BaseURL)
}

// Parse parses a raw loza:// connection URI into a LozaDSN.
//
// Validation rules:
//   - Scheme must be loza://
//   - Host is required (loza:// or loza:///project are rejected)
//   - Project path is required (loza://host is rejected)
//   - Optional userinfo must contain non-empty username/password
//   - Username cannot contain a colon or whitespace after decoding
//   - Password URL-reserved characters must be percent-encoded
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
		// Do not wrap URL parser details: malformed input may contain secrets.
		return nil, fmt.Errorf("invalid Loza DSN: malformed URL")
	}

	username := ""
	password := ""
	if u.User != nil {
		var hasPassword bool
		username = u.User.Username()
		password, hasPassword = u.User.Password()
		if !hasPassword || username == "" || password == "" {
			return nil, fmt.Errorf("invalid Loza DSN: credentials require non-empty username and password")
		}
		if strings.Contains(username, ":") || hasWhitespace(username) {
			return nil, fmt.Errorf("invalid Loza DSN: username contains an invalid character")
		}
		if err := validateRawPassword(raw); err != nil {
			return nil, err
		}
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
		Username:  username,
		Password:  password,
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

func hasWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// validateRawPassword enforces that URL-reserved password bytes are encoded.
// url.Parse has already validated percent escapes and decoded u.User.
func validateRawPassword(raw string) error {
	const prefix = "loza://"
	authorityEnd := len(raw)
	for i := len(prefix); i < len(raw); i++ {
		switch raw[i] {
		case '/', '?', '#':
			authorityEnd = i
			i = len(raw)
		}
	}

	authority := raw[len(prefix):authorityEnd]
	at := strings.LastIndexByte(authority, '@')
	if at < 0 {
		return fmt.Errorf("invalid Loza DSN: malformed credentials")
	}
	userinfo := authority[:at]
	colon := strings.IndexByte(userinfo, ':')
	if colon < 0 {
		return fmt.Errorf("invalid Loza DSN: credentials require non-empty username and password")
	}

	rawPassword := userinfo[colon+1:]
	for i := range rawPassword {
		if rawPassword[i] == '%' {
			if i+2 >= len(rawPassword) || !isHexDigit(rawPassword[i+1]) || !isHexDigit(rawPassword[i+2]) {
				return fmt.Errorf("invalid Loza DSN: malformed credentials")
			}
			continue
		}
		if isURLReserved(rawPassword[i]) {
			return fmt.Errorf("invalid Loza DSN: password contains an unencoded reserved character")
		}
	}
	return nil
}

func isURLReserved(value byte) bool {
	switch value {
	case ':', '/', '?', '#', '[', ']', '@', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=':
		return true
	default:
		return false
	}
}

func isHexDigit(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

// isLocalhost returns true for localhost, 127.0.0.1, or ::1.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
