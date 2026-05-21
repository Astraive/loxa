#[derive(Clone, Debug, Default)]
pub struct TraceContext {
    pub trace_id: String,
    pub span_id: String,
}

pub fn enrich_trace(trace_id: impl Into<String>, span_id: impl Into<String>) -> TraceContext {
    TraceContext {
        trace_id: trace_id.into(),
        span_id: span_id.into(),
    }
}
