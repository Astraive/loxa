use thiserror::Error;

/// A source span pointing to a location in the input.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Span {
    pub start: usize,
    pub end: usize,
}

impl Span {
    pub fn new(start: usize, end: usize) -> Self {
        Self { start, end }
    }
}

/// An error with optional source span for precise diagnostics.
#[derive(Debug, Clone, Error)]
pub enum LqlError {
    #[error("unexpected character '{char}' at position {pos}")]
    UnexpectedChar { char: char, pos: usize },

    #[error("unexpected end of input")]
    UnexpectedEof,

    #[error("unexpected token '{token}' at position {pos}, expected {expected}")]
    UnexpectedToken {
        token: String,
        expected: String,
        pos: usize,
    },

    #[error("unknown function '{name}'")]
    UnknownFunction { name: String, span: Span },

    #[error("unknown field '{name}'")]
    UnknownField { name: String, span: Span },

    #[error("type mismatch: {message}")]
    TypeMismatch { message: String, span: Span },

    #[error("invalid duration: {input}")]
    InvalidDuration { input: String, span: Span },

    #[error("invalid aggregation: {message}")]
    InvalidAggregation { message: String, span: Span },

    #[error("{message}")]
    Compile { message: String, span: Option<Span> },
}

/// A diagnostic result that includes the source span.
#[derive(Debug, Clone)]
pub struct Diagnostic {
    pub message: String,
    pub span: Option<Span>,
    pub severity: Severity,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Severity {
    Error,
    Warning,
}

impl std::fmt::Display for Diagnostic {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}: {}", self.severity, self.message)
    }
}

impl std::fmt::Display for Severity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Severity::Error => write!(f, "error"),
            Severity::Warning => write!(f, "warning"),
        }
    }
}
