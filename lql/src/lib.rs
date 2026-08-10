//! Loza Query Language (LQL)
//!
//! A Kusto-inspired query language that compiles to DuckDB and ClickHouse SQL.
//!
//! ```rust
//! use loza_lql::{compile_to_duckdb, compile_to_clickhouse};
//!
//! let sql = compile_to_duckdb(r#"from events | where level = "error" | summarize count() by service | sort count desc | limit 10"#).unwrap();
//! assert!(sql.contains("COUNT(*)"));
//! ```

pub mod ast;
pub mod compiler;
pub mod error;
pub mod functions;
pub mod lexer;
pub mod parser;
pub mod schema;
pub mod validate;

pub use ast::{AggExpr, Expr, Pipeline, Source, Statement};
pub use compiler::Target;
pub use error::LqlError;
pub use schema::Schema;

/// Parse LQL input into an AST pipeline.
pub fn parse(input: &str) -> Result<Pipeline, LqlError> {
    let tokens = lexer::Lexer::new(input).tokenize()?;
    parser::Parser::new(tokens).parse()
}

/// Compile a LQL query string to DuckDB SQL.
pub fn compile_to_duckdb(input: &str) -> Result<String, LqlError> {
    let pipeline = parse(input)?;
    let schema = Schema::duckdb_default();
    compiler::compile(&pipeline, Target::DuckDB, &schema)
}

/// Compile a LQL query string to ClickHouse SQL.
pub fn compile_to_clickhouse(input: &str) -> Result<String, LqlError> {
    let pipeline = parse(input)?;
    let schema = Schema::clickhouse_default();
    compiler::compile(&pipeline, Target::ClickHouse, &schema)
}

/// Validate a LQL query string against the schema without compiling.
pub fn validate_query(input: &str) -> Result<(), LqlError> {
    let pipeline = parse(input)?;
    let schema = Schema::duckdb_default();
    validate::validate(&pipeline, &schema)
}

/// Compile a LQL query string to SQL for the given target.
pub fn compile(input: &str, target: Target) -> Result<String, LqlError> {
    match target {
        Target::DuckDB => compile_to_duckdb(input),
        Target::ClickHouse => compile_to_clickhouse(input),
    }
}

/// Get all known field names for autocomplete.
pub fn known_fields() -> Vec<String> {
    let schema = Schema::duckdb_default();
    schema.columns.keys().cloned().collect()
}

// ── WASM bindings ──────────────────────────────────────────────────────────

#[cfg(target_arch = "wasm32")]
mod wasm {
    use wasm_bindgen::prelude::*;

    #[wasm_bindgen]
    pub fn compile_to_duckdb(input: &str) -> Result<String, JsValue> {
        super::compile_to_duckdb(input).map_err(|e| JsValue::from_str(&e.to_string()))
    }

    #[wasm_bindgen]
    pub fn compile_to_clickhouse(input: &str) -> Result<String, JsValue> {
        super::compile_to_clickhouse(input).map_err(|e| JsValue::from_str(&e.to_string()))
    }

    #[wasm_bindgen]
    pub fn validate(input: &str) -> Result<String, JsValue> {
        match super::validate_query(input) {
            Ok(()) => Ok("{}".to_string()),
            Err(e) => Err(JsValue::from_str(&e.to_string())),
        }
    }

    #[wasm_bindgen]
    pub fn known_fields() -> String {
        let fields = super::known_fields();
        serde_json::to_string(&fields).unwrap_or_else(|_| "[]".to_string())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn full_pipeline_duckdb() {
        let sql = compile_to_duckdb(
            r#"from events | where level = "error" and service = "checkout" | summarize count(), avg(duration_ms) by event | sort count desc | limit 20"#,
        ).unwrap();
        assert!(sql.contains("SELECT"));
        assert!(sql.contains("events"));
        assert!(sql.contains("WHERE"));
        assert!(sql.contains("GROUP BY"));
        assert!(sql.contains("ORDER BY"));
        assert!(sql.contains("LIMIT 20"));
    }

    #[test]
    fn full_pipeline_clickhouse() {
        let sql = compile_to_clickhouse(
            r#"from events | where level = "error" | summarize p95(duration_ms) by service"#,
        )
        .unwrap();
        assert!(sql.contains("quantile(0.95)"));
    }

    #[test]
    fn parse_only() {
        let pipeline = parse(r#"from events | where level = "error" | limit 10"#).unwrap();
        assert_eq!(pipeline.statements.len(), 3);
    }

    #[test]
    fn timeseries_query() {
        let sql = compile_to_duckdb("from events | timeseries 5m").unwrap();
        assert!(sql.contains("date_trunc"));
        assert!(sql.contains("COUNT(*)"));
    }

    #[test]
    fn known_fields_returns_values() {
        let fields = known_fields();
        assert!(fields.contains(&"service".to_string()));
        assert!(fields.contains(&"level".to_string()));
        assert!(fields.contains(&"event_id".to_string()));
    }
}
