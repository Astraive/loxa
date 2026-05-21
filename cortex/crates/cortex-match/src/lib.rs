//! cortex-match: High-performance incident shape matching for LOXA Cortex
//! 
//! CPU-heavy pure algorithm work:
//! - incident shape similarity scoring
//! - top-k similar incident search  
//! - graph pattern matching
//! - topology-independent matching
//! - feature vector distance
//! - fast pattern comparison
//! - approximate matching

pub mod signature;
pub mod similarity;
pub mod topk;
pub mod graph;

pub use signature::{Signature, SymptomClass, Event, normalize_signature, compute_shape_hash};
pub use similarity::ScoredMatch;
pub use topk::TopK;
pub use graph::{IncidentGraph, Node, NodeType, Edge, EdgeType};

/// Main matcher interface - matches incidents by shape
pub struct Matcher {
    topk: TopK,
}

impl Matcher {
    pub fn new() -> Self {
        Self {
            topk: TopK::new(5),
        }
    }

    pub fn with_k(mut self, k: usize) -> Self {
        self.topk = TopK::new(k);
        self
    }

    pub fn with_threshold(mut self, threshold: f64) -> Self {
        self.topk = self.topk.with_threshold(threshold);
        self
    }

    pub fn with_metric(mut self, metric: &str) -> Self {
        self.topk = self.topk.with_metric(metric);
        self
    }

    /// Score a query signature against a candidate
    pub fn score(&self, query: &Signature, candidate: &Signature) -> f64 {
        similarity::score(query, candidate, "shape")
    }

    /// Find top-k similar signatures to query
    pub fn top_k(&self, query: &Signature, candidates: &[Signature]) -> Vec<ScoredMatch> {
        self.topk.search(query, candidates)
    }

    /// Find top-k using a specific metric
    pub fn search(&self, query: &Signature, candidates: &[Signature], metric: &str) -> Vec<ScoredMatch> {
        let tk = TopK::new(self.topk.k()).with_metric(metric);
        tk.search(query, candidates)
    }
}

impl Default for Matcher {
    fn default() -> Self {
        Self::new()
    }
}

/// Convenient function for matching a query incident against candidates
/// Returns top-k closest matches with similarity scores
/// 
/// # Example
/// 
/// ```ignore
/// let query = normalize_signature(&events, &services);
/// let candidates = load_signatures(); // from DB
/// let matches = match_incident(&query, &candidates, 5);
/// for m in matches {
///     println!("{}: {:.2}", m.signature_id, m.similarity);
/// }
/// ```
pub fn match_incident(
    query: &Signature,
    candidates: &[Signature],
    k: usize,
) -> Vec<ScoredMatch> {
    let topk = TopK::new(k).with_metric("shape");
    topk.search(query, candidates)
}

/// Match using topology-independent similarity (ignores service names)
pub fn match_topology_independent(
    query: &Signature,
    candidates: &[Signature],
    k: usize,
) -> Vec<ScoredMatch> {
    let topk = TopK::new(k).with_metric("topology");
    topk.search(query, candidates)
}

/// Fast pattern comparison for graph matching
pub fn match_graph(
    query: &graph::IncidentGraph,
    candidates: &[graph::IncidentGraph],
    k: usize,
) -> Vec<ScoredMatch> {
    graph::match_graph_pattern(query, candidates, 0.5)
        .into_iter()
        .take(k)
        .collect()
}

/// Version info
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg(test)]
mod tests {
    use super::*;
    use signature::Signature;

    #[test]
    fn test_matcher_interface() {
        let matcher = Matcher::new().with_k(10).with_threshold(0.3);

        let query = Signature::from_incident(
            &["deploy".to_string()],
            &["api".to_string()],
            &["latency_spike".to_string()],
            &["rollback".to_string()],
        );

        let candidates: Vec<Signature> = (0..50)
            .map(|_| {
                Signature::from_incident(
                    &["deploy".to_string()],
                    &["svc".to_string()],
                    &["latency_spike".to_string()],
                    &["rollback".to_string()],
                )
            })
            .collect();

        let matches = matcher.top_k(&query, &candidates);
        assert_eq!(matches.len(), 10);
    }

    #[test]
    fn test_match_incident() {
        let query = normalize_signature(
            &[
                signature::Event { kind: "deploy".to_string(), service: None, timestamp_ms: 0 },
                signature::Event { kind: "latency_spike".to_string(), service: None, timestamp_ms: 5000 },
                signature::Event { kind: "timeout".to_string(), service: None, timestamp_ms: 10000 },
            ],
            &["payments-svc".to_string(), "db".to_string()],
        );

        let candidates: Vec<Signature> = (0..100)
            .map(|_| {
                normalize_signature(
                    &[
                        signature::Event { kind: "deploy".to_string(), service: None, timestamp_ms: 0 },
                        signature::Event { kind: "latency_spike".to_string(), service: None, timestamp_ms: 5000 },
                        signature::Event { kind: "timeout".to_string(), service: None, timestamp_ms: 10000 },
                    ],
                    &["billing-svc".to_string(), "postgres".to_string()],
                )
            })
            .collect();

        let matches = match_incident(&query, &candidates, 5);
        assert_eq!(matches.len(), 5);
    }

    #[test]
    fn test_topology_independent() {
        let query = normalize_signature(
            &[
                signature::Event { kind: "deploy".to_string(), service: None, timestamp_ms: 0 },
                signature::Event { kind: "latency_spike".to_string(), service: None, timestamp_ms: 5000 },
            ],
            &["payments-svc".to_string()],
        );

        let candidates = vec![
            normalize_signature(
                &[
                    signature::Event { kind: "deploy".to_string(), service: None, timestamp_ms: 0 },
                    signature::Event { kind: "latency_spike".to_string(), service: None, timestamp_ms: 5000 },
                ],
                &["billing-svc".to_string()],
            ),
            normalize_signature(
                &[
                    signature::Event { kind: "deploy".to_string(), service: None, timestamp_ms: 0 },
                    signature::Event { kind: "error_rate".to_string(), service: None, timestamp_ms: 5000 },
                ],
                &["invoice-svc".to_string()],
            ),
        ];

        let matches = match_topology_independent(&query, &candidates, 2);
        assert!(matches[0].similarity > matches[1].similarity);
    }

    #[test]
    fn test_version() {
        assert!(!VERSION.is_empty());
    }
}
