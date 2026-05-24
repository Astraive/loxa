#[derive(Clone, Debug)]
pub struct AsyncConfig {
    pub enabled: bool,
    pub queue_size: usize,
    pub workers: usize,
    pub max_batch_bytes: usize,
    pub batch_size: usize,
    pub flush_interval_ms: u64,
    pub max_retries: u32,
    pub backpressure: BackpressurePolicy,
}

impl Default for AsyncConfig {
    fn default() -> Self {
        Self {
            enabled: false,
            queue_size: 10_000,
            workers: 1,
            max_batch_bytes: 1024 * 1024,
            batch_size: 100,
            flush_interval_ms: 5000,
            max_retries: 3,
            backpressure: BackpressurePolicy::DropNewest,
        }
    }
}

#[derive(Clone, Debug, Default)]
pub enum BackpressurePolicy {
    #[default]
    DropNewest,
    DropOldest,
    Block,
    Error,
}
