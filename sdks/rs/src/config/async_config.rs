#[derive(Clone, Debug)]
pub struct AsyncConfig {
    pub enabled: bool,
    pub queue_size: usize,
    pub workers: usize,
    pub max_batch_bytes: usize,
    pub flush_interval_ms: u64,
    pub backpressure: BackpressurePolicy,
}

impl Default for AsyncConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            queue_size: 10_000,
            workers: 1,
            max_batch_bytes: 1024 * 1024,
            flush_interval_ms: 5000,
            backpressure: BackpressurePolicy::DropNewest,
        }
    }
}

#[derive(Clone, Debug)]
pub enum BackpressurePolicy {
    DropNewest,
    DropOldest,
    Block,
    Error,
}

impl Default for BackpressurePolicy {
    fn default() -> Self {
        Self::DropNewest
    }
}
