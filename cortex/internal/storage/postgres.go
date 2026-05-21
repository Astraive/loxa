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
		timestamp TIMESTAMP NOT NULL,
		kind TEXT NOT NULL,
		service TEXT NOT NULL,
		trace_id TEXT,
		incident_id TEXT,
		raw JSONB,
		provenance TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_service ON events(service);
	CREATE INDEX IF NOT EXISTS idx_events_trace_id ON events(trace_id);
	CREATE INDEX IF NOT EXISTS idx_events_incident_id ON events(incident_id);

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
		occurrence_count INTEGER DEFAULT 0,
		avg_resolution_time_seconds BIGINT DEFAULT 0,
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

func (s *PostgresEventStore) Save(ctx context.Context, event *models.Event) error {
	rawJSON, _ := json.Marshal(event.Raw)
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO events (id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		event.ID, event.Timestamp, event.Kind, event.Service, event.TraceID, event.IncidentID, rawJSON, event.Provenance, time.Now())
	return err
}

func (s *PostgresEventStore) SaveBatch(ctx context.Context, events []*models.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO events (id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range events {
		rawJSON, _ := json.Marshal(e.Raw)
		if _, err := stmt.ExecContext(ctx, e.ID, e.Timestamp, e.Kind, e.Service, e.TraceID, e.IncidentID, rawJSON, e.Provenance, time.Now()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresEventStore) Get(ctx context.Context, id string) (*models.Event, error) {
	var event models.Event
	var rawJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at FROM events WHERE id = $1", id).
		Scan(&event.ID, &event.Timestamp, &event.Kind, &event.Service, &event.TraceID, &event.IncidentID, &rawJSON, &event.Provenance, &event.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(rawJSON, &event.Raw)
	return &event, nil
}

func (s *PostgresEventStore) List(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at FROM events ORDER BY timestamp DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var event models.Event
		var rawJSON []byte
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Kind, &event.Service, &event.TraceID, &event.IncidentID, &rawJSON, &event.Provenance, &event.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(rawJSON, &event.Raw)
		events = append(events, &event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at FROM events WHERE trace_id = $1 ORDER BY timestamp", traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var event models.Event
		var rawJSON []byte
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Kind, &event.Service, &event.TraceID, &event.IncidentID, &rawJSON, &event.Provenance, &event.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(rawJSON, &event.Raw)
		events = append(events, &event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at FROM events WHERE incident_id = $1 ORDER BY timestamp", incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		var event models.Event
		var rawJSON []byte
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Kind, &event.Service, &event.TraceID, &event.IncidentID, &rawJSON, &event.Provenance, &event.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(rawJSON, &event.Raw)
		events = append(events, &event)
	}
	return events, nil
}

func (s *PostgresEventStore) FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error) {
	query := "SELECT id, timestamp, kind, service, trace_id, incident_id, raw, provenance, created_at FROM events WHERE service = $1"
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
		var event models.Event
		var rawJSON []byte
		if err := rows.Scan(&event.ID, &event.Timestamp, &event.Kind, &event.Service, &event.TraceID, &event.IncidentID, &rawJSON, &event.Provenance, &event.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(rawJSON, &event.Raw)
		events = append(events, &event)
	}
	return events, nil
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
	json.Unmarshal(attrsJSON, &node.Attributes)
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
		json.Unmarshal(attrsJSON, &node.Attributes)
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
		json.Unmarshal(attrsJSON, &edge.Attributes)
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
				json.Unmarshal(attrsJSON, &edge.Attributes)

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
	json.Unmarshal(servicesJSON, &inc.AffectedServices)
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
	json.Unmarshal(servicesJSON, &inc.AffectedServices)
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
		json.Unmarshal(servicesJSON, &inc.AffectedServices)
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
	json.Unmarshal(rolesJSON, &sig.ServiceRoles)
	json.Unmarshal(symptomsJSON, &sig.Symptoms)
	json.Unmarshal(patternJSON, &sig.TemporalPattern)
	json.Unmarshal(remediationJSON, &sig.Remediation)
	json.Unmarshal(vectorJSON, &sig.FeatureVector)
	json.Unmarshal(weightsJSON, &sig.FeatureWeights)
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
		json.Unmarshal(rolesJSON, &sig.ServiceRoles)
		json.Unmarshal(symptomsJSON, &sig.Symptoms)
		json.Unmarshal(patternJSON, &sig.TemporalPattern)
		json.Unmarshal(remediationJSON, &sig.Remediation)
		json.Unmarshal(vectorJSON, &sig.FeatureVector)
		json.Unmarshal(weightsJSON, &sig.FeatureWeights)
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
		json.Unmarshal(rolesJSON, &sig.ServiceRoles)
		json.Unmarshal(symptomsJSON, &sig.Symptoms)
		json.Unmarshal(patternJSON, &sig.TemporalPattern)
		json.Unmarshal(remediationJSON, &sig.Remediation)
		json.Unmarshal(vectorJSON, &sig.FeatureVector)
		json.Unmarshal(weightsJSON, &sig.FeatureWeights)
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
		json.Unmarshal(rolesJSON, &candidate.ServiceRoles)
		json.Unmarshal(symptomsJSON, &candidate.Symptoms)
		json.Unmarshal(patternJSON, &candidate.TemporalPattern)
		json.Unmarshal(remediationJSON, &candidate.Remediation)
		json.Unmarshal(vectorJSON, &candidate.FeatureVector)

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
	json.Unmarshal(attrsJSON, &rem.Attributes)
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
		json.Unmarshal(attrsJSON, &rem.Attributes)
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
