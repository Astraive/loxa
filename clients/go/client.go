package lqlclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/astraive/loza/spec/dsn"
)

const (
	ErrorInvalidConfiguration = "invalid_configuration"
	ErrorTransport            = "transport"
	ErrorAuthentication       = "authentication"
	ErrorScope                = "scope"
	ErrorDiagnostics          = "diagnostics"
	ErrorCompilerUnavailable  = "compiler_unavailable"
	ErrorExecution            = "execution"
	ErrorTimeout              = "timeout"
	ErrorMalformedResponse    = "malformed_response"
)

var collectorSlug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// QueryValue is a typed value bound to a named $parameter by Collector.
type QueryValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// QueryColumn describes one result column.
type QueryColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Nullable bool   `json:"nullable,omitempty"`
}

// QueryResult is the server-owned LQL result envelope.
type QueryResult struct {
	Columns    []QueryColumn    `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	Duration   time.Duration    `json:"-"`
	DurationMS int64            `json:"duration_ms"`
	RowCount   int              `json:"row_count"`
}

// QueryError is a stable, redacted client failure.
type QueryError struct {
	Category    string           `json:"category"`
	Status      int              `json:"status,omitempty"`
	Message     string           `json:"message"`
	Diagnostics []map[string]any `json:"diagnostics,omitempty"`
	Cause       error            `json:"-"`
}

func (e *QueryError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *QueryError) Unwrap() error { return e.Cause }

// ErrorCategoryOf returns the stable category for an error.
func ErrorCategoryOf(err error) string {
	var queryErr *QueryError
	if errors.As(err, &queryErr) {
		return queryErr.Category
	}
	return ErrorTransport
}

// ConnectionConfig describes a canonical loza:// or explicit HTTP connection.
type ConnectionConfig struct {
	DSN              string
	Endpoint         string
	Collector        string
	APIKey           string
	Username         string
	Password         string
	Env              string
	Service          string
	HTTPClient       *http.Client
	Timeout          time.Duration
	MaxResponseBytes int64
}

// Client executes source-forwarding LQL against Collector. It never compiles or executes SQL locally.
type Client struct {
	endpoint         string
	collector        string
	apiKey           string
	username         string
	password         string
	env              string
	service          string
	httpClient       *http.Client
	timeout          time.Duration
	maxResponseBytes int64
}

// New resolves DSN/configuration precedence and validates the connection without making a request.
func New(config ConnectionConfig) (*Client, error) {
	if config.DSN == "" {
		config.DSN = os.Getenv("LOZA_DSN")
	}
	var parsed *dsn.LozaDSN
	if config.DSN != "" {
		var err error
		parsed, err = dsn.Parse(config.DSN)
		if err != nil {
			return nil, newConfigError("invalid DSN")
		}
		if config.Endpoint == "" {
			config.Endpoint = parsed.BaseURL
		}
		if config.Collector == "" {
			config.Collector = parsed.CollectorName
		}
		if config.Env == "" {
			config.Env = parsed.Env
		}
		if config.Service == "" {
			config.Service = parsed.Service
		}
		if config.Username == "" {
			config.Username = parsed.Username
			if config.Password == "" {
				config.Password = parsed.Password
			}
		}
	}
	if config.APIKey == "" {
		config.APIKey = os.Getenv("LOZA_API_KEY")
	}
	if config.Username == "" {
		config.Username = os.Getenv("LOZA_USERNAME")
	}
	if config.Password == "" {
		config.Password = os.Getenv("LOZA_PASSWORD")
	}
	if config.Endpoint == "" {
		return nil, newConfigError("endpoint is required")
	}
	endpoint, err := url.Parse(strings.TrimRight(config.Endpoint, "/"))
	if err != nil || endpoint.User != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, newConfigError("endpoint must be an HTTP(S) URL without userinfo")
	}
	if config.Collector == "" || !collectorSlug.MatchString(config.Collector) {
		return nil, newConfigError("collector slug is required and must contain only letters, digits, '_' or '-'")
	}
	if config.Username != "" && config.Password == "" && !strings.HasPrefix(config.Username, "lx_pub_") {
		return nil, newConfigError("basic username requires a password")
	}
	if config.APIKey == "" && config.Username != "" && endpoint.Scheme == "http" && !isLocalhost(endpoint.Hostname()) {
		return nil, newConfigError("basic authentication requires TLS for non-local endpoints")
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxResponseBytes <= 0 {
		config.MaxResponseBytes = 8 << 20
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{}
	}
	return &Client{
		endpoint: strings.TrimRight(config.Endpoint, "/"), collector: config.Collector,
		apiKey: config.APIKey, username: config.Username, password: config.Password,
		env: config.Env, service: config.Service, httpClient: config.HTTPClient,
		timeout: config.Timeout, maxResponseBytes: config.MaxResponseBytes,
	}, nil
}

func newConfigError(message string) error {
	return &QueryError{Category: ErrorInvalidConfiguration, Message: "invalid LQL connection configuration: " + message}
}

func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// Query forwards source and typed named parameters to Collector's scoped route.
func (c *Client) Query(ctx context.Context, source string, parameters map[string]QueryValue, limit int) (QueryResult, error) {
	if strings.TrimSpace(source) == "" {
		return QueryResult{}, &QueryError{Category: ErrorInvalidConfiguration, Message: "LQL query source is required"}
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	body, err := json.Marshal(struct {
		Query      string                `json:"query"`
		Parameters map[string]QueryValue `json:"parameters,omitempty"`
		Limit      int                   `json:"limit"`
	}{source, parameters, limit})
	if err != nil {
		return QueryResult{}, &QueryError{Category: ErrorInvalidConfiguration, Message: "invalid LQL parameters", Cause: err}
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	endpoint := c.endpoint + "/collectors/" + url.PathEscape(c.collector) + "/lql/query"
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return QueryResult{}, &QueryError{Category: ErrorInvalidConfiguration, Message: "invalid LQL request configuration", Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else if c.username != "" {
		token := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
		req.Header.Set("Authorization", "Basic "+token)
	}
	if c.env != "" {
		req.Header.Set("X-Loza-Env", c.env)
	}
	if c.service != "" {
		req.Header.Set("X-Loza-Service", c.service)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return QueryResult{}, &QueryError{Category: ErrorTimeout, Message: "LQL query timed out", Cause: err}
		}
		if errors.Is(requestCtx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return QueryResult{}, &QueryError{Category: ErrorTimeout, Message: "LQL query canceled", Cause: err}
		}
		return QueryResult{}, &QueryError{Category: ErrorTransport, Message: "LQL query transport failed", Cause: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseBytes+1))
	if err != nil {
		return QueryResult{}, &QueryError{Category: ErrorTransport, Message: "LQL response could not be read", Cause: err}
	}
	if int64(len(raw)) > c.maxResponseBytes {
		return QueryResult{}, &QueryError{Category: ErrorMalformedResponse, Message: "LQL response exceeds the configured size limit"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return QueryResult{}, decodeHTTPError(resp.StatusCode, raw)
	}
	var result QueryResult
	if err := json.Unmarshal(raw, &result); err != nil || result.Columns == nil || result.Rows == nil {
		return QueryResult{}, &QueryError{Category: ErrorMalformedResponse, Message: "LQL response has an invalid result envelope", Cause: err}
	}
	if result.RowCount == 0 {
		result.RowCount = len(result.Rows)
	}
	result.Duration = time.Duration(result.DurationMS) * time.Millisecond
	return result, nil
}

func decodeHTTPError(status int, raw []byte) error {
	var payload struct {
		Error       string           `json:"error"`
		Message     string           `json:"message"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	_ = json.Unmarshal(raw, &payload)
	message := payload.Error
	if message == "" {
		message = payload.Message
	}
	if message == "" {
		message = fmt.Sprintf("LQL query failed with HTTP %d", status)
	}
	category := ErrorExecution
	switch status {
	case http.StatusUnauthorized:
		category = ErrorAuthentication
	case http.StatusForbidden:
		category = ErrorScope
	case http.StatusBadRequest:
		category = ErrorDiagnostics
	case http.StatusServiceUnavailable:
		category = ErrorCompilerUnavailable
	}
	return &QueryError{Category: category, Status: status, Message: message, Diagnostics: payload.Diagnostics}
}
