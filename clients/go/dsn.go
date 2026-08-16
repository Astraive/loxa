package lqlclient

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// lozaDSN is the credential-bearing, resolved form of the canonical loza:// URI.
// It stays private so the client exposes the same connection contract without
// depending on the unreleased workspace-only Loza spec module.
type lozaDSN struct {
	Username      string
	Password      string
	CollectorName string
	Env           string
	Service       string
	BaseURL       string
}

func parseLozaDSN(raw string) (*lozaDSN, error) {
	if raw == "" || !strings.HasPrefix(raw, "loza://") {
		return nil, fmt.Errorf("invalid Loza DSN")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid Loza DSN")
	}
	username, password := "", ""
	if u.User != nil {
		var hasPassword bool
		username = u.User.Username()
		password, hasPassword = u.User.Password()
		if !hasPassword || username == "" || (password == "" && !strings.HasPrefix(username, "lx_pub_")) {
			return nil, fmt.Errorf("invalid Loza DSN credentials")
		}
		if strings.Contains(username, ":") || strings.IndexFunc(username, unicode.IsSpace) >= 0 {
			return nil, fmt.Errorf("invalid Loza DSN username")
		}
	}
	collector := strings.TrimPrefix(u.Path, "/")
	if collector == "" {
		return nil, fmt.Errorf("invalid Loza DSN collector")
	}
	if extra := strings.TrimPrefix(collector, "/"); extra != collector {
		return nil, fmt.Errorf("invalid Loza DSN collector")
	}
	host := u.Hostname()
	tls := !isLocalhost(host)
	switch value := u.Query().Get("tls"); value {
	case "", "auto":
	case "true":
		tls = true
	case "false":
		tls = false
	default:
		return nil, fmt.Errorf("invalid Loza DSN tls")
	}
	switch transport := u.Query().Get("transport"); transport {
	case "", "http", "otlp", "grpc":
	default:
		return nil, fmt.Errorf("invalid Loza DSN transport")
	}
	port := 443
	if !tls {
		port = 80
	}
	if isLocalhost(host) && u.Port() == "" {
		port = 9308
	}
	if rawPort := u.Port(); rawPort != "" {
		parsedPort, parseErr := strconv.Atoi(rawPort)
		if parseErr != nil || parsedPort < 1 || parsedPort > 65535 {
			return nil, fmt.Errorf("invalid Loza DSN port")
		}
		port = parsedPort
	}
	hostPart := host
	if strings.Contains(host, ":") {
		hostPart = "[" + host + "]"
	}
	scheme := "https"
	if !tls {
		scheme = "http"
	}
	env := u.Query().Get("env")
	if env == "" {
		env = "default"
	}
	return &lozaDSN{
		Username: username, Password: password, CollectorName: collector,
		Env: env, Service: u.Query().Get("service"),
		BaseURL: fmt.Sprintf("%s://%s:%d", scheme, hostPart, port),
	}, nil
}
