use std::panic::{catch_unwind, AssertUnwindSafe};

#[derive(Clone, Debug)]
pub struct SecurityConfig {
    pub redact_by_default: bool,
    pub allow_pii: bool,
    pub max_field_bytes: usize,
    pub max_event_bytes: usize,
    pub max_attr_count: usize,
    pub drop_oversized_events: bool,
}

impl Default for SecurityConfig {
    fn default() -> Self {
        Self {
            redact_by_default: true,
            allow_pii: false,
            max_field_bytes: 4096,
            max_event_bytes: 256 * 1024,
            max_attr_count: 512,
            drop_oversized_events: true,
        }
    }
}

pub fn recover_to_error<T>(f: impl FnOnce() -> T) -> Result<T, String> {
    catch_unwind(AssertUnwindSafe(f)).map_err(|panic| {
        panic
            .downcast_ref::<&str>()
            .map(|s| s.to_string())
            .or_else(|| panic.downcast_ref::<String>().cloned())
            .unwrap_or_else(|| "panic".to_string())
    })
}
