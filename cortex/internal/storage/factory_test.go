package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
)

func TestNewStorageSupportsDuckDB(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.Backend = "duckdb"
	cfg.Storage.DuckDB.Path = filepath.Join(t.TempDir(), "cortex.duckdb")

	stor, err := NewStorage(cfg)
	if err != nil {
		t.Fatalf("expected duckdb storage, got error: %v", err)
	}
	defer stor.Close()
}

func TestNewStorageRejectsUnsupportedBackend(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.Backend = "sqlite"

	_, err := NewStorage(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unsupported storage backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}
