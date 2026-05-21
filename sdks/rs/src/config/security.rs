#[derive(Clone, Debug)]
pub struct SecurityConfig {
    /// Always redact these keys regardless of redactor config
    pub force_redact_keys: Vec<String>,
    /// Enable PII detection patterns
    pub pii_detection: bool,
    /// Custom PII patterns (regex)
    pub pii_patterns: Vec<String>,
    /// Hash algorithm for sensitive values ("sha256", "sha512")
    pub hash_algorithm: String,
    /// Maximum bytes for a single field value
    pub max_field_bytes: usize,
    /// Maximum number of attributes per event
    pub max_attr_count: usize,
    /// Drop events that exceed size limits instead of truncating
    pub drop_oversized_events: bool,
}

impl Default for SecurityConfig {
    fn default() -> Self {
        Self {
            force_redact_keys: Vec::new(),
            pii_detection: false,
            pii_patterns: Vec::new(),
            hash_algorithm: "sha256".to_string(),
            max_field_bytes: 0,
            max_attr_count: 0,
            drop_oversized_events: false,
        }
    }
}
