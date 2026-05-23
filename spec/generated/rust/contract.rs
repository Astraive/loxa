use serde::{Deserialize, Serialize};
pub const CONTRACT_JSON: &str = r#"{
  \"spec_version\": \"v1\",
  \"api_version\": \"v1\",
  \"event_version\": \"v1\",
  \"schemas\": {
    \"event\": \"spec/schemas/json/event.schema.json\",
    \"event_strict\": \"spec/schemas/json/event.strict.schema.json\",
    \"event_loose\": \"spec/schemas/json/event.loose.schema.json\",
    \"ingest_envelope\": \"spec/schemas/json/ingest-envelope.schema.json\",
    \"collector_response\": \"spec/schemas/json/collector-response.schema.json\"
  },
  \"required_fields\": [
    \"schema_version\",
    \"event_version\",
    \"event_id\",
    \"timestamp\",
    \"service\",
    \"event\",
    \"kind\"
  ],
  \"allowed_top_level_fields\": [
    \"attrs\",
    \"checkpoints\",
    \"collector\",
    \"delivery_attempts\",
    \"deployment\",
    \"duration_ms\",
    \"environment\",
    \"error\",
    \"errors\",
    \"event\",
    \"event_id\",
    \"event_state\",
    \"event_version\",
    \"groups\",
    \"http\",
    \"kind\",
    \"level\",
    \"links\",
    \"message\",
    \"method\",
    \"organization\",
    \"outcome\",
    \"partial\",
    \"partial_reason\",
    \"path\",
    \"pii\",
    \"processes\",
    \"redaction\",
    \"release\",
    \"request_id\",
    \"resource\",
    \"route\",
    \"sampling\",
    \"schema_version\",
    \"sdk\",
    \"service\",
    \"source\",
    \"span_id\",
    \"status_code\",
    \"tenant\",
    \"timers\",
    \"timestamp\",
    \"trace_flags\",
    \"trace_id\",
    \"user\",
    \"version\",
    \"workspace\"
  ],
  \"canonical_fields\": [
    \"attrs\",
    \"checkpoints\",
    \"collector\",
    \"delivery_attempts\",
    \"deployment\",
    \"duration_ms\",
    \"environment\",
    \"error\",
    \"errors\",
    \"event\",
    \"event_id\",
    \"event_state\",
    \"event_version\",
    \"groups\",
    \"http\",
    \"kind\",
    \"level\",
    \"links\",
    \"message\",
    \"method\",
    \"organization\",
    \"outcome\",
    \"partial\",
    \"partial_reason\",
    \"path\",
    \"pii\",
    \"processes\",
    \"redaction\",
    \"release\",
    \"request_id\",
    \"resource\",
    \"route\",
    \"sampling\",
    \"schema_version\",
    \"sdk\",
    \"service\",
    \"source\",
    \"span_id\",
    \"status_code\",
    \"tenant\",
    \"timers\",
    \"timestamp\",
    \"trace_flags\",
    \"trace_id\",
    \"user\",
    \"version\",
    \"workspace\"
  ],
  \"enums\": {
    \"schema_versions\": [
      \"v1\"
    ],
    \"event_versions\": [
      \"v1\"
    ],
    \"kinds\": [
      \"event\",
      \"http\",
      \"job\",
      \"queue\",
      \"cli\",
      \"cron\",
      \"log\",
      \"checkpoint\",
      \"agent\",
      \"ai\"
    ],
    \"levels\": [
      \"debug\",
      \"info\",
      \"notice\",
      \"warn\",
      \"error\",
      \"fatal\"
    ],
    \"outcomes\": [
      \"success\",
      \"error\",
      \"partial\",
      \"abandoned\",
      \"retried\",
      \"cancelled\",
      \"timeout\",
      \"skipped\",
      \"rejected\",
      \"quarantined\",
      \"unknown\"
    ],
    \"partial_reasons\": [
      \"not_finished\",
      \"process_exit\",
      \"timeout\",
      \"panic\",
      \"collector_unavailable\"
    ],
    \"event_states\": [
      \"created\",
      \"active\",
      \"finished\",
      \"emitting\",
      \"emitted\",
      \"invalid\",
      \"dropped\",
      \"emit_failed\",
      \"spooled\",
      \"dlq_written\",
      \"failed_validation\",
      \"delivery_failed\"
    ],
    \"source_sdks\": [
      \"loxa-cli\",
      \"loxa-go\",
      \"loxa-py\",
      \"loxa-rs\"
    ]
  },
  \"collector_statuses\": [
    \"accepted\",
    \"partial\",
    \"rejected\",
    \"invalid\",
    \"quarantined\"
  ],
  \"collector_ack_statuses\": [
    \"accepted\",
    \"rejected\",
    \"invalid\"
  ],
  \"aliases\": {
    \"event_type\": \"event\"
  },
  \"alias_policy\": {
    \"strict_mode\": \"reject_before_normalization\",
    \"loose_mode\": \"normalize_then_use\"
  },
  \"strict_mode\": {
    \"allow_unknown_top_level_fields\": false,
    \"allow_aliases\": false,
    \"enforce_required_fields\": true,
    \"enforce_enums\": true,
    \"enforce_status_codes\": true,
    \"enforce_timestamps\": true,
    \"normalize_aliases\": false
  },
  \"loose_mode\": {
    \"allow_unknown_top_level_fields\": true,
    \"allow_aliases\": true,
    \"enforce_required_fields\": true,
    \"enforce_enums\": true,
    \"normalize_aliases\": true
  },
  \"validation_modes\": {
    \"off\": {
      \"name\": \"off\",
      \"description\": \"Accept everything, still normalize payload\",
      \"accept_all\": true,
      \"normalize\": true,
      \"validate\": false,
      \"reject_on_failure\": false
    },
    \"warn\": {
      \"name\": \"warn\",
      \"description\": \"Accept and report schema issues as warnings\",
      \"accept_all\": true,
      \"normalize\": true,
      \"validate\": true,
      \"reject_on_failure\": false
    },
    \"enforce\": {
      \"name\": \"enforce\",
      \"description\": \"Reject invalid events, accept valid ones only\",
      \"accept_all\": false,
      \"normalize\": true,
      \"validate\": true,
      \"reject_on_failure\": true
    },
    \"quarantine\": {
      \"name\": \"quarantine\",
      \"description\": \"Store invalid events separately in quarantine, accept valid ones\",
      \"accept_all\": false,
      \"normalize\": true,
      \"validate\": true,
      \"reject_on_failure\": false,
      \"quarantine_on_failure\": true
    },
    \"strict\": {
      \"name\": \"strict\",
      \"description\": \"Reject unknown fields, aliases, and enforce all constraints\",
      \"allow_aliases\": false,
      \"allow_unknown_top_level_fields\": false,
      \"reject_on_failure\": true
    },
    \"loose\": {
      \"name\": \"loose\",
      \"description\": \"Accept aliases and some unknown fields, normalize before validation\",
      \"allow_aliases\": true,
      \"allow_unknown_top_level_fields\": true
    }
  },
  \"wire_formats\": [
    \"json\",
    \"jsonl\",
    \"protobuf\"
  ],
  \"limits\": {
    \"max_event_size_bytes\": 65536,
    \"max_batch_events\": 1000,
    \"max_batch_size_bytes\": 1048576,
    \"max_attrs_depth\": 8,
    \"max_attr_key_length\": 128,
    \"max_attr_value_length\": 4096,
    \"max_error_stack_length\": 16384
  },
  \"fixtures\": {
    \"version\": \"v1\",
    \"strict_schema\": \"../schema/event.strict.schema.json\",
    \"loose_schema\": \"../schema/event.loose.schema.json\",
    \"valid\": [
      \"valid/http_success.json\",
      \"valid/http_error.json\",
      \"valid/job_success.json\",
      \"valid/queue_retry.json\",
      \"valid/cron_run.json\",
      \"valid/partial_abandoned.json\",
      \"valid/cli_run.json\",
      \"valid/duplicate_fields.json\",
      \"valid/minimal_event.json\",
      \"valid/error_event.json\",
      \"valid/trace_context_event.json\",
      \"valid/agent_run.json\",
      \"valid/rag_query.json\",
      \"valid/release_field.json\",
      \"valid/notice_level.json\"
    ],
    \"loose_only_valid\": [
      \"valid/loose_event_type_alias.json\"
    ],
    \"invalid\": [
      \"invalid/missing_event_id.json\",
      \"invalid/missing_versions.json\",
      \"invalid/bad_timestamp.json\",
      \"invalid/bad_duration.json\",
      \"invalid/invalid_enum_values.json\"
    ],
    \"strict_only_invalid\": [
      \"invalid/strict_event_type_alias.json\"
    ],
    \"invalid_ingest\": [
      \"invalid/bad_ingest_events_array.json\"
    ],
    \"invalid_collector_response\": [
      \"invalid/bad_collector_status.json\"
    ],
    \"invalid_limits\": [
      \"invalid/oversized.json\"
    ],
    \"emitted_shape\": [
      \"emitted-shape/structured_http_success.json\"
    ],
    \"collector_ack_behavior\": [
      \"valid/collector_ack.json\",
      \"collector-acks/accepted_clean.json\",
      \"collector-acks/accepted_duplicate.json\",
      \"collector-acks/partial_invalid.json\",
      \"collector-acks/retryable_rate_limited.json\",
      \"collector-acks/partial_quarantined.json\"
    ],
    \"ingest_envelopes\": [
      \"ingest-envelopes/single_event_json.json\",
      \"ingest-envelopes/wrapped_batch_json.json\",
      \"ingest-envelopes/ndjson_ingest.json\"
    ],
    \"fixtures_by_coverage\": {
      \"http_success.json\": \"Standard HTTP event with all canonical fields\",
      \"http_error.json\": \"HTTP error response event\",
      \"error_event.json\": \"Error/exception event with stack trace\",
      \"job_success.json\": \"Background job success event\",
      \"queue_retry.json\": \"Queue/retry event\",
      \"cron_run.json\": \"Cron/scheduled task event\",
      \"partial_abandoned.json\": \"Partial/abandoned outcome event\",
      \"cli_run.json\": \"CLI/process event\",
      \"collector_ack.json\": \"Collector acknowledgment response payload\",
      \"duplicate_fields.json\": \"Canonical duplicate field handling test\",
      \"minimal_event.json\": \"Minimal valid event (required fields only)\",
      \"loose_event_type_alias.json\": \"Legacy event_type alias accepted only in loose mode\",
      \"strict_event_type_alias.json\": \"Legacy event_type alias rejected in strict mode\",
      \"missing_versions.json\": \"Missing schema/event versions should fail event validation\",
      \"bad_ingest_events_array.json\": \"Ingest envelope with non-array events should fail ingest schema\",
      \"bad_collector_status.json\": \"Collector response with unsupported status should fail collector schema\",
      \"trace_context_event.json\": \"Distributed trace context propagation\",
      \"agent_run.json\": \"AI agent run event with agent kind and processes\",
      \"rag_query.json\": \"RAG pipeline query event with ai kind\",
      \"release_field.json\": \"Event with release and trace_flags fields\",
      \"notice_level.json\": \"Event with notice level\",
      \"structured_http_success.json\": \"Cross-SDK emitted payload layout with structured groups and finished event_state\",
      \"accepted_clean.json\": \"Collector sink should treat clean accepted acknowledgements as success\",
      \"accepted_duplicate.json\": \"Collector sink should treat duplicate accepted acknowledgements as success\",
      \"partial_invalid.json\": \"Collector sink should fail on partial invalid acknowledgements\",
      \"retryable_rate_limited.json\": \"Collector sink should fail on retryable/rate-limited responses\",
      \"single_event_json.json\": \"Collector accepts a single-event JSON ingest payload\",
      \"wrapped_batch_json.json\": \"Canonical wrapped batch envelope generated by SDK sinks\",
      \"ndjson_ingest.json\": \"Collector accepts NDJSON ingest payloads\"
    }
  },
  \"paths\": {
    \"event_schema\": \"schema/event.schema.json\",
    \"strict_schema\": \"schema/event.strict.schema.json\",
    \"loose_schema\": \"schema/event.loose.schema.json\",
    \"ingest_schema\": \"schema/ingest.schema.json\",
    \"collector_response_schema\": \"schema/collector-response.schema.json\",
    \"manifest\": \"conformance/manifest.json\"
  }
}"#;
pub const LOXA_SPEC_VERSION: &str = "v1";
pub const LOXA_INGEST_API_VERSION: &str = "v1";
pub const LOXA_EVENT_VERSION: &str = "v1";
pub const MAX_EVENT_BYTES: usize = 65536;
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct ValidationError { pub field: String, pub code: String, pub message: String, #[serde(default)] pub event_id: String, #[serde(default)] pub retryable: bool }
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorAck { #[serde(default)] pub event_id: String, #[serde(default)] pub status: String, #[serde(default)] pub retryable: bool, #[serde(default)] pub reason: String, #[serde(default)] pub message: String }
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorError { #[serde(default)] pub code: String, #[serde(default)] pub message: String, #[serde(default)] pub retryable: bool, #[serde(default)] pub field: String, #[serde(default)] pub event_id: String }
#[derive(Clone, Debug, Default, Eq, PartialEq, Serialize, Deserialize)]
pub struct CollectorResponse { #[serde(default)] pub request_id: String, #[serde(default)] pub status: String, #[serde(default)] pub accepted: u64, #[serde(default)] pub rejected: u64, #[serde(default)] pub invalid: u64, #[serde(default)] pub deduped: u64, #[serde(default)] pub reason: String, #[serde(default)] pub error: String, #[serde(default)] pub acks: Vec<CollectorAck>, #[serde(default)] pub errors: Vec<CollectorError> }
