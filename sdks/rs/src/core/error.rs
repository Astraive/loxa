pub use crate::errors::{LoxaError, ValidationError};

use serde_json::{Map, Value};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ErrorInfo {
    pub error_type: String,
    pub message: String,
    pub code: String,
    pub retryable: bool,
}

impl ErrorInfo {
    pub fn to_json(&self) -> Value {
        let mut out = Map::new();
        out.insert("type".to_string(), Value::String(self.error_type.clone()));
        out.insert("message".to_string(), Value::String(self.message.clone()));
        if !self.code.is_empty() {
            out.insert("code".to_string(), Value::String(self.code.clone()));
        }
        out.insert("retryable".to_string(), Value::Bool(self.retryable));
        Value::Object(out)
    }
}
