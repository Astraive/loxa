package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
)

func NewStorage(cfg *config.Config) (Storage, error) {
	var (
		base Storage
		err  error
	)
	switch cfg.Storage.Backend {
	case "duckdb":
		base, err = NewDuckDBStorage(cfg.Storage.DuckDB.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to create duckdb storage: %w", err)
		}
		if err := base.Init(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to initialize duckdb storage: %w", err)
		}
	case "postgres":
		base, err = NewPostgresStorage(cfg.Storage.PostgreSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres storage: %w", err)
		}
		if err := base.Init(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to initialize postgres storage: %w", err)
		}
	case "collector_file":
		// Unified mode: open collector's DuckDB file directly
		if cfg.Storage.CollectorDBPath == "" {
			return nil, fmt.Errorf("collector_file backend requires storage.collector_db_path")
		}
		db, err := sql.Open("duckdb", cfg.Storage.CollectorDBPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open collector db: %w", err)
		}
		if err := db.Ping(); err != nil {
			return nil, fmt.Errorf("failed to ping collector db: %w", err)
		}
		base = NewDuckDBStorageFromDB(db)
		// Only create cortex tables (events table already exists from collector)
		if err := base.Init(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to initialize cortex schema in collector db: %w", err)
		}
		return base, nil

	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", cfg.Storage.Backend)
	}

	if cfg.Collector.SourceOfTruth {
		return &storageWithExternalEvents{
			base:   base,
			events: newCollectorBackedEventStore(cfg.Collector),
		}, nil
	}
	return base, nil
}
