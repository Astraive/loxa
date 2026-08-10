package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	collectorconfig "github.com/astraive/loza/collector/internal/config"
	"github.com/astraive/loza/collector/internal/eventbus"
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
	eventBusType            string
	eventBusTopic           string
	eventBusDLQTopic        string
	eventBusConsumerGroup   string
	eventBusMemoryBuffer    int
	eventBusRedisAddr       string
	eventBusRedisPassword   string
	eventBusRedisDB         int
	eventBusRedisStream     string
	eventBusRedisGroup      string
	eventBusRedisMaxLen     int64
	eventBusNATSURL         string
	eventBusNATSStream      string
	eventBusNATSSubject     string
	eventBusNATSDurable     string
	eventBusKafkaBrokers    []string
	eventBusKafkaTopic      string
	eventBusKafkaGroup      string
	eventBusKafkaAcks       string
	eventBusKafkaIdempotent bool
	eventBusKafkaMaxRetries int
	eventBusKafkaCompress   string
}

func (c *workerConfig) buildEventBusConfig() eventbus.Config {
	cfg := eventbus.Config{
		Type:          c.eventBusType,
		Topic:         c.eventBusTopic,
		DLQTopic:      c.eventBusDLQTopic,
		ConsumerGroup: c.eventBusConsumerGroup,
		Memory: eventbus.MemoryConfig{
			BufferSize: c.eventBusMemoryBuffer,
		},
		Redis: eventbus.RedisConfig{
			Addr:     c.eventBusRedisAddr,
			Password: c.eventBusRedisPassword,
			DB:       c.eventBusRedisDB,
			Stream:   c.eventBusRedisStream,
			Group:    c.eventBusRedisGroup,
			MaxLen:   c.eventBusRedisMaxLen,
		},
		NATS: eventbus.NATSConfig{
			URL:     c.eventBusNATSURL,
			Stream:  c.eventBusNATSStream,
			Subject: c.eventBusNATSSubject,
			Durable: c.eventBusNATSDurable,
		},
		Kafka: eventbus.KafkaConfig{
			Brokers:           c.eventBusKafkaBrokers,
			Topic:             c.eventBusKafkaTopic,
			ConsumerGroup:     c.eventBusKafkaGroup,
			Acks:              c.eventBusKafkaAcks,
			EnableIdempotence: c.eventBusKafkaIdempotent,
			MaxRetries:        c.eventBusKafkaMaxRetries,
			Compression:       c.eventBusKafkaCompress,
		},
	}
	if cfg.Topic == "" {
		cfg.Topic = "loza.events.raw"
	}
	if cfg.ConsumerGroup == "" {
		cfg.ConsumerGroup = "loza-worker"
	}
	return cfg
}

func loadWorkerConfigFromArgs(args []string) (workerConfig, error) {
	cfgFile := "loza.yaml"
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
	setStringLower := func(key string, dst *string) {
		if v, ok := get(key); ok {
			*dst = strings.ToLower(v)
		}
	}

	setString("DUCKDB_PATH", &fc.DuckDB.Path)
	setCSV("COLLECTOR_KAFKA_BROKERS", &fc.Kafka.Brokers)
	setString("COLLECTOR_KAFKA_TOPIC", &fc.Kafka.Topic)
	setString("COLLECTOR_DEDUPE_REDIS_ADDR", &fc.Dedupe.RedisAddr)
	setString("COLLECTOR_DEDUPE_REDIS_PASSWORD", &fc.Dedupe.RedisPassword)
	setString("LOZA_WORKER_CONSUMER_GROUP", &fc.Worker.ConsumerGroup)
	if err := setDuration("LOZA_WORKER_POLL_TIMEOUT", &fc.Worker.PollTimeout); err != nil {
		return err
	}
	// Eventbus env overrides
	setStringLower("LOZA_EVENTBUS", &fc.EventBus.Type)
	setString("LOZA_EVENTBUS_TOPIC", &fc.EventBus.Topic)
	setString("LOZA_EVENTBUS_DLQ_TOPIC", &fc.EventBus.DLQTopic)
	setString("LOZA_EVENTBUS_GROUP", &fc.EventBus.ConsumerGroup)
	if err := setInt("LOZA_EVENTBUS_MEMORY_BUFFER", &fc.EventBus.Memory.BufferSize); err != nil {
		return err
	}
	setString("LOZA_EVENTBUS_REDIS_ADDR", &fc.EventBus.Redis.Addr)
	setString("LOZA_EVENTBUS_REDIS_PASSWORD", &fc.EventBus.Redis.Password)
	if err := setInt("LOZA_EVENTBUS_REDIS_DB", &fc.EventBus.Redis.DB); err != nil {
		return err
	}
	setString("LOZA_EVENTBUS_REDIS_STREAM", &fc.EventBus.Redis.Stream)
	setString("LOZA_EVENTBUS_REDIS_GROUP", &fc.EventBus.Redis.Group)
	if err := setInt64("LOZA_EVENTBUS_REDIS_MAX_LEN", &fc.EventBus.Redis.MaxLen); err != nil {
		return err
	}
	setString("LOZA_EVENTBUS_NATS_URL", &fc.EventBus.NATS.URL)
	setString("LOZA_EVENTBUS_NATS_STREAM", &fc.EventBus.NATS.Stream)
	setString("LOZA_EVENTBUS_NATS_SUBJECT", &fc.EventBus.NATS.Subject)
	setString("LOZA_EVENTBUS_NATS_DURABLE", &fc.EventBus.NATS.Durable)
	setCSV("LOZA_EVENTBUS_KAFKA_BROKERS", &fc.EventBus.Kafka.Brokers)
	setString("LOZA_EVENTBUS_KAFKA_TOPIC", &fc.EventBus.Kafka.Topic)
	setString("LOZA_EVENTBUS_KAFKA_GROUP", &fc.EventBus.Kafka.ConsumerGroup)
	setCSV("LOZA_KAFKA_BROKERS", &fc.EventBus.Kafka.Brokers)
	setString("LOZA_KAFKA_TOPIC", &fc.EventBus.Kafka.Topic)
	setString("LOZA_KAFKA_GROUP", &fc.EventBus.Kafka.ConsumerGroup)
	return nil
}

func validateWorkerConfig(fc workerFileConfig) error {
	// Skip kafka.brokers validation when eventbus is configured
	busType := strings.ToLower(strings.TrimSpace(fc.EventBus.Type))
	if busType == "" {
		if len(fc.Kafka.Brokers) == 0 {
			return errors.New("kafka.brokers must include at least one broker")
		}
		for i, broker := range fc.Kafka.Brokers {
			if strings.TrimSpace(broker) == "" {
				return fmt.Errorf("kafka.brokers[%d] must not be empty", i)
			}
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
