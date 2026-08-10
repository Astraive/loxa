//! loza:// DSN parser — parses connection URIs into resolved URLs.
//!
//! Re-exported from `config::dsn` for public API access as `loza::dsn::parse()`.

pub use crate::config::dsn::{parse, DsnError, LozaDSN};
