package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileConfig is the YAML-serializable representation of SDK configuration.
// It maps to the loza.yaml file format.
type FileConfig struct {
	CollectorURL      string `yaml:"collector_url"`
	ServiceName       string `yaml:"service_name"`
	ServiceVersion    string `yaml:"service_version"`
	Environment       string `yaml:"environment"`
	TenantID          string `yaml:"tenant_id"`
	BatchSize         int    `yaml:"batch_size"`
	FlushInterval     string `yaml:"flush_interval"`
	MaxBufferSize     int    `yaml:"max_buffer_size"`
	MaxRetries        int    `yaml:"max_retries"`
	MaxBackoff        string `yaml:"max_backoff"`
	Timeout           string `yaml:"timeout"`
	ConnectionTimeout string `yaml:"connection_timeout"`
	EnableCompression *bool  `yaml:"enable_compression"`
}

// ErrConfigFileNotFound is returned when no config file is found.
var ErrConfigFileNotFound = errors.New("loza: config file not found")

// LoadFromFile loads configuration from a loza.yaml file.
// If path is empty, it searches for loza.yaml in the current directory,
// then in the user's home directory (~/.loza/loza.yaml).
//
// Returns ErrConfigFileNotFound if no config file is found.
// Returns a parse error if the file exists but cannot be parsed.
//
// This implements Requirement 32.3.
func LoadFromFile(path string) (FileConfig, error) {
	if path == "" {
		path = findConfigFile()
	}
	if path == "" {
		return FileConfig{}, ErrConfigFileNotFound
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, ErrConfigFileNotFound
		}
		return FileConfig{}, fmt.Errorf("loza: read config file %q: %w", path, err)
	}

	var fc FileConfig
	if err := parseYAML(data, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("loza: parse config file %q: %w", path, err)
	}
	return fc, nil
}

// LoadDefaultsFile loads repo-level SDK defaults from loza-go.defaults.yaml.
func LoadDefaultsFile() (FileConfig, error) {
	path := findDefaultsConfigFile()
	data, err := os.ReadFile(path)
	if err != nil {
		return FileConfig{}, fmt.Errorf("loza: read defaults file %q: %w", path, err)
	}

	var fc FileConfig
	if err := parseYAML(data, &fc); err != nil {
		return FileConfig{}, fmt.Errorf("loza: parse defaults file %q: %w", path, err)
	}
	return fc, nil
}

// findConfigFile searches for loza.yaml in standard locations.
func findConfigFile() string {
	// 1. Current directory
	if _, err := os.Stat("loza.yaml"); err == nil {
		return "loza.yaml"
	}
	// 2. Home directory
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".loza", "loza.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func findDefaultsConfigFile() string {
	if override := strings.TrimSpace(os.Getenv("LOZA_GO_DEFAULTS")); override != "" {
		return override
	}

	candidates := []string{
		"loza-go.defaults.yaml",
		filepath.Join("..", "loza-go.defaults.yaml"),
		filepath.Join("..", "..", "loza-go.defaults.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 5; i++ {
			candidate := filepath.Join(dir, "loza-go.defaults.yaml")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			dir = filepath.Dir(dir)
		}
	}
	return "loza-go.defaults.yaml"
}

// mergeFileConfig applies file-based config values to base, only overriding
// zero-value fields (file config has lower precedence than env vars).
func mergeFileConfig(base Config, fc FileConfig) Config {
	if fc.CollectorURL != "" && base.CollectorURL == "" {
		base.CollectorURL = fc.CollectorURL
	}
	if fc.ServiceName != "" && base.Service == "" {
		base.Service = fc.ServiceName
	}
	if fc.ServiceVersion != "" && base.Version == "" {
		base.Version = fc.ServiceVersion
	}
	if fc.Environment != "" && base.Environment == "" {
		base.Environment = fc.Environment
	}
	if fc.TenantID != "" && base.TenantID == "" {
		base.TenantID = fc.TenantID
	}
	if fc.BatchSize > 0 && base.BatchSize == 0 {
		base.BatchSize = fc.BatchSize
	}
	if fc.FlushInterval != "" && base.FlushInterval == 0 {
		if d, err := time.ParseDuration(fc.FlushInterval); err == nil {
			base.FlushInterval = d
		}
	}
	if fc.MaxBufferSize > 0 && base.MaxBufferSize == 0 {
		base.MaxBufferSize = fc.MaxBufferSize
	}
	if fc.MaxRetries > 0 && base.MaxRetries == 0 {
		base.MaxRetries = fc.MaxRetries
	}
	if fc.MaxBackoff != "" && base.MaxBackoff == 0 {
		if d, err := time.ParseDuration(fc.MaxBackoff); err == nil {
			base.MaxBackoff = d
		}
	}
	if fc.Timeout != "" && base.Timeout == 0 {
		if d, err := time.ParseDuration(fc.Timeout); err == nil {
			base.Timeout = d
		}
	}
	if fc.ConnectionTimeout != "" && base.ConnectionTimeout == 0 {
		if d, err := time.ParseDuration(fc.ConnectionTimeout); err == nil {
			base.ConnectionTimeout = d
		}
	}
	if fc.EnableCompression != nil {
		base.EnableCompression = *fc.EnableCompression
	}
	return base
}

func overlayFileConfig(base, override FileConfig) FileConfig {
	if override.CollectorURL != "" {
		base.CollectorURL = override.CollectorURL
	}
	if override.ServiceName != "" {
		base.ServiceName = override.ServiceName
	}
	if override.ServiceVersion != "" {
		base.ServiceVersion = override.ServiceVersion
	}
	if override.Environment != "" {
		base.Environment = override.Environment
	}
	if override.TenantID != "" {
		base.TenantID = override.TenantID
	}
	if override.BatchSize > 0 {
		base.BatchSize = override.BatchSize
	}
	if override.FlushInterval != "" {
		base.FlushInterval = override.FlushInterval
	}
	if override.MaxBufferSize > 0 {
		base.MaxBufferSize = override.MaxBufferSize
	}
	if override.MaxRetries > 0 {
		base.MaxRetries = override.MaxRetries
	}
	if override.MaxBackoff != "" {
		base.MaxBackoff = override.MaxBackoff
	}
	if override.Timeout != "" {
		base.Timeout = override.Timeout
	}
	if override.ConnectionTimeout != "" {
		base.ConnectionTimeout = override.ConnectionTimeout
	}
	if override.EnableCompression != nil {
		base.EnableCompression = override.EnableCompression
	}
	return base
}

// parseYAML parses YAML data into a FileConfig without requiring an external
// dependency. It uses a simple line-by-line parser for the flat key: value
// format used by loza.yaml.
//
// This avoids adding a yaml dependency to the core module. For production use,
// callers can use LoadFromFileWithYAML which accepts pre-parsed data.
func parseYAML(data []byte, fc *FileConfig) error {
	return parseSimpleYAML(string(data), fc)
}

// parseSimpleYAML parses a simple flat YAML file (key: value pairs only).
// This handles the loza.yaml format without requiring an external YAML library.
func parseSimpleYAML(content string, fc *FileConfig) error {
	lines := splitLines(content)
	for _, line := range lines {
		// Skip comments and empty lines
		trimmed := trimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		key, value, ok := splitKeyValue(trimmed)
		if !ok {
			continue
		}

		switch key {
		case "collector_url":
			fc.CollectorURL = value
		case "service_name":
			fc.ServiceName = value
		case "service_version":
			fc.ServiceVersion = value
		case "environment":
			fc.Environment = value
		case "tenant_id":
			fc.TenantID = value
		case "batch_size":
			if n, err := parseInt(value); err == nil {
				fc.BatchSize = n
			}
		case "flush_interval":
			fc.FlushInterval = value
		case "max_buffer_size":
			if n, err := parseInt(value); err == nil {
				fc.MaxBufferSize = n
			}
		case "max_retries":
			if n, err := parseInt(value); err == nil {
				fc.MaxRetries = n
			}
		case "max_backoff":
			fc.MaxBackoff = value
		case "timeout":
			fc.Timeout = value
		case "connection_timeout":
			fc.ConnectionTimeout = value
		case "enable_compression":
			b := value == "true" || value == "yes" || value == "1"
			fc.EnableCompression = &b
		}
	}
	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitKeyValue(s string) (key, value string, ok bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			key = trimSpace(s[:i])
			value = trimSpace(s[i+1:])
			// Strip inline comments
			for j := 0; j < len(value); j++ {
				if value[j] == '#' {
					value = trimSpace(value[:j])
					break
				}
			}
			// Strip surrounding quotes
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			return key, value, key != ""
		}
	}
	return "", "", false
}

func parseInt(s string) (int, error) {
	n := 0
	if len(s) == 0 {
		return 0, fmt.Errorf("empty string")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
