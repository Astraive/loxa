use std::sync::{Mutex, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};

static FROZEN_TIME_MS: OnceLock<Mutex<Option<u128>>> = OnceLock::new();

pub fn unix_millis() -> u128 {
    if let Some(frozen) = FROZEN_TIME_MS
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap()
        .as_ref()
    {
        return *frozen;
    }
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis()
}

pub fn freeze_at(ms: u128) {
    *FROZEN_TIME_MS
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap() = Some(ms);
}

pub fn unfreeze() {
    *FROZEN_TIME_MS
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap() = None;
}
