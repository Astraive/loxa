package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	collectorconfig "github.com/astraive/loxa-collector/internal/config"
	collectorevent "github.com/astraive/loxa-collector/internal/event"
	processing "github.com/astraive/loxa-collector/internal/processing"
	"github.com/astraive/loxa-collector/internal/sinks/clickhouse"
	"github.com/astraive/loxa-collector/internal/sinks/duckdb"
	"github.com/astraive/loxa-collector/internal/sinks/gcs"
	"github.com/astraive/loxa-collector/internal/sinks/loki"
	"github.com/astraive/loxa-collector/internal/sinks/otlp"
	"github.com/astraive/loxa-collector/internal/sinks/postgres"
	"github.com/astraive/loxa-collector/internal/sinks/s3"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3svc "github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/api/option"
)

const (
	fanoutRoleSecondary  = "secondary"
	fanoutRoleFallback   = "fallback"
	fanoutSinkDuckDB     = "duckdb"
	fanoutSinkLoki       = "loki"
	fanoutSinkOTLP       = "otlp"
	fanoutSinkClickHouse = "clickhouse"
	fanoutSinkPostgres   = "postgres"
	fanoutSinkS3         = "s3"
	fanoutSinkGCS        = "gcs"
)

type workerFanoutOutput struct {
	name            string
	role            string
	sinkType        string
	enabled         bool
	duckDBPath      string
	duckDBTable     string
	duckDBRawColumn string
	duckDBStoreRaw  *bool
	lokiURL         string
	lokiTenantID    string
	lokiLabels      map[string]string
	lokiBatchSize   int
	lokiFlushIntvl  time.Duration
	lokiTimeout     time.Duration
	otlpEndpoint    string
	otlpInsecure    bool
	otlpProtocol    string
	otlpTimeout     time.Duration
	otlpLogger      string
	chAddrs         []string
	chDatabase      string
	chUsername      string
	chPassword      string
	chTable         string
	chSchema        map[string]string
	chStoreRaw      bool
	chRawColumn     string
	pgDSN           string
	pgTable         string
	pgSchema        map[string]string
	pgStoreRaw      bool
	pgRawColumn     string
	s3Bucket        string
	s3Prefix        string
	s3Region        string
	s3Endpoint      string
	s3AccessKey     string
	s3SecretKey     string
	s3Schema        map[string]string
	s3BatchSize     int
	s3FlushIntvl    time.Duration
	gcsBucket       string
	gcsPrefix       string
	gcsCredsFile    string
	gcsSchema       map[string]string
	gcsBatchSize    int
	gcsFlushIntvl   time.Duration
}

func fanoutOutputsFromFile(outputs []collectorconfig.FanoutOutputConfig) []workerFanoutOutput {
	mapped := make([]workerFanoutOutput, 0, len(outputs))
	for _, output := range outputs {
		mapped = append(mapped, workerFanoutOutput{
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

func createFanoutSinks(cfg workerConfig) ([]processing.NamedSink, *processing.NamedSink, []*sql.DB, error) {
	secondary := make([]processing.NamedSink, 0, len(cfg.fanoutOutputs))
	var fallback *processing.NamedSink
	dbs := make([]*sql.DB, 0, len(cfg.fanoutOutputs))

	for _, output := range cfg.fanoutOutputs {
		if !output.enabled {
			continue
		}

		var (
			sink collectorevent.Sink
			db   *sql.DB
			err  error
		)

		sinkType := output.sinkType
		if sinkType == "" {
			sinkType = fanoutSinkDuckDB
		}

		switch sinkType {
		case fanoutSinkDuckDB:
			sink, db, err = newDuckDBFanoutSink(cfg, output)
		case fanoutSinkLoki:
			sink, err = newLokiFanoutSink(output)
		case fanoutSinkOTLP:
			sink, err = newOTLPFanoutSink(output)
		case fanoutSinkClickHouse:
			sink, err = newClickHouseFanoutSink(cfg, output)
		case fanoutSinkPostgres:
			sink, err = newPostgresFanoutSink(cfg, output)
		case fanoutSinkS3:
			sink, err = newS3FanoutSink(cfg, output)
		case fanoutSinkGCS:
			sink, err = newGCSFanoutSink(cfg, output)
		default:
			for _, fanoutDB := range dbs {
				_ = fanoutDB.Close()
			}
			return nil, nil, nil, fmt.Errorf("fanout output %q: unsupported type %q", output.name, sinkType)
		}

		if err != nil {
			for _, fanoutDB := range dbs {
				_ = fanoutDB.Close()
			}
			return nil, nil, nil, err
		}
		if db != nil {
			dbs = append(dbs, db)
		}

		named := processing.NamedSink{Name: output.name, Sink: sink}
		if output.role == fanoutRoleFallback {
			fallback = &named
			continue
		}
		secondary = append(secondary, named)
	}

	return secondary, fallback, dbs, nil
}

func ensureSchema(db *sql.DB, cfg workerConfig) error {
	columns := make([]string, 0, len(cfg.duckDBSchema)+1)
	for col, path := range cfg.duckDBSchema {
		if typ, ok := cfg.duckDBColumnTypes[path]; ok {
			columns = append(columns, fmt.Sprintf("%s %s", col, typ))
		} else {
			columns = append(columns, fmt.Sprintf("%s TEXT", col))
		}
	}
	if cfg.duckDBStoreRaw {
		columns = append(columns, fmt.Sprintf("%s TEXT", cfg.duckDBRawColumn))
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", cfg.duckDBTable, strings.Join(columns, ", "))
	_, err := db.Exec(query)
	return err
}

func newDuckDBFanoutSink(cfg workerConfig, output workerFanoutOutput) (collectorevent.Sink, *sql.DB, error) {
	path := strings.TrimSpace(output.duckDBPath)
	if path == "" {
		return nil, nil, fmt.Errorf("fanout output %q: duckdb.path must not be empty", output.name)
	}
	table := strings.TrimSpace(output.duckDBTable)
	if table == "" {
		table = cfg.duckDBTable
	}
	rawColumn := strings.TrimSpace(output.duckDBRawColumn)
	if rawColumn == "" {
		rawColumn = cfg.duckDBRawColumn
	}
	storeRaw := cfg.duckDBStoreRaw
	if output.duckDBStoreRaw != nil {
		storeRaw = *output.duckDBStoreRaw
	}

	db, err := sql.Open(cfg.duckDBDriver, path)
	if err != nil {
		return nil, nil, fmt.Errorf("fanout output %q: open duckdb: %w", output.name, err)
	}
	db.SetMaxOpenConns(cfg.duckDBMaxOpenConns)
	db.SetMaxIdleConns(cfg.duckDBMaxIdleConns)

	schemaCfg := cfg
	schemaCfg.duckDBTable = table
	schemaCfg.duckDBRawColumn = rawColumn
	schemaCfg.duckDBStoreRaw = storeRaw
	if err := ensureSchema(db, schemaCfg); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("fanout output %q: ensure schema: %w", output.name, err)
	}

	sink, err := duckdb.New(duckdb.Config{
		DB:              db,
		Driver:          cfg.duckDBDriver,
		Table:           table,
		StoreRaw:        storeRaw,
		RawColumn:       rawColumn,
		Schema:          cfg.duckDBSchema,
		BatchSize:       cfg.duckDBBatchSize,
		FlushInterval:   cfg.duckDBFlushInterval,
		WriterLoop:      cfg.duckDBWriterLoop,
		WriterQueueSize: cfg.duckDBWriterQueueSize,
		EncryptRaw:      true,
		EncryptKey:      cfg.storageEncryptionKey,
	})
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("fanout output %q: create sink: %w", output.name, err)
	}
	return sink, db, nil
}

func newLokiFanoutSink(output workerFanoutOutput) (collectorevent.Sink, error) {
	if strings.TrimSpace(output.lokiURL) == "" {
		return nil, fmt.Errorf("fanout output %q: loki.url must not be empty", output.name)
	}
	return loki.New(loki.Config{
		URL:           output.lokiURL,
		TenantID:      output.lokiTenantID,
		Labels:        output.lokiLabels,
		BatchSize:     output.lokiBatchSize,
		FlushInterval: output.lokiFlushIntvl,
		Timeout:       output.lokiTimeout,
	})
}

func newOTLPFanoutSink(output workerFanoutOutput) (collectorevent.Sink, error) {
	if strings.TrimSpace(output.otlpEndpoint) == "" {
		return nil, fmt.Errorf("fanout output %q: otlp.endpoint must not be empty", output.name)
	}
	return otlp.New(context.Background(), otlp.Config{
		Endpoint: output.otlpEndpoint,
		Insecure: output.otlpInsecure,
		Protocol: output.otlpProtocol,
		Timeout:  output.otlpTimeout,
		Logger:   output.otlpLogger,
	})
}

func newClickHouseFanoutSink(cfg workerConfig, output workerFanoutOutput) (collectorevent.Sink, error) {
	if len(output.chAddrs) == 0 {
		return nil, fmt.Errorf("fanout output %q: clickhouse.addrs must not be empty", output.name)
	}
	return clickhouse.New(clickhouse.Config{
		Addrs:      output.chAddrs,
		Database:   output.chDatabase,
		Username:   output.chUsername,
		Password:   output.chPassword,
		Table:      output.chTable,
		Schema:     output.chSchema,
		StoreRaw:   output.chStoreRaw,
		RawColumn:  output.chRawColumn,
		EncryptRaw: true,
		EncryptKey: cfg.storageEncryptionKey,
	})
}

func newPostgresFanoutSink(cfg workerConfig, output workerFanoutOutput) (collectorevent.Sink, error) {
	if strings.TrimSpace(output.pgDSN) == "" {
		return nil, fmt.Errorf("fanout output %q: postgres.dsn must not be empty", output.name)
	}
	return postgres.New(context.Background(), postgres.Config{
		DSN:        output.pgDSN,
		Table:      output.pgTable,
		Schema:     output.pgSchema,
		StoreRaw:   output.pgStoreRaw,
		RawColumn:  output.pgRawColumn,
		EncryptRaw: true,
		EncryptKey: cfg.storageEncryptionKey,
	})
}

func newS3FanoutSink(cfg workerConfig, output workerFanoutOutput) (collectorevent.Sink, error) {
	if output.s3Bucket == "" {
		return nil, fmt.Errorf("fanout output %q: s3.bucket must not be empty", output.name)
	}

	cfgOpts := []func(*config.LoadOptions) error{}
	if output.s3Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(output.s3Region))
	}
	if output.s3AccessKey != "" && output.s3SecretKey != "" {
		cfgOpts = append(cfgOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(output.s3AccessKey, output.s3SecretKey, "")))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("fanout output %q: load aws config: %w", output.name, err)
	}

	s3Client := s3svc.NewFromConfig(awsCfg, func(o *s3svc.Options) {
		if output.s3Endpoint != "" {
			o.BaseEndpoint = &output.s3Endpoint
		}
	})

	return s3.New(s3.Config{
		Client:        s3Client,
		Bucket:        output.s3Bucket,
		Prefix:        output.s3Prefix,
		Schema:        output.s3Schema,
		BatchSize:     output.s3BatchSize,
		FlushInterval: output.s3FlushIntvl,
		EncryptKey:    cfg.storageEncryptionKey,
	})
}

func newGCSFanoutSink(cfg workerConfig, output workerFanoutOutput) (collectorevent.Sink, error) {
	if output.gcsBucket == "" {
		return nil, fmt.Errorf("fanout output %q: gcs.bucket must not be empty", output.name)
	}

	ctx := context.Background()
	opts := []option.ClientOption{}
	if output.gcsCredsFile != "" {
		opts = append(opts, option.WithCredentialsFile(output.gcsCredsFile))
	}

	gcsClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("fanout output %q: create gcs client: %w", output.name, err)
	}

	return gcs.New(gcs.Config{
		Client:        gcsClient,
		Bucket:        output.gcsBucket,
		Prefix:        output.gcsPrefix,
		Schema:        output.gcsSchema,
		BatchSize:     output.gcsBatchSize,
		FlushInterval: output.gcsFlushIntvl,
		EncryptKey:    cfg.storageEncryptionKey,
	})
}
