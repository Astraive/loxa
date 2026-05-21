//! Envelope types for collector ingest API.

pub use crate::core::client::validate_ingest_envelope;
pub use crate::generated::spec_contract::{
    build_ingest_envelope, parse_collector_response_value, CollectorResponse as EnvelopeResponse,
};
