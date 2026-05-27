package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	collectorconfig "github.com/astraive/loxa-collector/internal/config"
	serverconfig "github.com/astraive/loxa-collector/internal/server"
	"gopkg.in/yaml.v3"
)

var (
	configIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	configPathPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
)

type fileConfig = collectorconfig.Config

func defaultFileConfig() fileConfig { return collectorconfig.Default() }

func loadCollectorConfigFromArgs(args []string) (collectorConfig, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	cfgFile := fs.String("c", "loxa.yaml", "path to config file")
	addr := fs.String("addr", "", "collector listen address (overrides config)")
	apiKey := fs.String("api-key", "", "collector API key (overrides config)")
	duckDBPath := fs.String("duckdb-path", "", "duckdb path (overrides config)")
	duckDBBatchSize := fs.Int("batch-size", 0, "duckdb batch size (overrides config)")
	duckDBFlushInterval := fs.Duration("flush-interval", 0, "duckdb flush interval (overrides config)")
	duckDBUseAppender := fs.Bool("use-appender", false, "enable duckdb appender (overrides config)")
	duckDBWriteTimeout := fs.Duration("write-timeout", 0, "duckdb write timeout (overrides config)")
	duckDBRetryAttempts := fs.Int("retry-attempts", -1, "duckdb retry attempts (overrides config)")
	duckDBRetryBackoff := fs.Duration("retry-backoff", 0, "duckdb retry backoff (overrides config)")
	maxBodyBytes := fs.Int64("max-body-bytes", 0, "max body bytes (overrides config)")
	if err := fs.Parse(args); err != nil {
		return collectorConfig{}, err
	}

	fc := defaultFileConfig()

	if *cfgFile != "" {
		if _, err := os.Stat(*cfgFile); err == nil {
			if err := collectorconfig.LoadFile(&fc, *cfgFile); err != nil {
				return collectorConfig{}, err
			}
		}
	}
	if err := applyEnvOverrides(&fc); err != nil {
		return collectorConfig{}, err
	}
	if err := loadSchemaRegistryFile(&fc); err != nil {
		return collectorConfig{}, err
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if set["addr"] && *addr != "" {
		fc.Collector.Addr = *addr
	}
	if set["api-key"] {
		fc.Auth.Value = *apiKey
		fc.Auth.Enabled = true
	}
	if set["duckdb-path"] {
		fc.DuckDB.Path = *duckDBPath
	}
	if set["batch-size"] {
		fc.DuckDB.BatchSize = *duckDBBatchSize
	}
	if set["flush-interval"] {
		fc.DuckDB.FlushInterval = *duckDBFlushInterval
	}
	if set["use-appender"] {
		fc.DuckDB.UseAppender = *duckDBUseAppender
	}
	if set["write-timeout"] {
		fc.DuckDB.WriteTimeout = *duckDBWriteTimeout
	}
	if set["retry-attempts"] {
		// only override if provided (default flag -1 indicates unset)
		if *duckDBRetryAttempts >= 0 {
			fc.DuckDB.RetryAttempts = *duckDBRetryAttempts
		}
	}
	if set["retry-backoff"] {
		fc.DuckDB.RetryBackoff = *duckDBRetryBackoff
	}
	if set["max-body-bytes"] {
		fc.Collector.MaxBodyBytes = *maxBodyBytes
	}

	if err := validateFileConfig(fc); err != nil {
		return collectorConfig{}, err
	}
	cfg := runtimeConfigFromFile(fc)
	cfg.configFile = *cfgFile
	cfg.configArgs = append([]string(nil), args...)
	return cfg, nil
}

func configCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("expected subcommand: print or validate")
	}
	switch args[0] {
	case "print":
		fc, err := loadPrintableConfig(args[1:])
		if err != nil {
			return err
		}
		out, err := marshalPrintableConfig(fc)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		return nil
	case "validate":
		if _, err := loadPrintableConfig(args[1:]); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func loadPrintableConfig(args []string) (fileConfig, error) {
	cfgFile := "loxa.yaml"
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.StringVar(&cfgFile, "c", cfgFile, "path to config file")
	if err := fs.Parse(args); err != nil {
		return fileConfig{}, err
	}
	fc := defaultFileConfig()
	if _, err := os.Stat(cfgFile); err == nil {
		if err := collectorconfig.LoadFile(&fc, cfgFile); err != nil {
			return fileConfig{}, err
		}
	}
	if err := applyEnvOverrides(&fc); err != nil {
		return fileConfig{}, err
	}
	if err := validateFileConfig(fc); err != nil {
		return fileConfig{}, err
	}
	return fc, nil
}

func applyEnvOverrides(fc *fileConfig) error {
	get := func(key string) (string, bool) {
		v, ok := os.LookupEnv(key)
		if !ok {
			return "", false
		}
		v = strings.TrimSpace(v)
		if v == "" {
			return "", false
		}
		return v, true
	}
	setDuration := func(key string, dst *time.Duration) error {
		v, ok := get(key)
		if !ok {
			return nil
		}
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("%s: invalid duration %q", key, v)
		}
		*dst = d
		return nil
	}
	setInt := func(key string, dst *int) error {
		v, ok := get(key)
		if !ok {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: invalid int %q", key, v)
		}
		*dst = n
		return nil
	}
	setInt64 := func(key string, dst *int64) error {
		v, ok := get(key)
		if !ok {
			return nil
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid int64 %q", key, v)
		}
		*dst = n
		return nil
	}
	setFloat64 := func(key string, dst *float64) error {
		v, ok := get(key)
		if !ok {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: invalid float %q", key, v)
		}
		*dst = f
		return nil
	}
	setBool := func(key string, dst *bool) error {
		v, ok := get(key)
		if !ok {
			return nil
		}
		b, err := parseConfigBool(v)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		*dst = b
		return nil
	}
	setString := func(key string, dst *string) {
		if v, ok := get(key); ok {
			*dst = v
		}
	}
	setStringLower := func(key string, dst *string) {
		if v, ok := get(key); ok {
			*dst = strings.ToLower(v)
		}
	}
	setCSV := func(key string, dst *[]string) {
		v, ok := get(key)
		if !ok {
			return
		}
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		*dst = out
	}

	setString("COLLECTOR_ADDR", &fc.Collector.Addr)
	if err := setDuration("COLLECTOR_READ_HEADER_TIMEOUT", &fc.Collector.ReadHeaderTimeout); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_SHUTDOWN_TIMEOUT", &fc.Collector.ShutdownTimeout); err != nil {
		return err
	}
	if err := setInt64("COLLECTOR_MAX_BODY_BYTES", &fc.Collector.MaxBodyBytes); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_EVENTS", &fc.Collector.MaxEventsPerReq); err != nil {
		return err
	}
	setString("COLLECTOR_API_KEY_HEADER", &fc.Auth.Header)
	if v, ok := get("COLLECTOR_API_KEY"); ok {
		fc.Auth.Value = v
		fc.Auth.Enabled = true
	}
	if v, ok := get("LOXA_STORAGE_ENCRYPTION_KEY"); ok {
		fc.Storage.EncryptionKey = v
	}
	if err := setBool("COLLECTOR_RATE_LIMIT_ENABLED", &fc.RateLimit.Enabled); err != nil {
		return err
	}
	if err := setFloat64("COLLECTOR_RATE_LIMIT_RPS", &fc.RateLimit.RPS); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_RATE_LIMIT_BURST", &fc.RateLimit.Burst); err != nil {
		return err
	}
	setString("DUCKDB_PATH", &fc.DuckDB.Path)
	setString("DUCKDB_DRIVER", &fc.DuckDB.Driver)
	setString("DUCKDB_TABLE", &fc.DuckDB.Table)
	setString("DUCKDB_RAW_COLUMN", &fc.DuckDB.RawColumn)
	setCSV("COLLECTOR_KAFKA_BROKERS", &fc.Kafka.Brokers)
	setString("COLLECTOR_KAFKA_TOPIC", &fc.Kafka.Topic)
	setString("LOXA_WORKER_CONSUMER_GROUP", &fc.Worker.ConsumerGroup)
	if err := setDuration("LOXA_WORKER_POLL_TIMEOUT", &fc.Worker.PollTimeout); err != nil {
		return err
	}
	if err := setBool("DUCKDB_CHECKPOINT_ON_SHUTDOWN", &fc.DuckDB.CheckpointOnShutdown); err != nil {
		return err
	}
	if err := setInt("DUCKDB_BATCH_SIZE", &fc.DuckDB.BatchSize); err != nil {
		return err
	}
	if err := setDuration("DUCKDB_FLUSH_INTERVAL", &fc.DuckDB.FlushInterval); err != nil {
		return err
	}
	if err := setBool("DUCKDB_WRITER_LOOP", &fc.DuckDB.WriterLoop); err != nil {
		return err
	}
	if err := setInt("DUCKDB_WRITER_QUEUE_SIZE", &fc.DuckDB.WriterQueueSize); err != nil {
		return err
	}
	if err := setBool("DUCKDB_USE_APPENDER", &fc.DuckDB.UseAppender); err != nil {
		return err
	}
	if err := setDuration("DUCKDB_WRITE_TIMEOUT", &fc.DuckDB.WriteTimeout); err != nil {
		return err
	}
	if err := setInt("DUCKDB_RETRY_ATTEMPTS", &fc.DuckDB.RetryAttempts); err != nil {
		return err
	}
	if err := setDuration("DUCKDB_RETRY_BACKOFF", &fc.DuckDB.RetryBackoff); err != nil {
		return err
	}
	if err := setDuration("DUCKDB_CHECKPOINT_INTERVAL", &fc.DuckDB.CheckpointInterval); err != nil {
		return err
	}
	if err := setBool("DUCKDB_EXPORT_ENABLED", &fc.DuckDB.Export.Enabled); err != nil {
		return err
	}
	setStringLower("DUCKDB_EXPORT_FORMAT", &fc.DuckDB.Export.Format)
	if err := setDuration("DUCKDB_EXPORT_INTERVAL", &fc.DuckDB.Export.Interval); err != nil {
		return err
	}
	setString("DUCKDB_EXPORT_PATH", &fc.DuckDB.Export.Path)
	setStringLower("COLLECTOR_RELIABILITY_MODE", &fc.Reliability.Mode)
	setString("COLLECTOR_SPOOL_DIR", &fc.Reliability.SpoolDir)
	setString("COLLECTOR_SPOOL_FILE", &fc.Reliability.SpoolFile)
	if err := setInt64("COLLECTOR_MAX_SPOOL_BYTES", &fc.Reliability.MaxSpoolBytes); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_SPOOL_FSYNC", &fc.Reliability.Fsync); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_DELIVERY_QUEUE_SIZE", &fc.Reliability.DeliveryQueueSize); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_INFLIGHT_REQUESTS", &fc.Limits.MaxInflightRequests); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_INFLIGHT_EVENTS", &fc.Limits.MaxInflightEvents); err != nil {
		return err
	}
	if err := setInt64("COLLECTOR_MAX_QUEUE_BYTES", &fc.Limits.MaxQueueBytes); err != nil {
		return err
	}
	if err := setInt64("COLLECTOR_MAX_EVENT_BYTES", &fc.Limits.MaxEventBytes); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_ATTR_COUNT", &fc.Limits.MaxAttrCount); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_ATTR_DEPTH", &fc.Limits.MaxAttrDepth); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_MAX_STRING_LENGTH", &fc.Limits.MaxStringLength); err != nil {
		return err
	}
	setString("COLLECTOR_QUEUE_DIR", &fc.Reliability.QueueDir)
	if err := setInt("COLLECTOR_QUEUE_BATCH_SIZE", &fc.Reliability.QueueBatchSize); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_QUEUE_BATCH_TIMEOUT", &fc.Reliability.QueueBatchTimeout); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_QUEUE_FLUSH_INTERVAL", &fc.Reliability.QueueFlushInterval); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_QUEUE_CIRCUIT_THRESHOLD", &fc.Reliability.QueueCircuitThreshold); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_QUEUE_CIRCUIT_TIMEOUT", &fc.Reliability.QueueCircuitTimeout); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_RETRY_ENABLED", &fc.Retry.Enabled); err != nil {
		return err
	}
	if err := setInt("COLLECTOR_RETRY_MAX_ATTEMPTS", &fc.Retry.MaxAttempts); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_RETRY_INITIAL_BACKOFF", &fc.Retry.InitialBackoff); err != nil {
		return err
	}
	if err := setDuration("COLLECTOR_RETRY_MAX_BACKOFF", &fc.Retry.MaxBackoff); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_RETRY_JITTER", &fc.Retry.Jitter); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_DLQ_ENABLED", &fc.DeadLetter.Enabled); err != nil {
		return err
	}
	setString("COLLECTOR_DLQ_PATH", &fc.DeadLetter.Path)
	setStringLower("COLLECTOR_DELIVERY_POLICY", &fc.Fanout.Delivery.Policy)
	if err := setBool("COLLECTOR_FANOUT_FALLBACK_ENABLED", &fc.Fanout.Delivery.Fallback.Enabled); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_FALLBACK_ON_PRIMARY_FAILURE", &fc.Fanout.Delivery.Fallback.OnPrimaryFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_FALLBACK_ON_SECONDARY_FAILURE", &fc.Fanout.Delivery.Fallback.OnSecondaryFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_FALLBACK_ON_POLICY_FAILURE", &fc.Fanout.Delivery.Fallback.OnPolicyFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_DLQ_ON_PRIMARY_FAILURE", &fc.Fanout.Delivery.DLQ.OnPrimaryFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_DLQ_ON_SECONDARY_FAILURE", &fc.Fanout.Delivery.DLQ.OnSecondaryFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_DLQ_ON_FALLBACK_FAILURE", &fc.Fanout.Delivery.DLQ.OnFallbackFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_FANOUT_DLQ_ON_POLICY_FAILURE", &fc.Fanout.Delivery.DLQ.OnPolicyFailure); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_DEDUPE_ENABLED", &fc.Dedupe.Enabled); err != nil {
		return err
	}
	setString("COLLECTOR_DEDUPE_KEY", &fc.Dedupe.Key)
	if err := setDuration("COLLECTOR_DEDUPE_WINDOW", &fc.Dedupe.Window); err != nil {
		return err
	}
	setStringLower("COLLECTOR_DEDUPE_BACKEND", &fc.Dedupe.Backend)
	setString("COLLECTOR_DEDUPE_REDIS_ADDR", &fc.Dedupe.RedisAddr)
	setString("COLLECTOR_DEDUPE_REDIS_PASSWORD", &fc.Dedupe.RedisPassword)
	if err := setInt("COLLECTOR_DEDUPE_REDIS_DB", &fc.Dedupe.RedisDB); err != nil {
		return err
	}
	setString("COLLECTOR_DEDUPE_REDIS_PREFIX", &fc.Dedupe.RedisPrefix)
	if fc.Storage.EncryptionKey == "" && strings.TrimSpace(fc.Storage.EncryptionKeyEnv) != "" {
		if v, ok := get(fc.Storage.EncryptionKeyEnv); ok {
			fc.Storage.EncryptionKey = v
		}
	}
	setStringLower("COLLECTOR_IDENTITY_MODE", &fc.Identity.Mode)
	if err := setBool("COLLECTOR_AUTH_IDENTITY_WINS", &fc.Identity.AuthIdentityWins); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_ALLOW_PAYLOAD_IDENTITY", &fc.Identity.AllowPayloadIdentity); err != nil {
		return err
	}
	setString("COLLECTOR_SERVICE_NAME", &fc.Identity.ServiceName)
	setString("COLLECTOR_SERVICE_VERSION", &fc.Identity.ServiceVersion)
	setString("COLLECTOR_DEPLOYMENT_ENVIRONMENT", &fc.Identity.DeploymentEnvironment)
	setString("COLLECTOR_DEPLOYMENT_REGION", &fc.Identity.DeploymentRegion)
	setString("COLLECTOR_TENANT_ID", &fc.Identity.TenantID)
	setString("COLLECTOR_WORKSPACE_ID", &fc.Identity.WorkspaceID)
	setString("COLLECTOR_ORGANIZATION_ID", &fc.Identity.OrganizationID)
	setStringLower("COLLECTOR_PRIVACY_MODE", &fc.Privacy.Mode)
	if err := setBool("COLLECTOR_REDACTION_ENABLED", &fc.Privacy.CollectorRedaction); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_EMERGENCY_REDACTION", &fc.Privacy.EmergencyRedaction); err != nil {
		return err
	}
	if err := setBool("COLLECTOR_SECRET_SCAN", &fc.Privacy.SecretScan); err != nil {
		return err
	}
	if err := setBool("CORTEX_BRIDGE_ENABLED", &fc.CortexBridge.Enabled); err != nil {
		return err
	}
	setStringLower("CORTEX_BRIDGE_MODE", &fc.CortexBridge.Mode)
	setString("CORTEX_BRIDGE_ENDPOINT", &fc.CortexBridge.Endpoint)
	if err := setBool("CORTEX_BRIDGE_INSECURE", &fc.CortexBridge.Insecure); err != nil {
		return err
	}
	if err := setDuration("CORTEX_BRIDGE_TIMEOUT", &fc.CortexBridge.Timeout); err != nil {
		return err
	}
	if err := setInt("CORTEX_BRIDGE_BATCH_SIZE", &fc.CortexBridge.BatchSize); err != nil {
		return err
	}
	if err := setDuration("CORTEX_BRIDGE_FLUSH_INTERVAL", &fc.CortexBridge.FlushInterval); err != nil {
		return err
	}
	if err := setInt("CORTEX_BRIDGE_QUEUE_SIZE", &fc.CortexBridge.QueueSize); err != nil {
		return err
	}
	setString("CORTEX_BRIDGE_HEADER", &fc.CortexBridge.Header)
	setString("CORTEX_BRIDGE_API_KEY", &fc.CortexBridge.APIKey)

	// Eventbus env overrides
	setStringLower("LOXA_EVENTBUS", &fc.EventBus.Type)
	setString("LOXA_EVENTBUS_TOPIC", &fc.EventBus.Topic)
	setString("LOXA_EVENTBUS_DLQ_TOPIC", &fc.EventBus.DLQTopic)
	setString("LOXA_EVENTBUS_GROUP", &fc.EventBus.ConsumerGroup)
	if err := setInt("LOXA_EVENTBUS_MEMORY_BUFFER", &fc.EventBus.Memory.BufferSize); err != nil {
		return err
	}
	setString("LOXA_EVENTBUS_REDIS_ADDR", &fc.EventBus.Redis.Addr)
	setString("LOXA_EVENTBUS_REDIS_PASSWORD", &fc.EventBus.Redis.Password)
	if err := setInt("LOXA_EVENTBUS_REDIS_DB", &fc.EventBus.Redis.DB); err != nil {
		return err
	}
	setString("LOXA_EVENTBUS_REDIS_STREAM", &fc.EventBus.Redis.Stream)
	setString("LOXA_EVENTBUS_REDIS_GROUP", &fc.EventBus.Redis.Group)
	if err := setInt64("LOXA_EVENTBUS_REDIS_MAX_LEN", &fc.EventBus.Redis.MaxLen); err != nil {
		return err
	}
	setString("LOXA_EVENTBUS_NATS_URL", &fc.EventBus.NATS.URL)
	setString("LOXA_EVENTBUS_NATS_STREAM", &fc.EventBus.NATS.Stream)
	setString("LOXA_EVENTBUS_NATS_SUBJECT", &fc.EventBus.NATS.Subject)
	setString("LOXA_EVENTBUS_NATS_DURABLE", &fc.EventBus.NATS.Durable)
	setCSV("LOXA_EVENTBUS_KAFKA_BROKERS", &fc.EventBus.Kafka.Brokers)
	setString("LOXA_EVENTBUS_KAFKA_TOPIC", &fc.EventBus.Kafka.Topic)
	setString("LOXA_EVENTBUS_KAFKA_GROUP", &fc.EventBus.Kafka.ConsumerGroup)
	setCSV("LOXA_KAFKA_BROKERS", &fc.EventBus.Kafka.Brokers)
	setString("LOXA_KAFKA_TOPIC", &fc.EventBus.Kafka.Topic)
	setString("LOXA_KAFKA_GROUP", &fc.EventBus.Kafka.ConsumerGroup)

	if fc.Auth.Value == "" && fc.Auth.ValueEnv != "" {
		fc.Auth.Value = strings.TrimSpace(os.Getenv(fc.Auth.ValueEnv))
		if fc.Auth.Value != "" {
			fc.Auth.Enabled = true
		}
	}
	return nil
}

func validateFileConfig(fc fileConfig) error {
	if strings.TrimSpace(fc.Collector.Addr) == "" {
		return errors.New("collector.addr must not be empty")
	}
	if fc.Collector.ReadHeaderTimeout <= 0 {
		return errors.New("collector.read_header_timeout must be > 0")
	}
	if fc.Collector.ShutdownTimeout <= 0 {
		return errors.New("collector.shutdown_timeout must be > 0")
	}
	if fc.Collector.MaxBodyBytes <= 0 {
		return errors.New("collector.max_body_bytes must be > 0")
	}
	if fc.Collector.MaxEventsPerReq <= 0 {
		return errors.New("collector.max_events_per_request must be > 0")
	}
	if fc.Auth.Enabled && strings.TrimSpace(fc.Auth.Value) == "" {
		return errors.New("auth enabled but no API key resolved")
	}
	if strings.TrimSpace(fc.Auth.Header) == "" {
		return errors.New("auth.header must not be empty")
	}
	if fc.RateLimit.Enabled {
		if fc.RateLimit.RPS <= 0 {
			return errors.New("rate_limit.rps must be > 0")
		}
		if fc.RateLimit.Burst <= 0 {
			return errors.New("rate_limit.burst must be > 0")
		}
	}
	if strings.TrimSpace(fc.Storage.Primary) != "duckdb" {
		return errors.New("storage.primary must currently be duckdb")
	}
	mode := strings.ToLower(strings.TrimSpace(fc.Reliability.Mode))
	if mode != "direct" && mode != "spool" && mode != "queue" && mode != "hybrid" {
		return errors.New("reliability.mode must be direct, spool, queue, or hybrid")
	}
	if mode == "spool" || mode == "hybrid" {
		if strings.TrimSpace(fc.Reliability.SpoolDir) == "" {
			return errors.New("reliability.spool_dir must not be empty in spool/hybrid mode")
		}
		if strings.TrimSpace(fc.Reliability.SpoolFile) == "" {
			return errors.New("reliability.spool_file must not be empty in spool/hybrid mode")
		}
		if fc.Reliability.MaxSpoolBytes <= 0 {
			return errors.New("reliability.max_spool_bytes must be > 0 in spool/hybrid mode")
		}
		if fc.Reliability.DeliveryQueueSize <= 0 {
			return errors.New("reliability.delivery_queue_size must be > 0 in spool/hybrid mode")
		}
	}
	if fc.Limits.MaxInflightRequests < 0 {
		return errors.New("limits.max_inflight_requests must be >= 0")
	}
	if fc.Limits.MaxInflightEvents < 0 {
		return errors.New("limits.max_inflight_events must be >= 0")
	}
	if fc.Limits.MaxQueueBytes < 0 {
		return errors.New("limits.max_queue_bytes must be >= 0")
	}
	if fc.Limits.MaxEventBytes < 0 {
		return errors.New("limits.max_event_bytes must be >= 0")
	}
	if fc.Limits.MaxAttrCount < 0 {
		return errors.New("limits.max_attr_count must be >= 0")
	}
	if fc.Limits.MaxAttrDepth < 0 {
		return errors.New("limits.max_attr_depth must be >= 0")
	}
	if fc.Limits.MaxStringLength < 0 {
		return errors.New("limits.max_string_length must be >= 0")
	}
	switch strings.ToLower(strings.TrimSpace(fc.Identity.Mode)) {
	case "", "payload", "api_key", "jwt", "mtls":
	default:
		return errors.New("identity.mode must be payload, api_key, jwt, or mtls")
	}
	switch strings.ToLower(strings.TrimSpace(fc.Privacy.Mode)) {
	case "", "off", "warn", "enforce":
	default:
		return errors.New("privacy.mode must be off, warn, or enforce")
	}
	if err := validateComponentRegistry(fc.Components); err != nil {
		return err
	}
	if mode == "queue" || mode == "hybrid" {
		if len(fc.Kafka.Brokers) == 0 {
			return errors.New("kafka.brokers must be configured in queue/hybrid mode")
		}
		for i, broker := range fc.Kafka.Brokers {
			if strings.TrimSpace(broker) == "" {
				return fmt.Errorf("kafka.brokers[%d] must not be empty", i)
			}
		}
		if strings.TrimSpace(fc.Kafka.Topic) == "" {
			return errors.New("kafka.topic must not be empty in queue/hybrid mode")
		}
	}
	acks := strings.ToLower(strings.TrimSpace(fc.Kafka.Acks))
	if acks != "" && acks != "0" && acks != "1" && acks != "all" {
		return errors.New("kafka.acks must be one of 0, 1, or all")
	}
	if fc.Kafka.RequestTimeout < 0 {
		return errors.New("kafka.request_timeout must be >= 0")
	}
	if fc.Kafka.MaxRetries < 0 {
		return errors.New("kafka.max_retries must be >= 0")
	}
	if fc.Kafka.RetryBackoff < 0 {
		return errors.New("kafka.retry_backoff must be >= 0")
	}
	dedupeBackend := strings.ToLower(strings.TrimSpace(fc.Dedupe.Backend))
	switch dedupeBackend {
	case "", "memory", "redis":
	default:
		return errors.New("dedupe.backend must be memory or redis")
	}
	if dedupeBackend == "redis" && strings.TrimSpace(fc.Dedupe.RedisAddr) == "" {
		return errors.New("dedupe.redis_addr must be configured when dedupe.backend is redis")
	}
	if fc.Dedupe.Enabled && fc.Dedupe.Window <= 0 {
		return errors.New("dedupe.window must be > 0 when dedupe is enabled")
	}
	if strings.TrimSpace(fc.Worker.ConsumerGroup) == "" {
		return errors.New("worker.consumer_group must not be empty")
	}
	if fc.Worker.PollTimeout <= 0 {
		return errors.New("worker.poll_timeout must be > 0")
	}
	if fc.Retry.Enabled {
		if fc.Retry.MaxAttempts <= 0 {
			return errors.New("retry.max_attempts must be > 0")
		}
		if fc.Retry.InitialBackoff <= 0 || fc.Retry.MaxBackoff <= 0 {
			return errors.New("retry backoff durations must be > 0")
		}
	}
	if fc.Collector.Server.HTTP.TLS.Enabled {
		if strings.TrimSpace(fc.Collector.Server.HTTP.TLS.CertFile) == "" || strings.TrimSpace(fc.Collector.Server.HTTP.TLS.KeyFile) == "" {
			return errors.New("collector.server.http.tls cert_file and key_file must be set when http tls is enabled")
		}
	}
	if fc.DeadLetter.Enabled && strings.TrimSpace(fc.DeadLetter.Path) == "" {
		return errors.New("dead_letter.path must not be empty when dead_letter.enabled is true")
	}
	if fc.CortexBridge.Enabled {
		switch strings.ToLower(strings.TrimSpace(fc.CortexBridge.Mode)) {
		case "", "sync", "async":
		default:
			return errors.New("cortex_bridge.mode must be sync or async")
		}
		if strings.TrimSpace(fc.CortexBridge.Endpoint) == "" {
			return errors.New("cortex_bridge.endpoint must not be empty when cortex_bridge.enabled is true")
		}
		if fc.CortexBridge.Timeout < 0 || fc.CortexBridge.FlushInterval < 0 {
			return errors.New("cortex_bridge timeout and flush_interval must be >= 0")
		}
		if fc.CortexBridge.BatchSize < 0 || fc.CortexBridge.QueueSize < 0 {
			return errors.New("cortex_bridge batch_size and queue_size must be >= 0")
		}
	}
	policy := strings.ToLower(strings.TrimSpace(fc.Fanout.Delivery.Policy))
	switch policy {
	case deliveryPolicyBestEffort, deliveryPolicyRequirePrimary, deliveryPolicyRequireAll:
	default:
		return errors.New("fanout.delivery.policy must be best_effort, require_primary, or require_all")
	}
	seenFanoutNames := map[string]struct{}{}
	enabledFallbackOutputs := 0
	for i, output := range fc.Fanout.Outputs {
		name := strings.TrimSpace(output.Name)
		if name == "" {
			return fmt.Errorf("fanout.outputs[%d].name must not be empty", i)
		}
		if _, ok := seenFanoutNames[name]; ok {
			return fmt.Errorf("fanout.outputs[%d].name %q must be unique", i, name)
		}
		seenFanoutNames[name] = struct{}{}

		role := strings.ToLower(strings.TrimSpace(output.Role))
		if role == "" {
			role = fanoutRoleSecondary
		}
		if role != fanoutRoleSecondary && role != fanoutRoleFallback {
			return fmt.Errorf("fanout.outputs[%d].role must be %q or %q", i, fanoutRoleSecondary, fanoutRoleFallback)
		}
		sinkType := strings.ToLower(strings.TrimSpace(output.Type))
		if sinkType == "" {
			sinkType = fanoutSinkDuckDB
		}
		if sinkType != fanoutSinkDuckDB && sinkType != fanoutSinkLoki && sinkType != fanoutSinkOTLP &&
			sinkType != fanoutSinkClickHouse && sinkType != fanoutSinkPostgres && sinkType != fanoutSinkS3 && sinkType != fanoutSinkGCS {
			return fmt.Errorf("fanout.outputs[%d].type %q is not supported", i, sinkType)
		}

		if !output.Enabled {
			continue
		}
		switch sinkType {
		case fanoutSinkDuckDB:
			if strings.TrimSpace(output.DuckDB.Path) == "" {
				return fmt.Errorf("fanout.outputs[%d].duckdb.path must not be empty when enabled", i)
			}
			if table := strings.TrimSpace(output.DuckDB.Table); table != "" && !validTableName(table) {
				return fmt.Errorf("fanout.outputs[%d].duckdb.table %q is invalid", i, table)
			}
			if rawColumn := strings.TrimSpace(output.DuckDB.RawColumn); rawColumn != "" && !configIdentPattern.MatchString(rawColumn) {
				return fmt.Errorf("fanout.outputs[%d].duckdb.raw_column %q is invalid", i, rawColumn)
			}
		case fanoutSinkLoki:
			if strings.TrimSpace(output.Loki.URL) == "" {
				return fmt.Errorf("fanout.outputs[%d].loki.url must not be empty when enabled", i)
			}
		case fanoutSinkOTLP:
			if strings.TrimSpace(output.OTLP.Endpoint) == "" {
				return fmt.Errorf("fanout.outputs[%d].otlp.endpoint must not be empty when enabled", i)
			}
		case fanoutSinkClickHouse:
			if len(output.ClickHouse.Addrs) == 0 {
				return fmt.Errorf("fanout.outputs[%d].clickhouse.addrs must not be empty when enabled", i)
			}
		case fanoutSinkPostgres:
			if strings.TrimSpace(output.Postgres.DSN) == "" {
				return fmt.Errorf("fanout.outputs[%d].postgres.dsn must not be empty when enabled", i)
			}
			if table := strings.TrimSpace(output.Postgres.Table); table != "" && !validTableName(table) {
				return fmt.Errorf("fanout.outputs[%d].postgres.table %q is invalid", i, table)
			}
			if rawColumn := strings.TrimSpace(output.Postgres.RawColumn); rawColumn != "" && !configIdentPattern.MatchString(rawColumn) {
				return fmt.Errorf("fanout.outputs[%d].postgres.raw_column %q is invalid", i, rawColumn)
			}
		case fanoutSinkS3:
			if strings.TrimSpace(output.S3.Bucket) == "" {
				return fmt.Errorf("fanout.outputs[%d].s3.bucket must not be empty when enabled", i)
			}
		case fanoutSinkGCS:
			if strings.TrimSpace(output.GCS.Bucket) == "" {
				return fmt.Errorf("fanout.outputs[%d].gcs.bucket must not be empty when enabled", i)
			}
		}
		if role == fanoutRoleFallback {
			enabledFallbackOutputs++
		}
	}
	if enabledFallbackOutputs > 1 {
		return errors.New("fanout supports at most one enabled fallback output")
	}
	if fc.Fanout.Delivery.Fallback.Enabled && enabledFallbackOutputs != 1 {
		return errors.New("fanout.delivery.fallback.enabled requires exactly one enabled fallback output")
	}
	if fc.Dedupe.Enabled {
		if strings.TrimSpace(fc.Dedupe.Key) == "" {
			return errors.New("dedupe.key must not be empty when dedupe.enabled is true")
		}
		if fc.Dedupe.Window <= 0 {
			return errors.New("dedupe.window must be > 0 when dedupe.enabled is true")
		}
		switch strings.ToLower(strings.TrimSpace(fc.Dedupe.Backend)) {
		case "", "memory":
		case "redis":
			if strings.TrimSpace(fc.Dedupe.RedisAddr) == "" {
				return errors.New("dedupe.redis_addr must not be empty when dedupe.backend is redis")
			}
		default:
			return errors.New("dedupe.backend must be memory or redis")
		}
	}
	if !configIdentPattern.MatchString(fc.DuckDB.RawColumn) {
		return fmt.Errorf("invalid duckdb.raw_column %q", fc.DuckDB.RawColumn)
	}
	if strings.TrimSpace(fc.DuckDB.Path) == "" {
		return errors.New("duckdb.path must not be empty")
	}
	if strings.TrimSpace(fc.DuckDB.Driver) == "" {
		return errors.New("duckdb.driver must not be empty")
	}
	if !validTableName(fc.DuckDB.Table) {
		return fmt.Errorf("invalid duckdb.table %q", fc.DuckDB.Table)
	}
	if fc.DuckDB.BatchSize < 0 {
		return errors.New("duckdb.batch_size must be >= 0")
	}
	if fc.DuckDB.WriterQueueSize < 0 {
		return errors.New("duckdb.writer_queue_size must be >= 0")
	}
	if fc.DuckDB.MaxOpenConns <= 0 || fc.DuckDB.MaxIdleConns <= 0 {
		return errors.New("duckdb max_open_conns and max_idle_conns must be > 0")
	}
	if fc.DuckDB.CheckpointInterval < 0 {
		return errors.New("duckdb.checkpoint_interval must be >= 0")
	}
	if fc.DuckDB.FlushInterval < 0 {
		return errors.New("duckdb.flush_interval must be >= 0")
	}
	if fc.DuckDB.Export.Enabled {
		if strings.ToLower(strings.TrimSpace(fc.DuckDB.Export.Format)) != "parquet" {
			return errors.New("duckdb.export.format must currently be parquet")
		}
		if fc.DuckDB.Export.Interval <= 0 {
			return errors.New("duckdb.export.interval must be > 0 when export is enabled")
		}
		if strings.TrimSpace(fc.DuckDB.Export.Path) == "" {
			return errors.New("duckdb.export.path must not be empty when export is enabled")
		}
	}
	for col, path := range fc.DuckDB.Schema {
		if !configIdentPattern.MatchString(col) {
			return fmt.Errorf("invalid duckdb schema column %q", col)
		}
		if !configPathPattern.MatchString(path) {
			return fmt.Errorf("invalid duckdb schema path %q", path)
		}
	}
	for path, typ := range fc.DuckDB.ColumnTypes {
		if !configPathPattern.MatchString(path) {
			return fmt.Errorf("invalid duckdb column_types path %q", path)
		}
		if strings.TrimSpace(typ) == "" {
			return fmt.Errorf("duckdb column_types type for path %q must not be empty", path)
		}
	}
	for _, route := range []string{fc.Routes.Ingest, fc.Routes.Health, fc.Routes.Ready, fc.Routes.Metrics} {
		if !strings.HasPrefix(route, "/") {
			return fmt.Errorf("route %q must start with /", route)
		}
	}
	return nil
}

func validateComponentRegistry(reg collectorconfig.ComponentRegistryConfig) error {
	allowed := map[string]map[string]struct{}{
		"receivers": {
			"loxa_http":   {},
			"loxa_ndjson": {},
			"loxa_grpc":   {},
			"otlp":        {},
		},
		"processors": {
			"validate":          {},
			"redact":            {},
			"dedupe":            {},
			"memory_limiter":    {},
			"cardinality_limit": {},
			"size_limit":        {},
			"identity":          {},
			"tenant_resolve":    {},
			"enrich":            {},
			"route":             {},
		},
		"exporters": {
			"duckdb":     {},
			"kafka":      {},
			"loki":       {},
			"otlp":       {},
			"clickhouse": {},
			"postgres":   {},
			"s3":         {},
			"gcs":        {},
		},
		"extensions": {
			"health":  {},
			"ready":   {},
			"metrics": {},
		},
	}
	check := func(kind string, values []string) error {
		seen := map[string]struct{}{}
		for i, value := range values {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" {
				return fmt.Errorf("components.%s[%d] must not be empty", kind, i)
			}
			if _, ok := seen[value]; ok {
				return fmt.Errorf("components.%s contains duplicate component %q", kind, value)
			}
			if _, ok := allowed[kind][value]; !ok {
				return fmt.Errorf("components.%s[%d] unknown component %q", kind, i, value)
			}
			seen[value] = struct{}{}
		}
		return nil
	}
	if err := check("receivers", reg.Receivers); err != nil {
		return err
	}
	if err := check("processors", reg.Processors); err != nil {
		return err
	}
	if err := check("exporters", reg.Exporters); err != nil {
		return err
	}
	if err := check("extensions", reg.Extensions); err != nil {
		return err
	}
	return nil
}

func runtimeConfigFromFile(fc fileConfig) collectorConfig {
	return collectorConfig{
		addr:                fc.Collector.Addr,
		readHeaderTimeout:   fc.Collector.ReadHeaderTimeout,
		shutdownTimeout:     fc.Collector.ShutdownTimeout,
		maxBodyBytes:        fc.Collector.MaxBodyBytes,
		maxEventsPerRequest: fc.Collector.MaxEventsPerReq,
		serverConfig: serverConfig{
			HTTP: serverconfig.HTTPConfig{
				Enabled:              fc.Collector.Server.HTTP.Enabled,
				Addr:                 fc.Collector.Server.HTTP.Addr,
				ReadHeaderTimeout:    fc.Collector.Server.HTTP.ReadHeaderTimeout,
				MaxBodyBytes:         fc.Collector.Server.HTTP.MaxBodyBytes,
				MaxHeaderBytes:       fc.Collector.Server.HTTP.MaxHeaderBytes,
				IdleTimeout:          fc.Collector.Server.HTTP.IdleTimeout,
				TLSEnabled:           fc.Collector.Server.HTTP.TLS.Enabled,
				TLSCertFile:          fc.Collector.Server.HTTP.TLS.CertFile,
				TLSKeyFile:           fc.Collector.Server.HTTP.TLS.KeyFile,
				TLSClientCAFile:      fc.Collector.Server.HTTP.TLS.ClientCAFile,
				TLSRequireClientCert: fc.Collector.Server.HTTP.TLS.RequireClientCert,
				IngestPath:           fc.Routes.Ingest,
				HealthPath:           fc.Routes.Health,
				ReadyPath:            fc.Routes.Ready,
				MetricsPath:          fc.Routes.Metrics,
				MetricsEnabled:       fc.Metrics.Prometheus,
				AuthEnabled:          fc.Auth.Enabled,
				AuthHeader:           fc.Auth.Header,
				AuthValue:            fc.Auth.Value,
				RateLimitEnabled:     fc.RateLimit.Enabled,
				RateLimitRPS:         fc.RateLimit.RPS,
				RateLimitBurst:       fc.RateLimit.Burst,
				AllowedOrigins:       append([]string(nil), fc.Identity.AllowedOrigins...),
			},
			GRPC: serverconfig.GRPCConfig{
				Enabled:               fc.Collector.Server.GRPC.Enabled,
				Port:                  fc.Collector.Server.GRPC.Port,
				MaxConnections:        fc.Collector.Server.GRPC.MaxConnections,
				MaxConcurrentStreams:  fc.Collector.Server.GRPC.MaxConcurrentStreams,
				MaxRecvMsgSize:        fc.Collector.Server.GRPC.MaxRecvMsgSize,
				MaxSendMsgSize:        fc.Collector.Server.GRPC.MaxSendMsgSize,
				MaxConnectionAge:      serverconfig.NewDuration(fc.Collector.Server.GRPC.Keepalive.MaxConnectionAge),
				MaxConnectionAgeGrace: serverconfig.NewDuration(fc.Collector.Server.GRPC.Keepalive.MaxConnectionAgeGrace),
				KeepaliveTime:         serverconfig.NewDuration(fc.Collector.Server.GRPC.Keepalive.Time),
				KeepaliveTimeout:      serverconfig.NewDuration(fc.Collector.Server.GRPC.Keepalive.Timeout),
				TLSEnabled:            fc.Collector.Server.GRPC.TLS.Enabled,
				TLSCertFile:           fc.Collector.Server.GRPC.TLS.CertFile,
				TLSKeyFile:            fc.Collector.Server.GRPC.TLS.KeyFile,
			},
			GraphQL: serverconfig.GraphQLConfig{
				Enabled:    fc.Collector.Server.GraphQL.Enabled,
				Port:       fc.Collector.Server.GraphQL.Port,
				Playground: fc.Collector.Server.GraphQL.Playground,
				DepthLimit: fc.Collector.Server.GraphQL.DepthLimit,
				BatchLimit: fc.Collector.Server.GraphQL.BatchLimit,
			},
		},
		authEnabled:             fc.Auth.Enabled,
		authAllowLocalDevKeys:   fc.Auth.AllowLocalDevKeys,
		apiKeyHeader:            fc.Auth.Header,
		apiKey:                  fc.Auth.Value,
		rateLimitEnabled:        fc.RateLimit.Enabled,
		rateLimitRPS:            fc.RateLimit.RPS,
		rateLimitBurst:          fc.RateLimit.Burst,
		ingestPath:              fc.Routes.Ingest,
		healthPath:              fc.Routes.Health,
		readyPath:               fc.Routes.Ready,
		metricsPath:             fc.Routes.Metrics,
		storagePrimary:          fc.Storage.Primary,
		storageEncryptionKey:    fc.Storage.EncryptionKey,
		duckDBPath:              fc.DuckDB.Path,
		duckDBDriver:            fc.DuckDB.Driver,
		duckDBTable:             fc.DuckDB.Table,
		duckDBRawColumn:         fc.DuckDB.RawColumn,
		duckDBStoreRaw:          fc.DuckDB.StoreRaw,
		duckDBCheckpointOnStop:  fc.DuckDB.CheckpointOnShutdown,
		duckDBMaxOpenConns:      fc.DuckDB.MaxOpenConns,
		duckDBMaxIdleConns:      fc.DuckDB.MaxIdleConns,
		duckDBBatchSize:         fc.DuckDB.BatchSize,
		duckDBFlushInterval:     fc.DuckDB.FlushInterval,
		duckDBWriterLoop:        fc.DuckDB.WriterLoop,
		duckDBWriterQueueSize:   fc.DuckDB.WriterQueueSize,
		duckDBUseAppender:       fc.DuckDB.UseAppender,
		duckDBWriteTimeout:      fc.DuckDB.WriteTimeout,
		duckDBRetryAttempts:     fc.DuckDB.RetryAttempts,
		duckDBRetryBackoff:      fc.DuckDB.RetryBackoff,
		duckDBCheckpointIntvl:   fc.DuckDB.CheckpointInterval,
		duckDBExportEnabled:     fc.DuckDB.Export.Enabled,
		duckDBExportFormat:      strings.ToLower(fc.DuckDB.Export.Format),
		duckDBExportInterval:    fc.DuckDB.Export.Interval,
		duckDBExportPath:        fc.DuckDB.Export.Path,
		duckDBSchema:            fc.DuckDB.Schema,
		duckDBColumnTypes:       fc.DuckDB.ColumnTypes,
		loggingLevel:            fc.Logging.Level,
		loggingFormat:           fc.Logging.Format,
		metricsPrometheus:       fc.Metrics.Prometheus,
		reliabilityMode:         strings.ToLower(fc.Reliability.Mode),
		spoolDir:                fc.Reliability.SpoolDir,
		spoolFile:               fc.Reliability.SpoolFile,
		maxSpoolBytes:           fc.Reliability.MaxSpoolBytes,
		spoolFsync:              fc.Reliability.Fsync,
		deliveryQueueSize:       fc.Reliability.DeliveryQueueSize,
		maxInflightRequests:     fc.Limits.MaxInflightRequests,
		maxInflightEvents:       fc.Limits.MaxInflightEvents,
		maxQueueBytes:           fc.Limits.MaxQueueBytes,
		maxEventBytes:           fc.Limits.MaxEventBytes,
		maxAttrCount:            fc.Limits.MaxAttrCount,
		maxAttrDepth:            fc.Limits.MaxAttrDepth,
		maxStringLength:         fc.Limits.MaxStringLength,
		mtlsAllowedCNs:          append([]string(nil), fc.Identity.MTLSAllowedCNs...),
		mtlsAllowedDNS:          append([]string(nil), fc.Identity.MTLSAllowedDNS...),
		mtlsAllowedEmails:       append([]string(nil), fc.Identity.MTLSAllowedEmails...),
		allowedOrigins:          append([]string(nil), fc.Identity.AllowedOrigins...),
		trustedProxies:          append([]string(nil), fc.Identity.TrustedProxies...),
		identityMode:            strings.ToLower(strings.TrimSpace(fc.Identity.Mode)),
		authIdentityWins:        fc.Identity.AuthIdentityWins,
		allowPayloadIdentity:    fc.Identity.AllowPayloadIdentity,
		boundServiceName:        strings.TrimSpace(fc.Identity.ServiceName),
		boundServiceVersion:     strings.TrimSpace(fc.Identity.ServiceVersion),
		boundEnvironment:        strings.TrimSpace(fc.Identity.DeploymentEnvironment),
		boundRegion:             strings.TrimSpace(fc.Identity.DeploymentRegion),
		boundTenantID:           strings.TrimSpace(fc.Identity.TenantID),
		boundWorkspaceID:        strings.TrimSpace(fc.Identity.WorkspaceID),
		boundOrganizationID:     strings.TrimSpace(fc.Identity.OrganizationID),
		privacyMode:             strings.ToLower(strings.TrimSpace(fc.Privacy.Mode)),
		collectorRedaction:      fc.Privacy.CollectorRedaction,
		emergencyRedaction:      fc.Privacy.EmergencyRedaction,
		privacyAllowlist:        append([]string(nil), fc.Privacy.Allowlist...),
		privacyBlocklist:        append([]string(nil), fc.Privacy.Blocklist...),
		secretScan:              fc.Privacy.SecretScan,
		rightToDeleteEnabled:    fc.Privacy.RightToDeleteEnabled,
		receiverRegistry:        append([]string(nil), fc.Components.Receivers...),
		processorRegistry:       append([]string(nil), fc.Components.Processors...),
		exporterRegistry:        append([]string(nil), fc.Components.Exporters...),
		extensionRegistry:       append([]string(nil), fc.Components.Extensions...),
		queueDir:                fc.Reliability.QueueDir,
		queueBatchSize:          fc.Reliability.QueueBatchSize,
		queueBatchTimeout:       fc.Reliability.QueueBatchTimeout,
		queueFlushInterval:      fc.Reliability.QueueFlushInterval,
		queueCircuitThreshold:   fc.Reliability.QueueCircuitThreshold,
		queueCircuitTimeout:     fc.Reliability.QueueCircuitTimeout,
		retryEnabled:            fc.Retry.Enabled,
		retryMaxAttempts:        fc.Retry.MaxAttempts,
		retryInitialBackoff:     fc.Retry.InitialBackoff,
		retryMaxBackoff:         fc.Retry.MaxBackoff,
		retryJitter:             fc.Retry.Jitter,
		dlqEnabled:              fc.DeadLetter.Enabled,
		dlqPath:                 fc.DeadLetter.Path,
		fanoutOutputs:           fanoutOutputsFromFile(fc.Fanout.Outputs),
		deliveryPolicy:          strings.ToLower(fc.Fanout.Delivery.Policy),
		fallbackEnabled:         fc.Fanout.Delivery.Fallback.Enabled,
		fallbackOnPrimaryFail:   fc.Fanout.Delivery.Fallback.OnPrimaryFailure,
		fallbackOnSecondaryFail: fc.Fanout.Delivery.Fallback.OnSecondaryFailure,
		fallbackOnPolicyFail:    fc.Fanout.Delivery.Fallback.OnPolicyFailure,
		dlqOnPrimaryFail:        fc.Fanout.Delivery.DLQ.OnPrimaryFailure,
		dlqOnSecondaryFail:      fc.Fanout.Delivery.DLQ.OnSecondaryFailure,
		dlqOnFallbackFail:       fc.Fanout.Delivery.DLQ.OnFallbackFailure,
		dlqOnPolicyFail:         fc.Fanout.Delivery.DLQ.OnPolicyFailure,
		dedupeEnabled:           fc.Dedupe.Enabled,
		dedupeKey:               fc.Dedupe.Key,
		dedupeWindow:            fc.Dedupe.Window,
		dedupeBackend:           strings.ToLower(fc.Dedupe.Backend),
		dedupeRedisAddr:         strings.TrimSpace(fc.Dedupe.RedisAddr),
		dedupeRedisPassword:     fc.Dedupe.RedisPassword,
		dedupeRedisDB:           fc.Dedupe.RedisDB,
		dedupeRedisPrefix:       strings.TrimSpace(fc.Dedupe.RedisPrefix),
		kafkaBrokers:            append([]string(nil), fc.Kafka.Brokers...),
		kafkaTopic:              strings.TrimSpace(fc.Kafka.Topic),
		kafkaAcks:               strings.ToLower(strings.TrimSpace(fc.Kafka.Acks)),
		kafkaRequestTimeout:     fc.Kafka.RequestTimeout,
		kafkaIdempotence:        fc.Kafka.EnableIdempotence,
		kafkaMaxRetries:         fc.Kafka.MaxRetries,
		kafkaRetryBackoff:       fc.Kafka.RetryBackoff,
		workerConsumerGroup:     strings.TrimSpace(fc.Worker.ConsumerGroup),
		workerPollTimeout:       fc.Worker.PollTimeout,
		schemaMode:              strings.ToLower(strings.TrimSpace(fc.Schema.Mode)),
		schemaSchemaVersion:     strings.TrimSpace(fc.Schema.SchemaVersion),
		schemaEventVersion:      strings.TrimSpace(fc.Schema.EventVersion),
		schemaQuarantinePath:    strings.TrimSpace(fc.Schema.QuarantinePath),
		schemaRegistryFile:      strings.TrimSpace(fc.Schema.RegistryFile),
		schemaRegistry:          fc.Schema.Registry,
		retentionEnabled:        fc.Retention.Enabled,
		retentionDays:           fc.Retention.Days,
		retentionMaxSize:        fc.Retention.MaxSize,
		cortexBridgeEnabled:     fc.CortexBridge.Enabled,
		cortexBridgeMode:        strings.ToLower(strings.TrimSpace(fc.CortexBridge.Mode)),
		cortexBridgeEndpoint:    strings.TrimSpace(fc.CortexBridge.Endpoint),
		cortexBridgeInsecure:    fc.CortexBridge.Insecure,
		cortexBridgeTimeout:     fc.CortexBridge.Timeout,
		cortexBridgeBatchSize:   fc.CortexBridge.BatchSize,
		cortexBridgeFlushIntvl:  fc.CortexBridge.FlushInterval,
		cortexBridgeQueueSize:   fc.CortexBridge.QueueSize,
		cortexBridgeHeader:      strings.TrimSpace(fc.CortexBridge.Header),
		cortexBridgeAPIKey:      fc.CortexBridge.APIKey,
		eventBusType:            strings.ToLower(strings.TrimSpace(fc.EventBus.Type)),
		eventBusTopic:           strings.TrimSpace(fc.EventBus.Topic),
		eventBusDLQTopic:        strings.TrimSpace(fc.EventBus.DLQTopic),
		eventBusConsumerGroup:   strings.TrimSpace(fc.EventBus.ConsumerGroup),
		eventBusMemoryBuffer:    fc.EventBus.Memory.BufferSize,
		eventBusRedisAddr:       strings.TrimSpace(fc.EventBus.Redis.Addr),
		eventBusRedisPassword:   fc.EventBus.Redis.Password,
		eventBusRedisDB:         fc.EventBus.Redis.DB,
		eventBusRedisStream:     strings.TrimSpace(fc.EventBus.Redis.Stream),
		eventBusRedisGroup:      strings.TrimSpace(fc.EventBus.Redis.Group),
		eventBusRedisMaxLen:     fc.EventBus.Redis.MaxLen,
		eventBusNATSURL:         strings.TrimSpace(fc.EventBus.NATS.URL),
		eventBusNATSStream:      strings.TrimSpace(fc.EventBus.NATS.Stream),
		eventBusNATSSubject:     strings.TrimSpace(fc.EventBus.NATS.Subject),
		eventBusNATSDurable:     strings.TrimSpace(fc.EventBus.NATS.Durable),
		eventBusKafkaBrokers:    append([]string(nil), fc.EventBus.Kafka.Brokers...),
		eventBusKafkaTopic:      strings.TrimSpace(fc.EventBus.Kafka.Topic),
		eventBusKafkaGroup:      strings.TrimSpace(fc.EventBus.Kafka.ConsumerGroup),
		eventBusKafkaAcks:       strings.ToLower(strings.TrimSpace(fc.EventBus.Kafka.Acks)),
		eventBusKafkaIdempotent: fc.EventBus.Kafka.EnableIdempotence,
		eventBusKafkaMaxRetries: fc.EventBus.Kafka.MaxRetries,
		eventBusKafkaCompress:   strings.ToLower(strings.TrimSpace(fc.EventBus.Kafka.Compression)),
	}
}

func fanoutOutputsFromFile(outputs []collectorconfig.FanoutOutputConfig) []collectorFanoutOutput {
	mapped := make([]collectorFanoutOutput, 0, len(outputs))
	for _, output := range outputs {
		mapped = append(mapped, collectorFanoutOutput{
			name:            strings.TrimSpace(output.Name),
			role:            strings.ToLower(strings.TrimSpace(output.Role)),
			sinkType:        strings.ToLower(strings.TrimSpace(output.Type)),
			enabled:         output.Enabled,
			duckDBPath:      strings.TrimSpace(output.DuckDB.Path),
			duckDBTable:     strings.TrimSpace(output.DuckDB.Table),
			duckDBRawColumn: strings.TrimSpace(output.DuckDB.RawColumn),
			duckDBStoreRaw:  output.DuckDB.StoreRaw,
			lokiURL:         strings.TrimSpace(output.Loki.URL),
			lokiTenantID:    strings.TrimSpace(output.Loki.TenantID),
			lokiLabels:      output.Loki.Labels,
			lokiBatchSize:   output.Loki.BatchSize,
			lokiFlushIntvl:  output.Loki.FlushInterval,
			lokiTimeout:     output.Loki.Timeout,
			otlpEndpoint:    strings.TrimSpace(output.OTLP.Endpoint),
			otlpInsecure:    output.OTLP.Insecure,
			otlpProtocol:    strings.ToLower(strings.TrimSpace(output.OTLP.Protocol)),
			otlpTimeout:     output.OTLP.Timeout,
			otlpLogger:      strings.TrimSpace(output.OTLP.Logger),
			chAddrs:         output.ClickHouse.Addrs,
			chDatabase:      strings.TrimSpace(output.ClickHouse.Database),
			chUsername:      strings.TrimSpace(output.ClickHouse.Username),
			chPassword:      output.ClickHouse.Password,
			chTable:         strings.TrimSpace(output.ClickHouse.Table),
			chSchema:        output.ClickHouse.Schema,
			chStoreRaw:      output.ClickHouse.StoreRaw,
			chRawColumn:     strings.TrimSpace(output.ClickHouse.RawColumn),
			pgDSN:           strings.TrimSpace(output.Postgres.DSN),
			pgTable:         strings.TrimSpace(output.Postgres.Table),
			pgSchema:        output.Postgres.Schema,
			pgStoreRaw:      output.Postgres.StoreRaw,
			pgRawColumn:     strings.TrimSpace(output.Postgres.RawColumn),
			s3Bucket:        strings.TrimSpace(output.S3.Bucket),
			s3Prefix:        strings.TrimSpace(output.S3.Prefix),
			s3Region:        strings.TrimSpace(output.S3.Region),
			s3Endpoint:      strings.TrimSpace(output.S3.Endpoint),
			s3AccessKey:     strings.TrimSpace(output.S3.AccessKey),
			s3SecretKey:     output.S3.SecretKey,
			s3Schema:        output.S3.Schema,
			s3BatchSize:     output.S3.BatchSize,
			s3FlushIntvl:    output.S3.FlushInterval,
			gcsBucket:       strings.TrimSpace(output.GCS.Bucket),
			gcsPrefix:       strings.TrimSpace(output.GCS.Prefix),
			gcsCredsFile:    strings.TrimSpace(output.GCS.CredentialsFile),
			gcsSchema:       output.GCS.Schema,
			gcsBatchSize:    output.GCS.BatchSize,
			gcsFlushIntvl:   output.GCS.FlushInterval,
		})
	}
	return mapped
}

func validTableName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !configIdentPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func parseConfigBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", raw)
	}
}

func marshalPrintableConfig(fc fileConfig) ([]byte, error) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(fc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Deprecated: mergeConfigFile is preserved for backward compatibility
func mergeConfigFile(dst *fileConfig, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extraDoc fileConfig
	if err := dec.Decode(&extraDoc); err != io.EOF {
		if err == nil {
			return errors.New("config file must contain a single YAML document")
		}
		return err
	}
	return nil
}
