use serde_json::Value;

/// Pass-through redaction that returns the value unchanged.
/// Useful for testing or when redaction is explicitly disabled.
pub fn noop_redact(value: Value) -> Value {
    value
}
