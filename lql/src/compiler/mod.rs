pub mod clickhouse;
pub mod duckdb;

use crate::ast::Pipeline;
use crate::error::LqlError;
use crate::schema::Schema;

/// Target database dialect.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Target {
    DuckDB,
    ClickHouse,
}

/// Compile a LQL pipeline to SQL for the given target.
pub fn compile(pipeline: &Pipeline, target: Target, schema: &Schema) -> Result<String, LqlError> {
    match target {
        Target::DuckDB => duckdb::compile(pipeline, schema),
        Target::ClickHouse => clickhouse::compile(pipeline, schema),
    }
}
