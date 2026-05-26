package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Collector   CollectorConfig         `yaml:"collector"`
	Auth        AuthConfig              `yaml:"auth"`
	RateLimit   RateLimitConfig         `yaml:"rate_limit"`
	Routes      RoutesConfig            `yaml:"routes"`
	Storage     StorageConfig           `yaml:"storage"`
	DuckDB      DuckDBConfig            `yaml:"duckdb"`
	Retention   RetentionConfig         `yaml:"retention"`
	Kafka       KafkaConfig             `yaml:"kafka"`
	Worker      WorkerConfig            `yaml:"worker"`
	Logging     LoggingConfig           `yaml:"logging"`
	Metrics     MetricsConfig           `yaml:"metrics"`
	Reliability ReliabilityConfig       `yaml:"reliability"`
	Limits      LimitsConfig            `yaml:"limits"`
	Identity    IdentityConfig          `yaml:"identity"`
	Privacy     PrivacyConfig           `yaml:"privacy"`
	Components  ComponentRegistryConfig `yaml:"components"`
	Retry       RetryConfig             `yaml:"retry"`
	DeadLetter  DeadLetterConfig        `yaml:"dead_letter"`
	Fanout      FanoutConfig            `yaml:"fanout"`
	Dedupe      DedupeConfig            `yaml:"dedupe"`
	Schema      SchemaGovernanceConfig  `yaml:"schema_governance"`
	CortexBridge CortexBridgeConfig     `yaml:"cortex_bridge"`
	EventBus    EventBusConfig          `yaml:"eventbus"`
}

type CollectorConfig struct {
	Addr              string        `yaml:"addr"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	MaxBodyBytes      int64         `yaml:"max_body_bytes"`
	MaxEventsPerReq   int           `yaml:"max_events_per_request"`
	Server            ServerConfig  `yaml:"server"`
}

type ServerConfig struct {
	HTTP    HTTPConfig    `yaml:"http"`
	GRPC    GRPCConfig    `yaml:"grpc"`
	GraphQL GraphQLConfig `yaml:"graphql"`
}

type HTTPConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Addr              string        `yaml:"addr"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	MaxBodyBytes      int64         `yaml:"max_body_bytes"`
	MaxHeaderBytes    int64         `yaml:"max_header_bytes"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	TLS               TLSConfig     `yaml:"tls"`
}

type GRPCConfig struct {
	Enabled              bool            `yaml:"enabled"`
	Port                 string          `yaml:"port"`
	MaxConnections       int             `yaml:"max_connections"`
	MaxConcurrentStreams int             `yaml:"max_concurrent_streams"`
	MaxRecvMsgSize       int             `yaml:"max_recv_msg_size"`
	MaxSendMsgSize       int             `yaml:"max_send_msg_size"`
	Keepalive            KeepaliveConfig `yaml:"keepalive"`
	TLS                  TLSConfig       `yaml:"tls"`
}

type KeepaliveConfig struct {
	MaxConnectionAge      time.Duration `yaml:"max_connection_age"`
	MaxConnectionAgeGrace time.Duration `yaml:"max_connection_age_grace"`
	Time                  time.Duration `yaml:"time"`
	Timeout               time.Duration `yaml:"timeout"`
}

type TLSConfig struct {
	Enabled           bool   `yaml:"enabled"`
	CertFile          string `yaml:"cert_file"`
	KeyFile           string `yaml:"key_file"`
	ClientCAFile      string `yaml:"client_ca_file"`
	RequireClientCert bool   `yaml:"require_client_cert"`
}

type GraphQLConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Port       string `yaml:"port"`
	Playground bool   `yaml:"playground"`
	DepthLimit int    `yaml:"depth_limit"`
	BatchLimit int    `yaml:"batch_limit"`
}

type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Header   string `yaml:"header"`
	ValueEnv string `yaml:"value_env"`
	Value    string `yaml:"-"`
}

type RateLimitConfig struct {
	Enabled bool    `yaml:"enabled"`
	RPS     float64 `yaml:"rps"`
	Burst   int     `yaml:"burst"`
}

type RoutesConfig struct {
	Ingest  string `yaml:"ingest"`
	Health  string `yaml:"health"`
	Ready   string `yaml:"ready"`
	Metrics string `yaml:"metrics"`
}

type StorageConfig struct {
	Primary          string `yaml:"primary"`
	EncryptionKeyEnv string `yaml:"encryption_key_env"`
	EncryptionKey    string `yaml:"-"`
}

type DuckDBConfig struct {
	Path                 string            `yaml:"path"`
	Driver               string            `yaml:"driver"`
	Table                string            `yaml:"table"`
	RawColumn            string            `yaml:"raw_column"`
	StoreRaw             bool              `yaml:"store_raw"`
	CheckpointOnShutdown bool              `yaml:"checkpoint_on_shutdown"`
	CheckpointInterval   time.Duration     `yaml:"checkpoint_interval"`
	MaxOpenConns         int               `yaml:"max_open_conns"`
	MaxIdleConns         int               `yaml:"max_idle_conns"`
	BatchSize            int               `yaml:"batch_size"`
	FlushInterval        time.Duration     `yaml:"flush_interval"`
	WriterLoop           bool              `yaml:"writer_loop"`
	WriterQueueSize      int               `yaml:"writer_queue_size"`
	// UseAppender enables DuckDB Appender API for higher throughput when available.
	UseAppender          bool              `yaml:"use_appender"`
	// WriteTimeout bounds background writes when no caller deadline is present.
	WriteTimeout         time.Duration     `yaml:"write_timeout"`
	// RetryAttempts controls retries for transient DB errors.
	RetryAttempts        int               `yaml:"retry_attempts"`
	// RetryBackoff is the backoff between retry attempts.
	RetryBackoff         time.Duration     `yaml:"retry_backoff"`
	Export               DuckDBExport      `yaml:"export"`
	Schema               map[string]string `yaml:"schema"`
	ColumnTypes          map[string]string `yaml:"column_types"`
}

type DuckDBExport struct {
	Enabled  bool          `yaml:"enabled"`
	Format   string        `yaml:"format"`
	Interval time.Duration `yaml:"interval"`
	Path     string        `yaml:"path"`
}

type RetentionConfig struct {
	Enabled bool  `yaml:"enabled"`
	Days    int   `yaml:"days"`
	MaxSize int64 `yaml:"max_size"`
}

type KafkaConfig struct {
	Brokers           []string      `yaml:"brokers"`
	Topic             string        `yaml:"topic"`
	Acks              string        `yaml:"acks"`               // 0, 1, all (default: all)
	RequestTimeout    time.Duration `yaml:"request_timeout"`    // per-request timeout
	EnableIdempotence bool          `yaml:"enable_idempotence"` // exactly-once semantics
	MaxRetries        int           `yaml:"max_retries"`        // producer retry count
	RetryBackoff      time.Duration `yaml:"retry_backoff"`      // backoff between retries
}

type WorkerConfig struct {
	ConsumerGroup string        `yaml:"consumer_group"`
	PollTimeout   time.Duration `yaml:"poll_timeout"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type MetricsConfig struct {
	Prometheus bool `yaml:"prometheus"`
}

type ReliabilityConfig struct {
	Mode                  string        `yaml:"mode"`
	SpoolDir              string        `yaml:"spool_dir"`
	SpoolFile             string        `yaml:"spool_file"`
	MaxSpoolBytes         int64         `yaml:"max_spool_bytes"`
	Fsync                 bool          `yaml:"fsync"`
	DeliveryQueueSize     int           `yaml:"delivery_queue_size"`
	QueueDir              string        `yaml:"queue_dir"`               // local queue directory
	QueueBatchSize        int           `yaml:"queue_batch_size"`        // events per batch
	QueueBatchTimeout     time.Duration `yaml:"queue_batch_timeout"`     // batch timeout
	QueueFlushInterval    time.Duration `yaml:"queue_flush_interval"`    // flush interval
	QueueCircuitThreshold int           `yaml:"queue_circuit_threshold"` // failures before circuit opens
	QueueCircuitTimeout   time.Duration `yaml:"queue_circuit_timeout"`   // circuit reset timeout
}

type LimitsConfig struct {
	MaxInflightRequests int   `yaml:"max_inflight_requests"`
	MaxInflightEvents   int   `yaml:"max_inflight_events"`
	MaxQueueBytes       int64 `yaml:"max_queue_bytes"`
	MaxEventBytes       int64 `yaml:"max_event_bytes"`
	MaxAttrCount        int   `yaml:"max_attr_count"`
	MaxAttrDepth        int   `yaml:"max_attr_depth"`
	MaxStringLength     int   `yaml:"max_string_length"`
}

type IdentityConfig struct {
	Mode                  string `yaml:"mode"`
	AuthIdentityWins      bool   `yaml:"auth_identity_wins"`
	AllowPayloadIdentity  bool   `yaml:"allow_payload_identity"`
	ServiceName           string `yaml:"service_name"`
	ServiceVersion        string `yaml:"service_version"`
	DeploymentEnvironment string `yaml:"deployment_environment"`
	DeploymentRegion      string `yaml:"deployment_region"`
	TenantID              string `yaml:"tenant_id"`
	WorkspaceID           string `yaml:"workspace_id"`
	OrganizationID        string `yaml:"organization_id"`
}

type PrivacyConfig struct {
	Mode                 string   `yaml:"mode"`
	CollectorRedaction   bool     `yaml:"collector_redaction"`
	EmergencyRedaction   bool     `yaml:"emergency_redaction"`
	Allowlist            []string `yaml:"allowlist"`
	Blocklist            []string `yaml:"blocklist"`
	SecretScan           bool     `yaml:"secret_scan"`
	RightToDeleteEnabled bool     `yaml:"right_to_delete_enabled"`
}

type ComponentRegistryConfig struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
	Extensions []string `yaml:"extensions"`
}

type RetryConfig struct {
	Enabled        bool          `yaml:"enabled"`
	MaxAttempts    int           `yaml:"max_attempts"`
	InitialBackoff time.Duration `yaml:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff"`
	Jitter         bool          `yaml:"jitter"`
}

type DeadLetterConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type FanoutConfig struct {
	Outputs  []FanoutOutputConfig       `yaml:"outputs"`
	Delivery FanoutDeliveryPolicyConfig `yaml:"delivery"`
}

type FanoutOutputConfig struct {
	Name       string                 `yaml:"name"`
	Role       string                 `yaml:"role"`
	Type       string                 `yaml:"type"`
	Enabled    bool                   `yaml:"enabled"`
	DuckDB     FanoutDuckDBConfig     `yaml:"duckdb"`
	Loki       FanoutLokiConfig       `yaml:"loki"`
	OTLP       FanoutOTLPConfig       `yaml:"otlp"`
	ClickHouse FanoutClickHouseConfig `yaml:"clickhouse"`
	Postgres   FanoutPostgresConfig   `yaml:"postgres"`
	S3         FanoutS3Config         `yaml:"s3"`
	GCS        FanoutGCSConfig        `yaml:"gcs"`
}

type FanoutDuckDBConfig struct {
	Path      string `yaml:"path"`
	Table     string `yaml:"table"`
	RawColumn string `yaml:"raw_column"`
	StoreRaw  *bool  `yaml:"store_raw"`
}

type FanoutLokiConfig struct {
	URL           string            `yaml:"url"`
	TenantID      string            `yaml:"tenant_id"`
	Labels        map[string]string `yaml:"labels"`
	BatchSize     int               `yaml:"batch_size"`
	FlushInterval time.Duration     `yaml:"flush_interval"`
	Timeout       time.Duration     `yaml:"timeout"`
}

type FanoutOTLPConfig struct {
	Endpoint string        `yaml:"endpoint"`
	Insecure bool          `yaml:"insecure"`
	Protocol string        `yaml:"protocol"`
	Timeout  time.Duration `yaml:"timeout"`
	Logger   string        `yaml:"logger"`
}

type FanoutClickHouseConfig struct {
	Addrs     []string          `yaml:"addrs"`
	Database  string            `yaml:"database"`
	Username  string            `yaml:"username"`
	Password  string            `yaml:"password"`
	Table     string            `yaml:"table"`
	Schema    map[string]string `yaml:"schema"`
	StoreRaw  bool              `yaml:"store_raw"`
	RawColumn string            `yaml:"raw_column"`
}

type FanoutPostgresConfig struct {
	DSN       string            `yaml:"dsn"`
	Table     string            `yaml:"table"`
	Schema    map[string]string `yaml:"schema"`
	StoreRaw  bool              `yaml:"store_raw"`
	RawColumn string            `yaml:"raw_column"`
}

type FanoutS3Config struct {
	Bucket        string            `yaml:"bucket"`
	Prefix        string            `yaml:"prefix"`
	Region        string            `yaml:"region"`
	Endpoint      string            `yaml:"endpoint"`
	AccessKey     string            `yaml:"access_key"`
	SecretKey     string            `yaml:"secret_key"`
	Schema        map[string]string `yaml:"schema"`
	BatchSize     int               `yaml:"batch_size"`
	FlushInterval time.Duration     `yaml:"flush_interval"`
}

type FanoutGCSConfig struct {
	Bucket          string            `yaml:"bucket"`
	Prefix          string            `yaml:"prefix"`
	CredentialsFile string            `yaml:"credentials_file"`
	Schema          map[string]string `yaml:"schema"`
	BatchSize       int               `yaml:"batch_size"`
	FlushInterval   time.Duration     `yaml:"flush_interval"`
}

type FanoutDeliveryPolicyConfig struct {
	Policy   string               `yaml:"policy"`
	Fallback FanoutFallbackConfig `yaml:"fallback"`
	DLQ      FanoutDLQConfig      `yaml:"dlq"`
}

type FanoutFallbackConfig struct {
	Enabled            bool `yaml:"enabled"`
	OnPrimaryFailure   bool `yaml:"on_primary_failure"`
	OnSecondaryFailure bool `yaml:"on_secondary_failure"`
	OnPolicyFailure    bool `yaml:"on_policy_failure"`
}

type FanoutDLQConfig struct {
	OnPrimaryFailure   bool `yaml:"on_primary_failure"`
	OnSecondaryFailure bool `yaml:"on_secondary_failure"`
	OnFallbackFailure  bool `yaml:"on_fallback_failure"`
	OnPolicyFailure    bool `yaml:"on_policy_failure"`
}

type DedupeConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Key           string        `yaml:"key"`
	Window        time.Duration `yaml:"window"`
	Backend       string        `yaml:"backend"`
	RedisAddr     string        `yaml:"redis_addr"`
	RedisPassword string        `yaml:"redis_password"`
	RedisDB       int           `yaml:"redis_db"`
	RedisPrefix   string        `yaml:"redis_prefix"`
}

type SchemaGovernanceConfig struct {
	Mode           string                `yaml:"mode"`
	SchemaVersion  string                `yaml:"schema_version"`
	EventVersion   string                `yaml:"event_version"`
	RegistryFile   string                `yaml:"registry_file"`
	Registry       []SchemaRegistryEntry `yaml:"registry"`
	QuarantinePath string                `yaml:"quarantine_path"`
}

type SchemaRegistryEntry struct {
	SchemaVersion  string   `yaml:"schema_version"`
	EventVersion   string   `yaml:"event_version"`
	RequiredFields []string `yaml:"required_fields"`
}

type CortexBridgeConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Mode          string        `yaml:"mode"`
	Endpoint      string        `yaml:"endpoint"`
	Insecure      bool          `yaml:"insecure"`
	Timeout       time.Duration `yaml:"timeout"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	QueueSize     int           `yaml:"queue_size"`
	Header        string        `yaml:"header"`
	APIKey        string        `yaml:"api_key"`
}

type EventBusConfig struct {
	Type          string           `yaml:"type"`
	Topic         string           `yaml:"topic"`
	DLQTopic      string           `yaml:"dlq_topic"`
	ConsumerGroup string           `yaml:"consumer_group"`
	Memory        EBMemoryConfig   `yaml:"memory"`
	Redis         EBRedisConfig    `yaml:"redis"`
	NATS          EBNATSConfig     `yaml:"nats"`
	Kafka         EBKafkaConfig    `yaml:"kafka"`
}

type EBMemoryConfig struct {
	BufferSize int `yaml:"buffer_size"`
}

type EBRedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	Stream   string `yaml:"stream"`
	Group    string `yaml:"group"`
	MaxLen   int64  `yaml:"max_len"`
}

type EBNATSConfig struct {
	URL      string `yaml:"url"`
	Stream   string `yaml:"stream"`
	Subject  string `yaml:"subject"`
	Durable  string `yaml:"durable"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type EBKafkaConfig struct {
	Brokers           []string `yaml:"brokers"`
	Topic             string   `yaml:"topic"`
	ConsumerGroup     string   `yaml:"consumer_group"`
	Acks              string   `yaml:"acks"`
	EnableIdempotence bool     `yaml:"enable_idempotence"`
	MaxRetries        int      `yaml:"max_retries"`
	Compression       string   `yaml:"compression"`
}

func Default() Config {
	path := findDefaultsFile()
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("read collector defaults %s: %w", path, err))
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		panic(fmt.Errorf("parse collector defaults %s: %w", path, err))
	}
	return cfg
}

func Load(data []byte, cfg *Config) error {
	return yaml.Unmarshal(data, cfg)
}

// LoadFile reads and parses a YAML config file into cfg.
func LoadFile(cfg *Config, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	return nil
}

func findDefaultsFile() string {
	if override := strings.TrimSpace(os.Getenv("LOXA_COLLECTOR_DEFAULTS")); override != "" {
		return override
	}

	candidates := []string{
		"loxa-collector.defaults.yaml",
		filepath.Join("..", "loxa-collector.defaults.yaml"),
		filepath.Join("..", "..", "loxa-collector.defaults.yaml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 5; i++ {
			candidate := filepath.Join(dir, "loxa-collector.defaults.yaml")
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate
			}
			dir = filepath.Dir(dir)
		}
	}
	return "loxa-collector.defaults.yaml"
}
