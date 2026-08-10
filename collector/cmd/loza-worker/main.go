package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	lozacollector "github.com/astraive/loza/collector"
	collectorevent "github.com/astraive/loza/collector/internal/event"
	"github.com/astraive/loza/collector/internal/eventbus"
	processing "github.com/astraive/loza/collector/internal/processing"
	"github.com/astraive/loza/collector/internal/sinks/duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/twmb/franz-go/pkg/kgo"
)

const workerCommitTimeout = 5 * time.Second

type queueRecord struct {
	value  []byte
	commit func(context.Context) error
}

type queueReader interface {
	Poll(context.Context, time.Duration) ([]queueRecord, error)
	Close() error
}

type kafkaQueueReader struct {
	client *kgo.Client
}

func newKafkaQueueReader(cfg workerConfig) (*kafkaQueueReader, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.kafkaBrokers...),
		kgo.ConsumeTopics(cfg.kafkaTopic),
		kgo.ConsumerGroup(cfg.workerConsumerGroup),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		return nil, err
	}
	return &kafkaQueueReader{client: client}, nil
}

func (r *kafkaQueueReader) Poll(ctx context.Context, timeout time.Duration) ([]queueRecord, error) {
	pollCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		pollCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	fetches := r.client.PollFetches(pollCtx)
	if err := pollCtx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	if err := firstFetchErr(fetches.Errors()); err != nil {
		return nil, err
	}

	records := make([]queueRecord, 0, 64)
	fetches.EachRecord(func(rec *kgo.Record) {
		value := append([]byte(nil), rec.Value...)
		commitRec := &kgo.Record{
			Topic:     rec.Topic,
			Partition: rec.Partition,
			Offset:    rec.Offset,
		}
		records = append(records, queueRecord{
			value: value,
			commit: func(commitCtx context.Context) error {
				return r.client.CommitRecords(commitCtx, commitRec)
			},
		})
	})
	return records, nil
}

func (r *kafkaQueueReader) Close() error {
	r.client.Close()
	return nil
}

func firstFetchErr(errs []kgo.FetchError) error {
	parts := make([]string, 0, len(errs))
	for _, fetchErr := range errs {
		if fetchErr.Err == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s[%d]: %v", fetchErr.Topic, fetchErr.Partition, fetchErr.Err))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

type workerState struct {
	cfg       workerConfig
	processor *processing.Processor
}

var version = lozacollector.Version

func main() {
	if err := executeWorkerCLI(os.Args[1:]); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

func executeWorkerCLI(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			args = args[1:]
		case "version", "-v", "--version":
			fmt.Println("loza-worker version", version)
			return nil
		default:
			if !strings.HasPrefix(args[0], "-") {
				return fmt.Errorf("unknown command %q", args[0])
			}
		}
	}
	cfg, err := loadWorkerConfigFromArgs(args)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := runWorker(cfg); err != nil {
		return fmt.Errorf("run: %w", err)
	}
	return nil
}

func runWorker(cfg workerConfig) error {
	db, err := sql.Open(cfg.duckDBDriver, cfg.duckDBPath)
	if err != nil {
		return fmt.Errorf("failed to open duckdb: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(cfg.duckDBMaxOpenConns)
	db.SetMaxIdleConns(cfg.duckDBMaxIdleConns)

	if err := ensureSchema(db, cfg); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	primarySink, err := duckdb.New(duckdb.Config{
		DB:              db,
		Driver:          cfg.duckDBDriver,
		Table:           cfg.duckDBTable,
		StoreRaw:        cfg.duckDBStoreRaw,
		RawColumn:       cfg.duckDBRawColumn,
		Schema:          cfg.duckDBSchema,
		BatchSize:       cfg.duckDBBatchSize,
		FlushInterval:   cfg.duckDBFlushInterval,
		WriterLoop:      cfg.duckDBWriterLoop,
		WriterQueueSize: cfg.duckDBWriterQueueSize,
		EncryptRaw:      true,
		EncryptKey:      cfg.storageEncryptionKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create duckdb sink: %w", err)
	}

	secondarySinks, fallbackSink, fanoutDBs, err := createFanoutSinks(cfg)
	if err != nil {
		return err
	}
	defer func() {
		for _, fanoutDB := range fanoutDBs {
			_ = fanoutDB.Close()
		}
	}()

	processor, err := processing.New(processing.Config{
		DeliveryPolicy:          cfg.deliveryPolicy,
		RetryEnabled:            cfg.retryEnabled,
		RetryMaxAttempts:        cfg.retryMaxAttempts,
		RetryInitialBackoff:     cfg.retryInitialBackoff,
		RetryMaxBackoff:         cfg.retryMaxBackoff,
		RetryJitter:             cfg.retryJitter,
		FallbackEnabled:         cfg.fallbackEnabled,
		FallbackOnPrimaryFail:   cfg.fallbackOnPrimaryFail,
		FallbackOnSecondaryFail: cfg.fallbackOnSecondaryFail,
		FallbackOnPolicyFail:    cfg.fallbackOnPolicyFail,
		DLQEnabled:              cfg.dlqEnabled,
		DLQPath:                 cfg.dlqPath,
		DLQOnPrimaryFail:        cfg.dlqOnPrimaryFail,
		DLQOnSecondaryFail:      cfg.dlqOnSecondaryFail,
		DLQOnFallbackFail:       cfg.dlqOnFallbackFail,
		DLQOnPolicyFail:         cfg.dlqOnPolicyFail,
		DedupeEnabled:           cfg.dedupeEnabled,
		DedupeKey:               cfg.dedupeKey,
		DedupeWindow:            cfg.dedupeWindow,
		DedupeBackend:           cfg.dedupeBackend,
		DedupeRedisAddr:         cfg.dedupeRedisAddr,
		DedupeRedisPassword:     cfg.dedupeRedisPassword,
		DedupeRedisDB:           cfg.dedupeRedisDB,
		DedupeRedisPrefix:       cfg.dedupeRedisPrefix,
		ValidateJSONObjects:     true,
	}, primarySink, secondarySinks, fallbackSink, nil)
	if err != nil {
		return err
	}
	defer func() { _ = processor.Close() }()

	var reader queueReader
	busType := cfg.eventBusType
	if busType != "" {
		// New eventbus path
		busCfg := cfg.buildEventBusConfig()
		bus, busErr := eventbus.New(context.Background(), busCfg)
		if busErr != nil {
			return fmt.Errorf("failed to create eventbus: %w", busErr)
		}
		topic := busCfg.Topic
		if topic == "" {
			topic = "loza.events.raw"
		}
		reader = newEventbusQueueReader(bus, topic, cfg.workerConsumerGroup)
		logJSON("info", "worker_eventbus_initialized", map[string]any{
			"type":  busType,
			"topic": topic,
			"group": cfg.workerConsumerGroup,
		})
	} else {
		// Legacy Kafka path
		var kErr error
		reader, kErr = newKafkaQueueReader(cfg)
		if kErr != nil {
			return fmt.Errorf("failed to create kafka reader: %w", kErr)
		}
	}
	defer func() { _ = reader.Close() }()

	state := &workerState{cfg: cfg, processor: processor}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	go func() {
		<-quit
		cancel()
	}()

	logJSON("info", "worker_start", map[string]any{
		"eventbus_type":  busType,
		"kafka_topic":    cfg.kafkaTopic,
		"consumer_group": cfg.workerConsumerGroup,
		"duckdb_path":    cfg.duckDBPath,
	})
	if err := state.run(ctx, reader); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	// Graceful shutdown: flush pending records before closing sinks
	logJSON("info", "worker_shutdown_begin", nil)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer shutdownCancel()

	// Commit any remaining offsets before closing
	if err := reader.Close(); err != nil {
		logJSON("error", "worker_kafka_close_failed", map[string]any{"error": err.Error()})
	}

	for _, sink := range sinksForShutdown(primarySink, secondarySinks, fallbackSink) {
		if err := sink.Sink.Flush(shutdownCtx); err != nil {
			logJSON("error", "worker_sink_flush_failed", map[string]any{"sink": sink.Name, "error": err.Error()})
		}
		if err := sink.Sink.Close(shutdownCtx); err != nil {
			logJSON("error", "worker_sink_close_failed", map[string]any{"sink": sink.Name, "error": err.Error()})
		}
	}
	logJSON("info", "worker_shutdown_complete", nil)
	return nil
}

func (s *workerState) run(ctx context.Context, reader queueReader) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		records, err := reader.Poll(ctx, s.cfg.workerPollTimeout)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			logJSON("error", "worker_kafka_poll_failed", map[string]any{"error": err.Error()})
			continue
		}
		for _, record := range records {
			s.handleRecord(ctx, record)
		}
	}
}

func (s *workerState) handleRecord(ctx context.Context, record queueRecord) {
	result := s.processor.Process(ctx, record.value)
	if result.Invalid {
		logJSON("warn", "worker_event_invalid", nil)
	}
	if failures := result.Outcome.FailureCount(); failures > 0 {
		logJSON("error", "worker_sink_write_failed", map[string]any{"failures": failures})
	}
	if result.Deduped {
		logJSON("info", "worker_event_deduped", nil)
	}
	if result.Err != nil {
		logJSON("error", "worker_event_process_failed", map[string]any{"error": result.Err.Error()})
	}

	if record.commit == nil {
		return
	}
	commitCtx, cancel := context.WithTimeout(context.Background(), workerCommitTimeout)
	defer cancel()
	if err := record.commit(commitCtx); err != nil {
		logJSON("error", "worker_offset_commit_failed", map[string]any{"error": err.Error()})
	}
}

func sinksForShutdown(primary collectorevent.Sink, secondary []processing.NamedSink, fallback *processing.NamedSink) []processing.NamedSink {
	sinks := []processing.NamedSink{{Name: "primary", Sink: primary}}
	sinks = append(sinks, secondary...)
	if fallback != nil {
		sinks = append(sinks, *fallback)
	}
	seen := make(map[string]struct{}, len(sinks))
	out := make([]processing.NamedSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink.Sink == nil {
			continue
		}
		if _, ok := seen[sink.Name]; ok {
			continue
		}
		seen[sink.Name] = struct{}{}
		out = append(out, sink)
	}
	return out
}

func logJSON(level, message string, fields map[string]any) {
	body := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level,
		"message":   message,
	}
	for k, v := range fields {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		log.Printf(`{"level":"error","message":"worker_log_encode_failed","error":"%s"}`, err.Error())
		return
	}
	log.Print(string(b))
}
