use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Metadata for a single field in the Loza event schema.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FieldInfo {
    pub name: String,
    pub field_type: FieldType,
    pub description: String,
    pub is_nested: bool,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum FieldType {
    String,
    Integer,
    Float,
    Boolean,
    Timestamp,
    Duration,
    Array,
    Object,
    Any,
}

/// The full Loza event schema, used for validation and column mapping.
#[derive(Debug, Clone)]
pub struct Schema {
    pub table: String,
    pub columns: HashMap<String, FieldInfo>,
    pub raw_column: String,
    pub ts_column: String,
}

impl Schema {
    /// Create the default DuckDB schema (used by the collector).
    pub fn duckdb_default() -> Self {
        let mut columns = HashMap::new();
        let fields = vec![
            ("event_id", FieldType::String, "Unique event identifier"),
            ("timestamp", FieldType::Timestamp, "Event timestamp"),
            ("service", FieldType::String, "Service name"),
            ("version", FieldType::String, "Service version"),
            ("environment", FieldType::String, "Deployment environment"),
            ("event", FieldType::String, "Event name"),
            ("kind", FieldType::String, "Event kind (event, log, metric, span)"),
            ("level", FieldType::String, "Log level (debug, info, notice, warn, error, fatal)"),
            ("outcome", FieldType::String, "Event outcome (success, error, partial, abandoned, retried, cancelled, timeout, skipped, rejected, quarantined, unknown)"),
            ("message", FieldType::String, "Human-readable message"),
            ("error_code", FieldType::String, "Error code"),
            ("error_type", FieldType::String, "Error type"),
            ("error_message", FieldType::String, "Error message"),
            ("duration_ms", FieldType::Float, "Duration in milliseconds"),
            ("started_at", FieldType::Timestamp, "Event start time"),
            ("finished_at", FieldType::Timestamp, "Event finish time"),
            ("event_state", FieldType::String, "Lifecycle state"),
            ("request_id", FieldType::String, "Request identifier"),
            ("trace_id", FieldType::String, "Distributed trace identifier"),
            ("span_id", FieldType::String, "Span identifier"),
            ("incident_id", FieldType::String, "Incident identifier"),
            ("method", FieldType::String, "HTTP method"),
            ("path", FieldType::String, "HTTP path"),
            ("route", FieldType::String, "HTTP route"),
            ("status_code", FieldType::Integer, "HTTP status code"),
            ("user_agent", FieldType::String, "User agent string"),
            ("http_status", FieldType::Integer, "HTTP status"),
            ("region", FieldType::String, "Deployment region"),
            ("host", FieldType::String, "Host name"),
            ("release", FieldType::String, "Release identifier"),
            ("deployment_id", FieldType::String, "Deployment identifier"),
            ("sdk_name", FieldType::String, "SDK name"),
            ("sdk_version", FieldType::String, "SDK version"),
            ("schema_version", FieldType::String, "Event schema version"),
            ("event_version", FieldType::String, "Event version"),
        ];

        for (name, ft, desc) in fields {
            columns.insert(
                name.to_string(),
                FieldInfo {
                    name: name.to_string(),
                    field_type: ft,
                    description: desc.to_string(),
                    is_nested: false,
                },
            );
        }

        Schema {
            table: "events".to_string(),
            columns,
            raw_column: "raw".to_string(),
            ts_column: "ts".to_string(),
        }
    }

    /// Create the default ClickHouse schema.
    pub fn clickhouse_default() -> Self {
        let mut schema = Self::duckdb_default();
        schema.table = "loza_events".to_string();
        schema.ts_column = "ts".to_string();
        schema.raw_column = "raw".to_string();
        schema
    }

    /// Check if a field name exists in the schema.
    pub fn has_field(&self, name: &str) -> bool {
        self.columns.contains_key(name)
    }

    /// Get the SQL column name for a field. For DuckDB with raw storage,
    /// nested fields are extracted via json_extract_string.
    pub fn column_expr(&self, field: &str, target: crate::compiler::Target) -> String {
        if self.columns.contains_key(field) {
            match target {
                crate::compiler::Target::DuckDB => {
                    format!("json_extract_string({}, '$.{}')", self.raw_column, field)
                }
                crate::compiler::Target::ClickHouse => {
                    format!("JSONExtractString(raw, '{}')", field)
                }
            }
        } else {
            // Nested field like user.id — extract from raw JSON
            match target {
                crate::compiler::Target::DuckDB => {
                    format!("json_extract_string({}, '$.{}')", self.raw_column, field)
                }
                crate::compiler::Target::ClickHouse => {
                    format!("JSONExtractString(raw, '{}')", field)
                }
            }
        }
    }

    /// Return all known field names for autocomplete/validation.
    pub fn field_names(&self) -> Vec<&str> {
        self.columns.keys().map(|s| s.as_str()).collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_schema_has_core_fields() {
        let schema = Schema::duckdb_default();
        assert!(schema.has_field("event_id"));
        assert!(schema.has_field("service"));
        assert!(schema.has_field("level"));
        assert!(schema.has_field("duration_ms"));
        assert!(schema.has_field("trace_id"));
        assert!(!schema.has_field("nonexistent"));
    }

    #[test]
    fn column_expr_duckdb() {
        let schema = Schema::duckdb_default();
        let expr = schema.column_expr("service", crate::compiler::Target::DuckDB);
        assert_eq!(expr, "json_extract_string(raw, '$.service')");
    }

    #[test]
    fn column_expr_clickhouse() {
        let schema = Schema::clickhouse_default();
        let expr = schema.column_expr("service", crate::compiler::Target::ClickHouse);
        assert_eq!(expr, "JSONExtractString(raw, 'service')");
    }
}
