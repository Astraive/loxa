package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *collectorState) initRetention() {
	if !s.cfg.retentionEnabled {
		return
	}
	if s.cfg.retentionDays <= 0 && s.cfg.retentionMaxSize <= 0 {
		return
	}
	go s.retentionWorker()
}

func (s *collectorState) retentionWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.executeRetention(); err != nil {
				logJSON("error", "retention_execution_failed", map[string]any{"error": err.Error()})
			}
		case <-s.reliabilityCtx.Done():
			return
		}
	}
}

func (s *collectorState) executeRetention() error {
	if s.queryDB == nil {
		return fmt.Errorf("query db not available for retention")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if s.cfg.retentionDays > 0 {
		cutoffTime := time.Now().UTC().AddDate(0, 0, -s.cfg.retentionDays)
		if err := s.deleteByAge(ctx, s.queryDB, cutoffTime); err != nil {
			logJSON("warn", "retention_age_delete_failed", map[string]any{"error": err.Error()})
		}
	}

	if s.cfg.retentionMaxSize > 0 {
		if err := s.deleteBySize(ctx, s.queryDB); err != nil {
			logJSON("warn", "retention_size_delete_failed", map[string]any{"error": err.Error()})
		}
	}

	logJSON("info", "retention_executed", map[string]any{
		"retention_days": s.cfg.retentionDays,
		"retention_size": s.cfg.retentionMaxSize,
	})
	return nil
}

func (s *collectorState) deleteByAge(ctx context.Context, db *sql.DB, cutoffTime time.Time) error {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE timestamp IS NOT NULL AND timestamp < $1
	`, s.cfg.duckDBTable)

	result, err := db.ExecContext(ctx, query, cutoffTime.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logJSON("info", "retention_age_deleted", map[string]any{
		"rows_deleted": rowsAffected,
		"cutoff_time":  cutoffTime.Format(time.RFC3339),
	})
	return nil
}

func (s *collectorState) deleteBySize(ctx context.Context, db *sql.DB) error {
	var currentSize int64
	err := db.QueryRowContext(ctx, "SELECT SUM(octet_length(raw)) FROM "+s.cfg.duckDBTable).Scan(&currentSize)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if currentSize <= s.cfg.retentionMaxSize {
		return nil
	}

	excessSize := currentSize - s.cfg.retentionMaxSize
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE event_id IN (
			SELECT event_id FROM %s
			ORDER BY timestamp ASC
			LIMIT (SELECT COUNT(*) FROM %s WHERE octet_length(raw) > 0 LIMIT CEIL($1 / (SELECT AVG(octet_length(raw)) FROM %s)))
		)
	`, s.cfg.duckDBTable, s.cfg.duckDBTable, s.cfg.duckDBTable, s.cfg.duckDBTable)

	result, err := db.ExecContext(ctx, query, excessSize)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logJSON("info", "retention_size_deleted", map[string]any{
		"rows_deleted": rowsAffected,
		"excess_size":  excessSize,
	})
	return nil
}
