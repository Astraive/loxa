use super::super::clock::unix_millis;
use std::sync::{Mutex, OnceLock};

type IdGenerator = Box<dyn Fn() -> String + Send + Sync>;

static CUSTOM_ID_GEN: OnceLock<Mutex<Option<IdGenerator>>> = OnceLock::new();

pub fn new_event_id() -> String {
    if let Some(gen) = CUSTOM_ID_GEN
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap()
        .as_ref()
    {
        return gen();
    }
    format!("evt_{}", unix_millis())
}

pub fn set_id_generator(f: IdGenerator) {
    *CUSTOM_ID_GEN
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap() = Some(f);
}

pub fn reset_id_generator() {
    *CUSTOM_ID_GEN
        .get_or_init(|| Mutex::new(None))
        .lock()
        .unwrap() = None;
}
