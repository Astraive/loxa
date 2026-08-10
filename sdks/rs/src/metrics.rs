use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Mutex, MutexGuard};
use std::time::Duration;

const DEFAULT_HISTOGRAM_BUCKETS: [f64; 13] = [
    0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
];

fn lock_or_recover<T>(mutex: &Mutex<T>) -> MutexGuard<'_, T> {
    match mutex.lock() {
        Ok(guard) => guard,
        Err(poisoned) => poisoned.into_inner(),
    }
}

#[derive(Debug)]
struct HistogramState {
    buckets: Vec<f64>,
    counts: Vec<u64>,
    sum: f64,
    total: u64,
}

impl HistogramState {
    fn new(buckets: Vec<f64>) -> Self {
        let counts = vec![0; buckets.len()];
        Self {
            buckets,
            counts,
            sum: 0.0,
            total: 0,
        }
    }

    fn observe(&mut self, value: f64) {
        self.total += 1;
        self.sum += value;
        for (i, &bucket) in self.buckets.iter().enumerate() {
            if value <= bucket {
                self.counts[i] += 1;
            }
        }
    }
}

#[derive(Debug)]
pub struct MetricsCollector {
    // Counters
    events_created: AtomicU64,
    events_finished: AtomicU64,
    events_emitted_success: AtomicU64,
    events_emitted_failure: AtomicU64,
    backpressure: AtomicU64,

    // Labeled counters
    events_dropped_by_reason: Mutex<HashMap<String, u64>>,
    retry_by_attempt: Mutex<HashMap<u32, u64>>,

    // Gauges
    buffer_size: AtomicU64,
    buffer_capacity: AtomicU64,

    // Histogram
    emit_duration: Mutex<HistogramState>,

    // Info
    last_error: Mutex<String>,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct MetricsSnapshot {
    pub events_created: u64,
    pub events_finished: u64,
    pub events_emitted_success: u64,
    pub events_emitted_failure: u64,
    pub events_dropped_by_reason: HashMap<String, u64>,
    pub retry_by_attempt: HashMap<u32, u64>,
    pub backpressure: u64,
    pub buffer_size: u64,
    pub buffer_capacity: u64,
    pub emit_duration_buckets: Vec<(f64, u64)>,
    pub emit_duration_sum: f64,
    pub emit_duration_count: u64,
    pub last_error: String,
}

impl MetricsCollector {
    pub fn new() -> Self {
        Self::with_capacity(0)
    }

    pub fn with_capacity(buffer_capacity: usize) -> Self {
        Self {
            events_created: AtomicU64::new(0),
            events_finished: AtomicU64::new(0),
            events_emitted_success: AtomicU64::new(0),
            events_emitted_failure: AtomicU64::new(0),
            backpressure: AtomicU64::new(0),
            events_dropped_by_reason: Mutex::new(HashMap::new()),
            retry_by_attempt: Mutex::new(HashMap::new()),
            buffer_size: AtomicU64::new(0),
            buffer_capacity: AtomicU64::new(buffer_capacity as u64),
            emit_duration: Mutex::new(HistogramState::new(DEFAULT_HISTOGRAM_BUCKETS.to_vec())),
            last_error: Mutex::new(String::new()),
        }
    }

    pub fn record_event_created(&self) {
        self.events_created.fetch_add(1, Ordering::Relaxed);
    }

    pub fn record_event_finished(&self) {
        self.events_finished.fetch_add(1, Ordering::Relaxed);
    }

    pub fn record_event_emitted(&self, success: bool) {
        if success {
            self.events_emitted_success.fetch_add(1, Ordering::Relaxed);
        } else {
            self.events_emitted_failure.fetch_add(1, Ordering::Relaxed);
        }
    }

    pub fn record_event_dropped(&self, reason: &str) {
        let mut map = lock_or_recover(&self.events_dropped_by_reason);
        *map.entry(reason.to_string()).or_insert(0) += 1;
        if !reason.is_empty() {
            *lock_or_recover(&self.last_error) = reason.to_string();
        }
    }

    pub fn record_retry(&self, attempt: u32) {
        let mut map = lock_or_recover(&self.retry_by_attempt);
        *map.entry(attempt.max(1)).or_insert(0) += 1;
    }

    pub fn record_backpressure(&self) {
        self.backpressure.fetch_add(1, Ordering::Relaxed);
    }

    pub fn observe_emit_duration(&self, duration: Duration) {
        let seconds = duration.as_secs_f64();
        let mut hist = lock_or_recover(&self.emit_duration);
        hist.observe(seconds);
    }

    pub fn set_buffer_size(&self, size: u64) {
        self.buffer_size.store(size, Ordering::Relaxed);
    }

    pub fn set_buffer_capacity(&self, capacity: u64) {
        self.buffer_capacity.store(capacity, Ordering::Relaxed);
    }

    pub fn snapshot(&self) -> MetricsSnapshot {
        let dropped = lock_or_recover(&self.events_dropped_by_reason).clone();
        let retry = lock_or_recover(&self.retry_by_attempt).clone();
        let hist = lock_or_recover(&self.emit_duration);
        let buckets: Vec<(f64, u64)> = hist
            .buckets
            .iter()
            .zip(hist.counts.iter())
            .map(|(&b, &c)| (b, c))
            .collect();
        MetricsSnapshot {
            events_created: self.events_created.load(Ordering::Relaxed),
            events_finished: self.events_finished.load(Ordering::Relaxed),
            events_emitted_success: self.events_emitted_success.load(Ordering::Relaxed),
            events_emitted_failure: self.events_emitted_failure.load(Ordering::Relaxed),
            events_dropped_by_reason: dropped,
            retry_by_attempt: retry,
            backpressure: self.backpressure.load(Ordering::Relaxed),
            buffer_size: self.buffer_size.load(Ordering::Relaxed),
            buffer_capacity: self.buffer_capacity.load(Ordering::Relaxed),
            emit_duration_buckets: buckets,
            emit_duration_sum: hist.sum,
            emit_duration_count: hist.total,
            last_error: lock_or_recover(&self.last_error).clone(),
        }
    }

    pub fn render_prometheus(&self, namespace: &str) -> String {
        let namespace = if namespace.trim().is_empty() {
            "loza_sdk"
        } else {
            namespace.trim()
        };
        let snap = self.snapshot();
        let mut out = String::new();

        // Counters
        push_counter(
            &mut out,
            namespace,
            "events_created_total",
            snap.events_created,
        );
        push_counter(
            &mut out,
            namespace,
            "events_finished_total",
            snap.events_finished,
        );
        push_labeled_counter(
            &mut out,
            namespace,
            "events_emitted_total",
            ("status", "success"),
            snap.events_emitted_success,
        );
        push_labeled_counter(
            &mut out,
            namespace,
            "events_emitted_total",
            ("status", "failure"),
            snap.events_emitted_failure,
        );

        // Dropped with reason labels
        push_counter_vec(
            &mut out,
            namespace,
            "events_dropped_total",
            "reason",
            &snap.events_dropped_by_reason,
        );

        // Retry with attempt labels
        let retry_strings: HashMap<String, u64> = snap
            .retry_by_attempt
            .iter()
            .map(|(&k, &v)| (k.to_string(), v))
            .collect();
        push_counter_vec(
            &mut out,
            namespace,
            "retry_total",
            "attempt",
            &retry_strings,
        );

        push_counter(&mut out, namespace, "backpressure_total", snap.backpressure);

        // Gauges
        push_gauge(&mut out, namespace, "buffer_size", snap.buffer_size as f64);
        push_gauge(
            &mut out,
            namespace,
            "buffer_capacity",
            snap.buffer_capacity as f64,
        );

        // Histogram
        push_histogram(
            &mut out,
            namespace,
            "emit_duration_seconds",
            &snap.emit_duration_buckets,
            snap.emit_duration_sum,
            snap.emit_duration_count,
        );

        // Info gauge
        if !snap.last_error.is_empty() {
            out.push_str(&format!(
                "# HELP {namespace}_last_error_info Most recent observed error reason\n"
            ));
            out.push_str(&format!("# TYPE {namespace}_last_error_info gauge\n"));
            out.push_str(&format!(
                "{namespace}_last_error_info{{reason=\"{}\"}} 1\n",
                escape_label_value(&snap.last_error)
            ));
        }

        out
    }
}

impl Default for MetricsCollector {
    fn default() -> Self {
        Self::new()
    }
}

fn push_counter(out: &mut String, namespace: &str, name: &str, value: u64) {
    out.push_str(&format!(
        "# HELP {namespace}_{name} Auto-generated LOZA metric\n"
    ));
    out.push_str(&format!("# TYPE {namespace}_{name} counter\n"));
    out.push_str(&format!("{namespace}_{name} {value}\n"));
}

fn push_labeled_counter(
    out: &mut String,
    namespace: &str,
    name: &str,
    label: (&str, &str),
    value: u64,
) {
    out.push_str(&format!(
        "# HELP {namespace}_{name} Auto-generated LOZA metric\n"
    ));
    out.push_str(&format!("# TYPE {namespace}_{name} counter\n"));
    out.push_str(&format!(
        "{namespace}_{name}{{{}=\"{}\"}} {value}\n",
        label.0,
        escape_label_value(label.1)
    ));
}

fn push_counter_vec(
    out: &mut String,
    namespace: &str,
    name: &str,
    label_key: &str,
    entries: &HashMap<String, u64>,
) {
    if entries.is_empty() {
        return;
    }
    out.push_str(&format!(
        "# HELP {namespace}_{name} Auto-generated LOZA metric\n"
    ));
    out.push_str(&format!("# TYPE {namespace}_{name} counter\n"));
    let mut sorted: Vec<_> = entries.iter().collect();
    sorted.sort_by_key(|(k, _)| k.as_str());
    for (label_val, count) in sorted {
        out.push_str(&format!(
            "{namespace}_{name}{{{}=\"{}\"}} {count}\n",
            label_key,
            escape_label_value(label_val)
        ));
    }
}

fn push_gauge(out: &mut String, namespace: &str, name: &str, value: f64) {
    out.push_str(&format!(
        "# HELP {namespace}_{name} Auto-generated LOZA metric\n"
    ));
    out.push_str(&format!("# TYPE {namespace}_{name} gauge\n"));
    out.push_str(&format!("{namespace}_{name} {value}\n"));
}

fn push_histogram(
    out: &mut String,
    namespace: &str,
    name: &str,
    buckets: &[(f64, u64)],
    sum: f64,
    total: u64,
) {
    out.push_str(&format!(
        "# HELP {namespace}_{name} Auto-generated LOZA metric\n"
    ));
    out.push_str(&format!("# TYPE {namespace}_{name} histogram\n"));
    let mut cumulative = 0u64;
    for &(bucket, count) in buckets {
        cumulative += count;
        out.push_str(&format!(
            "{namespace}_{name}_bucket{{le=\"{}\"}} {cumulative}\n",
            format_float_g(bucket)
        ));
    }
    out.push_str(&format!(
        "{namespace}_{name}_bucket{{le=\"+Inf\"}} {total}\n"
    ));
    out.push_str(&format!("{namespace}_{name}_sum {sum}\n"));
    out.push_str(&format!("{namespace}_{name}_count {total}\n"));
}

/// Format a float like Python's :g — trim trailing zeros and trailing dot.
fn format_float_g(value: f64) -> String {
    let s = format!("{value}");
    if s.contains('.') {
        let trimmed = s.trim_end_matches('0');
        trimmed.strip_suffix('.').unwrap_or(trimmed).to_string()
    } else {
        s
    }
}

fn escape_label_value(value: &str) -> String {
    value
        .replace('\\', r"\\")
        .replace('\n', r"\n")
        .replace('"', r#"\""#)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn renders_prometheus_metrics() {
        let metrics = MetricsCollector::new();
        metrics.record_event_created();
        metrics.record_event_emitted(true);
        metrics.record_event_dropped("queue_full");
        metrics.record_event_dropped("sampled");
        metrics.record_retry(1);
        metrics.record_retry(2);
        metrics.record_retry(1);
        metrics.set_buffer_size(42);
        metrics.set_buffer_capacity(8192);
        metrics.observe_emit_duration(Duration::from_millis(15));

        let rendered = metrics.render_prometheus("loza");

        // Counters
        assert!(rendered.contains("loza_events_created_total 1"));
        assert!(rendered.contains("loza_events_emitted_total{status=\"success\"} 1"));

        // Dropped with reason labels
        assert!(rendered.contains("loza_events_dropped_total{reason=\"queue_full\"} 1"));
        assert!(rendered.contains("loza_events_dropped_total{reason=\"sampled\"} 1"));

        // Retry with attempt labels
        assert!(rendered.contains("loza_retry_total{attempt=\"1\"} 2"));
        assert!(rendered.contains("loza_retry_total{attempt=\"2\"} 1"));

        // Gauges
        assert!(rendered.contains("loza_buffer_size 42"));
        assert!(rendered.contains("loza_buffer_capacity 8192"));

        // Histogram
        assert!(rendered.contains("loza_emit_duration_seconds_bucket{le=\"0.025\"} 1"));
        assert!(rendered.contains("loza_emit_duration_seconds_bucket{le=\"+Inf\"} 1"));
        assert!(rendered.contains("loza_emit_duration_seconds_sum"));
        assert!(rendered.contains("loza_emit_duration_seconds_count 1"));
    }

    #[test]
    fn snapshot_includes_all_fields() {
        let metrics = MetricsCollector::with_capacity(100);
        metrics.record_event_created();
        metrics.record_event_finished();
        metrics.record_event_emitted(true);
        metrics.record_event_emitted(false);
        metrics.record_event_dropped("test");
        metrics.record_retry(3);
        metrics.record_backpressure();
        metrics.set_buffer_size(50);

        let snap = metrics.snapshot();
        assert_eq!(snap.events_created, 1);
        assert_eq!(snap.events_finished, 1);
        assert_eq!(snap.events_emitted_success, 1);
        assert_eq!(snap.events_emitted_failure, 1);
        assert_eq!(snap.events_dropped_by_reason.get("test"), Some(&1));
        assert_eq!(snap.retry_by_attempt.get(&3), Some(&1));
        assert_eq!(snap.backpressure, 1);
        assert_eq!(snap.buffer_size, 50);
        assert_eq!(snap.buffer_capacity, 100);
    }

    #[test]
    fn empty_metrics_render_cleanly() {
        let metrics = MetricsCollector::new();
        let rendered = metrics.render_prometheus("test");
        assert!(rendered.contains("test_events_created_total 0"));
        assert!(!rendered.contains("test_events_dropped_total{"));
        assert!(!rendered.contains("test_retry_total{"));
    }
}
