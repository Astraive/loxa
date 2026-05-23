use std::hash::Hash;

use crate::signature::Signature;
use serde::{Deserialize, Serialize};

#[allow(clippy::module_inception)]
/// Multiple similarity metrics for incident matching
pub mod similarity {

    use super::*;

    /// Cosine similarity between feature vectors
    pub fn cosine(a: &[f64], b: &[f64]) -> f64 {
        if a.is_empty() || b.is_empty() {
            return 0.0;
        }

        let min_len = a.len().min(b.len());
        let dot: f64 = a
            .iter()
            .take(min_len)
            .zip(b.iter().take(min_len))
            .map(|(x, y)| x * y)
            .sum();

        let a_mag = a.iter().map(|x| x * x).sum::<f64>().sqrt();
        let b_mag = b.iter().map(|x| x * x).sum::<f64>().sqrt();

        if a_mag == 0.0 || b_mag == 0.0 {
            return 0.0;
        }

        (dot / (a_mag * b_mag)).clamp(0.0, 1.0)
    }

    /// Euclidean distance (normalized to similarity)
    pub fn euclidean(a: &[f64], b: &[f64]) -> f64 {
        if a.is_empty() || b.is_empty() {
            return 0.0;
        }

        let min_len = a.len().min(b.len());
        let sum_sq: f64 = a
            .iter()
            .take(min_len)
            .zip(b.iter().take(min_len))
            .map(|(x, y)| (x - y).powi(2))
            .sum();

        let dist = sum_sq.sqrt();

        // Normalize: max possible distance for n-dimensional space is sqrt(n)
        // Convert distance to similarity: 1 - normalized_distance
        let max_dist = (min_len as f64).sqrt();
        if max_dist == 0.0 {
            return 0.0;
        }

        (1.0 - (dist / max_dist)).clamp(0.0, 1.0)
    }

    /// Jaccard similarity for sets/arrays (symptoms, services, remediation)
    pub fn jaccard<T: Eq + Hash>(a: &[T], b: &[T]) -> f64 {
        if a.is_empty() && b.is_empty() {
            return 0.0;
        }

        let set_a: std::collections::HashSet<_> = a.iter().collect();
        let set_b: std::collections::HashSet<_> = b.iter().collect();

        let intersection = set_a.intersection(&set_b).count();
        let union = set_a.union(&set_b).count();

        if union == 0 {
            return 0.0;
        }

        intersection as f64 / union as f64
    }

    /// Temporal similarity - compare event sequences ignoring exact timing
    /// Uses dynamic time warping simplified: compare relative positions
    pub fn temporal(a: &[i64], b: &[i64]) -> f64 {
        if a.is_empty() && b.is_empty() {
            return 1.0;
        }
        if a.is_empty() || b.is_empty() {
            return 0.0;
        }

        // Normalize to relative positions (0 to 1)
        let normalize = |ts: &[i64]| -> Vec<f64> {
            if ts.len() < 2 {
                return ts.iter().map(|&t| t as f64).collect();
            }
            let min = ts[0];
            let max = ts.iter().max().copied().unwrap_or(ts[0]);
            let range = (max - min).max(1) as f64;
            ts.iter().map(|&t| (t - min) as f64 / range).collect()
        };

        let a_norm = normalize(a);
        let b_norm = normalize(b);

        // For sequences of same length, compare position correlation
        if a_norm.len() == b_norm.len() {
            let diffs: f64 = a_norm
                .iter()
                .zip(b_norm.iter())
                .map(|(x, y)| (x - y).abs())
                .sum();
            let avg_diff = diffs / a_norm.len() as f64;
            return (1.0 - avg_diff).clamp(0.0, 1.0);
        }

        // Different lengths: compare using common subsequence
        let min_len = a_norm.len().min(b_norm.len());
        cosine(&a_norm[..min_len], &b_norm[..min_len])
    }

    /// Combined shape similarity - weighted combination of all metrics
    pub fn shape_similarity(query: &Signature, candidate: &Signature) -> f64 {
        let feature_sim = cosine(&query.feature_vector, &candidate.feature_vector);

        let symptom_sim = jaccard(&query.symptom_classes, &candidate.symptom_classes);

        let temporal_sim = temporal(&query.temporal_sequence, &candidate.temporal_sequence);

        // Weighted combination
        // Feature vector is most important for exact matching
        // Symptom class for semantic matching
        // Temporal for ordering matching
        let w_feature = 0.4;
        let w_symptom = 0.4;
        let w_temporal = 0.2;

        w_feature * feature_sim + w_symptom * symptom_sim + w_temporal * temporal_sim
    }

    /// Topology-independent similarity
    /// Ignores exact service names, only compares symptom patterns
    pub fn topology_independent(query: &Signature, candidate: &Signature) -> f64 {
        let mut total_weight = 0.0;
        let mut total_score = 0.0;

        let symptom_sim = jaccard(&query.symptom_classes, &candidate.symptom_classes);
        total_score += 0.5 * symptom_sim;
        total_weight += 0.5;

        if !query.temporal_sequence.is_empty() || !candidate.temporal_sequence.is_empty() {
            total_score += 0.3 * temporal(&query.temporal_sequence, &candidate.temporal_sequence);
            total_weight += 0.3;
        }

        if !query.resolution_pattern.is_empty() || !candidate.resolution_pattern.is_empty() {
            total_score += 0.2 * jaccard(&query.resolution_pattern, &candidate.resolution_pattern);
            total_weight += 0.2;
        }

        if total_weight == 0.0 {
            return 0.0;
        }

        (total_score / total_weight).clamp(0.0, 1.0)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ScoredMatch {
    pub signature_id: String,
    pub similarity: f64,
    pub matched_symptoms: usize,
    pub temporal_similarity: f64,
}

impl ScoredMatch {
    pub fn new(id: String, sim: f64) -> Self {
        Self {
            signature_id: id,
            similarity: sim,
            matched_symptoms: 0,
            temporal_similarity: 0.0,
        }
    }
}

/// Score two signatures using specified metric
pub fn score(query: &Signature, candidate: &Signature, metric: &str) -> f64 {
    match metric {
        "cosine" => similarity::cosine(&query.feature_vector, &candidate.feature_vector),
        "euclidean" => similarity::euclidean(&query.feature_vector, &candidate.feature_vector),
        "shape" => similarity::shape_similarity(query, candidate),
        "topology" => similarity::topology_independent(query, candidate),
        _ => similarity::shape_similarity(query, candidate),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::signature::{Signature, SymptomClass};

    #[test]
    fn test_cosine() {
        let a = vec![1.0, 0.0, 0.0];
        let b = vec![1.0, 0.0, 0.0];
        let c = vec![0.0, 1.0, 0.0];

        assert!((similarity::cosine(&a, &b) - 1.0).abs() < 1e-6);
        assert!((similarity::cosine(&a, &c) - 0.0).abs() < 1e-6);
    }

    #[test]
    fn test_jaccard() {
        let a = vec!["latency", "error"];
        let b = vec!["latency", "error"];
        let c = vec!["timeout"];

        assert!((similarity::jaccard(&a, &b) - 1.0).abs() < 1e-6);
        assert!((similarity::jaccard(&a, &c) - 0.0).abs() < 1e-6);
    }

    #[test]
    fn test_shape_similarity() {
        let mut q = Signature::new();
        q.symptom_classes = vec![SymptomClass::Latency, SymptomClass::Timeout];
        q.feature_vector = vec![1.0, 1.0, 1.0, 0.0, 0.0, 0.5];
        q.temporal_sequence = vec![0, 5000, 10000];

        let c = q.clone();

        let sim = similarity::shape_similarity(&q, &c);
        assert!(sim > 0.9);
    }

    #[test]
    fn test_topology_independent() {
        let mut q = Signature::new();
        q.symptom_classes = vec![SymptomClass::Latency, SymptomClass::Timeout];

        let mut c = Signature::new();
        c.symptom_classes = vec![SymptomClass::Latency, SymptomClass::Timeout];

        let sim = similarity::topology_independent(&q, &c);
        assert!(sim > 0.9);
    }
}
