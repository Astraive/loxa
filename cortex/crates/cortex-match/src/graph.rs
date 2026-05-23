use std::collections::{HashMap, HashSet};

use crate::similarity::ScoredMatch;

/// Graph node representation
#[derive(Debug, Clone, Hash, PartialEq, Eq)]
pub struct Node {
    pub id: String,
    pub node_type: NodeType,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
pub enum NodeType {
    Service,
    Deploy,
    Metric,
    Log,
    Incident,
}

/// Graph edge representation
#[derive(Debug, Clone, PartialEq)]
pub struct Edge {
    pub from: String,
    pub to: String,
    pub edge_type: EdgeType,
    pub weight: f64,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
pub enum EdgeType {
    DependsOn,
    SameTrace,
    SameIncident,
    DeployedBefore,
    MetricSpikedAfter,
    LogErrorAfter,
    CausedProbably,
    RemediatedBy,
    SimilarShape,
}

/// Graph structure for incident analysis
#[derive(Debug, Clone)]
pub struct IncidentGraph {
    nodes: HashMap<String, Node>,
    edges: Vec<Edge>,
}

impl IncidentGraph {
    pub fn new() -> Self {
        Self {
            nodes: HashMap::new(),
            edges: Vec::new(),
        }
    }

    pub fn add_node(&mut self, id: String, node_type: NodeType) {
        self.nodes.insert(id.clone(), Node { id, node_type });
    }

    pub fn add_edge(&mut self, edge: Edge) {
        if self.nodes.contains_key(&edge.from) && self.nodes.contains_key(&edge.to) {
            self.edges.push(edge);
        }
    }

    pub fn nodes(&self) -> &HashMap<String, Node> {
        &self.nodes
    }

    pub fn edges(&self) -> &Vec<Edge> {
        &self.edges
    }

    /// Extract subgraph around a node
    pub fn extract_subgraph(&self, center: &str, depth: usize) -> IncidentGraph {
        let mut subgraph = IncidentGraph::new();
        let mut visited = HashSet::new();
        let mut queue: Vec<(String, usize)> = vec![(center.to_string(), 0)];

        while let Some((node_id, d)) = queue.pop() {
            if visited.contains(&node_id) || d > depth {
                continue;
            }
            visited.insert(node_id.clone());

            if let Some(node) = self.nodes.get(&node_id) {
                subgraph.add_node(node.id.clone(), node.node_type.clone());
            }

            for edge in &self.edges {
                if edge.from == node_id {
                    if let Some(node) = self.nodes.get(&edge.to) {
                        subgraph.add_node(node.id.clone(), node.node_type.clone());
                        subgraph.add_edge(edge.clone());
                    }
                    if d < depth {
                        queue.push((edge.to.clone(), d + 1));
                    }
                }
            }
        }

        subgraph
    }
}

impl Default for IncidentGraph {
    fn default() -> Self {
        Self::new()
    }
}

/// Compute graph signature - topology pattern without exact service names
pub fn compute_graph_signature(graph: &IncidentGraph) -> String {
    let mut pattern = String::new();
    let mut node_types: Vec<String> = graph
        .nodes()
        .values()
        .map(|n| format!("{:?}", n.node_type))
        .collect();
    node_types.sort();
    pattern.push_str(&node_types.join(":"));

    let mut edge_types: Vec<String> = graph
        .edges()
        .iter()
        .map(|e| format!("{:?}", e.edge_type))
        .collect();
    edge_types.sort();
    if !edge_types.is_empty() {
        pattern.push_str("->");
        pattern.push_str(&edge_types.join(","));
    }

    pattern
}

/// Find similar graphs by pattern matching
pub fn match_graph_pattern(
    query: &IncidentGraph,
    candidates: &[IncidentGraph],
    min_similarity: f64,
) -> Vec<ScoredMatch> {
    let query_sig = compute_graph_signature(query);

    candidates
        .iter()
        .enumerate()
        .map(|(i, cand)| {
            let cand_sig = compute_graph_signature(cand);
            let sim = string_similarity(&query_sig, &cand_sig);
            ScoredMatch {
                signature_id: format!("graph_{}", i),
                similarity: sim,
                matched_symptoms: 0,
                temporal_similarity: 0.0,
            }
        })
        .filter(|m| m.similarity >= min_similarity)
        .collect()
}

/// Simple string similarity for pattern comparison
fn string_similarity(a: &str, b: &str) -> f64 {
    if a.is_empty() && b.is_empty() {
        return 1.0;
    }
    if a.is_empty() || b.is_empty() {
        return 0.0;
    }

    let set_a: HashSet<_> = a.split(':').collect();
    let set_b: HashSet<_> = b.split(':').collect();

    let intersection = set_a.intersection(&set_b).count();
    let union = set_a.union(&set_b).count();

    if union == 0 {
        return 0.0;
    }

    intersection as f64 / union as f64
}

/// Extract causality pattern from incident graph
/// Returns: deploy → symptom → symptom → remediation
pub fn extract_causality_pattern(graph: &IncidentGraph) -> Vec<String> {
    let mut pattern = Vec::new();

    // Find deploy nodes
    let deploys: Vec<_> = graph
        .nodes()
        .values()
        .filter(|n| matches!(n.node_type, NodeType::Deploy))
        .collect();

    // Find symptom nodes (metrics, logs)
    let _symptoms: Vec<_> = graph
        .nodes()
        .values()
        .filter(|n| matches!(n.node_type, NodeType::Metric | NodeType::Log))
        .collect();

    for deploy in deploys {
        pattern.push(format!("{:?}", deploy.node_type));

        // Find edges from deploy to symptoms
        for edge in graph.edges() {
            if edge.from == deploy.id {
                if let Some(node) = graph.nodes().get(&edge.to) {
                    pattern.push(format!("{:?}", node.node_type));
                }
            }
        }
    }

    // Find remediation
    let remediations: Vec<_> = graph
        .nodes()
        .values()
        .filter(|n| matches!(n.node_type, NodeType::Incident))
        .collect();

    if !remediations.is_empty() {
        pattern.push("Remediation".to_string());
    }

    pattern
}

/// Graph isomorphism check (simplified - just node/edge counts)
pub fn graph_相似度(a: &IncidentGraph, b: &IncidentGraph) -> f64 {
    let a_nodes = a.nodes().len();
    let b_nodes = b.nodes().len();
    let a_edges = a.edges().len();
    let b_edges = b.edges().len();

    if a_nodes == 0 && b_nodes == 0 {
        return 1.0;
    }

    let node_sim = if a_nodes == 0 && b_nodes == 0 {
        1.0
    } else {
        a_nodes.min(b_nodes) as f64 / a_nodes.max(b_nodes).max(1) as f64
    };
    let edge_sim = if a_edges == 0 && b_edges == 0 {
        1.0
    } else {
        a_edges.min(b_edges) as f64 / a_edges.max(b_edges).max(1) as f64
    };

    0.6 * node_sim + 0.4 * edge_sim
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_graph_construction() {
        let mut graph = IncidentGraph::new();

        graph.add_node("api".to_string(), NodeType::Service);
        graph.add_node("deploy1".to_string(), NodeType::Deploy);
        graph.add_node("metric1".to_string(), NodeType::Metric);

        graph.add_edge(Edge {
            from: "deploy1".to_string(),
            to: "api".to_string(),
            edge_type: EdgeType::DeployedBefore,
            weight: 1.0,
        });

        assert_eq!(graph.nodes().len(), 3);
        assert_eq!(graph.edges().len(), 1);
    }

    #[test]
    fn test_causality_pattern() {
        let mut graph = IncidentGraph::new();

        graph.add_node("svc".to_string(), NodeType::Service);
        graph.add_node("deploy1".to_string(), NodeType::Deploy);
        graph.add_node("latency".to_string(), NodeType::Metric);
        graph.add_node("error".to_string(), NodeType::Log);

        graph.add_edge(Edge {
            from: "deploy1".to_string(),
            to: "svc".to_string(),
            edge_type: EdgeType::DeployedBefore,
            weight: 1.0,
        });

        graph.add_edge(Edge {
            from: "deploy1".to_string(),
            to: "latency".to_string(),
            edge_type: EdgeType::MetricSpikedAfter,
            weight: 1.0,
        });

        let pattern = extract_causality_pattern(&graph);
        assert!(!pattern.is_empty());
    }

    #[test]
    fn test_graph_signature() {
        let mut graph = IncidentGraph::new();
        graph.add_node("a".to_string(), NodeType::Deploy);
        graph.add_node("b".to_string(), NodeType::Service);
        graph.add_node("c".to_string(), NodeType::Metric);

        let sig = compute_graph_signature(&graph);
        assert!(!sig.is_empty());
    }

    #[test]
    fn test_topology_independent_matching() {
        let mut graph1 = IncidentGraph::new();
        graph1.add_node("x".to_string(), NodeType::Deploy);
        graph1.add_node("y".to_string(), NodeType::Metric);

        let mut graph2 = IncidentGraph::new();
        graph2.add_node("p".to_string(), NodeType::Deploy);
        graph2.add_node("q".to_string(), NodeType::Metric);

        let sim = graph_相似度(&graph1, &graph2);
        assert!(sim > 0.9);
    }
}
