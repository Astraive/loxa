# LOZA v0.0.1 Migration Guide

## For Existing LOZA Users

### Upgrade from Pre-Release (v0.x) to v0.0.1

If you've been using LOZA pre-release versions, follow these steps to upgrade safely.

#### 1. Backup Your Data

```bash
# DuckDB backup
cp ~/.loza/events.db ~/.loza/events.db.backup-v0

# PostgreSQL backup
pg_dump -h localhost -U loza loza_events | gzip > loza_backup_v0.sql.gz

# Stop running collector
systemctl stop loza-collector
```

#### 2. Configuration Updates

**Before (v0.x)**:
```yaml
collector:
  port: 9308
storage:
  backend: "duckdb"
  path: "/data/events.db"
```

**After (v0.0.1)**:
```yaml
collector:
  http_listen: "0.0.0.0:9308"
storage:
  type: "duckdb"
  path: "/data/events.db"

# NEW: Retention policy
retention:
  enabled: true
  days: 30
  max_size_gb: 0  # Disabled by default

# NEW: Privacy settings
privacy:
  enabled: true
  mode: "warn"

# NEW: Auth settings (see docs/authentication.md)
auth:
  enabled: true
  api_key: "${LOZA_API_KEY}"
```

#### 3. Update SDKs

```bash
# Go
go get github.com/astraive/loza/sdks/go@v0.0.1

# Python
pip install --upgrade loza==0.2.0

# Rust
cargo update loza --allow-prerelease
```

#### 4. Data Migration

**No automatic data migration required** - v0.0.1 is compatible with v0.x event schemas.

However, if you're upgrading storage backends:

```bash
# Export from DuckDB (v0.x)
loza query --sql "SELECT * FROM events" --format json > events_backup.jsonl

# Import to new backend
loza import --file events_backup.jsonl --target postgres://localhost/loza_events_v1
```

#### 5. Restart Services

```bash
# Upgrade binary
go install github.com/astraive/loza/collector/cmd/loza-collector@v0.0.1
go install github.com/astraive/loza/cli/cmd/loza@v0.0.1

# Update config
vi ~/.loza/loza.yaml  # Apply changes from step 2

# Start collector
systemctl start loza-collector

# Verify upgrade
loza doctor
curl http://localhost:9308/status
```

#### 6. Verify Event Flow

```bash
# Emit test event
loza emit --event upgrade.test --key version --value 0.2.0

# Query events
loza query --sql "SELECT * FROM events WHERE event_type = 'upgrade.test'"

# Check metrics
curl http://localhost:9308/metrics | grep collector_events_total
```

### Breaking Changes

**None in v0.0.1** - All v0.x APIs are compatible.

### Deprecations

**None in v0.0.1** - This is the first stable release.

### New in v0.0.1

- ✨ **Retention Policies**: Age-based and size-based automatic deletion
- ✨ **Privacy Enforcement**: Field-level redaction with configurable modes
- ✨ **API Key Auth**: Authentication on all control endpoints
- ✨ **Multi-Sink Fanout**: Deliver events to multiple backends simultaneously
- ✨ **DLQ with Replay**: Dead Letter Queue for undeliverable events
- ✨ **Real-Time Tailing**: Stream events in real-time with `loza tail`
- ✨ **Distributed Collector**: Clustering support with coordinator
- ✨ **Cardinality Budgets**: Protection against high-cardinality fields
- ✨ **Query API**: SQL interface for querying stored events

---

## For Users from Alternative Systems

### Migrating from OpenTelemetry Collector

LOZA complements OpenTelemetry by providing event-centric (vs. signal-centric) observability.

#### Architecture Comparison

| Aspect | OpenTelemetry | LOZA |
|--------|------|------|
| Focus | Signals (traces, metrics, logs) | Business events (operation-centric) |
| Schema | Flexible (any attributes) | Structured (canonical schema) |
| Delivery | Push/Pull | HTTP/Kafka (push) |
| Processing | Extensive (filtering, sampling, etc.) | Privacy, validation, enrichment |
| Retention | Via storage backend | Built-in policies |

#### Coexistence

LOZA and OpenTelemetry can run in parallel:

```yaml
# Collector config with both
otlp:
  enabled: true
  grpc_port: 4317
  http_port: 4318

collector:
  http_listen: "0.0.0.0:9308"
  
sinks:
  - name: "otel"
    type: "otlp"
    otlp:
      endpoint: "otel-collector:4317"
  
  - name: "loza"
    type: "duckdb"
    duckdb:
      path: "/data/loza-events.db"
```

#### SDK Migration Path

**From OpenTelemetry Python SDK**:
```python
# Before (OpenTelemetry)
from opentelemetry import trace
tracer = trace.get_tracer(__name__)
with tracer.start_as_current_span("operation"):
    pass

# After (LOZA)
from loza import start_event
with start_event("operation.complete") as event:
    event.enrich("duration_ms", 1234)
    event.enrich("status", "success")
```

### Migrating from Datadog

Move event ingestion and querying to LOZA, while keeping Datadog for other signals.

#### Dual Ingestion (Gradual Migration)

```yaml
# Day 1: Ingest to both
sinks:
  - name: "datadog"
    type: "datadog"
    datadog:
      api_key: "${DATADOG_API_KEY}"
      site: "datadoghq.com"
  
  - name: "loza"
    type: "duckdb"
    duckdb:
      path: "/data/loza-events.db"

delivery_policy:
  require_all: true  # Success only if both succeed
```

#### Query Translation

**Datadog Query**:
```
@service.name:my-app @status:error
| stats count by @event_type
```

**LOZA Query** (equivalent):
```sql
SELECT 
  event_type,
  COUNT(*) as count
FROM events
WHERE service_name = 'my-app' 
  AND status = 'error'
GROUP BY event_type
```

### Migrating from Elastic

LOZA provides structured event storage; Elastic provides full-text search.

#### Architecture

```
Applications
    ↓
[LOZA Collector] ← Canonical events
    ↓
  ┌─┴─┐
  ↓   ↓
DuckDB  Elasticsearch
 (Query) (Full-text search)
```

#### Setup

```yaml
sinks:
  - name: "duckdb"
    type: "duckdb"
    duckdb:
      path: "/data/loza-events.db"
  
  - name: "elasticsearch"
    type: "elasticsearch"
    elasticsearch:
      endpoints: ["http://localhost:9200"]
      index: "loza-events"
```

#### Query Examples

**LOZA (Structured Query)**:
```sql
SELECT * FROM events 
WHERE error_code = 'ECONNREFUSED' 
  AND timestamp > now() - INTERVAL 1 HOUR
```

**Elasticsearch (Full-text search)**:
```json
{
  "query": {
    "bool": {
      "must": [
        {"match": {"error_message": "connection refused"}},
        {"range": {"timestamp": {"gte": "now-1h"}}}
      ]
    }
  }
}
```

### Migrating from Honeycomb

Honeycomb is an event-centric observability platform. LOZA is a lightweight alternative for on-premise deployments.

#### Event Model Comparison

| Aspect | Honeycomb | LOZA |
|--------|-----------|------|
| Events | Flexible (any JSON) | Structured (v1 schema) |
| Cardinality | Unlimited | Budgeted (configurable) |
| Retention | Cloud-managed | Configurable policies |
| Query | Honeycomb syntax | SQL |
| Cost | Per-GB ingested | Flat infrastructure |

#### SDK Migration

**From Honeycomb SDK**:
```python
# Before
import beeline
beeline.init(dataset='my-dataset', write_key='xxx')
beeline.tracer.add_trace_field('user_id', 123)

# After
from loza import start_event
event = start_event('user.action')
event.enrich('user_id', 123)
event.finish()
event.emit()
```

#### Query Translation

**Honeycomb Query**:
```
QUERY user.action
  WHERE status = 'error'
  | COUNT by error_code
  | TOPK(10)
```

**LOZA Query** (equivalent):
```sql
SELECT 
  error_code, 
  COUNT(*) as count
FROM events
WHERE event_type = 'user.action'
  AND status = 'error'
GROUP BY error_code
ORDER BY count DESC
LIMIT 10
```

---

## General Migration Checklist

- [ ] Back up all existing event data
- [ ] Update collector configuration file
- [ ] Update all SDK versions across applications
- [ ] Test event emission in development environment
- [ ] Verify query functionality against migrated data
- [ ] Update any alerting/dashboards for new metric names
- [ ] Run full system integration tests
- [ ] Perform canary deployment (1-2% traffic)
- [ ] Monitor metrics and error rates during rollout
- [ ] Complete gradual rollout (100% traffic)
- [ ] Archive old configuration/data for compliance

## Common Issues During Migration

### Issue: Auth Failures After Upgrade

**Symptom**: `401 Unauthorized` on collector endpoints

**Solution**:
```bash
# Check if auth is enabled
curl http://localhost:9308/status

# If auth is required, set API key
export LOZA_API_KEY="sk_prod_xxx"
loza query --sql "SELECT 1"
```

### Issue: Retention Policy Deletes Too Much Data

**Symptom**: Events older than expected are being deleted

**Solution**:
```yaml
# Increase retention window
retention:
  enabled: true
  days: 60       # Increase from 30
  max_size_gb: 0 # Disable size-based deletion
```

### Issue: High Memory Usage

**Symptom**: Collector process consuming 5GB+ RAM

**Solution**:
```bash
# Reduce buffer/batch sizes
export LOZA_BATCH_SIZE=500
export LOZA_BUFFER_SIZE=10000
systemctl restart loza-collector

# Monitor memory usage
curl http://localhost:9308/metrics | grep memory
```

### Issue: Slow Queries After Migration

**Symptom**: `SELECT * FROM events` takes >10 seconds

**Solution**:
```bash
# Add indexes to storage backend
loza query --sql "CREATE INDEX idx_timestamp ON events(timestamp)"
loza query --sql "CREATE INDEX idx_event_type ON events(event_type)"

# Analyze query plan
loza query --sql "EXPLAIN SELECT * FROM events LIMIT 1000"
```

---

## Support

- **Documentation**: https://docs.loza.dev
- **GitHub Issues**: https://github.com/astraive/loza/issues
- **Discussion**: https://github.com/astraive/loza/discussions
- **Community Slack**: [Join](https://loza-community.slack.com)

**Need Help?** Create an issue on GitHub with migration details and we'll assist you.
