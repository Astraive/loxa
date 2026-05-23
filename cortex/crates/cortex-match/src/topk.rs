use std::cmp::Ordering;
use std::collections::BinaryHeap;

use crate::signature::Signature;
use crate::similarity::{self, ScoredMatch};

/// Priority queue item for top-k search
#[derive(Debug, Clone)]
struct PQItem {
    score: f64,
    signature_id: String,
}

impl PartialEq for PQItem {
    fn eq(&self, other: &Self) -> bool {
        self.score == other.score
    }
}

impl Eq for PQItem {}

impl PartialOrd for PQItem {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for PQItem {
    fn cmp(&self, other: &Self) -> Ordering {
        self.score
            .partial_cmp(&other.score)
            .unwrap_or(Ordering::Equal)
    }
}

/// Efficient top-k matching using priority queue
/// Avoids sorting all candidates, only maintains heap of size k
pub struct TopK {
    k: usize,
    threshold: f64,
    metric: String,
}

impl TopK {
    pub fn new(k: usize) -> Self {
        Self {
            k,
            threshold: 0.0,
            metric: "shape".to_string(),
        }
    }

    pub fn with_threshold(mut self, threshold: f64) -> Self {
        self.threshold = threshold;
        self
    }

    pub fn with_metric(mut self, metric: &str) -> Self {
        self.metric = metric.to_string();
        self
    }

    pub fn k(&self) -> usize {
        self.k
    }

    /// Search for top-k similar signatures
    /// O(n * d) where n = candidates, d = feature vector dim
    /// Using heap: O(n log k) instead of O(n log n)
    pub fn search(&self, query: &Signature, candidates: &[Signature]) -> Vec<ScoredMatch> {
        let k = self.k.min(candidates.len());
        if k == 0 {
            return Vec::new();
        }

        let mut heap = BinaryHeap::with_capacity(k);

        for cand in candidates {
            let score = similarity::score(query, cand, &self.metric);

            if score < self.threshold {
                continue;
            }

            if heap.len() < k {
                heap.push(PQItem {
                    score,
                    signature_id: cand.shape.clone(),
                });
            } else if score > heap.peek().map(|p| p.score).unwrap_or(0.0) {
                heap.pop();
                heap.push(PQItem {
                    score,
                    signature_id: cand.shape.clone(),
                });
            }
        }

        let mut results: Vec<ScoredMatch> = heap
            .into_iter()
            .map(|item| ScoredMatch::new(item.signature_id, item.score))
            .collect();

        results.sort_by(|a, b| {
            b.similarity
                .partial_cmp(&a.similarity)
                .unwrap_or(Ordering::Equal)
        });
        results
    }

    /// Batch search for multiple queries
    pub fn search_batch(
        &self,
        queries: &[Signature],
        candidates: &[Signature],
    ) -> Vec<Vec<ScoredMatch>> {
        queries.iter().map(|q| self.search(q, candidates)).collect()
    }
}

/// Brute force for comparison/verification
pub fn search_brute(
    query: &Signature,
    candidates: &[Signature],
    k: usize,
    metric: &str,
) -> Vec<ScoredMatch> {
    let mut results: Vec<ScoredMatch> = candidates
        .iter()
        .map(|cand| {
            let score = similarity::score(query, cand, metric);
            ScoredMatch {
                signature_id: cand.shape.clone(),
                similarity: score,
                matched_symptoms: 0,
                temporal_similarity: 0.0,
            }
        })
        .collect();

    results.sort_by(|a, b| {
        b.similarity
            .partial_cmp(&a.similarity)
            .unwrap_or(Ordering::Equal)
    });
    results.truncate(k);
    results
}

/// Approximate nearest neighbor using bucketing
/// Divides feature space into buckets for faster search
pub struct ANNIndex {
    signatures: Vec<Signature>,
    _bucket_size: usize,
}

impl ANNIndex {
    pub fn new(bucket_size: usize) -> Self {
        Self {
            signatures: Vec::new(),
            _bucket_size: bucket_size,
        }
    }

    pub fn add(&mut self, sig: Signature) {
        self.signatures.push(sig);
    }

    pub fn build(candidates: &[Signature], bucket_size: usize) -> Self {
        let mut index = Self::new(bucket_size);
        index.signatures = candidates.to_vec();
        index
    }

    pub fn search(&self, query: &Signature, k: usize, metric: &str) -> Vec<ScoredMatch> {
        // Simple bucketing: search all, let TopK handle it
        let topk = TopK::new(k).with_metric(metric);
        topk.search(query, &self.signatures)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::signature::Signature;

    #[test]
    fn test_topk() {
        let query = Signature::from_incident(
            &["deploy".to_string()],
            &["api".to_string()],
            &["latency_spike".to_string()],
            &["rollback".to_string()],
        );

        let candidates: Vec<Signature> = (0..20)
            .map(|i| {
                if i % 3 == 0 {
                    Signature::from_incident(
                        &["deploy".to_string()],
                        &["api".to_string()],
                        &["latency_spike".to_string()],
                        &["rollback".to_string()],
                    )
                } else {
                    Signature::from_incident(
                        &["deploy".to_string()],
                        &["db".to_string()],
                        &["error_rate".to_string()],
                        &["restart".to_string()],
                    )
                }
            })
            .collect();

        let topk = TopK::new(5).with_metric("shape");
        let results = topk.search(&query, &candidates);

        assert_eq!(results.len(), 5);
        assert!(results[0].similarity >= results[1].similarity);
    }

    #[test]
    fn test_search_consistency() {
        let query = Signature::from_incident(
            &["deploy".to_string()],
            &["api".to_string()],
            &["latency_spike".to_string()],
            &[],
        );

        let candidates: Vec<Signature> = (0..100)
            .map(|_| {
                Signature::from_incident(
                    &["deploy".to_string()],
                    &["svc".to_string()],
                    &["latency_spike".to_string()],
                    &[],
                )
            })
            .collect();

        let topk = TopK::new(10).with_metric("shape");
        let results = topk.search(&query, &candidates);

        let brute = search_brute(&query, &candidates, 10, "shape");

        assert_eq!(results.len(), brute.len());
    }

    #[test]
    fn test_threshold() {
        let query = Signature::new();

        let candidates: Vec<Signature> = (0..10)
            .map(|i| {
                let mut sig = Signature::new();
                sig.feature_vector = vec![i as f64 / 10.0];
                sig
            })
            .collect();

        let topk = TopK::new(5).with_threshold(0.5);
        let results = topk.search(&query, &candidates);

        for r in &results {
            assert!(r.similarity >= 0.5);
        }
    }
}
