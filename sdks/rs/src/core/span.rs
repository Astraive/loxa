use std::collections::BTreeMap;

/// SpanContext represents a distributed trace span for SDK-side propagation.
///
/// This is a lightweight type for creating and propagating span context
/// within the SDK. Full tracing integration (OTLP export, sampling decisions)
/// lives in the `integrations/otel` module.
#[derive(Clone, Debug, Default)]
pub struct SpanContext {
    pub trace_id: Option<String>,
    pub span_id: Option<String>,
    pub tracestate: Option<String>,
    pub baggage: BTreeMap<String, String>,
}

impl SpanContext {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn with_trace_id(mut self, trace_id: impl Into<String>) -> Self {
        self.trace_id = Some(trace_id.into());
        self
    }

    pub fn with_span_id(mut self, span_id: impl Into<String>) -> Self {
        self.span_id = Some(span_id.into());
        self
    }

    pub fn with_tracestate(mut self, tracestate: impl Into<String>) -> Self {
        self.tracestate = Some(tracestate.into());
        self
    }

    pub fn with_baggage(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.baggage.insert(key.into(), value.into());
        self
    }

    /// Create a child span context from this context.
    /// Generates a new span_id, preserving trace_id and baggage.
    pub fn child(&self) -> Self {
        Self {
            trace_id: self.trace_id.clone(),
            span_id: Some(generate_span_id()),
            tracestate: self.tracestate.clone(),
            baggage: self.baggage.clone(),
        }
    }

    /// Parse a W3C traceparent header into a SpanContext.
    pub fn from_traceparent(traceparent: &str) -> Option<Self> {
        let mut parts = traceparent.trim().split('-');
        let version = parts.next()?;
        let trace_id = parts.next()?;
        let span_id = parts.next()?;
        let flags = parts.next()?;
        if parts.next().is_some() {
            return None;
        }
        if version.len() != 2
            || trace_id.len() != 32
            || span_id.len() != 16
            || flags.len() != 2
            || !trace_id.chars().all(|ch| ch.is_ascii_hexdigit())
            || !span_id.chars().all(|ch| ch.is_ascii_hexdigit())
            || !version.chars().all(|ch| ch.is_ascii_hexdigit())
            || !flags.chars().all(|ch| ch.is_ascii_hexdigit())
        {
            return None;
        }
        Some(
            Self::new()
                .with_trace_id(trace_id.to_ascii_lowercase())
                .with_span_id(span_id.to_ascii_lowercase()),
        )
    }

    /// Format as a W3C traceparent header value.
    pub fn traceparent(&self) -> Option<String> {
        let trace_id = self.trace_id.as_deref()?;
        let span_id = self.span_id.as_deref()?;
        if trace_id.len() != 32
            || span_id.len() != 16
            || !trace_id.chars().all(|ch| ch.is_ascii_hexdigit())
            || !span_id.chars().all(|ch| ch.is_ascii_hexdigit())
        {
            return None;
        }
        Some(format!(
            "00-{}-{}-01",
            trace_id.to_ascii_lowercase(),
            span_id.to_ascii_lowercase()
        ))
    }
}

/// Generate a random 16-character hex span ID.
fn generate_span_id() -> String {
    let mut out = String::with_capacity(16);
    for _ in 0..16 {
        out.push_str(&format!("{:x}", fastrand::u8(0..16)));
    }
    out
}
