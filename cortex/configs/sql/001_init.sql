CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp TIMESTAMPTZ NOT NULL,
    kind VARCHAR(50) NOT NULL,
    service VARCHAR(255) NOT NULL,
    trace_id VARCHAR(255),
    incident_id VARCHAR(255),
    raw JSONB,
    provenance VARCHAR(50) NOT NULL DEFAULT 'collector',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id VARCHAR(255) PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL,
    signature_id VARCHAR(255),
    status VARCHAR(50) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    primary_service VARCHAR(255) NOT NULL,
    affected_services JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS incident_signatures (
    signature_id VARCHAR(255) PRIMARY KEY,
    shape TEXT NOT NULL,
    service_roles JSONB,
    symptoms JSONB,
    temporal_pattern JSONB,
    remediation JSONB,
    feature_vector JSONB,
    feature_weights JSONB,
    occurrence_count INT DEFAULT 1,
    avg_resolution_time_seconds BIGINT,
    version INT DEFAULT 1,
    parent_signature_id VARCHAR(255),
    decay_factor DOUBLE PRECISION DEFAULT 1.0,
    last_matched_at TIMESTAMPTZ,
    behavioral_hash VARCHAR(255),
    embedding FLOAT[],
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS service_topology (
    alias VARCHAR(255) PRIMARY KEY,
    canonical VARCHAR(255) NOT NULL,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS graph_nodes (
    id VARCHAR(255) PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    label VARCHAR(255) NOT NULL,
    attributes JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS graph_edges (
    id VARCHAR(255) PRIMARY KEY,
    from_node_id VARCHAR(255) NOT NULL,
    to_node_id VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    weight DOUBLE PRECISION DEFAULT 1.0,
    attributes JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS remediations (
    id VARCHAR(255) PRIMARY KEY,
    incident_id VARCHAR(255),
    signature_id VARCHAR(255),
    action VARCHAR(255) NOT NULL,
    description TEXT,
    timestamp TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS remediation_feedback (
    feedback_id VARCHAR(255) PRIMARY KEY,
    remediation_id VARCHAR(255),
    incident_id VARCHAR(255),
    outcome_code INTEGER NOT NULL,
    outcome_category VARCHAR(50) NOT NULL,
    time_to_resolve_seconds BIGINT,
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_service ON events(service);
CREATE INDEX IF NOT EXISTS idx_events_incident_id ON events(incident_id);
CREATE INDEX IF NOT EXISTS idx_events_trace_id ON events(trace_id);

CREATE INDEX IF NOT EXISTS idx_incidents_timestamp ON incidents(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_primary_service ON incidents(primary_service);
CREATE INDEX IF NOT EXISTS idx_incidents_signature ON incidents(signature_id);

CREATE INDEX IF NOT EXISTS idx_graph_edges_from ON graph_edges(from_node_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_to ON graph_edges(to_node_id);

CREATE INDEX IF NOT EXISTS idx_signatures_shape ON incident_signatures USING GIN(shape gin_trgm_ops);