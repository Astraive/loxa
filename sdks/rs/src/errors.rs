use std::fmt;

#[derive(Clone, Debug)]
pub struct ValidationError {
    pub field: Option<String>,
    pub code: String,
    pub message: String,
    pub retryable: Option<bool>,
}
impl ValidationError {
    pub fn new(field: Option<&str>, code: &str, message: String) -> Self {
        Self {
            field: field.map(|s| s.to_string()),
            code: code.to_string(),
            message,
            retryable: None,
        }
    }
}

impl fmt::Display for ValidationError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for ValidationError {}

#[derive(Debug)]
pub enum LozaError {
    DuplicateEmit { event_id: String },
    EventClosed { event_id: String, state: String },
    EventAlreadyFinished { event_id: String },
    Validation(ValidationError),
    Serialization(serde_json::Error),
    Transport(String),
}

impl fmt::Display for LozaError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            LozaError::DuplicateEmit { event_id } => {
                write!(f, "duplicate emit for event {event_id}")
            }
            LozaError::EventClosed { event_id, state } => {
                write!(f, "event {event_id} is closed in state {state}")
            }
            LozaError::EventAlreadyFinished { event_id } => {
                write!(f, "event {event_id} already finished")
            }
            LozaError::Validation(v) => f.write_str(&v.message),
            LozaError::Serialization(err) => write!(f, "serialization failed: {err}"),
            LozaError::Transport(message) => f.write_str(message),
        }
    }
}

impl std::error::Error for LozaError {}

impl From<serde_json::Error> for LozaError {
    fn from(value: serde_json::Error) -> Self {
        LozaError::Serialization(value)
    }
}
