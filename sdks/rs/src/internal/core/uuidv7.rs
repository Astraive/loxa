use super::super::clock::unix_millis;

pub fn new_event_id() -> String {
    format!("evt_{}", unix_millis())
}
