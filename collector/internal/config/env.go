package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type EnvOverrideFunc func(string) error

func ApplyEnvOverrides(cfg *Config) error {
	setters := map[string]EnvOverrideFunc{
		"COLLECTOR_ADDR": func(v string) error { cfg.Collector.Addr = v; return nil },
		"COLLECTOR_READ_HEADER_TIMEOUT": func(v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			cfg.Collector.ReadHeaderTimeout = d
			return nil
		},
		"COLLECTOR_SHUTDOWN_TIMEOUT": func(v string) error {
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			cfg.Collector.ShutdownTimeout = d
			return nil
		},
		"COLLECTOR_MAX_BODY_BYTES": func(v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid int64: %w", err)
			}
			cfg.Collector.MaxBodyBytes = n
			return nil
		},
		"COLLECTOR_MAX_EVENTS": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid int: %w", err)
			}
			cfg.Collector.MaxEventsPerReq = n
			return nil
		},
		"COLLECTOR_API_KEY_HEADER":    func(v string) error { cfg.Auth.Header = v; return nil },
		"COLLECTOR_API_KEY":           func(v string) error { cfg.Auth.Value = v; cfg.Auth.Enabled = true; return nil },
		"LOXA_STORAGE_ENCRYPTION_KEY": func(v string) error { cfg.Storage.EncryptionKey = v; return nil },

		"COLLECTOR_RATE_LIMIT_ENABLED": boolSetter(&cfg.RateLimit.Enabled),
		"COLLECTOR_RATE_LIMIT_RPS": func(v string) error {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("invalid float: %w", err)
			}
			cfg.RateLimit.RPS = f
			return nil
		},
		"COLLECTOR_RATE_LIMIT_BURST": intSetter(&cfg.RateLimit.Burst),

		"DUCKDB_PATH":                   func(v string) error { cfg.DuckDB.Path = v; return nil },
		"DUCKDB_DRIVER":                 func(v string) error { cfg.DuckDB.Driver = v; return nil },
		"DUCKDB_TABLE":                  func(v string) error { cfg.DuckDB.Table = v; return nil },
		"DUCKDB_RAW_COLUMN":             func(v string) error { cfg.DuckDB.RawColumn = v; return nil },
		"DUCKDB_BATCH_SIZE":             intSetter(&cfg.DuckDB.BatchSize),
		"DUCKDB_FLUSH_INTERVAL":         durationSetter(&cfg.DuckDB.FlushInterval),
		"DUCKDB_CHECKPOINT_ON_SHUTDOWN": boolSetter(&cfg.DuckDB.CheckpointOnShutdown),
		"DUCKDB_CHECKPOINT_INTERVAL":    durationSetter(&cfg.DuckDB.CheckpointInterval),
		"DUCKDB_WRITER_LOOP":            boolSetter(&cfg.DuckDB.WriterLoop),
		"DUCKDB_WRITER_QUEUE_SIZE":      intSetter(&cfg.DuckDB.WriterQueueSize),
		"DUCKDB_USE_APPENDER":           boolSetter(&cfg.DuckDB.UseAppender),
		"DUCKDB_WRITE_TIMEOUT":          durationSetter(&cfg.DuckDB.WriteTimeout),
		"DUCKDB_RETRY_ATTEMPTS":         intSetter(&cfg.DuckDB.RetryAttempts),
		"DUCKDB_RETRY_BACKOFF":          durationSetter(&cfg.DuckDB.RetryBackoff),

		"DUCKDB_EXPORT_ENABLED":  boolSetter(&cfg.DuckDB.Export.Enabled),
		"DUCKDB_EXPORT_FORMAT":   func(v string) error { cfg.DuckDB.Export.Format = v; return nil },
		"DUCKDB_EXPORT_INTERVAL": durationSetter(&cfg.DuckDB.Export.Interval),
		"DUCKDB_EXPORT_PATH":     func(v string) error { cfg.DuckDB.Export.Path = v; return nil },

		"COLLECTOR_KAFKA_BROKERS": csvSetter(&cfg.Kafka.Brokers),
		"COLLECTOR_KAFKA_TOPIC":   func(v string) error { cfg.Kafka.Topic = v; return nil },

		"LOXA_EVENTBUS":        func(v string) error { cfg.EventBus.Type = v; return nil },
		"LOXA_EVENTBUS_TOPIC":  func(v string) error { cfg.EventBus.Topic = v; return nil },
		"LOXA_EVENTBUS_GROUP":  func(v string) error { cfg.EventBus.ConsumerGroup = v; return nil },
		"LOXA_EVENTBUS_DLQ":    func(v string) error { cfg.EventBus.DLQTopic = v; return nil },
		"LOXA_EVENTBUS_NATS_URL":     func(v string) error { cfg.EventBus.NATS.URL = v; return nil },
		"LOXA_EVENTBUS_NATS_STREAM":  func(v string) error { cfg.EventBus.NATS.Stream = v; return nil },
		"LOXA_EVENTBUS_REDIS_ADDR":     func(v string) error { cfg.EventBus.Redis.Addr = v; return nil },
		"LOXA_EVENTBUS_REDIS_PASSWORD": func(v string) error { cfg.EventBus.Redis.Password = v; return nil },
		"LOXA_EVENTBUS_REDIS_DB": func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid int: %w", err)
			}
			cfg.EventBus.Redis.DB = n
			return nil
		},
		"LOXA_KAFKA_BROKERS":         csvSetter(&cfg.EventBus.Kafka.Brokers),
		"LOXA_KAFKA_TOPIC":           func(v string) error { cfg.EventBus.Kafka.Topic = v; return nil },
		"LOXA_KAFKA_GROUP":           func(v string) error { cfg.EventBus.Kafka.ConsumerGroup = v; return nil },

		"LOXA_WORKER_CONSUMER_GROUP": func(v string) error { cfg.Worker.ConsumerGroup = v; return nil },
		"LOXA_WORKER_POLL_TIMEOUT":   durationSetter(&cfg.Worker.PollTimeout),

		"COLLECTOR_RELIABILITY_MODE": func(v string) error { cfg.Reliability.Mode = v; return nil },
		"COLLECTOR_SPOOL_DIR":        func(v string) error { cfg.Reliability.SpoolDir = v; return nil },
		"COLLECTOR_SPOOL_FILE":       func(v string) error { cfg.Reliability.SpoolFile = v; return nil },
		"COLLECTOR_MAX_SPOOL_BYTES": func(v string) error {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid int64: %w", err)
			}
			cfg.Reliability.MaxSpoolBytes = n
			return nil
		},
		"COLLECTOR_SPOOL_FSYNC":         boolSetter(&cfg.Reliability.Fsync),
		"COLLECTOR_DELIVERY_QUEUE_SIZE": intSetter(&cfg.Reliability.DeliveryQueueSize),

		"COLLECTOR_RETRY_ENABLED":         boolSetter(&cfg.Retry.Enabled),
		"COLLECTOR_RETRY_MAX_ATTEMPTS":    intSetter(&cfg.Retry.MaxAttempts),
		"COLLECTOR_RETRY_INITIAL_BACKOFF": durationSetter(&cfg.Retry.InitialBackoff),
		"COLLECTOR_RETRY_MAX_BACKOFF":     durationSetter(&cfg.Retry.MaxBackoff),
		"COLLECTOR_RETRY_JITTER":          boolSetter(&cfg.Retry.Jitter),

		"COLLECTOR_DLQ_ENABLED": boolSetter(&cfg.DeadLetter.Enabled),
		"COLLECTOR_DLQ_PATH":    func(v string) error { cfg.DeadLetter.Path = v; return nil },

		"COLLECTOR_DELIVERY_POLICY": func(v string) error { cfg.Fanout.Delivery.Policy = v; return nil },

		"COLLECTOR_FANOUT_FALLBACK_ENABLED":              boolSetter(&cfg.Fanout.Delivery.Fallback.Enabled),
		"COLLECTOR_FANOUT_FALLBACK_ON_PRIMARY_FAILURE":   boolSetter(&cfg.Fanout.Delivery.Fallback.OnPrimaryFailure),
		"COLLECTOR_FANOUT_FALLBACK_ON_SECONDARY_FAILURE": boolSetter(&cfg.Fanout.Delivery.Fallback.OnSecondaryFailure),
		"COLLECTOR_FANOUT_FALLBACK_ON_POLICY_FAILURE":    boolSetter(&cfg.Fanout.Delivery.Fallback.OnPolicyFailure),

		"COLLECTOR_FANOUT_DLQ_ON_PRIMARY_FAILURE":   boolSetter(&cfg.Fanout.Delivery.DLQ.OnPrimaryFailure),
		"COLLECTOR_FANOUT_DLQ_ON_SECONDARY_FAILURE": boolSetter(&cfg.Fanout.Delivery.DLQ.OnSecondaryFailure),
		"COLLECTOR_FANOUT_DLQ_ON_FALLBACK_FAILURE":  boolSetter(&cfg.Fanout.Delivery.DLQ.OnFallbackFailure),
		"COLLECTOR_FANOUT_DLQ_ON_POLICY_FAILURE":    boolSetter(&cfg.Fanout.Delivery.DLQ.OnPolicyFailure),

		"COLLECTOR_DEDUPE_ENABLED":        boolSetter(&cfg.Dedupe.Enabled),
		"COLLECTOR_DEDUPE_KEY":            func(v string) error { cfg.Dedupe.Key = v; return nil },
		"COLLECTOR_DEDUPE_WINDOW":         durationSetter(&cfg.Dedupe.Window),
		"COLLECTOR_DEDUPE_BACKEND":        func(v string) error { cfg.Dedupe.Backend = v; return nil },
		"COLLECTOR_DEDUPE_REDIS_ADDR":     func(v string) error { cfg.Dedupe.RedisAddr = v; return nil },
		"COLLECTOR_DEDUPE_REDIS_PASSWORD": func(v string) error { cfg.Dedupe.RedisPassword = v; return nil },
		"COLLECTOR_DEDUPE_REDIS_DB":       intSetter(&cfg.Dedupe.RedisDB),
		"COLLECTOR_DEDUPE_REDIS_PREFIX":   func(v string) error { cfg.Dedupe.RedisPrefix = v; return nil },

		// Fanout sink secrets — applied to ALL outputs of matching type
		// (FanoutConfig.Outputs is an array; these iterate over all outputs)
		"COLLECTOR_FANOUT_CLICKHOUSE_PASSWORD": func(v string) error {
			for i := range cfg.Fanout.Outputs {
				if cfg.Fanout.Outputs[i].Type == "clickhouse" || len(cfg.Fanout.Outputs[i].ClickHouse.Addrs) > 0 {
					cfg.Fanout.Outputs[i].ClickHouse.Password = v
				}
			}
			return nil
		},
		"COLLECTOR_FANOUT_S3_ACCESS_KEY": func(v string) error {
			for i := range cfg.Fanout.Outputs {
				if cfg.Fanout.Outputs[i].Type == "s3" || cfg.Fanout.Outputs[i].S3.Bucket != "" {
					cfg.Fanout.Outputs[i].S3.AccessKey = v
				}
			}
			return nil
		},
		"COLLECTOR_FANOUT_S3_SECRET_KEY": func(v string) error {
			for i := range cfg.Fanout.Outputs {
				if cfg.Fanout.Outputs[i].Type == "s3" || cfg.Fanout.Outputs[i].S3.Bucket != "" {
					cfg.Fanout.Outputs[i].S3.SecretKey = v
				}
			}
			return nil
		},
		"COLLECTOR_FANOUT_POSTGRES_DSN": func(v string) error {
			for i := range cfg.Fanout.Outputs {
				if cfg.Fanout.Outputs[i].Type == "postgres" || cfg.Fanout.Outputs[i].Postgres.DSN != "" {
					cfg.Fanout.Outputs[i].Postgres.DSN = v
				}
			}
			return nil
		},

		"COLLECTOR_ALLOW_LOCAL_DEV_KEYS": boolSetter(&cfg.Auth.AllowLocalDevKeys),

		// Cortex bridge API key
		"COLLECTOR_CORTEX_BRIDGE_API_KEY": func(v string) error { cfg.CortexBridge.APIKey = v; return nil },

		// mTLS allowlists
		"COLLECTOR_MTLS_ALLOWED_CNS":    csvSetter(&cfg.Identity.MTLSAllowedCNs),
		"COLLECTOR_MTLS_ALLOWED_DNS":    csvSetter(&cfg.Identity.MTLSAllowedDNS),
		"COLLECTOR_MTLS_ALLOWED_EMAILS": csvSetter(&cfg.Identity.MTLSAllowedEmails),
		"COLLECTOR_ALLOWED_ORIGINS":     csvSetter(&cfg.Identity.AllowedOrigins),
		"COLLECTOR_TRUSTED_PROXIES":     csvSetter(&cfg.Identity.TrustedProxies),

		// EventBus NATS credentials
		"LOXA_EVENTBUS_NATS_USERNAME": func(v string) error { cfg.EventBus.NATS.Username = v; return nil },
		"LOXA_EVENTBUS_NATS_PASSWORD": func(v string) error { cfg.EventBus.NATS.Password = v; return nil },
	}

	for key, setter := range setters {
		if val := os.Getenv(key); val != "" {
			if err := setter(val); err != nil {
				return fmt.Errorf("env override %s: %w", key, err)
			}
		}
	}

	if cfg.Auth.Value == "" && cfg.Auth.ValueEnv != "" {
		cfg.Auth.Value = os.Getenv(cfg.Auth.ValueEnv)
		if cfg.Auth.Value != "" {
			cfg.Auth.Enabled = true
		}
	}
	if cfg.Storage.EncryptionKey == "" && cfg.Storage.EncryptionKeyEnv != "" {
		cfg.Storage.EncryptionKey = os.Getenv(cfg.Storage.EncryptionKeyEnv)
	}

	return nil
}

func boolSetter(dst *bool) EnvOverrideFunc {
	return func(v string) error {
		val := strings.ToLower(strings.TrimSpace(v))
		switch val {
		case "1", "true", "yes", "on":
			*dst = true
		case "0", "false", "no", "off":
			*dst = false
		default:
			return fmt.Errorf("invalid bool: %q", v)
		}
		return nil
	}
}

func intSetter(dst *int) EnvOverrideFunc {
	return func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid int: %w", err)
		}
		*dst = n
		return nil
	}
}

func durationSetter(dst *time.Duration) EnvOverrideFunc {
	return func(v string) error {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		*dst = d
		return nil
	}
}

func csvSetter(dst *[]string) EnvOverrideFunc {
	return func(v string) error {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		*dst = out
		return nil
	}
}

