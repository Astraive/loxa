package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/astraive/loza/cortex/internal/models"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/rs/zerolog/log"
)

type DuckDBStorage struct {
	db *sql.DB
}

func NewDuckDBStorage(path string) (*DuckDBStorage, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping duckdb: %w", err)
	}

	return &DuckDBStorage{db: db}, nil
}

// NewDuckDBStorageFromDB creates a storage using an existing DB connection (for unified mode).
func NewDuckDBStorageFromDB(db *sql.DB) *DuckDBStorage {
	return &DuckDBStorage{db: db}
}

func (s *DuckDBStorage) Init(ctx context.Context) error {
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
		duration_ms DOUBLE,
		trace_id TEXT,
		span_id TEXT,
		trace_flags TEXT,
		request_id TEXT,
		http JSON,
		user JSON,
		tenant JSON,
		attrs JSON,
		error JSON,
		checkpoints JSON,
		processes JSON,
		groups JSON,
		timers JSON,
		links JSON,
		sdk_name TEXT,
		sdk_version TEXT,
		sdk_language TEXT,
		raw JSON,
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
		attributes JSON,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_graph_nodes_type ON graph_nodes(type);

	CREATE TABLE IF NOT EXISTS graph_edges (
		id TEXT PRIMARY KEY,
		from_node_id TEXT NOT NULL,
		to_node_id TEXT NOT NULL,
		type TEXT NOT NULL,
		weight DOUBLE DEFAULT 1.0,
		attributes JSON,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (from_node_id) REFERENCES graph_nodes(id),
		FOREIGN KEY (to_node_id) REFERENCES graph_nodes(id)
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
		affected_services JSON,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		resolved_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS incident_signatures (
		signature_id TEXT PRIMARY KEY,
		shape TEXT NOT NULL,
		service_roles JSON,
		symptoms JSON,
		temporal_pattern JSON,
		remediation JSON,
		feature_vector JSON,
		feature_weights JSON,
		occurrence_count INTEGER DEFAULT 0,
		avg_resolution_time_seconds BIGINT DEFAULT 0,
		version INTEGER DEFAULT 1,
		parent_signature_id TEXT,
		decay_factor DOUBLE DEFAULT 1.0,
		last_matched_at TIMESTAMP,
		behavioral_hash TEXT,
		embedding FLOAT[768],
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS remediations (
		remediation_id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		signature_id TEXT,
		action TEXT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		operator TEXT,
		attributes JSON,
		FOREIGN KEY (incident_id) REFERENCES incidents(id)
	);

	CREATE TABLE IF NOT EXISTS remediation_feedback (
		feedback_id TEXT PRIMARY KEY,
		remediation_id TEXT NOT NULL,
		incident_id TEXT NOT NULL,
		outcome_code INTEGER NOT NULL,
		outcome_category TEXT NOT NULL,
		time_to_resolve_seconds BIGINT,
		timestamp TIMESTAMP NOT NULL,
		notes TEXT,
		FOREIGN KEY (remediation_id) REFERENCES remediations(remediation_id)
	);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *DuckDBStorage) Close() error {
	return s.db.Close()
}

func (s *DuckDBStorage) Events() EventStore {
	return &DuckDBEventStore{db: s.db}
}

func (s *DuckDBStorage) Topology() TopologyStore {
	return &DuckDBTopologyStore{db: s.db}
}

func (s *DuckDBStorage) Graph() GraphStore {
	return &DuckDBGraphStore{db: s.db}
}

func (s *DuckDBStorage) Incidents() IncidentStore {
	return &DuckDBIncidentStore{db: s.db}
}

func (s *DuckDBStorage) Signatures() SignatureStore {
	return &DuckDBSignatureStore{db: s.db}
}

func (s *DuckDBStorage) Remediations() RemediationStore {
	return &DuckDBRemediationStore{db: s.db}
}

func (s *DuckDBStorage) Feedback() FeedbackStore {
	return &DuckDBFeedbackStore{db: s.db}
}

type DuckDBEventStore struct {
	db *sql.DB
}

func (s *DuckDBEventStore) Save(ctx context.Context, event *models.Event, lifecycle *LifecycleData) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.EventID, event.Timestamp, event.Service, event.Environment, event.Release,
		event.SchemaVersion, event.EventVersion, event.Event, event.Kind, event.Level, event.Outcome, event.DurationMs,
		event.TraceID, event.SpanID, event.TraceFlags, event.RequestID,
		httpJSON, userJSON, tenantJSON, attrsJSON, errorJSON,
		checkpointsJSON, processesJSON, groupsJSON, timersJSON, linksJSON,
		event.SDKName, event.SDKVersion, event.SDKLanguage,
		rawJSON, event.Provenance, event.IncidentID, time.Now())
	return err
}

func (s *DuckDBEventStore) SaveBatch(ctx context.Context, events []*models.Event, lifecycles []*LifecycleData) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
		_ = lifecycles // lifecycles are embedded in the event struct
	}

	return tx.Commit()
}

func scanRowEvent(scanner interface{ Scan(dest ...any) error }) (*models.Event, error) {
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

func queryAllEvents(ctx context.Context, db *sql.DB, query string, args ...interface{}) ([]*models.Event, error) {
	selectCols := "id, event_id, timestamp, service, environment, release, schema_version, event_version, event, kind, level, outcome, duration_ms, trace_id, span_id, trace_flags, request_id, http, user, tenant, attrs, error, checkpoints, processes, groups, timers, links, sdk_name, sdk_version, sdk_language, raw, provenance, incident_id, created_at"
	fullQuery := "SELECT " + selectCols + " FROM events " + query
	rows, err := db.QueryContext(ctx, fullQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		event, err := scanRowEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *DuckDBEventStore) Get(ctx context.Context, id string) (*models.Event, error) {
	return querySingleEvent(ctx, s.db, "WHERE id = ?", id)
}

func querySingleEvent(ctx context.Context, db *sql.DB, where string, args ...interface{}) (*models.Event, error) {
	selectCols := "id, event_id, timestamp, service, environment, release, schema_version, event_version, event, kind, level, outcome, duration_ms, trace_id, span_id, trace_flags, request_id, http, user, tenant, attrs, error, checkpoints, processes, groups, timers, links, sdk_name, sdk_version, sdk_language, raw, provenance, incident_id, created_at"
	query := "SELECT " + selectCols + " FROM events " + where + " LIMIT 1"
	row := db.QueryRowContext(ctx, query, args...)
	return scanRowEvent(row)
}

func (s *DuckDBEventStore) List(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "ORDER BY timestamp DESC LIMIT ? OFFSET ?", limit, offset)
}

func (s *DuckDBEventStore) FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE trace_id = ? ORDER BY timestamp", traceID)
}

func (s *DuckDBEventStore) FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE incident_id = ? ORDER BY timestamp", incidentID)
}

func (s *DuckDBEventStore) FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error) {
	query := "WHERE service = ?"
	args := []interface{}{service}
	if from != "" {
		query += " AND timestamp >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND timestamp <= ?"
		args = append(args, to)
	}
	query += " ORDER BY timestamp"
	return queryAllEvents(ctx, s.db, query, args...)
}

// Lifecycle-aware query methods
func (s *DuckDBEventStore) FindByEventName(ctx context.Context, eventName string, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE event = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?", eventName, limit, offset)
}

func (s *DuckDBEventStore) FindByOutcome(ctx context.Context, outcome string, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE outcome = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?", outcome, limit, offset)
}

func (s *DuckDBEventStore) FindByLevel(ctx context.Context, level string, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE level = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?", level, limit, offset)
}

func (s *DuckDBEventStore) FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE duration_ms >= ? AND duration_ms <= ? ORDER BY duration_ms DESC LIMIT ? OFFSET ?", minMs, maxMs, limit, offset)
}

func (s *DuckDBEventStore) FindByEnvironment(ctx context.Context, env string, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE environment = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?", env, limit, offset)
}

func (s *DuckDBEventStore) FindByRelease(ctx context.Context, release string, limit, offset int) ([]*models.Event, error) {
	return queryAllEvents(ctx, s.db, "WHERE release = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?", release, limit, offset)
}

func (s *DuckDBEventStore) CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT outcome, COUNT(*) as cnt FROM events WHERE service = ? AND timestamp >= ? AND timestamp <= ? GROUP BY outcome", service, from, to)
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

func (s *DuckDBEventStore) CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT event, COUNT(*) as cnt FROM events WHERE service = ? AND timestamp >= ? AND timestamp <= ? AND event IS NOT NULL GROUP BY event", service, from, to)
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

func (s *DuckDBEventStore) AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error) {
	var avg sql.NullFloat64
	err := s.db.QueryRowContext(ctx, "SELECT AVG(duration_ms) FROM events WHERE event = ? AND timestamp >= ? AND timestamp <= ? AND duration_ms IS NOT NULL", eventName, from, to).Scan(&avg)
	if err != nil || !avg.Valid {
		return 0, err
	}
	return avg.Float64, nil
}

func (s *DuckDBEventStore) PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error) {
	var val sql.NullFloat64
	// Use DuckDB's approx_quantile function
	err := s.db.QueryRowContext(ctx, "SELECT approx_quantile(duration_ms, ?) FROM events WHERE event = ? AND timestamp >= ? AND timestamp <= ? AND duration_ms IS NOT NULL", percentile, eventName, from, to).Scan(&val)
	if err != nil || !val.Valid {
		return 0, err
	}
	return val.Float64, nil
}

func (s *DuckDBEventStore) DistinctServices(ctx context.Context) ([]string, error) {
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

func (s *DuckDBEventStore) DistinctEventNames(ctx context.Context) ([]string, error) {
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

func (s *DuckDBEventStore) ListLifecycleSummaries(ctx context.Context, filter *LifecycleFilter) ([]*models.LifecycleSummary, int, error) {
	where := "WHERE 1=1"
	var args []interface{}
	if filter != nil {
		if filter.Service != "" {
			where += " AND service = ?"
			args = append(args, filter.Service)
		}
		if filter.EventName != "" {
			where += " AND event = ?"
			args = append(args, filter.EventName)
		}
		if filter.Outcome != "" {
			where += " AND outcome = ?"
			args = append(args, filter.Outcome)
		}
		if filter.Level != "" {
			where += " AND level = ?"
			args = append(args, filter.Level)
		}
		if filter.TraceID != "" {
			where += " AND trace_id = ?"
			args = append(args, filter.TraceID)
		}
		if !filter.From.IsZero() {
			where += " AND timestamp >= ?"
			args = append(args, filter.From)
		}
		if !filter.To.IsZero() {
			where += " AND timestamp <= ?"
			args = append(args, filter.To)
		}
		if filter.MinDuration > 0 {
			where += " AND duration_ms >= ?"
			args = append(args, filter.MinDuration)
		}
		if filter.MaxDuration > 0 {
			where += " AND duration_ms <= ?"
			args = append(args, filter.MaxDuration)
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

	query := "SELECT id, event, service, outcome, duration_ms," +
		" (SELECT COUNT(*) FROM json_array_length(COALESCE(checkpoints, '[]'))) as cp_count," +
		" (SELECT COUNT(*) FROM json_array_length(COALESCE(processes, '[]'))) as pr_count," +
		" (SELECT COUNT(*) FROM json_array_length(COALESCE(groups, '[]'))) as gr_count," +
		" (SELECT COUNT(*) FROM json_array_length(COALESCE(timers, '[]'))) as ti_count," +
		" (SELECT COUNT(*) FROM json_array_length(COALESCE(links, '[]'))) as li_count," +
		" trace_id" +
		" FROM events " + where + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
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

type DuckDBTopologyStore struct {
	db *sql.DB
}

func (s *DuckDBTopologyStore) SaveAlias(ctx context.Context, alias *models.ServiceAlias) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO topology_aliases (id, alias, canonical, valid_from, valid_to, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		alias.ID, alias.Alias, alias.Canonical, alias.ValidFrom, alias.ValidTo, time.Now())
	return err
}

func (s *DuckDBTopologyStore) GetAlias(ctx context.Context, alias string, timestamp string) (*models.ServiceAlias, error) {
	var a models.ServiceAlias
	ts, _ := time.Parse(time.RFC3339, timestamp)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, alias, canonical, valid_from, valid_to, created_at FROM topology_aliases WHERE alias = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) ORDER BY valid_from DESC LIMIT 1",
		alias, ts, ts).Scan(&a.ID, &a.Alias, &a.Canonical, &a.ValidFrom, &a.ValidTo, &a.CreatedAt)
	return &a, err
}

func (s *DuckDBTopologyStore) GetHistory(ctx context.Context, service string) ([]*models.ServiceAlias, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, alias, canonical, valid_from, valid_to, created_at FROM topology_aliases WHERE alias = ? OR canonical = ? ORDER BY valid_from",
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

type DuckDBGraphStore struct {
	db *sql.DB
}

func (s *DuckDBGraphStore) SaveNode(ctx context.Context, node *models.Node) error {
	attrsJSON, _ := json.Marshal(node.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO graph_nodes (id, type, label, attributes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO UPDATE SET label = EXCLUDED.label, attributes = EXCLUDED.attributes, updated_at = EXCLUDED.updated_at",
		node.ID, node.Type, node.Label, attrsJSON, node.CreatedAt, time.Now())
	return err
}

func (s *DuckDBGraphStore) GetNode(ctx context.Context, id string) (*models.Node, error) {
	var node models.Node
	var attrsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, type, label, attributes, created_at, updated_at FROM graph_nodes WHERE id = ?", id).
		Scan(&node.ID, &node.Type, &node.Label, &attrsJSON, &node.CreatedAt, &node.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &node.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal node attributes")
	}
	return &node, nil
}

func (s *DuckDBGraphStore) ListNodes(ctx context.Context, nodeType string, limit int) ([]*models.Node, error) {
	query := "SELECT id, type, label, attributes, created_at, updated_at FROM graph_nodes"
	var args []interface{}
	if nodeType != "" {
		query += " WHERE type = ?"
		args = append(args, nodeType)
	}
	query += " LIMIT ?"
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

func (s *DuckDBGraphStore) SaveEdge(ctx context.Context, edge *models.Edge) error {
	attrsJSON, _ := json.Marshal(edge.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO graph_edges (id, from_node_id, to_node_id, type, weight, attributes, created_at) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO NOTHING",
		edge.ID, edge.FromNodeID, edge.ToNodeID, edge.Type, edge.Weight, attrsJSON, time.Now())
	return err
}

func (s *DuckDBGraphStore) GetEdges(ctx context.Context, nodeID string, edgeType string) ([]*models.Edge, error) {
	var args []interface{}
	args = append(args, nodeID, nodeID)
	query := "SELECT id, from_node_id, to_node_id, type, weight, attributes, created_at FROM graph_edges WHERE (from_node_id = ? OR to_node_id = ?)"
	if edgeType != "" {
		query += " AND type = ?"
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

func (s *DuckDBGraphStore) Traverse(ctx context.Context, startNodeID string, opts models.TraversalOptions) (*models.GraphView, error) {
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 3
	}

	// Build edge type filter
	edgeTypeFilter := ""
	if len(opts.EdgeTypes) > 0 {
		quoted := make([]string, len(opts.EdgeTypes))
		for i, t := range opts.EdgeTypes {
			quoted[i] = "'" + strings.ReplaceAll(string(t), "'", "''") + "'"
		}
		edgeTypeFilter = "AND e.type IN (" + strings.Join(quoted, ",") + ")"
	}

	// Recursive CTE for graph traversal
	query := fmt.Sprintf(`
		WITH RECURSIVE traverse AS (
			SELECT
				n.id AS node_id, n.type AS node_type, n.label, n.attributes AS node_attrs,
				n.created_at AS node_created, n.updated_at AS node_updated,
				e.id AS edge_id, e.from_node_id, e.to_node_id, e.type AS edge_type,
				e.weight, e.attributes AS edge_attrs, e.created_at AS edge_created,
				0 AS depth
			FROM graph_nodes n
			LEFT JOIN graph_edges e ON e.from_node_id = n.id %s
			WHERE n.id = ?

			UNION ALL

			SELECT
				n.id, n.type, n.label, n.attributes,
				n.created_at, n.updated_at,
				e.id, e.from_node_id, e.to_node_id, e.type,
				e.weight, e.attributes, e.created_at,
				t.depth + 1
			FROM traverse t
			JOIN graph_nodes n ON n.id = t.to_node_id
			LEFT JOIN graph_edges e ON e.from_node_id = n.id %s
			WHERE t.to_node_id IS NOT NULL
			  AND t.depth < ?
			  AND t.to_node_id != ?
		)
		SELECT DISTINCT
			node_id, node_type, label, node_attrs, node_created, node_updated,
			edge_id, from_node_id, to_node_id, edge_type, weight, edge_attrs, edge_created
		FROM traverse
		WHERE edge_id IS NOT NULL
		ORDER BY depth
	`, edgeTypeFilter, edgeTypeFilter)

	rows, err := s.db.QueryContext(ctx, query, startNodeID, opts.MaxDepth, startNodeID)
	if err != nil {
		return nil, fmt.Errorf("graph traverse: %w", err)
	}
	defer rows.Close()

	seenNodes := make(map[string]bool)
	var resultNodes []*models.Node
	var resultEdges []*models.Edge

	for rows.Next() {
		var nodeID, nodeType, label string
		var nodeAttrsJSON []byte
		var nodeCreated, nodeUpdated time.Time
		var edgeID, fromNodeID, toNodeID, edgeType string
		var weight float64
		var edgeAttrsJSON []byte
		var createdAt time.Time

		if err := rows.Scan(&nodeID, &nodeType, &label, &nodeAttrsJSON, &nodeCreated, &nodeUpdated,
			&edgeID, &fromNodeID, &toNodeID, &edgeType, &weight, &edgeAttrsJSON, &createdAt); err != nil {
			continue
		}

		// Add node if not seen
		if !seenNodes[nodeID] {
			seenNodes[nodeID] = true
			nodeAttrs := make(map[string]interface{})
			if err := json.Unmarshal(nodeAttrsJSON, &nodeAttrs); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal node attrs JSON")
			}
			resultNodes = append(resultNodes, &models.Node{
				ID:        nodeID,
				Type:      models.NodeType(nodeType),
				Label:     label,
				Attributes: nodeAttrs,
				CreatedAt: nodeCreated,
				UpdatedAt: nodeUpdated,
			})
		}

		// Add edge
		edgeAttrs := make(map[string]interface{})
		if err := json.Unmarshal(edgeAttrsJSON, &edgeAttrs); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal edge attrs JSON")
		}
		resultEdges = append(resultEdges, &models.Edge{
			ID:         edgeID,
			FromNodeID: fromNodeID,
			ToNodeID:   toNodeID,
			Type:       models.EdgeType(edgeType),
			Weight:     weight,
			Attributes: edgeAttrs,
			CreatedAt:  createdAt,
		})
	}

	return &models.GraphView{Nodes: resultNodes, Edges: resultEdges}, nil
}

type DuckDBIncidentStore struct {
	db *sql.DB
}

func (s *DuckDBIncidentStore) Save(ctx context.Context, incident *models.Incident) error {
	servicesJSON, _ := json.Marshal(incident.AffectedServices)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO incidents (id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, resolved_at = EXCLUDED.resolved_at",
		incident.ID, incident.Timestamp, incident.SignatureID, incident.Status, incident.Severity, incident.PrimaryService, servicesJSON, time.Now(), incident.ResolvedAt)
	return err
}

func (s *DuckDBIncidentStore) Get(ctx context.Context, id string) (*models.Incident, error) {
	var inc models.Incident
	var servicesJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents WHERE id = ?", id).
		Scan(&inc.ID, &inc.Timestamp, &inc.SignatureID, &inc.Status, &inc.Severity, &inc.PrimaryService, &servicesJSON, &inc.CreatedAt, &inc.ResolvedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(servicesJSON, &inc.AffectedServices); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal incident affected services")
	}
	return &inc, nil
}

func (s *DuckDBIncidentStore) GetBySignature(ctx context.Context, signatureID string) (*models.Incident, error) {
	var inc models.Incident
	var servicesJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents WHERE signature_id = ? LIMIT 1", signatureID).
		Scan(&inc.ID, &inc.Timestamp, &inc.SignatureID, &inc.Status, &inc.Severity, &inc.PrimaryService, &servicesJSON, &inc.CreatedAt, &inc.ResolvedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(servicesJSON, &inc.AffectedServices); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal incident affected services")
	}
	return &inc, nil
}

func (s *DuckDBIncidentStore) List(ctx context.Context, limit, offset int) ([]*models.Incident, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, signature_id, status, severity, primary_service, affected_services, created_at, resolved_at FROM incidents ORDER BY timestamp DESC LIMIT ? OFFSET ?", limit, offset)
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

type DuckDBSignatureStore struct {
	db *sql.DB
}

func (s *DuckDBSignatureStore) Save(ctx context.Context, sig *models.IncidentSignature) error {
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT (signature_id) DO UPDATE SET
		shape = EXCLUDED.shape, service_roles = EXCLUDED.service_roles, symptoms = EXCLUDED.symptoms, temporal_pattern = EXCLUDED.temporal_pattern, remediation = EXCLUDED.remediation, feature_vector = EXCLUDED.feature_vector, feature_weights = EXCLUDED.feature_weights, occurrence_count = EXCLUDED.occurrence_count, avg_resolution_time_seconds = EXCLUDED.avg_resolution_time_seconds, version = EXCLUDED.version, parent_signature_id = EXCLUDED.parent_signature_id, decay_factor = EXCLUDED.decay_factor, last_matched_at = EXCLUDED.last_matched_at, behavioral_hash = EXCLUDED.behavioral_hash, updated_at = EXCLUDED.updated_at`,
		sig.SignatureID, sig.Shape, rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON, sig.OccurrenceCount, sig.AvgResolutionTime, sig.Version, sig.ParentSignatureID, sig.DecayFactor, sig.LastMatchedAt, sig.BehavioralHash, sig.CreatedAt, time.Now())
	return err
}

func (s *DuckDBSignatureStore) Get(ctx context.Context, id string) (*models.IncidentSignature, error) {
	var sig models.IncidentSignature
	var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON, weightsJSON []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures WHERE signature_id = ?",
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

func (s *DuckDBSignatureStore) List(ctx context.Context, limit int) ([]*models.IncidentSignature, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures ORDER BY occurrence_count DESC LIMIT ?", limit)
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

func (s *DuckDBSignatureStore) FindByBehavioralHash(ctx context.Context, hash string) ([]*models.IncidentSignature, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, feature_weights, occurrence_count, avg_resolution_time_seconds, version, parent_signature_id, decay_factor, last_matched_at, behavioral_hash, created_at, updated_at FROM incident_signatures WHERE behavioral_hash = ? AND decay_factor >= 0.1 ORDER BY occurrence_count DESC", hash)
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

func (s *DuckDBSignatureStore) UpdateDecay(ctx context.Context, signatureID string, factor float64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE incident_signatures SET decay_factor = ?, updated_at = ? WHERE signature_id = ?", factor, time.Now(), signatureID)
	return err
}

func (s *DuckDBSignatureStore) ArchiveStale(ctx context.Context, threshold float64) (int, error) {
	result, err := s.db.ExecContext(ctx, "DELETE FROM incident_signatures WHERE decay_factor < ?", threshold)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

func (s *DuckDBSignatureStore) UpdateLastMatched(ctx context.Context, signatureID string) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx, "UPDATE incident_signatures SET last_matched_at = ?, updated_at = ? WHERE signature_id = ?", now, now, signatureID)
	return err
}

func (s *DuckDBSignatureStore) FindSimilar(ctx context.Context, sig *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error) {
	// Try SQL-based cosine similarity on embeddings first
	if len(sig.FeatureVector) > 0 {
		results, err := s.findSimilarSQL(ctx, sig, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
	}

	// Fallback: Go-level similarity computation
	rows, err := s.db.QueryContext(ctx, "SELECT signature_id, shape, service_roles, symptoms, temporal_pattern, remediation, feature_vector, occurrence_count, avg_resolution_time_seconds, decay_factor FROM incident_signatures WHERE signature_id != ? AND decay_factor >= 0.1 ORDER BY occurrence_count DESC LIMIT ?", sig.SignatureID, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.SimilarIncident
	for rows.Next() {
		var candidate models.IncidentSignature
		var rolesJSON, symptomsJSON, patternJSON, remediationJSON, vectorJSON []byte
		if err := rows.Scan(&candidate.SignatureID, &candidate.Shape, &rolesJSON, &symptomsJSON, &patternJSON, &remediationJSON, &vectorJSON, &candidate.OccurrenceCount, &candidate.AvgResolutionTime, &candidate.DecayFactor); err != nil {
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

		similarity := computeSimilarity(sig, &candidate)
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

func computeSimilarity(a, b *models.IncidentSignature) float64 {
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

// findSimilarSQL uses DuckDB array functions for SQL-based cosine similarity.
func (s *DuckDBSignatureStore) findSimilarSQL(ctx context.Context, sig *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error) {
	// Build the query vector as a DuckDB array literal
	vectorJSON, _ := json.Marshal(sig.FeatureVector)

	query := `
		SELECT
			signature_id,
			shape,
			occurrence_count,
			avg_resolution_time_seconds,
			remediation,
			-- Cosine similarity using DuckDB array functions
			CASE
				WHEN embedding IS NOT NULL AND len(embedding) > 0 THEN
					list_sum(list_transform(list_zip(embedding, $2::FLOAT[]), x -> x[1] * x[2]))
					/ (sqrt(list_sum(list_transform(embedding, x -> x * x)))
					* sqrt(list_sum(list_transform($2::FLOAT[], x -> x * x))))
				ELSE 0.0
			END AS similarity
		FROM incident_signatures
		WHERE signature_id != $1
		  AND decay_factor >= 0.1
		HAVING similarity >= 0.3
		ORDER BY similarity DESC
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, sig.SignatureID, string(vectorJSON), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*models.SimilarIncident
	for rows.Next() {
		var candidate models.SimilarIncident
		var remediationJSON []byte
		var occCount int64
		if err := rows.Scan(&candidate.IncidentID, &candidate.Shape, &occCount, &candidate.ResolutionTime, &remediationJSON, &candidate.Similarity); err != nil {
			continue
		}
		if len(remediationJSON) > 0 {
			var remediations []string
			if err := json.Unmarshal(remediationJSON, &remediations); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal remediations")
	}
			if len(remediations) > 0 {
				candidate.Resolution = remediations[0]
			}
		}
		results = append(results, &candidate)
	}
	return results, nil
}

type DuckDBRemediationStore struct {
	db *sql.DB
}

func (s *DuckDBRemediationStore) Save(ctx context.Context, rem *models.Remediation) error {
	attrsJSON, _ := json.Marshal(rem.Attributes)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO remediations (remediation_id, incident_id, signature_id, action, timestamp, operator, attributes) VALUES (?, ?, ?, ?, ?, ?, ?)",
		rem.RemediationID, rem.IncidentID, rem.SignatureID, rem.Action, rem.Timestamp, rem.Operator, attrsJSON)
	return err
}

func (s *DuckDBRemediationStore) Get(ctx context.Context, id string) (*models.Remediation, error) {
	var rem models.Remediation
	var attrsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT remediation_id, incident_id, signature_id, action, timestamp, operator, attributes FROM remediations WHERE remediation_id = ?", id).
		Scan(&rem.RemediationID, &rem.IncidentID, &rem.SignatureID, &rem.Action, &rem.Timestamp, &rem.Operator, &attrsJSON)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &rem.Attributes); err != nil {
		log.Warn().Err(err).Msg("failed to unmarshal remediation attributes")
	}
	return &rem, nil
}

func (s *DuckDBRemediationStore) ListByIncident(ctx context.Context, incidentID string) ([]*models.Remediation, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT remediation_id, incident_id, signature_id, action, timestamp, operator, attributes FROM remediations WHERE incident_id = ? ORDER BY timestamp", incidentID)
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

func (s *DuckDBRemediationStore) ListBySignature(ctx context.Context, signatureID string, limit int) ([]*models.RemediationStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.action, 
		       COUNT(*) as total,
		       SUM(CASE WHEN f.outcome_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END) as successful,
		       AVG(f.time_to_resolve_seconds) as avg_time
		FROM remediations r
		LEFT JOIN remediation_feedback f ON r.remediation_id = f.remediation_id
		WHERE r.signature_id = ?
		GROUP BY r.action
		ORDER BY successful DESC
		LIMIT ?`, signatureID, limit)
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

type DuckDBFeedbackStore struct {
	db *sql.DB
}

func (s *DuckDBFeedbackStore) Save(ctx context.Context, fb *models.RemediationFeedback) error {
	fb.OutcomeCategory = models.OutcomeCategory(fb.OutcomeCode)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO remediation_feedback (feedback_id, remediation_id, incident_id, outcome_code, outcome_category, time_to_resolve_seconds, timestamp, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		fb.FeedbackID, fb.RemediationID, fb.IncidentID, fb.OutcomeCode, fb.OutcomeCategory, fb.TimeToResolve, fb.Timestamp, fb.Notes)
	return err
}

func (s *DuckDBFeedbackStore) GetByRemediation(ctx context.Context, remediationID string) ([]*models.RemediationFeedback, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT feedback_id, remediation_id, incident_id, outcome_code, outcome_category, time_to_resolve_seconds, timestamp, notes FROM remediation_feedback WHERE remediation_id = ? ORDER BY timestamp", remediationID)
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

func (s *DuckDBFeedbackStore) GetSuccessRate(ctx context.Context, action string, signatureID string) (float64, error) {
	var rate float64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(AVG(CASE WHEN outcome_code BETWEEN 200 AND 299 THEN 1.0 ELSE 0.0 END), 0)
		FROM remediation_feedback f
		JOIN remediations r ON f.remediation_id = r.remediation_id
		WHERE r.action = ? AND r.signature_id = ?`, action, signatureID).Scan(&rate)
	return rate, err
}
