package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgresStorage(cfg config.PostgresConfig) (*PostgresStorage, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Init(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		event_id TEXT,
		timestamp TIMESTAMP NOT NULL,
		service TEXT NOT NULL,
		environment TEXT,
		release TEXT,
		schema_version TEXT,
		event_version TEXT,
		event TEXT,
		kind TEXT NOT NULL,
		level TEXT,
		outcome TEXT,
		duration_ms DOUBLE PRECISION,
		trace_id TEXT,
		span_id TEXT,
		trace_flags TEXT,
		request_id TEXT,
		http JSONB,
		"user" JSONB,
		tenant JSONB,
		attrs JSONB,
		error JSONB,
		checkpoints JSONB,
		processes JSONB,
		groups JSONB,
		timers JSONB,
		links JSONB,
		sdk_name TEXT,
		sdk_version TEXT,
		sdk_language TEXT,
		raw JSONB,
		provenance TEXT NOT NULL,
		incident_id TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_service ON events(service);
	CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);
	CREATE INDEX IF NOT EXISTS idx_events_trace_id ON events(trace_id);
	CREATE INDEX IF NOT EXISTS idx_events_incident_id ON events(incident_id);
	CREATE INDEX IF NOT EXISTS idx_events_outcome ON events(outcome);
	CREATE INDEX IF NOT EXISTS idx_events_level ON events(level);
	CREATE INDEX IF NOT EXISTS idx_events_environment ON events(environment);
	CREATE INDEX IF NOT EXISTS idx_events_duration_ms ON events(duration_ms);

	CREATE TABLE IF NOT EXISTS topology_aliases (
		id TEXT PRIMARY KEY,
		alias TEXT NOT NULL,
		canonical TEXT NOT NULL,
		valid_from TIMESTAMP NOT NULL,
		valid_to TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_topology_aliases_alias ON topology_aliases(alias);
	CREATE INDEX IF NOT EXISTS idx_topology_aliases_canonical ON topology_aliases(canonical);

	CREATE TABLE IF NOT EXISTS graph_nodes (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		label TEXT NOT NULL,
		attributes JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_graph_nodes_type ON graph_nodes(type);

	CREATE TABLE IF NOT EXISTS graph_edges (
		id TEXT PRIMARY KEY,
		from_node_id TEXT NOT NULL REFERENCES graph_nodes(id),
		to_node_id TEXT NOT NULL REFERENCES graph_nodes(id),
		type TEXT NOT NULL,
		weight DOUBLE PRECISION DEFAULT 1.0,
		attributes JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_graph_edges_from ON graph_edges(from_node_id);
	CREATE INDEX IF NOT EXISTS idx_graph_edges_to ON graph_edges(to_node_id);
	CREATE INDEX IF NOT EXISTS idx_graph_edges_type ON graph_edges(type);

	CREATE TABLE IF NOT EXISTS incidents (
		id TEXT PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		signature_id TEXT,
		status TEXT NOT NULL,
		severity TEXT NOT NULL,
		primary_service TEXT NOT NULL,
		affected_services JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		resolved_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS incident_signatures (
		signature_id TEXT PRIMARY KEY,
		shape TEXT NOT NULL,
		service_roles JSONB,
		symptoms JSONB,
		temporal_pattern JSONB,
		remediation JSONB,
		feature_vector JSONB,
		feature_weights JSONB,
		occurrence_count INTEGER DEFAULT 0,
		avg_resolution_time_seconds BIGINT DEFAULT 0,
		version INTEGER DEFAULT 1,
		parent_signature_id TEXT,
		decay_factor DOUBLE PRECISION DEFAULT 1.0,
		last_matched_at TIMESTAMP,
		behavioral_hash TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS remediations (
		remediation_id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL REFERENCES incidents(id),
		signature_id TEXT,
		action TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		operator TEXT,
		attributes JSONB
	);

	CREATE TABLE IF NOT EXISTS remediation_feedback (
		feedback_id TEXT PRIMARY KEY,
		remediation_id TEXT NOT NULL REFERENCES remediations(remediation_id),
		incident_id TEXT NOT NULL,
		outcome_code INTEGER NOT NULL,
		outcome_category TEXT NOT NULL,
		time_to_resolve_seconds BIGINT,
		timestamp TIMESTAMP NOT NULL,
		notes TEXT
	);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) Events() EventStore {
	return &PostgresEventStore{db: s.db}
}

func (s *PostgresStorage) Topology() TopologyStore {
	return &PostgresTopologyStore{db: s.db}
}

func (s *PostgresStorage) Graph() GraphStore {
	return &PostgresGraphStore{db: s.db}
}

func (s *PostgresStorage) Incidents() IncidentStore {
	return &PostgresIncidentStore{db: s.db}
}

func (s *PostgresStorage) Signatures() SignatureStore {
	return &PostgresSignatureStore{db: s.db}
}

func (s *PostgresStorage) Remediations() RemediationStore {
	return &PostgresRemediationStore{db: s.db}
}

func (s *PostgresStorage) Feedback() FeedbackStore {
	return &PostgresFeedbackStore{db: s.db}
}

type PostgresEventStore struct {
	db *sql.DB
}

func (s *PostgresEventStore) Save(ctx context.Context, event *models.Event, lifecycle *LifecycleData) error {
	rawJSON, _ := json.Marshal(event.Raw)
	attrsJSON, _ := json.Marshal(event.Attrs)
	checkpointsJSON, _ := json.Marshal(event.Checkpoints)
	processesJSON, _ := json.Marshal(event.Processes)
	groupsJSON, _ := json.Marshal(event.Groups)
	timersJSON, _ := json.Marshal(event.Timers)
	linksJSON, _ := json.Marshal(event.Links)
	httpJSON, _ := json.Marshal(event.HTTP)
	userJSON, _ := json.Marshal(event.User)
	tenantJSON, _ := json.Marshal(event.Tenant)
	errorJSON, _ := json.Marshal(event.Error)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO events (id, event_id, timestamp, service, environment, release,
			schema_version, event_version, event, kind, level, outcome, duration_ms,
			trace_id, span_id, trace_flags, request_id,
			http, user, tenant, attrs, error,
			checkpoints, processes, groups, timers, links,
			sdk_name, sdk_version, sdk_language,
			raw, provenance, incident_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34)`,
		event.ID, event.EventID, event.Timestamp, event.Service, event.Environment, event.Release,
		event.SchemaVersion, event.EventVersion, event.Event, event.Kind, event.Level, event.Outcome, event.DurationMs,
		event.TraceID, event.SpanID, event.TraceFlags, event.RequestID,
		httpJSON, userJSON, tenantJSON, attrsJSON, errorJSON,
		checkpointsJSON, processesJSON, groupsJSON, timersJSON, linksJSON,
		event.SDKName, event.SDKVersion, event.SDKLanguage,
		rawJSON, event.Provenance, event.IncidentID, time.Now())
	return err
}

func (s *PostgresEventStore) SaveBatch(ctx context.Context, events []*models.Event, lifecycles []*LifecycleData) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO events (id, event_id, timestamp, service, environment, release,
			schema_version, event_version, event, kind, level, outcome, duration_ms,
			trace_id, span_id, trace_flags, request_id,
			http, user, tenant, attrs, error,
			checkpoints, processes, groups, timers, links,
			sdk_name, sdk_version, sdk_language,
			raw, provenance, incident_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		rawJSON, _ := json.Marshal(e.Raw)
		attrsJSON, _ := json.Marshal(e.Attrs)
		checkpointsJSON, _ := json.Marshal(e.Checkpoints)
		processesJSON, _ := json.Marshal(e.Processes)
		groupsJSON, _ := json.Marshal(e.Groups)
		timersJSON, _ := json.Marshal(e.Timers)
		linksJSON, _ := json.Marshal(e.Links)
		httpJSON, _ := json.Marshal(e.HTTP)
		userJSON, _ := json.Marshal(e.User)
		tenantJSON, _ := json.Marshal(e.Tenant)
		errorJSON, _ := json.Marshal(e.Error)

		if _, err := stmt.ExecContext(ctx,
			e.ID, e.EventID, e.Timestamp, e.Service, e.Environment, e.Release,
			e.SchemaVersion, e.EventVersion, e.Event, e.Kind, e.Level, e.Outcome, e.DurationMs,
			e.TraceID, e.SpanID, e.TraceFlags, e.RequestID,
			httpJSON, userJSON, tenantJSON, attrsJSON, errorJSON,
			checkpointsJSON, processesJSON, groupsJSON, timersJSON, linksJSON,
			e.SDKName, e.SDKVersion, e.SDKLanguage,
			rawJSON, e.Provenance, e.IncidentID, time.Now()); err != nil {
			return err
		}
		_ = lifecycles
	}

	return tx.Commit()
}

func scanPostgresRow(scanner interface{ Scan(dest ...any) error }) (*models.Event, error) {
	var event models.Event
	var rawJSON, attrsJSON, httpJSON, userJSON, tenantJSON, errorJSON []byte
	var checkpointsJSON, processesJSON, groupsJSON, timersJSON, linksJSON []byte
	err := scanner.Scan(
		&event.ID, &event.EventID, &event.Timestamp, &event.Service, &event.Environment, &event.Release,
		&event.SchemaVersion, &event.EventVersion, &event.Event, &event.Kind, &event.Level, &event.Outcome, &event.DurationMs,
		&event.TraceID, &event.SpanID, &event.TraceFlags, &event.RequestID,
		&httpJSON, &userJSON, &tenantJSON, &attrsJSON, &errorJSON,
		&checkpointsJSON, &processesJSON, &groupsJSON, &timersJSON, &linksJSON,
		&event.SDKName, &event.SDKVersion, &event.SDKLanguage,
		&rawJSON, &event.Provenance, &event.IncidentID, &event.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawJSON, &event.Raw); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal raw event JSON")
	}
	if err := json.Unmarshal(attrsJSON, &event.Attrs); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal attrs JSON")
	}
	if err := json.Unmarshal(httpJSON, &event.HTTP); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal http JSON")
	}
	if err := json.Unmarshal(userJSON, &event.User); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal user JSON")
	}
	if err := json.Unmarshal(tenantJSON, &event.Tenant); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal tenant JSON")
	}
	if err := json.Unmarshal(errorJSON, &event.Error); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal error JSON")
	}
	if err := json.Unmarshal(checkpointsJSON, &event.Checkpoints); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal checkpoints JSON")
	}
	if err := json.Unmarshal(processesJSON, &event.Processes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal processes JSON")
	}
	if err := json.Unmarshal(groupsJSON, &event.Groups); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal groups JSON")
	}
	if err := json.Unmarshal(timersJSON, &event.Timers); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal timers JSON")
	}
	if err := json.Unmarshal(linksJSON, &event.Links); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal links JSON")
	}
	return &event, nil
}

var pgSelectCols = "id, event_id, timestamp, service, environment, release, schema_version, event_version, event, kind, level, outcome, duration_ms, trace_id, span_id, trace_flags, request_id, http, \"user\", tenant, attrs, error, checkpoints, processes, groups, timers, links, sdk_name, sdk_version, sdk_language, raw, provenance, incident_id, created_at"

func (s *PostgresEventStore) Get(ctx context.Context, id string) (*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE id = $1"
	row := s.db.QueryRowContext(ctx, query, id)
	return scanPostgresRow(row)
}

func (s *PostgresEventStore) List(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events ORDER BY timestamp DESC LIMIT $1 OFFSET $2"
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE trace_id = $1 ORDER BY timestamp"
	rows, err := s.db.QueryContext(ctx, query, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE incident_id = $1 ORDER BY timestamp"
	rows, err := s.db.QueryContext(ctx, query, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE service = $1"
	args := []interface{}{service}
	argIdx := 2

	if from != "" {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		args = append(args, from)
		argIdx++
	}
	if to != "" {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		args = append(args, to)
	}
	query += " ORDER BY timestamp"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

// Lifecycle-aware query methods
func (s *PostgresEventStore) FindByEventName(ctx context.Context, eventName string, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE event = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3"
	rows, err := s.db.QueryContext(ctx, query, eventName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByOutcome(ctx context.Context, outcome string, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE outcome = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3"
	rows, err := s.db.QueryContext(ctx, query, outcome, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByLevel(ctx context.Context, level string, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE level = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3"
	rows, err := s.db.QueryContext(ctx, query, level, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE duration_ms >= $1 AND duration_ms <= $2 ORDER BY duration_ms DESC LIMIT $3 OFFSET $4"
	rows, err := s.db.QueryContext(ctx, query, minMs, maxMs, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByEnvironment(ctx context.Context, env string, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE environment = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3"
	rows, err := s.db.QueryContext(ctx, query, env, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByRelease(ctx context.Context, release string, limit, offset int) ([]*models.Event, error) {
	query := "SELECT " + pgSelectCols + " FROM events WHERE release = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3"
	rows, err := s.db.QueryContext(ctx, query, release, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanPostgresRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *PostgresEventStore) CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT outcome, COUNT(*) as cnt FROM events WHERE service = $1 AND timestamp >= $2 AND timestamp <= $3 GROUP BY outcome", service, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var outcome string
		var count int64
		if err := rows.Scan(&outcome, &count); err != nil {
			return nil, err
		}
		result[outcome] = count
	}
	return result, nil
}

func (s *PostgresEventStore) CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT event, COUNT(*) as cnt FROM events WHERE service = $1 AND timestamp >= $2 AND timestamp <= $3 AND event IS NOT NULL GROUP BY event", service, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]int64)
	for rows.Next() {
		var eventName string
		var count int64
		if err := rows.Scan(&eventName, &count); err != nil {
			return nil, err
		}
		result[eventName] = count
	}
	return result, nil
}

func (s *PostgresEventStore) AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error) {
	var avg sql.NullFloat64
	err := s.db.QueryRowContext(ctx, "SELECT AVG(duration_ms) FROM events WHERE event = $1 AND timestamp >= $2 AND timestamp <= $3 AND duration_ms IS NOT NULL", eventName, from, to).Scan(&avg)
	if err != nil || !avg.Valid {
		return 0, err
	}
	return avg.Float64, nil
}

func (s *PostgresEventStore) PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error) {
	var val sql.NullFloat64
	err := s.db.QueryRowContext(ctx, "SELECT percentile_cont($1) WITHIN GROUP (ORDER BY duration_ms) FROM events WHERE event = $2 AND timestamp >= $3 AND timestamp <= $4 AND duration_ms IS NOT NULL", percentile, eventName, from, to).Scan(&val)
	if err != nil || !val.Valid {
		return 0, err
	}
	return val.Float64, nil
}

func (s *PostgresEventStore) DistinctServices(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT service FROM events ORDER BY service")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var services []string
	for rows.Next() {
		var svc string
		if err := rows.Scan(&svc); err != nil {
			return nil, err
		}
		services = append(services, svc)
	}
	return services, nil
}

func (s *PostgresEventStore) DistinctEventNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT event FROM events WHERE event IS NOT NULL ORDER BY event")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (s *PostgresEventStore) ListLifecycleSummaries(ctx context.Context, filter *LifecycleFilter) ([]*models.LifecycleSummary, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	argIdx := 1

	if filter != nil {
		if filter.Service != "" {
			where += fmt.Sprintf(" AND service = $%d", argIdx)
			args = append(args, filter.Service)
			argIdx++
		}
		if filter.EventName != "" {
			where += fmt.Sprintf(" AND event = $%d", argIdx)
			args = append(args, filter.EventName)
			argIdx++
		}
		if filter.Outcome != "" {
			where += fmt.Sprintf(" AND outcome = $%d", argIdx)
			args = append(args, filter.Outcome)
			argIdx++
		}
		if filter.Level != "" {
			where += fmt.Sprintf(" AND level = $%d", argIdx)
			args = append(args, filter.Level)
			argIdx++
		}
		if filter.TraceID != "" {
			where += fmt.Sprintf(" AND trace_id = $%d", argIdx)
			args = append(args, filter.TraceID)
			argIdx++
		}
		if !filter.From.IsZero() {
			where += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
			args = append(args, filter.From)
			argIdx++
		}
		if !filter.To.IsZero() {
			where += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
			args = append(args, filter.To)
			argIdx++
		}
		if filter.MinDuration > 0 {
			where += fmt.Sprintf(" AND duration_ms >= $%d", argIdx)
			args = append(args, filter.MinDuration)
			argIdx++
		}
		if filter.MaxDuration > 0 {
			where += fmt.Sprintf(" AND duration_ms <= $%d", argIdx)
			args = append(args, filter.MaxDuration)
			argIdx++
		}
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM events " + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch summaries
	limit := 50
	offset := 0
	if filter != nil {
		if filter.Limit > 0 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}

	query := fmt.Sprintf("SELECT id, event, service, outcome, duration_ms,"+
		" json_array_length(COALESCE(checkpoints, '[]'::jsonb)) as cp_count,"+
		" json_array_length(COALESCE(processes, '[]'::jsonb)) as pr_count,"+
		" json_array_length(COALESCE(groups, '[]'::jsonb)) as gr_count,"+
		" json_array_length(COALESCE(timers, '[]'::jsonb)) as ti_count,"+
		" json_array_length(COALESCE(links, '[]'::jsonb)) as li_count,"+
		" trace_id"+
		" FROM events %s ORDER BY timestamp DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var summaries []*models.LifecycleSummary
	for rows.Next() {
		var s models.LifecycleSummary
		if err := rows.Scan(&s.EventID, &s.Event, &s.Service, &s.Outcome, &s.DurationMs,
			&s.CheckpointCount, &s.ProcessCount, &s.GroupCount, &s.TimerCount, &s.LinkCount,
			&s.TraceID); err != nil {
			return nil, 0, err
		}
		summaries = append(summaries, &s)
	}
	return summaries, total, nil
}

type PostgresTopologyStore struct {
	db *sql.DB
}

func (s *PostgresTopologyStore) SaveAlias(ctx context.Context, alias *models.ServiceAlias) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO topology_aliases (id, alias, canonical, valid_from, valid_to, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		alias.ID, alias.Alias, alias.Canonical, alias.ValidFrom, alias.ValidTo, time.Now())
	return err
}

func (s *PostgresTopologyStore) GetAlias(ctx context.Context, alias string, timestamp string) (*models.ServiceAlias, error) {
	ts, _ := time.Parse(time.RFC3339, timestamp)
	var a models.ServiceAlias
	err := s.db.QueryRowContext(ctx,
		"SELECT id, alias, canonical, valid_from, valid_to, created_at FROM topology_aliases WHERE alias = $1 AND valid_from <= $2 AND (valid_to IS NULL OR valid_to > $2) ORDER BY valid_from DESC LIMIT 1",
		alias, ts, ts).Scan(&a.ID, &a.Alias, &a.Canonical, &a.ValidFrom, &a.ValidTo, &a.CreatedAt)
	return &a, err
}

func (s *PostgresTopologyStore) GetHistory(ctx context.Context, service string) ([]*models.ServiceAlias, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, alias, canonical, valid_from, valid_to, created_at FROM topology_aliases WHERE alias = $1 OR canonical = $1 ORDER BY valid_from",
		service, service)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aliases []*models.ServiceAlias
	for rows.Next() {
		var a models.ServiceAlias
		if err := rows.Scan(&a.ID, &a.Alias, &a.Canonical, &a.ValidFrom, &a.ValidTo, &a.CreatedAt); err != nil {
			return nil, err
		}
		aliases = append(aliases, &a)
	}
	return aliases, nil
}

type PostgresGraphStore struct {
	db *sql.DB
}

func (s *PostgresGraphStore) SaveNode(ctx context.Context, node *models.Node) error {
	attrsJSON, _ := json.Marshal(node.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO graph_nodes (id, type, label, attributes, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO UPDATE SET label = EXCLUDED.label, attributes = EXCLUDED.attributes, updated_at = EXCLUDED.updated_at",
		node.ID, node.Type, node.Label, attrsJSON, node.CreatedAt, time.Now())
	return err
}

func (s *PostgresGraphStore) GetNode(ctx context.Context, id string) (*models.Node, error) {
	var node models.Node
	var attrsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, type, label, attributes, created_at, updated_at FROM graph_nodes WHERE id = $1", id).
		Scan(&node.ID, &node.Type, &node.Label, &attrsJSON, &node.CreatedAt, &node.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &node.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal node attributes")
	}
	return &node, nil
}

func (s *PostgresGraphStore) ListNodes(ctx context.Context, nodeType string, limit int) ([]*models.Node, error) {
	query := "SELECT id, type, label, attributes, created_at, updated_at FROM graph_nodes"
	var args []interface{}
	if nodeType != "" {
		query += " WHERE type = $1"
		args = append(args, nodeType)
	}
	query += fmt.Sprintf(" LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*models.Node
	for rows.Next() {
		var node models.Node
		var attrsJSON []byte
		if err := rows.Scan(&node.ID, &node.Type, &node.Label, &attrsJSON, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(attrsJSON, &node.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal node attributes")
	}
		nodes = append(nodes, &node)
	}
	return nodes, nil
}

func (s *PostgresGraphStore) SaveEdge(ctx context.Context, edge *models.Edge) error {
	attrsJSON, _ := json.Marshal(edge.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO graph_edges (id, from_node_id, to_node_id, type, weight, attributes, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING",
		edge.ID, edge.FromNodeID, edge.ToNodeID, edge.Type, edge.Weight, attrsJSON, time.Now())
	return err
}

func (s *PostgresGraphStore) GetEdges(ctx context.Context, nodeID string, edgeType string) ([]*models.Edge, error) {
	args := []interface{}{nodeID}
	query := "SELECT id, from_node_id, to_node_id, type, weight, attributes, created_at FROM graph_edges WHERE (from_node_id = $1 OR to_node_id = $1)"
	if edgeType != "" {
		query += " AND type = $2"
		args = append(args, edgeType)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*models.Edge
	for rows.Next() {
		var edge models.Edge
		var attrsJSON []byte
		if err := rows.Scan(&edge.ID, &edge.FromNodeID, &edge.ToNodeID, &edge.Type, &edge.Weight, &attrsJSON, &edge.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(attrsJSON, &edge.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal edge attributes")
	}
		edges = append(edges, &edge)
	}
	return edges, nil
}

func (s *PostgresGraphStore) Traverse(ctx context.Context, startNodeID string, opts models.TraversalOptions) (*models.GraphView, error) {
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 3
	}

	visited := make(map[string]bool)
	var resultNodes []*models.Node
	var resultEdges []*models.Edge

	queue := []string{startNodeID}
	depth := 0

	for len(queue) > 0 && depth < opts.MaxDepth {
		currentLen := len(queue)
		for i := 0; i < currentLen; i++ {
			nodeID := queue[i]
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			node, err := s.GetNode(ctx, nodeID)
			if err != nil {
				continue
			}
			resultNodes = append(resultNodes, node)

			edgeRows, err := s.db.QueryContext(ctx,
				"SELECT id, from_node_id, to_node_id, type, weight, attributes, created_at FROM graph_edges WHERE from_node_id = $1",
				nodeID)
			if err != nil {
				continue
			}
			for edgeRows.Next() {
				var edge models.Edge
				var attrsJSON []byte
				if err := edgeRows.Scan(&edge.ID, &edge.FromNodeID, &edge.ToNodeID, &edge.Type, &edge.Weight, &attrsJSON, &edge.CreatedAt); err != nil {
					continue
				}
				if err := json.Unmarshal(attrsJSON, &edge.Attributes); err != nil {
					log.Warn().Err(err).Msg("failed to unmarshal edge attributes JSON")
				}

				if len(opts.EdgeTypes) > 0 {
					found := false
					for _, t := range opts.EdgeTypes {
						if edge.Type == t {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				resultEdges = append(resultEdges, &edge)
				if !visited[edge.ToNodeID] {
					queue = append(queue, edge.ToNodeID)
				}
			}
			edgeRows.Close()
		}
		queue = queue[currentLen:]
		depth++
	}

	return &models.GraphView{Nodes: resultNodes, Edges: resultEdges}, nil
}

type PostgresIncidentStore struct {
	db *sql.DB
}

func (s *PostgresIncidentStore) Save(ctx context.Context, incident *models.Incident) error {
	servicesJSON, _ := json.Marshal(incident.AffectedServices)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO incidents (id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, resolved_at = EXCLUDED.resolved_at",
		incident.ID, incident.Timestamp, incident.SignatureID, incident.Status, incident.Severity, incident.PrimaryService, servicesJSON, time.Now(), incident.ResolvedAt)
	return err
}

func (s *PostgresIncidentStore) Get(ctx context.Context, id string) (*models.Incident, error) {
	var inc models.Incident
	var servicesJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents WHERE id = $1", id).
		Scan(&inc.ID, &inc.Timestamp, &inc.SignatureID, &inc.Status, &inc.Severity, &inc.PrimaryService, &servicesJSON, &inc.CreatedAt, &inc.ResolvedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(servicesJSON, &inc.AffectedServices); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal incident affected services")
	}
	return &inc, nil
}

func (s *PostgresIncidentStore) GetBySignature(ctx context.Context, signatureID string) (*models.Incident, error) {
	var inc models.Incident
	var servicesJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents WHERE signature_id = $1 LIMIT 1", signatureID).
		Scan(&inc.ID, &inc.Timestamp, &inc.SignatureID, &inc.Status, &inc.Severity, &inc.PrimaryService, &servicesJSON, &inc.CreatedAt, &inc.ResolvedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(servicesJSON, &inc.AffectedServices); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal incident affected services")
	}
	return &inc, nil
}

func (s *PostgresIncidentStore) List(ctx context.Context, limit, offset int) ([]*models.Incident, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents ORDER BY timestamp DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []*models.Incident
	for rows.Next() {
		var inc models.Incident
		var servicesJSON []byte
		if err := rows.Scan(&inc.ID, &inc.Timestamp, &inc.SignatureID, &inc.Status, &inc.Severity, &inc.PrimaryService, &servicesJSON, &inc.CreatedAt, &inc.ResolvedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(servicesJSON, &inc.AffectedServices); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal incident affected services")
	}
		incidents = append(incidents, &inc)
	}
	return incidents, nil
}

type PostgresSignatureStore struct {
	db *sql.DB
}

func (s *PostgresSignatureStore) Save(ctx context.Context, sig *models.IncidentSignature) error {
	rolesJSON, _ := json.Marshal(sig.ServiceRoles)
	symptomsJSON, _ := json.Marshal(sig.Symptoms)
	patternJSON, _ := json.Marshal(sig.TemporalPattern)
	remediationJSON, _ := json.Marshal(sig.Remediation)
	vectorJSON, _ := json.Marshal(sig.FeatureVector)
	weightsJSON, _ := json.Marshal(sig.FeatureWeights)

	if sig.DecayFactor == 0 {
		sig.DecayFactor = 1.0
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO incident_signatures (signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17) ON CONFLICT (signature_id) DO UPDATE SET
		shape = EXCLUDED.shape, service_roles = EXCLUDED.service_roles, symptoms = EXCLUDED.symptoms, temporal_pattern = EXCLUDED.temporal_pattern, remediation = EXCLUDED.remediation, feature_vector = EXCLUDED.feature_vector, feature_weights = EXCLUDED.feature_weights, occurrence_count = EXCLUDED.occurrence_count, avg_resolution_time_seconds = EXCLUDED.avg_resolution_time_seconds, version = EXCLUDED.version, parent_signature_id = EXCLUDED.parent_signature_id, decay_factor = EXCLUDED.decay_factor, last_matched_at = EXCLUDED.last_matched_at, behavioral_hash = EXCLUDED.behavioral_hash, updated_at = EXCLUDED.updated_at`,
		sig.SignatureID, sig.Shape, rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON, sig.OccurrenceCount, sig.AvgResolutionTime, sig.Version, sig.ParentSignatureID, sig.DecayFactor, sig.LastMatchedAt, sig.BehavioralHash, sig.CreatedAt, time.Now())
	return err
}

func (s *PostgresSignatureStore) Get(ctx context.Context, id string) (*models.IncidentSignature, error) {
	var sig models.IncidentSignature
	var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures WHERE signature_id = $1",
		id).Scan(&sig.SignatureID, &sig.Shape, &rolesJSON, &symptomsJSON, &patternJSON, &remediationJSON, &vectorJSON, &weightsJSON, &sig.OccurrenceCount, &sig.AvgResolutionTime, &sig.Version, &sig.ParentSignatureID, &sig.DecayFactor, &sig.LastMatchedAt, &sig.BehavioralHash, &sig.CreatedAt, &sig.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rolesJSON, &sig.ServiceRoles); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature service roles")
	}
	if err := json.Unmarshal(symptomsJSON, &sig.Symptoms); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature symptoms")
	}
	if err := json.Unmarshal(patternJSON, &sig.TemporalPattern); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature temporal pattern")
	}
	if err := json.Unmarshal(remediationJSON, &sig.Remediation); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature remediation")
	}
	if err := json.Unmarshal(vectorJSON, &sig.FeatureVector); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature vector")
	}
	if err := json.Unmarshal(weightsJSON, &sig.FeatureWeights); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature weights")
	}
	return &sig, nil
}

func (s *PostgresSignatureStore) List(ctx context.Context, limit int) ([]*models.IncidentSignature, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures ORDER BY occurrence_count DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sigs []*models.IncidentSignature
	for rows.Next() {
		var sig models.IncidentSignature
		var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON []byte
		if err := rows.Scan(&sig.SignatureID, &sig.Shape, &rolesJSON, &symptomsJSON, &patternJSON, &remediationJSON, &vectorJSON, &weightsJSON, &sig.OccurrenceCount, &sig.AvgResolutionTime, &sig.Version, &sig.ParentSignatureID, &sig.DecayFactor, &sig.LastMatchedAt, &sig.BehavioralHash, &sig.CreatedAt, &sig.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rolesJSON, &sig.ServiceRoles); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature service roles")
	}
	if err := json.Unmarshal(symptomsJSON, &sig.Symptoms); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature symptoms")
	}
	if err := json.Unmarshal(patternJSON, &sig.TemporalPattern); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature temporal pattern")
	}
	if err := json.Unmarshal(remediationJSON, &sig.Remediation); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature remediation")
	}
	if err := json.Unmarshal(vectorJSON, &sig.FeatureVector); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature vector")
	}
	if err := json.Unmarshal(weightsJSON, &sig.FeatureWeights); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature weights")
	}
		sigs = append(sigs, &sig)
	}
	return sigs, nil
}

func (s *PostgresSignatureStore) FindByBehavioralHash(ctx context.Context, hash string) ([]*models.IncidentSignature, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures WHERE behavioral_hash = $1 AND decay_factor >= 0.1 ORDER BY occurrence_count DESC", hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sigs []*models.IncidentSignature
	for rows.Next() {
		var sig models.IncidentSignature
		var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON []byte
		if err := rows.Scan(&sig.SignatureID, &sig.Shape, &rolesJSON, &symptomsJSON, &patternJSON, &remediationJSON, &vectorJSON, &weightsJSON, &sig.OccurrenceCount, &sig.AvgResolutionTime, &sig.Version, &sig.ParentSignatureID, &sig.DecayFactor, &sig.LastMatchedAt, &sig.BehavioralHash, &sig.CreatedAt, &sig.UpdatedAt); err != nil {
			continue
		}
		if err := json.Unmarshal(rolesJSON, &sig.ServiceRoles); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature service roles")
	}
	if err := json.Unmarshal(symptomsJSON, &sig.Symptoms); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature symptoms")
	}
	if err := json.Unmarshal(patternJSON, &sig.TemporalPattern); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature temporal pattern")
	}
	if err := json.Unmarshal(remediationJSON, &sig.Remediation); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature remediation")
	}
	if err := json.Unmarshal(vectorJSON, &sig.FeatureVector); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature vector")
	}
	if err := json.Unmarshal(weightsJSON, &sig.FeatureWeights); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal signature feature weights")
	}
		sigs = append(sigs, &sig)
	}
	return sigs, nil
}

func (s *PostgresSignatureStore) UpdateDecay(ctx context.Context, signatureID string, factor float64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE incident_signatures SET decay_factor = $1, updated_at = $2 WHERE signature_id = $3", factor, time.Now(), signatureID)
	return err
}

func (s *PostgresSignatureStore) ArchiveStale(ctx context.Context, threshold float64) (int, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM incident_signatures WHERE decay_factor < $1", threshold)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

func (s *PostgresSignatureStore) UpdateLastMatched(ctx context.Context, signatureID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, "UPDATE incident_signatures SET last_matched_at = $1, updated_at = $2 WHERE signature_id = $3", now, now, signatureID)
	return err
}

func (s *PostgresSignatureStore) FindSimilar(ctx context.Context, sig *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, occurrence_count, avg_resolution_time_seconds FROM incident_signatures WHERE signature_id != $1 ORDER BY occurrence_count DESC LIMIT $2", sig.SignatureID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.SimilarIncident
	for rows.Next() {
		var candidate models.IncidentSignature
		var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON []byte
		if err := rows.Scan(&candidate.SignatureID, &candidate.Shape, &rolesJSON, &symptomsJSON, &patternJSON, &remediationJSON, &vectorJSON, &candidate.OccurrenceCount, &candidate.AvgResolutionTime); err != nil {
			continue
		}
		if err := json.Unmarshal(rolesJSON, &candidate.ServiceRoles); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal candidate service roles")
	}
	if err := json.Unmarshal(symptomsJSON, &candidate.Symptoms); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal candidate symptoms")
	}
	if err := json.Unmarshal(patternJSON, &candidate.TemporalPattern); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal candidate temporal pattern")
	}
	if err := json.Unmarshal(remediationJSON, &candidate.Remediation); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal candidate remediation")
	}
	if err := json.Unmarshal(vectorJSON, &candidate.FeatureVector); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal candidate feature vector")
	}

		similarity := computeSimilarityPostgres(sig, &candidate)
		if similarity >= 0.5 {
			resolution := ""
			if len(candidate.Remediation) > 0 {
				resolution = candidate.Remediation[0]
			}
			results = append(results, &models.SimilarIncident{
				IncidentID:     candidate.SignatureID,
				Similarity:     similarity,
				Shape:          candidate.Shape,
				Resolution:     resolution,
				ResolutionTime: candidate.AvgResolutionTime,
			})
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func computeSimilarityPostgres(a, b *models.IncidentSignature) float64 {
	var shapeSim, symptomSim, vectorSim float64

	if a.Shape != "" && b.Shape != "" {
		if a.Shape == b.Shape {
			shapeSim = 1.0
		} else if strings.Contains(a.Shape, b.Shape) || strings.Contains(b.Shape, a.Shape) {
			shapeSim = 0.7
		} else {
			shapeSim = 0.3
		}
	}

	aSymptoms := make(map[string]bool)
	bSymptoms := make(map[string]bool)
	for _, s := range a.Symptoms {
		aSymptoms[string(s)] = true
	}
	for _, s := range b.Symptoms {
		bSymptoms[string(s)] = true
	}

	intersection := 0
	unionKeys := make(map[string]struct{}, len(aSymptoms)+len(bSymptoms))
	for k := range bSymptoms {
		bSymptoms[k] = true
	}
	for k := range aSymptoms {
		unionKeys[k] = struct{}{}
	}
	for k := range bSymptoms {
		unionKeys[k] = struct{}{}
		if aSymptoms[k] {
			intersection++
		}
	}
	if len(unionKeys) > 0 {
		symptomSim = float64(intersection) / float64(len(unionKeys))
	}

	if len(a.FeatureVector) > 0 && len(b.FeatureVector) > 0 {
		dotProduct := 0.0
		aMag := 0.0
		bMag := 0.0
		minLen := len(a.FeatureVector)
		if len(b.FeatureVector) < minLen {
			minLen = len(b.FeatureVector)
		}
		for i := 0; i < minLen; i++ {
			dotProduct += a.FeatureVector[i] * b.FeatureVector[i]
			aMag += a.FeatureVector[i] * a.FeatureVector[i]
			bMag += b.FeatureVector[i] * b.FeatureVector[i]
		}
		if aMag > 0 && bMag > 0 {
			vectorSim = dotProduct / (math.Sqrt(aMag) * math.Sqrt(bMag))
		}
	}

	return 0.4*shapeSim + 0.3*symptomSim + 0.3*vectorSim
}

type PostgresRemediationStore struct {
	db *sql.DB
}

func (s *PostgresRemediationStore) Save(ctx context.Context, rem *models.Remediation) error {
	attrsJSON, _ := json.Marshal(rem.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO remediations (remediation_id, incident_id, signature_id, action, timestamp, operator, attributes) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		rem.RemediationID, rem.IncidentID, rem.SignatureID, rem.Action, rem.Timestamp, rem.Operator, attrsJSON)
	return err
}

func (s *PostgresRemediationStore) Get(ctx context.Context, id string) (*models.Remediation, error) {
	var rem models.Remediation
	var attrsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT remediation_id, incident_id, signature_id, action, timestamp, operator, attributes FROM remediations WHERE remediation_id = $1", id).
		Scan(&rem.RemediationID, &rem.IncidentID, &rem.SignatureID, &rem.Action, &rem.Timestamp, &rem.Operator, &attrsJSON)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &rem.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal remediation attributes")
	}
	return &rem, nil
}

func (s *PostgresRemediationStore) ListByIncident(ctx context.Context, incidentID string) ([]*models.Remediation, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT remediation_id, incident_id, signature_id, action, timestamp, operator, attributes FROM remediations WHERE incident_id = $1 ORDER BY timestamp", incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var remediations []*models.Remediation
	for rows.Next() {
		var rem models.Remediation
		var attrsJSON []byte
		if err := rows.Scan(&rem.RemediationID, &rem.IncidentID, &rem.SignatureID, &rem.Action, &rem.Timestamp, &rem.Operator, &attrsJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(attrsJSON, &rem.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal remediation attributes")
	}
		remediations = append(remediations, &rem)
	}
	return remediations, nil
}

func (s *PostgresRemediationStore) ListBySignature(ctx context.Context, signatureID string, limit int) ([]*models.RemediationStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.action, 
		       COUNT(*) as total,
		       SUM(CASE WHEN f.outcome_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END) as successful,
		       AVG(f.time_to_resolve_seconds) as avg_time
		FROM remediations r
		LEFT JOIN remediation_feedback f ON r.remediation_id = f.remediation_id
		WHERE r.signature_id = $1
		GROUP BY r.action
		ORDER BY successful DESC
		LIMIT $2`, signatureID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*models.RemediationStats
	for rows.Next() {
		var s models.RemediationStats
		var avgTime sql.NullInt64
		if err := rows.Scan(&s.Action, &s.TotalAttempts, &s.SuccessfulCount, &avgTime); err != nil {
			return nil, err
		}
		if s.TotalAttempts > 0 {
			s.SuccessRate = float64(s.SuccessfulCount) / float64(s.TotalAttempts)
		}
		if avgTime.Valid {
			s.AvgTimeToResolve = avgTime.Int64
		}
		stats = append(stats, &s)
	}
	return stats, nil
}

type PostgresFeedbackStore struct {
	db *sql.DB
}

func (s *PostgresFeedbackStore) Save(ctx context.Context, fb *models.RemediationFeedback) error {
	fb.OutcomeCategory = models.OutcomeCategory(fb.OutcomeCode)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO remediation_feedback (feedback_id, remediation_id, incident_id, outcome_code, outcome_category, time_to_resolve_seconds, timestamp, notes) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		fb.FeedbackID, fb.RemediationID, fb.IncidentID, fb.OutcomeCode, fb.OutcomeCategory, fb.TimeToResolve, fb.Timestamp, fb.Notes)
	return err
}

func (s *PostgresFeedbackStore) GetByRemediation(ctx context.Context, remediationID string) ([]*models.RemediationFeedback, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT feedback_id, remediation_id, incident_id, outcome_code, outcome_category, time_to_resolve_seconds, timestamp, notes FROM remediation_feedback WHERE remediation_id = $1 ORDER BY timestamp", remediationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []*models.RemediationFeedback
	for rows.Next() {
		var fb models.RemediationFeedback
		if err := rows.Scan(&fb.FeedbackID, &fb.RemediationID, &fb.IncidentID, &fb.OutcomeCode, &fb.OutcomeCategory, &fb.TimeToResolve, &fb.Timestamp, &fb.Notes); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, &fb)
	}
	return feedbacks, nil
}

func (s *PostgresFeedbackStore) GetSuccessRate(ctx context.Context, action string, signatureID string) (float64, error) {
	var rate float64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(CASE WHEN outcome_code BETWEEN 200 AND 299 THEN 1.0 ELSE 0.0 END), 0)
		FROM remediation_feedback f
		JOIN remediations r ON f.remediation_id = r.remediation_id
		WHERE r.action = $1 AND r.signature_id = $2`, action, signatureID).Scan(&rate)
	return rate, err
}
