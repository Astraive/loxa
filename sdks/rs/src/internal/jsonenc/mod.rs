pub mod number;
pub mod string;

use serde::Serialize;

pub fn compact<T: Serialize>(value: &T) -> Result<String, serde_json::Error> {
    serde_json::to_string(value)
}

pub fn pretty<T: Serialize>(value: &T) -> Result<String, serde_json::Error> {
    serde_json::to_string_pretty(value)
}

pub fn json_encoder(payload: &serde_json::Value) -> Result<String, serde_json::Error> {
    serde_json::to_string(payload)
}

pub fn pretty_json_encoder(payload: &serde_json::Value) -> Result<String, serde_json::Error> {
    serde_json::to_string_pretty(payload)
}
