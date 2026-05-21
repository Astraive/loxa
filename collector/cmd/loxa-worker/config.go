package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	collectorconfig "github.com/astraive/loxa-collector/internal/config"
	"gopkg.in/yaml.v3"
)

var (
	workerConfigIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type workerFileConfig = collectorconfig.Config

type workerConfig struct {
	shutdownTimeout         time.Duration
	storageEncryptionKey    string
	duckDBPath              string
	duckDBDriver            string
	duckDBTable             string
	duckDBRawColumn         string
	duckDBStoreRaw          bool
	duckDBMaxOpenConns      int
	duckDBMaxIdleConns      int
	duckDBBatchSize         int
	duckDBFlushInterval     time.Duration
	duckDBWriterLoop        bool
	duckDBWriterQueueSize   int
	duckDBSchema            map[string]string
	duckDBColumnTypes       map[string]string
	kafkaBrokers            []string
	kafkaTopic              string
	workerConsumerGroup     string
	workerPollTimeout       time.Duration
	retryEnabled            bool
	retryMaxAttempts        int
	retryInitialBackoff     time.Duration
	retryMaxBackoff         time.Duration
	retryJitter             bool
	dlqEnabled              bool
	dlqPath                 string
	fanoutOutputs           []workerFanoutOutput
	deliveryPolicy          string
	fallbackEnabled         bool
	fallbackOnPrimaryFail   bool
	fallbackOnSecondaryFail bool
	fallbackOnPolicyFail    bool
	dlqOnPrimaryFail        bool
	dlqOnSecondaryFail      bool
	dlqOnFallbackFail       bool
	dlqOnPolicyFail         bool
	dedupeEnabled           bool
	dedupeKey               string
	dedupeWindow            time.Duration
	dedupeBackend           string
	dedupeRedisAddr         string
	dedupeRedisPassword     string
	dedupeRedisDB           int
	dedupeRedisPrefix       string
}

func loadWorkerConfigFromArgs(args []string) (workerConfig, error) {
	cfgFile := "loxa.yaml"
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&cfgFile, "c", cfgFile, "path to config file")
	if err := fs.Parse(args); err != nil {
		return workerConfig{}, err
	}

	fc := collectorconfig.Default()
	if _, err := os.Stat(cfgFile); err == nil {
		if err := collectorconfig.LoadFile(&fc, cfgFile); err != nil {
			return workerConfig{}, err
		}
	}
	if err := applyWorkerEnvOverrides(&fc); err != nil {
		return workerConfig{}, err
	}
	if fc.Storage.EncryptionKey == "" && strings.TrimSpace(fc.Storage.EncryptionKeyEnv) != "" {
		fc.Storage.EncryptionKey = os.Getenv(fc.Storage.EncryptionKeyEnv)
	}
	if err := validateWorkerConfig(fc); err != nil {
		return workerConfig{}, err
	}
	return workerRuntimeConfig(fc), nil
}

func mergeWorkerConfigFile(dst *workerFileConfig, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extraDoc workerFileConfig
	if err := dec.Decode(&extraDoc); err != io.EOF {
		if err == nil {
			return errors.New("config file must contain a single YAML document")
		}
		return err
	}
	return nil
}

func applyWorkerEnvOverrides(fc *workerFileConfig) error {
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
	setString := func(key string, dst *string) {
		if v, ok := get(key); ok {
			*dst = v
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

	setString("DUCKDB_PATH", &fc.DuckDB.Path)
	setCSV("COLLECTOR_KAFKA_BROKERS", &fc.Kafka.Brokers)
	setString("COLLECTOR_KAFKA_TOPIC", &fc.Kafka.Topic)
	setString("COLLECTOR_DEDUPE_REDIS_ADDR", &fc.Dedupe.RedisAddr)
	setString("COLLECTOR_DEDUPE_REDIS_PASSWORD", &fc.Dedupe.RedisPassword)
	setString("LOXA_WORKER_CONSUMER_GROUP", &fc.Worker.ConsumerGroup)
	if err := setDuration("LOXA_WORKER_POLL_TIMEOUT", &fc.Worker.PollTimeout); err != nil {
		return err
	}
	return nil
}

func validateWorkerConfig(fc workerFileConfig) error {
	if len(fc.Kafka.Brokers) == 0 {
		return errors.New("kafka.brokers must include at least one broker")
	}
	for i, broker := range fc.Kafka.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("kafka.brokers[%d] must not be empty", i)
		}
	}
	if strings.TrimSpace(fc.Kafka.Topic) == "" {
		return errors.New("kafka.topic must not be empty")
	}
	if strings.TrimSpace(fc.Worker.ConsumerGroup) == "" {
		return errors.New("worker.consumer_group must not be empty")
	}
	if fc.Worker.PollTimeout <= 0 {
		return errors.New("worker.poll_timeout must be > 0")
	}
	if strings.TrimSpace(fc.DuckDB.Path) == "" {
		return errors.New("duckdb.path must not be empty")
	}
	if strings.TrimSpace(fc.DuckDB.Driver) == "" {
		return errors.New("duckdb.driver must not be empty")
	}
	if !validWorkerTableName(fc.DuckDB.Table) {
		return fmt.Errorf("invalid duckdb.table %q", fc.DuckDB.Table)
	}
	if !workerConfigIdentPattern.MatchString(fc.DuckDB.RawColumn) {
		return fmt.Errorf("invalid duckdb.raw_column %q", fc.DuckDB.RawColumn)
	}
	if fc.Retry.Enabled {
		if fc.Retry.MaxAttempts <= 0 {
			return errors.New("retry.max_attempts must be > 0")
		}
		if fc.Retry.InitialBackoff <= 0 || fc.Retry.MaxBackoff <= 0 {
			return errors.New("retry backoff durations must be > 0")
		}
	}
	if fc.DeadLetter.Enabled && strings.TrimSpace(fc.DeadLetter.Path) == "" {
		return errors.New("dead_letter.path must not be empty when dead_letter.enabled is true")
	}
	if fc.Dedupe.Enabled {
		switch strings.ToLower(strings.TrimSpace(fc.Dedupe.Backend)) {
		case "", "memory":
		case "redis":
			if strings.TrimSpace(fc.Dedupe.RedisAddr) == "" {
				return errors.New("dedupe.redis_addr must be configured when dedupe.backend is redis")
			}
		default:
			return errors.New("dedupe.backend must be memory or redis")
		}
	}
	return nil
}

func workerRuntimeConfig(fc workerFileConfig) workerConfig {
	return workerConfig{
		shutdownTimeout:         fc.Collector.ShutdownTimeout,
		storageEncryptionKey:    fc.Storage.EncryptionKey,
		duckDBPath:              fc.DuckDB.Path,
		duckDBDriver:            fc.DuckDB.Driver,
		duckDBTable:             fc.DuckDB.Table,
		duckDBRawColumn:         fc.DuckDB.RawColumn,
		duckDBStoreRaw:          fc.DuckDB.StoreRaw,
		duckDBMaxOpenConns:      fc.DuckDB.MaxOpenConns,
		duckDBMaxIdleConns:      fc.DuckDB.MaxIdleConns,
		duckDBBatchSize:         fc.DuckDB.BatchSize,
		duckDBFlushInterval:     fc.DuckDB.FlushInterval,
		duckDBWriterLoop:        fc.DuckDB.WriterLoop,
		duckDBWriterQueueSize:   fc.DuckDB.WriterQueueSize,
		duckDBSchema:            fc.DuckDB.Schema,
		duckDBColumnTypes:       fc.DuckDB.ColumnTypes,
		kafkaBrokers:            append([]string(nil), fc.Kafka.Brokers...),
		kafkaTopic:              strings.TrimSpace(fc.Kafka.Topic),
		workerConsumerGroup:     strings.TrimSpace(fc.Worker.ConsumerGroup),
		workerPollTimeout:       fc.Worker.PollTimeout,
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
	}
}

func validWorkerTableName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !workerConfigIdentPattern.MatchString(part) {
			return false
		}
	}
	return true
}
