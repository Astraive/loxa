// Package dsn parses loza:// connection URIs into resolved HTTP/HTTPS/WebSocket endpoints.
//
// The loza:// URI is the standard connection string for Loza Collector.
// It resolves to HTTP/HTTPS/OTLP/gRPC/WebSocket endpoints — it is NOT a wire protocol.
//
// Format:
//
//	loza://[username:password@][host][:port]/[collector]?env=<env>&service=<service>&tls=<true|false>&transport=<http|otlp|grpc>
//
// A private credential is username:password. A public bearer capability uses
// lx_pub_...: with an explicitly empty password.
// Userinfo credentials are percent-decoded and exposed in Username/Password.
// They are never included in resolved endpoint URLs.
//
// Examples:
//
//	loza://localhost:9308/demo?env=dev&tls=false
//	loza://key-id:s%40cret@collector.example.com/my-app?env=prod
//	loza://lx_pub_...:@collector.example.com/my-app?env=prod
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
	Scheme        string // always "loza"
	Username      string // percent-decoded private key ID or public bearer capability
	Password      string // percent-decoded private key secret; empty for public capabilities
	Host          string // hostname (no port)
	Port          int    // resolved port number
	CollectorName string // canonical collector slug from the required path
	Project       string // deprecated compatibility alias for CollectorName
	Env           string // environment name (default: "default")
	Service       string // optional service name
	TLS           bool   // whether to use HTTPS
	Transport     string // "http", "otlp", or "grpc" (default: "http")
	BaseURL       string // resolved http(s)://host:port; never includes credentials
	EventsURL     string // base + /collectors/{collector}/events
	BatchURL      string // base + /collectors/{collector}/events/batch
	OTLPURL       string // base + /collectors/{collector}/otlp/logs
	TailWSURL     string // ws(s)://host:port/collectors/{collector}/tail
	LQLURL        string // base + /collectors/{collector}/lql/query
	LQLQueryURL   string // compatibility alias for LQLURL

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
//   - Collector path is required (loza://host is rejected)
//   - Private userinfo must contain non-empty username/password
//   - Public userinfo is lx_pub_...: with an explicitly empty password
//   - Username cannot contain a colon or whitespace after decoding
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
		if !hasPassword || username == "" || (password == "" && !IsPublicCredentialUsername(username)) {
			return nil, fmt.Errorf("invalid Loza DSN: credentials require username:password or lx_pub_...:")
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

	// The required path is the canonical collector identity. Project remains
	// available as a compatibility alias for existing SDK consumers.
	collectorName := strings.TrimPrefix(u.Path, "/")
	if collectorName == "" {
		return nil, fmt.Errorf("invalid Loza DSN: collector path is required, e.g. loza://host/my-collector")
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

	collectorPath := url.PathEscape(collectorName)
	collectorBaseURL := baseURL + "/collectors/" + collectorPath
	collectorTailBaseURL := fmt.Sprintf("%s://%s:%d/collectors/%s", wsScheme, hostPart, port, collectorPath)

	return &LozaDSN{
		Scheme:        "loza",
		Username:      username,
		Password:      password,
		Host:          host,
		Port:          port,
		CollectorName: collectorName,
		Project:       collectorName,
		Env:           env,
		Service:       service,
		TLS:           tls,
		Transport:     transport,
		BaseURL:       baseURL,
		EventsURL:     collectorBaseURL + "/events",
		BatchURL:      collectorBaseURL + "/events/batch",
		OTLPURL:       collectorBaseURL + "/otlp/logs",
		TailWSURL:     collectorTailBaseURL + "/tail",
		LQLURL:        collectorBaseURL + "/lql/query",
		LQLQueryURL:   collectorBaseURL + "/lql/query",
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

// IsPublicCredentialUsername reports whether username is the public DSN bearer
// capability form. Its empty Basic password is intentional.
func IsPublicCredentialUsername(username string) bool {
	const prefix = "lx_pub_"
	return strings.HasPrefix(username, prefix) && len(username) > len(prefix)
}

// isLocalhost returns true for localhost, 127.0.0.1, or ::1.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
