package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// userConfigFiles lists the user override file names searched in cwd order.
var userConfigFiles = []string{"loxa-cortex.yaml", "loxa.yaml"}

// Config represents the complete Cortex configuration
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	GRPC           GRPCConfig           `yaml:"grpc"`
	Storage        StorageConfig        `yaml:"storage"`
	Matcher        MatcherConfig        `yaml:"matcher"`
	Authentication AuthenticationConfig `yaml:"authentication"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	Correlation    CorrelationConfig    `yaml:"correlation"`
	Ingestion      IngestionConfig      `yaml:"ingestion"`
	Reconstructor  ReconstructorConfig  `yaml:"reconstructor"`
	Memory         MemoryConfig         `yaml:"memory"`
	Learner        LearnerConfig        `yaml:"learner"`
	Salience       SalienceConfig       `yaml:"salience"`
	EventBus       EventBusConfig       `yaml:"eventbus"`
	TLS            TLSConfig            `yaml:"tls"`
	PIIRedaction   PIIRedactionConfig   `yaml:"pii_redaction"`
	Logging        LoggingConfig        `yaml:"logging"`
	Collector      CollectorConfig      `yaml:"collector"`
}

// MatcherConfig controls the incident matching engine. The Cortex service
// remains Go-first; Rust is an optional acceleration backend only.
type MatcherConfig struct {
	Mode                   string             `yaml:"mode"`
	Parallelism            int                `yaml:"parallelism"`
	CacheSize              int                `yaml:"cache_size"`
	CacheTTL               time.Duration      `yaml:"cache_ttl"`
	MinSimilarityThreshold float64            `yaml:"min_similarity_threshold"`
	FindSimilarLimit       int                `yaml:"find_similar_limit"`
	TopKDefault            int                `yaml:"topk_default"`
	Weights                MatcherWeights     `yaml:"weights"`
	WeightsTopo            MatcherWeightsTopo `yaml:"weights_topo"`
	Temporal               TemporalConfig     `yaml:"temporal"`
}

type MatcherWeights struct {
	Feature  float64 `yaml:"feature"`
	Symptom  float64 `yaml:"symptom"`
	Temporal float64 `yaml:"temporal"`
}

type MatcherWeightsTopo struct {
	Symptom   float64 `yaml:"symptom"`
	Temporal  float64 `yaml:"temporal"`
	Resolution float64 `yaml:"resolution"`
}

type TemporalConfig struct {
	DefaultSimilarity  float64 `yaml:"default_similarity"`
	SameSpeedScore     float64 `yaml:"same_speed_score"`
	ZeroDurationScore  float64 `yaml:"zero_duration_score"`
	FastThresholdMs    int64   `yaml:"fast_threshold_ms"`
	SlowThresholdMs    int64   `yaml:"slow_threshold_ms"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	MaxBodyBytes    int64         `yaml:"max_body_bytes"`
	AllowedOrigins  []string      `yaml:"allowed_origins"`
	TrustedProxies  []string      `yaml:"trusted_proxies"`
}

// GRPCConfig contains gRPC server settings
type GRPCConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

// StorageConfig contains database settings
type StorageConfig struct {
	Backend              string               `yaml:"backend"`
	DuckDB               DuckDBConfig         `yaml:"duckdb"`
	PostgreSQL           PostgresConfig       `yaml:"postgresql"`
	CollectorDBPath      string               `yaml:"collector_db_path"`
	MinSimilarityThreshold float64            `yaml:"min_similarity_threshold"`
	Similarity           StorageSimilarityConfig `yaml:"similarity"`
}

type StorageSimilarityConfig struct {
	ShapePartial  float64            `yaml:"shape_partial"`
	ShapeMismatch float64            `yaml:"shape_mismatch"`
	Weights       SimilarityWeights  `yaml:"weights"`
}

type SimilarityWeights struct {
	Shape   float64 `yaml:"shape"`
	Symptom float64 `yaml:"symptom"`
	Vector  float64 `yaml:"vector"`
}

// DuckDBConfig contains DuckDB-specific settings
type DuckDBConfig struct {
	Path string `yaml:"path"`
}

// PostgresConfig contains PostgreSQL-specific settings
type PostgresConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Database        string        `yaml:"database"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	SSLMode         string        `yaml:"ssl_mode"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// AuthenticationConfig contains authentication settings
type AuthenticationConfig struct {
	Enabled    bool     `yaml:"enabled"`
	APIKeys    []APIKey `yaml:"api_keys"`
	HMACSecret string   `yaml:"hmac_secret"` // HMAC-SHA256 key for hashing API keys at rest
}

// APIKey represents an API key with role
type APIKey struct {
	Name string `yaml:"name"`
	Key  string `yaml:"key"`
	Role string `yaml:"role"` // "reader", "writer", "admin"
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled      bool `yaml:"enabled"`
	PerAPIKeyRPM int  `yaml:"per_api_key_rpm"`
	PerIPRPM     int  `yaml:"per_ip_rpm"`
}

// CorrelationConfig contains dynamic relationship synthesis settings
type CorrelationConfig struct {
	Enabled                   bool          `yaml:"enabled"`
	AnalysisInterval          time.Duration `yaml:"analysis_interval"`
	CoOccurrenceWindow        time.Duration `yaml:"co_occurrence_window"`
	DeploymentAdjacencyWindow time.Duration `yaml:"deployment_adjacency_window"`
	MinCoOccurrenceCount      int           `yaml:"min_co_occurrence_count"`
}

// IngestionConfig contains async ingestion pipeline settings
type IngestionConfig struct {
	AsyncWorkers   int           `yaml:"async_workers"`
	ChannelSize    int           `yaml:"channel_size"`
	MicroBatchSize int           `yaml:"micro_batch_size"`
	FlushInterval  time.Duration `yaml:"flush_interval"`
}

// ReconstructorConfig contains reconstruction settings
type ReconstructorConfig struct {
	Fast         ReconstructionMode      `yaml:"fast"`
	Deep         ReconstructionMode      `yaml:"deep"`
	Confidence   ConfidenceConfig        `yaml:"confidence"`
	Graph        ReconstructorGraphConfig `yaml:"graph"`
	Explain      ExplainConfig           `yaml:"explain"`
	SignalScore  SignalScoreConfig       `yaml:"signal_score"`
}

type ReconstructionMode struct {
	MaxDepth  int           `yaml:"max_depth"`
	MaxEvents int           `yaml:"max_events"`
	TimeWindow time.Duration `yaml:"time_window"`
}

type ConfidenceConfig struct {
	Base               float64 `yaml:"base"`
	CausalChainBonus   float64 `yaml:"causal_chain_bonus"`
	SymptomBonus       float64 `yaml:"symptom_bonus"`
	SimilarityWeight   float64 `yaml:"similarity_weight"`
	RemediationWeight  float64 `yaml:"remediation_weight"`
	MaxConfidence      float64 `yaml:"max_confidence"`
	MinConfidence      float64 `yaml:"min_confidence"`
}

type ReconstructorGraphConfig struct {
	DefaultMaxDepth int `yaml:"default_max_depth"`
}

type ExplainConfig struct {
	MaxKeyFindings   int `yaml:"max_key_findings"`
	MaxAlternatives  int `yaml:"max_alternatives"`
}

type SignalScoreConfig struct {
	Incident        float64 `yaml:"incident"`
	Deployment      float64 `yaml:"deployment"`
	MetricAnomaly   float64 `yaml:"metric_anomaly"`
	MetricThreshold float64 `yaml:"metric_threshold"`
	MetricNormal    float64 `yaml:"metric_normal"`
	LogError        float64 `yaml:"log_error"`
	LogWarn         float64 `yaml:"log_warn"`
	SeverityError   float64 `yaml:"severity_error"`
	LogDefault      float64 `yaml:"log_default"`
	Service         float64 `yaml:"service"`
	Default         float64 `yaml:"default"`
}

// MemoryConfig contains signature evolution settings
type MemoryConfig struct {
	DecayPeriod        time.Duration `yaml:"decay_period"`
	DecayRate          float64       `yaml:"decay_rate"`
	ArchiveThreshold   float64       `yaml:"archive_threshold"`
	MergeTolerance     float64       `yaml:"merge_tolerance"`
	FeatureVectorAlpha float64       `yaml:"feature_vector_alpha"`
}

// LearnerConfig contains continuous learning settings
type LearnerConfig struct {
	LearningRate   float64 `yaml:"learning_rate"`
	FeatureWeightMin float64 `yaml:"feature_weight_min"`
	FeatureWeightMax float64 `yaml:"feature_weight_max"`
}

// SalienceConfig contains salience tracking settings
type SalienceConfig struct {
	LearningRate float64 `yaml:"learning_rate"`
	DefaultScore float64 `yaml:"default_score"`
}

// EventBusConfig contains event bus settings
type EventBusConfig struct {
	SubscriberBufferSize int `yaml:"subscriber_buffer_size"`
}

// TLSConfig contains TLS settings
type TLSConfig struct {
	Enabled      bool   `yaml:"enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
	MinVersion   string `yaml:"min_version"` // "1.2" or "1.3"
	MutualTLS    bool   `yaml:"mutual_tls"`
	ClientCAFile string `yaml:"client_ca_file"`
}

// PIIRedactionConfig contains PII redaction settings
type PIIRedactionConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Mode      string   `yaml:"mode"` // "enforce" or "log"
	Blocklist []string `yaml:"blocklist"`
	Allowlist []string `yaml:"allowlist"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`  // "debug", "info", "warn", "error"
	Format string `yaml:"format"` // "json" or "console"
}

// CollectorConfig contains loxa-collector integration settings
type CollectorConfig struct {
	Mode                 string        `yaml:"mode"` // "fanout", "pull", "replay"
	URL                  string        `yaml:"url"`
	SourceOfTruth        bool          `yaml:"source_of_truth"`
	APIKey               string        `yaml:"api_key"`
	APIKeyHeader         string        `yaml:"api_key_header"`
	PollInterval         time.Duration `yaml:"poll_interval"`
	BatchSize            int           `yaml:"batch_size"`
	TailTransport        string        `yaml:"tail_transport"` // "http" or "websocket"
	TailEnabled          bool          `yaml:"tail_enabled"`
	TailBufferSize       int           `yaml:"tail_buffer_size"`
	TailBatchSize        int           `yaml:"tail_batch_size"`
	TailFlushInterval    time.Duration `yaml:"tail_flush_interval"`
	TailReconnectBackoff time.Duration `yaml:"tail_reconnect_backoff"`
	QueryTable           string        `yaml:"query_table"`
	RawColumn            string        `yaml:"raw_column"`
	TimestampColumn      string        `yaml:"timestamp_column"`
	CursorPath           string        `yaml:"cursor_path"`
}

// Load loads the bundled defaults, overlays a named YAML configuration file,
// then applies environment variable overrides and validates the resulting
// configuration.
func Load(configPath string) (*Config, error) {
	cfg, err := loadBundledDefaults()
	if err != nil {
		return nil, err
	}
	if err := loadFile(cfg, configPath); err != nil {
		return nil, err
	}
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// LoadDefault loads bundled defaults, optionally overlays a supported runtime
// override from the current directory, applies environment variable overrides,
// and validates the resulting configuration.
func LoadDefault() (*Config, error) {
	cfg, err := loadBundledDefaults()
	if err != nil {
		return nil, err
	}

	if path, err := findUserConfigFile(); err != nil {
		return nil, err
	} else if path != "" {
		if err := loadFile(cfg, path); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to configuration
func applyEnvOverrides(cfg *Config) {
	// Server overrides
	if v := os.Getenv("CORTEX_SERVER_HOST"); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv("CORTEX_SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("CORTEX_SERVER_ALLOWED_ORIGINS"); v != "" {
		cfg.Server.AllowedOrigins = strings.Split(v, ",")
		for i := range cfg.Server.AllowedOrigins {
			cfg.Server.AllowedOrigins[i] = strings.TrimSpace(cfg.Server.AllowedOrigins[i])
		}
	}

	// Storage overrides
	if v := os.Getenv("CORTEX_STORAGE_BACKEND"); v != "" {
		cfg.Storage.Backend = v
	}
	if v := os.Getenv("CORTEX_DUCKDB_PATH"); v != "" {
		cfg.Storage.DuckDB.Path = v
	}
	if v := os.Getenv("CORTEX_POSTGRES_HOST"); v != "" {
		cfg.Storage.PostgreSQL.Host = v
	}
	if v := os.Getenv("CORTEX_POSTGRES_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Storage.PostgreSQL.Port = port
		}
	}
	if v := os.Getenv("CORTEX_POSTGRES_DATABASE"); v != "" {
		cfg.Storage.PostgreSQL.Database = v
	}
	if v := os.Getenv("CORTEX_POSTGRES_USER"); v != "" {
		cfg.Storage.PostgreSQL.User = v
	}
	if v := os.Getenv("CORTEX_POSTGRES_PASSWORD"); v != "" {
		cfg.Storage.PostgreSQL.Password = v
	}
	if v := os.Getenv("CORTEX_POSTGRES_SSL_MODE"); v != "" {
		cfg.Storage.PostgreSQL.SSLMode = v
	}

	// Authentication overrides
	if v := os.Getenv("CORTEX_AUTH_ENABLED"); v != "" {
		cfg.Authentication.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("CORTEX_HMAC_SECRET"); v != "" {
		cfg.Authentication.HMACSecret = v
	}
	// CORTEX_API_KEYS: comma-separated list of name:key:role triples
	// e.g. "my-service:sk_abc123:writer,read-only:sk_def456:reader"
	if v := os.Getenv("CORTEX_API_KEYS"); v != "" {
		cfg.Authentication.APIKeys = parseAPIKeys(v)
	}

	// Matcher overrides
	if v := os.Getenv("CORTEX_MATCHER_MODE"); v != "" {
		cfg.Matcher.Mode = v
	}
	if v := os.Getenv("CORTEX_MATCHER_PARALLELISM"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Matcher.Parallelism = n
		}
	}
	if v := os.Getenv("CORTEX_MATCHER_CACHE_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Matcher.CacheSize = n
		}
	}
	if v := os.Getenv("CORTEX_MATCHER_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Matcher.CacheTTL = d
		}
	}

	// Rate limit overrides
	if v := os.Getenv("CORTEX_RATELIMIT_ENABLED"); v != "" {
		cfg.RateLimit.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("CORTEX_RATELIMIT_PER_API_KEY_RPM"); v != "" {
		if rpm, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit.PerAPIKeyRPM = rpm
		}
	}
	if v := os.Getenv("CORTEX_RATELIMIT_PER_IP_RPM"); v != "" {
		if rpm, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit.PerIPRPM = rpm
		}
	}

	// TLS overrides
	if v := os.Getenv("CORTEX_TLS_ENABLED"); v != "" {
		cfg.TLS.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("CORTEX_TLS_CERT_FILE"); v != "" {
		cfg.TLS.CertFile = v
	}
	if v := os.Getenv("CORTEX_TLS_KEY_FILE"); v != "" {
		cfg.TLS.KeyFile = v
	}

	// PII redaction overrides
	if v := os.Getenv("CORTEX_PII_REDACTION_ENABLED"); v != "" {
		cfg.PIIRedaction.Enabled = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("CORTEX_PII_REDACTION_MODE"); v != "" {
		cfg.PIIRedaction.Mode = v
	}

	// Logging overrides
	if v := os.Getenv("CORTEX_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("CORTEX_LOG_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}

	// Collector overrides
	if v := os.Getenv("CORTEX_COLLECTOR_MODE"); v != "" {
		cfg.Collector.Mode = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_URL"); v != "" {
		cfg.Collector.URL = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_SOURCE_OF_TRUTH"); v != "" {
		cfg.Collector.SourceOfTruth = strings.EqualFold(v, "true")
	}
	if v := os.Getenv("CORTEX_COLLECTOR_API_KEY"); v != "" {
		cfg.Collector.APIKey = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_API_KEY_HEADER"); v != "" {
		cfg.Collector.APIKeyHeader = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_TRANSPORT"); v != "" {
		cfg.Collector.TailTransport = strings.ToLower(strings.TrimSpace(v))
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_ENABLED"); v != "" {
		cfg.Collector.TailEnabled = strings.EqualFold(v, "true")
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_BUFFER_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Collector.TailBufferSize = n
		}
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_BATCH_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Collector.TailBatchSize = n
		}
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_FLUSH_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Collector.TailFlushInterval = d
		}
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TAIL_RECONNECT_BACKOFF"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Collector.TailReconnectBackoff = d
		}
	}
	if v := os.Getenv("CORTEX_COLLECTOR_QUERY_TABLE"); v != "" {
		cfg.Collector.QueryTable = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_RAW_COLUMN"); v != "" {
		cfg.Collector.RawColumn = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_TIMESTAMP_COLUMN"); v != "" {
		cfg.Collector.TimestampColumn = v
	}
	if v := os.Getenv("CORTEX_COLLECTOR_CURSOR_PATH"); v != "" {
		cfg.Collector.CursorPath = v
	}
}

// parseAPIKeys parses a comma-separated list of name:key:role triples.
// Example: "my-service:sk_abc123:writer,read-only:sk_def456:reader"
func parseAPIKeys(raw string) []APIKey {
	var keys []APIKey
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		key := strings.TrimSpace(parts[1])
		role := strings.TrimSpace(parts[2])
		if name == "" || key == "" || role == "" {
			continue
		}
		keys = append(keys, APIKey{Name: name, Key: key, Role: role})
	}
	return keys
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate storage backend
	if c.Storage.Backend != "duckdb" && c.Storage.Backend != "postgres" && c.Storage.Backend != "collector_file" {
		return fmt.Errorf("invalid storage backend: %s (must be 'duckdb', 'postgres', or 'collector_file')", c.Storage.Backend)
	}

	// Validate storage-specific config
	if c.Storage.Backend == "duckdb" && c.Storage.DuckDB.Path == "" {
		return fmt.Errorf("duckdb path is required when backend is 'duckdb'")
	}
	if c.Storage.Backend == "collector_file" && c.Storage.CollectorDBPath == "" {
		return fmt.Errorf("collector_db_path is required when backend is 'collector_file'")
	}

	// Validate matcher config
	if c.Matcher.Mode == "" {
		c.Matcher.Mode = "go"
	}
	if c.Matcher.Mode != "go" && c.Matcher.Mode != "rust" {
		return fmt.Errorf("invalid matcher mode: %s (must be 'go' or 'rust')", c.Matcher.Mode)
	}
	if c.Matcher.Parallelism < 0 {
		return fmt.Errorf("matcher parallelism must be >= 0")
	}
	if c.Matcher.CacheSize < 0 {
		return fmt.Errorf("matcher cache_size must be >= 0")
	}
	if c.Matcher.CacheTTL < 0 {
		return fmt.Errorf("matcher cache_ttl must be >= 0")
	}
	if c.Storage.Backend == "postgres" {
		if c.Storage.PostgreSQL.Host == "" {
			return fmt.Errorf("postgres host is required when backend is 'postgres'")
		}
		if c.Storage.PostgreSQL.Database == "" {
			return fmt.Errorf("postgres database is required when backend is 'postgres'")
		}
		if c.Storage.PostgreSQL.MaxConnections < 1 {
			return fmt.Errorf("postgres max_connections must be at least 1")
		}
		if c.Storage.PostgreSQL.MaxConnections > 100 {
			return fmt.Errorf("postgres max_connections must not exceed 100")
		}
	}

	// Validate authentication config
	if c.Authentication.Enabled {
		if len(c.Authentication.APIKeys) == 0 {
			return fmt.Errorf("at least one api key is required when authentication is enabled")
		}
		for _, key := range c.Authentication.APIKeys {
			if strings.TrimSpace(key.Name) == "" {
				return fmt.Errorf("api key name cannot be empty")
			}
			if strings.TrimSpace(key.Key) == "" {
				return fmt.Errorf("api key cannot be empty")
			}
			switch key.Role {
			case "reader", "writer", "admin":
			default:
				return fmt.Errorf("invalid api key role: %s (must be 'reader', 'writer', or 'admin')", key.Role)
			}
		}
	}

	// Validate rate limit config
	if c.RateLimit.Enabled {
		if c.RateLimit.PerAPIKeyRPM < 1 {
			return fmt.Errorf("per_api_key_rpm must be at least 1")
		}
		if c.RateLimit.PerIPRPM < 1 {
			return fmt.Errorf("per_ip_rpm must be at least 1")
		}
	}

	// Validate TLS config
	if c.TLS.Enabled {
		if c.TLS.CertFile == "" {
			return fmt.Errorf("tls cert_file is required when tls is enabled")
		}
		if c.TLS.KeyFile == "" {
			return fmt.Errorf("tls key_file is required when tls is enabled")
		}
		if c.TLS.MinVersion != "1.2" && c.TLS.MinVersion != "1.3" {
			return fmt.Errorf("invalid tls min_version: %s (must be '1.2' or '1.3')", c.TLS.MinVersion)
		}
		if c.TLS.MutualTLS && c.TLS.ClientCAFile == "" {
			return fmt.Errorf("tls client_ca_file is required when mutual_tls is enabled")
		}
	}

	// Validate PII redaction config
	if c.PIIRedaction.Enabled {
		if c.PIIRedaction.Mode != "enforce" && c.PIIRedaction.Mode != "log" {
			return fmt.Errorf("invalid pii_redaction mode: %s (must be 'enforce' or 'log')", c.PIIRedaction.Mode)
		}
	}

	// Validate logging config
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be 'debug', 'info', 'warn', or 'error')", c.Logging.Level)
	}
	if c.Logging.Format != "json" && c.Logging.Format != "console" {
		return fmt.Errorf("invalid log format: %s (must be 'json' or 'console')", c.Logging.Format)
	}

	// Validate collector config
	if c.Collector.Mode != "" && c.Collector.Mode != "fanout" && c.Collector.Mode != "pull" && c.Collector.Mode != "replay" {
		return fmt.Errorf("invalid collector mode: %s (must be 'fanout', 'pull', or 'replay')", c.Collector.Mode)
	}
	if c.Collector.SourceOfTruth {
		if strings.TrimSpace(c.Collector.URL) == "" {
			return fmt.Errorf("collector url is required when collector.source_of_truth is enabled")
		}
		if strings.TrimSpace(c.Collector.QueryTable) == "" || strings.TrimSpace(c.Collector.RawColumn) == "" || strings.TrimSpace(c.Collector.TimestampColumn) == "" {
			return fmt.Errorf("collector query_table, raw_column, and timestamp_column are required when collector.source_of_truth is enabled")
		}
		if c.Collector.TailTransport != "" && c.Collector.TailTransport != "http" && c.Collector.TailTransport != "websocket" {
			return fmt.Errorf("collector tail_transport must be http or websocket")
		}
		if c.Collector.TailBufferSize < 0 || c.Collector.TailBatchSize < 0 {
			return fmt.Errorf("collector tail buffer and batch sizes must be >= 0")
		}
		if c.Collector.TailFlushInterval < 0 || c.Collector.TailReconnectBackoff < 0 {
			return fmt.Errorf("collector tail durations must be >= 0")
		}
	}

	return nil
}

// Default returns the legacy in-memory defaults for library callers. Production
// startup must use LoadDefault or Load so configuration errors cannot be
// ignored.
func Default() *Config {
	cfg := defaultConfig()
	if path, err := findUserConfigFile(); err == nil && path != "" {
		_ = loadFile(cfg, path)
	}
	applyEnvOverrides(cfg)
	return cfg
}

const bundledDefaultFilename = "loxa-cortex.defaults.yaml"

var bundledDefaultCandidates = defaultBundledDefaultCandidates

func defaultBundledDefaultCandidates() []string {
	candidates := []string{filepath.Join(string(filepath.Separator), "app", "configs", bundledDefaultFilename)}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "configs", bundledDefaultFilename))
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			candidates = append(candidates,
				filepath.Join(dir, "configs", bundledDefaultFilename),
				filepath.Join(dir, "cortex", "configs", bundledDefaultFilename),
			)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	return uniquePaths(candidates)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}

func loadBundledDefaults() (*Config, error) {
	var attempted []string
	for _, path := range bundledDefaultCandidates() {
		attempted = append(attempted, path)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to inspect bundled defaults %s: %w", path, err)
		}
		cfg := defaultConfig()
		if err := loadFile(cfg, path); err != nil {
			return nil, fmt.Errorf("failed to load bundled defaults: %w", err)
		}
		return cfg, nil
	}
	return nil, fmt.Errorf("bundled Cortex defaults %q not found (searched: %s)", bundledDefaultFilename, strings.Join(attempted, ", "))
}

// findUserConfigFile returns the first supported runtime override from the
// current directory. Repository/package manifests are deliberately skipped:
// they use the same filenames but are not service configuration.
func findUserConfigFile() (string, error) {
	for _, name := range userConfigFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("failed to read runtime override %s: %w", name, err)
		}
		isManifest, err := isProjectManifest(data)
		if err != nil {
			return "", fmt.Errorf("failed to parse runtime override %s: %w", name, err)
		}
		if isManifest {
			continue
		}
		return name, nil
	}
	return "", nil
}

// isProjectManifest recognizes release manifests by their top-level kind and
// also skips the checked-in Cortex package manifest, which uses the same
// override filename convention.
func isProjectManifest(data []byte) (bool, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return false, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	var kind, module string
	mapping := document.Content[0]
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		switch mapping.Content[i].Value {
		case "kind":
			kind = mapping.Content[i+1].Value
		case "module":
			module = mapping.Content[i+1].Value
		}
	}
	return kind == "release" || (kind != "" && module != ""), nil
}

// loadFile reads a YAML file and overlays it onto cfg. Unknown fields are
// rejected so misspelled runtime settings never silently weaken deployment.
func loadFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	if err := ensureSingleYAMLDocument(decoder); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	return nil
}

func ensureSingleYAMLDocument(decoder *yaml.Decoder) error {
	var extra yaml.Node
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple YAML documents are not supported")
	}
	return err
}

// defaultConfig returns the raw in-memory configuration without file or
// environment processing.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            9312,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			MaxBodyBytes:    10 * 1024 * 1024, // 10MB
		},
		GRPC: GRPCConfig{
			Enabled: false,
			Host:    "0.0.0.0",
			Port:    9313,
		},
		Storage: StorageConfig{
			Backend: "duckdb",
			DuckDB: DuckDBConfig{
				Path: "./cortex.db",
			},
			PostgreSQL: PostgresConfig{
				Host:            "localhost",
				Port:            5432,
				Database:        "cortex",
				User:            "cortex",
				Password:        "",
				SSLMode:         "disable",
				MaxConnections:  50,
				MaxIdleConns:    10,
				ConnMaxLifetime: 1 * time.Hour,
			},
		},
		Matcher: MatcherConfig{
			Mode:        "go",
			Parallelism: 0,
			CacheSize:   1024,
			CacheTTL:    30 * time.Second,
		},
		Authentication: AuthenticationConfig{
			Enabled: false,
			APIKeys: []APIKey{},
		},
		RateLimit: RateLimitConfig{
			Enabled:      false,
			PerAPIKeyRPM: 1000,
			PerIPRPM:     100,
		},
		TLS: TLSConfig{
			Enabled:    false,
			MinVersion: "1.2",
			MutualTLS:  false,
		},
		PIIRedaction: PIIRedactionConfig{
			Enabled: false,
			Mode:    "enforce",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Collector: CollectorConfig{
			Mode:                 "fanout",
			URL:                  "http://localhost:9308",
			SourceOfTruth:        false,
			APIKeyHeader:         "X-API-Key",
			PollInterval:         30 * time.Second,
			BatchSize:            1000,
			TailTransport:        "http",
			TailEnabled:          true,
			TailBufferSize:       2048,
			TailBatchSize:        256,
			TailFlushInterval:    500 * time.Millisecond,
			TailReconnectBackoff: 2 * time.Second,
			QueryTable:           "events",
			RawColumn:            "raw",
			TimestampColumn:      "timestamp",
			CursorPath:           "./cortex-collector.cursor",
		},
	}
}
