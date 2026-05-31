use criterion::{black_box, criterion_group, criterion_main, Criterion};

use cortex_match::graph::{
    compute_graph_signature, extract_causality_pattern, graph_相似度, match_graph_pattern, Edge,
    EdgeType, IncidentGraph, Node, NodeType,
};
use cortex_match::signature::{
    compute_shape_hash, normalize_signature, Event, Signature, SymptomClass,
};
use cortex_match::similarity;
use cortex_match::topk::{search_brute, ANNIndex, TopK};
use cortex_match::{match_incident, match_topology_independent, Matcher};

// ── Helpers ──────────────────────────────────────────────────────────────────

fn make_query_signature() -> Signature {
    normalize_signature(
        &[
            Event {
                kind: "deploy".to_string(),
                service: None,
                timestamp_ms: 0,
            },
            Event {
                kind: "latency_spike".to_string(),
                service: None,
                timestamp_ms: 5000,
            },
            Event {
                kind: "timeout".to_string(),
                service: None,
                timestamp_ms: 10000,
            },
            Event {
                kind: "rollback".to_string(),
                service: None,
                timestamp_ms: 15000,
            },
        ],
        &["payments-svc".to_string(), "db".to_string()],
    )
}

fn make_candidate_signatures(n: usize) -> Vec<Signature> {
    let kinds = [
        vec!["deploy", "latency_spike", "timeout", "rollback"],
        vec!["deploy", "error_rate", "restart"],
        vec!["deploy", "memory_leak", "rollback"],
        vec!["deploy", "cpu_spike", "error_rate", "restart"],
        vec!["deploy", "latency_spike", "timeout", "deploy_fail"],
    ];
    (0..n)
        .map(|i| {
            let pattern = &kinds[i % kinds.len()];
            let events: Vec<Event> = pattern
                .iter()
                .enumerate()
                .map(|(j, k)| Event {
                    kind: k.to_string(),
                    service: None,
                    timestamp_ms: (j as i64) * 5000,
                })
                .collect();
            let services = match i % 4 {
                0 => vec!["api-svc".to_string(), "postgres".to_string()],
                1 => vec!["checkout".to_string(), "redis".to_string()],
                2 => vec!["billing-svc".to_string(), "kafka".to_string()],
                _ => vec!["gateway".to_string(), "db".to_string()],
            };
            normalize_signature(&events, &services)
        })
        .collect()
}

fn make_graph(prefix: &str, n_nodes: usize) -> IncidentGraph {
    let mut graph = IncidentGraph::new();
    let node_types = [
        NodeType::Service,
        NodeType::Deploy,
        NodeType::Metric,
        NodeType::Log,
        NodeType::Incident,
    ];
    for i in 0..n_nodes {
        let nt = node_types[i % node_types.len()].clone();
        graph.add_node(format!("{}_{}", prefix, i), nt);
    }
    let edge_types = [
        EdgeType::DependsOn,
        EdgeType::SameTrace,
        EdgeType::DeployedBefore,
        EdgeType::MetricSpikedAfter,
        EdgeType::LogErrorAfter,
        EdgeType::CausedProbably,
        EdgeType::RemediatedBy,
    ];
    for i in 0..n_nodes.saturating_sub(1) {
        let et = edge_types[i % edge_types.len()].clone();
        graph.add_edge(Edge {
            from: format!("{}_{}", prefix, i),
            to: format!("{}_{}", prefix, i + 1),
            edge_type: et,
            weight: 1.0,
        });
    }
    graph
}

// ── cortex-match benchmarks ──────────────────────────────────────────────────

fn bench_shape_matching(c: &mut Criterion) {
    let query = make_query_signature();
    let candidates = make_candidate_signatures(50);
    let matcher = Matcher::new().with_k(5);

    c.bench_function("shape_matching_topk", |b| {
        b.iter(|| {
            let results = matcher.top_k(black_box(&query), black_box(&candidates));
            black_box(&results);
        })
    });

    // Also bench the convenience function
    let query2 = make_query_signature();
    let candidates2 = make_candidate_signatures(50);
    c.bench_function("match_incident_fn", |b| {
        b.iter(|| {
            let results = match_incident(black_box(&query2), black_box(&candidates2), 5);
            black_box(&results);
        })
    });

    // Topology-independent matching
    let query3 = make_query_signature();
    let candidates3 = make_candidate_signatures(50);
    c.bench_function("match_topology_independent", |b| {
        b.iter(|| {
            let results =
                match_topology_independent(black_box(&query3), black_box(&candidates3), 5);
            black_box(&results);
        })
    });
}

fn bench_similarity_scoring(c: &mut Criterion) {
    let a = vec![1.0, 0.5, 0.3, 0.8, 0.2, 0.9, 0.1, 0.7, 0.4, 0.6];
    let b_vec = vec![0.9, 0.6, 0.2, 0.7, 0.3, 0.8, 0.2, 0.6, 0.5, 0.5];

    c.bench_function("cosine_similarity", |b| {
        b.iter(|| {
            let s = similarity::similarity::cosine(black_box(&a), black_box(&b_vec));
            black_box(s);
        })
    });

    c.bench_function("euclidean_similarity", |b| {
        b.iter(|| {
            let s = similarity::similarity::euclidean(black_box(&a), black_box(&b_vec));
            black_box(s);
        })
    });

    let set_a: Vec<SymptomClass> = vec![
        SymptomClass::Latency,
        SymptomClass::Error,
        SymptomClass::Timeout,
        SymptomClass::Resource,
    ];
    let set_b: Vec<SymptomClass> = vec![
        SymptomClass::Latency,
        SymptomClass::Timeout,
        SymptomClass::Deployment,
    ];
    c.bench_function("jaccard_similarity", |b| {
        b.iter(|| {
            let s = similarity::similarity::jaccard(black_box(&set_a), black_box(&set_b));
            black_box(s);
        })
    });

    let ta: Vec<i64> = vec![0, 3000, 7000, 12000, 18000];
    let tb: Vec<i64> = vec![0, 2500, 6000, 11000, 17000];
    c.bench_function("temporal_similarity", |b| {
        b.iter(|| {
            let s = similarity::similarity::temporal(black_box(&ta), black_box(&tb));
            black_box(s);
        })
    });

    // Full shape_similarity (combined metric)
    let query = make_query_signature();
    let candidate = normalize_signature(
        &[
            Event {
                kind: "deploy".to_string(),
                service: None,
                timestamp_ms: 0,
            },
            Event {
                kind: "latency_spike".to_string(),
                service: None,
                timestamp_ms: 5000,
            },
            Event {
                kind: "timeout".to_string(),
                service: None,
                timestamp_ms: 10000,
            },
            Event {
                kind: "rollback".to_string(),
                service: None,
                timestamp_ms: 15000,
            },
        ],
        &["billing-svc".to_string(), "postgres".to_string()],
    );
    c.bench_function("shape_similarity_combined", |b| {
        b.iter(|| {
            let s =
                similarity::similarity::shape_similarity(black_box(&query), black_box(&candidate));
            black_box(s);
        })
    });

    c.bench_function("topology_independent_similarity", |b| {
        b.iter(|| {
            let s = similarity::similarity::topology_independent(
                black_box(&query),
                black_box(&candidate),
            );
            black_box(s);
        })
    });

    // Shape hash
    c.bench_function("compute_shape_hash", |b| {
        b.iter(|| {
            let h = compute_shape_hash(black_box(&query));
            black_box(h);
        })
    });
}

fn bench_topk_search(c: &mut Criterion) {
    let query = make_query_signature();

    for &n in &[100, 500, 1000] {
        let candidates = make_candidate_signatures(n);
        let topk = TopK::new(10).with_metric("shape");

        c.bench_function(&format!("topk_search_{}_candidates", n), |b| {
            b.iter(|| {
                let results = topk.search(black_box(&query), black_box(&candidates));
                black_box(&results);
            })
        });

        // Brute force for comparison
        c.bench_function(&format!("brute_search_{}_candidates", n), |b| {
            b.iter(|| {
                let results = search_brute(black_box(&query), black_box(&candidates), 10, "shape");
                black_box(&results);
            })
        });
    }

    // ANN index
    let candidates = make_candidate_signatures(500);
    let index = ANNIndex::build(&candidates, 50);
    c.bench_function("ann_search_500_candidates", |b| {
        b.iter(|| {
            let results = index.search(black_box(&query), 10, "shape");
            black_box(&results);
        })
    });
}

fn bench_large_ruleset(c: &mut Criterion) {
    let query = make_query_signature();
    let candidates_100 = make_candidate_signatures(100);
    let candidates_500 = make_candidate_signatures(500);

    let matcher = Matcher::new().with_k(10).with_threshold(0.1);

    c.bench_function("large_ruleset_100", |b| {
        b.iter(|| {
            let results = matcher.top_k(black_box(&query), black_box(&candidates_100));
            black_box(&results);
        })
    });

    c.bench_function("large_ruleset_500", |b| {
        b.iter(|| {
            let results = matcher.top_k(black_box(&query), black_box(&candidates_500));
            black_box(&results);
        })
    });

    // Graph matching
    let query_graph = make_graph("q", 8);
    let cand_graphs: Vec<IncidentGraph> = (0..100)
        .map(|i| make_graph(&format!("c{}", i), 10))
        .collect();

    c.bench_function("graph_pattern_match_100", |b| {
        b.iter(|| {
            let results =
                match_graph_pattern(black_box(&query_graph), black_box(&cand_graphs), 0.3);
            black_box(&results);
        })
    });

    c.bench_function("compute_graph_signature", |b| {
        b.iter(|| {
            let sig = compute_graph_signature(black_box(&query_graph));
            black_box(sig);
        })
    });

    c.bench_function("extract_causality_pattern", |b| {
        b.iter(|| {
            let pat = extract_causality_pattern(black_box(&query_graph));
            black_box(pat);
        })
    });

    c.bench_function("graph_similarity", |b| {
        let g2 = make_graph("cand", 8);
        b.iter(|| {
            let sim = graph_相似度(black_box(&query_graph), black_box(&g2));
            black_box(sim);
        })
    });
}

fn bench_invalid_input(c: &mut Criterion) {
    // Empty signatures
    let empty = Signature::new();
    let query = make_query_signature();
    let matcher = Matcher::new().with_k(5);

    c.bench_function("score_empty_candidate", |b| {
        b.iter(|| {
            let s = matcher.score(black_box(&query), black_box(&empty));
            black_box(s);
        })
    });

    c.bench_function("topk_empty_candidates", |b| {
        let empty_vec: Vec<Signature> = vec![];
        b.iter(|| {
            let results = matcher.top_k(black_box(&query), black_box(&empty_vec));
            black_box(&results);
        })
    });

    c.bench_function("cosine_empty_vectors", |b| {
        let a: Vec<f64> = vec![];
        let b_vec: Vec<f64> = vec![];
        b.iter(|| {
            let s = similarity::similarity::cosine(black_box(&a), black_box(&b_vec));
            black_box(s);
        })
    });

    c.bench_function("jaccard_empty_sets", |b| {
        let a: Vec<SymptomClass> = vec![];
        let b_vec: Vec<SymptomClass> = vec![];
        b.iter(|| {
            let s = similarity::similarity::jaccard(black_box(&a), black_box(&b_vec));
            black_box(s);
        })
    });

    c.bench_function("topology_independent_empty", |b| {
        b.iter(|| {
            let s =
                similarity::similarity::topology_independent(black_box(&empty), black_box(&empty));
            black_box(s);
        })
    });

    // Graph with no edges
    let mut sparse_graph = IncidentGraph::new();
    sparse_graph.add_node("only".to_string(), NodeType::Service);
    c.bench_function("graph_signature_single_node", |b| {
        b.iter(|| {
            let sig = compute_graph_signature(black_box(&sparse_graph));
            black_box(sig);
        })
    });
}

criterion_group!(
    benches,
    bench_shape_matching,
    bench_similarity_scoring,
    bench_topk_search,
    bench_large_ruleset,
    bench_invalid_input,
);
criterion_main!(benches);
