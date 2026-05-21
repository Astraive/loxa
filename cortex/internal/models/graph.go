package models

import (
	"errors"
	"time"
)

type NodeType string

const (
	NodeTypeService    NodeType = "service"
	NodeTypeDeployment NodeType = "deployment"
	NodeTypeIncident  NodeType = "incident"
	NodeTypeMetric    NodeType = "metric"
	NodeTypeLog       NodeType = "log"
)

var validNodeTypes = map[NodeType]bool{
	NodeTypeService:    true,
	NodeTypeDeployment: true,
	NodeTypeIncident:  true,
	NodeTypeMetric:    true,
	NodeTypeLog:       true,
}

type Node struct {
	ID         string                 `json:"id" db:"id"`
	Type       NodeType               `json:"type" db:"type"`
	Label      string                 `json:"label" db:"label"`
	Attributes map[string]interface{} `json:"attributes" db:"attributes"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at" db:"updated_at"`
}

func (n *Node) Validate() error {
	if n.ID == "" {
		return errors.New("node ID cannot be empty")
	}
	if !validNodeTypes[n.Type] {
		return errors.New("invalid node type")
	}
	if n.Label == "" {
		return errors.New("node label cannot be empty")
	}
	return nil
}

type EdgeType string

const (
	EdgeTypeSameTrace        EdgeType = "same_trace"
	EdgeTypeSameIncident      EdgeType = "same_incident"
	EdgeTypeServiceRenamedTo  EdgeType = "service_renamed_to"
	EdgeTypeDependsOn        EdgeType = "depends_on"
	EdgeTypeDeployedBefore    EdgeType = "deployed_before"
	EdgeTypeMetricSpikedAfter EdgeType = "metric_spiked_after"
	EdgeTypeLogErrorAfter   EdgeType = "log_error_after"
	EdgeTypeCausedProbably EdgeType = "caused_probably"
	EdgeTypeRemediatedBy   EdgeType = "remediated_by"
	EdgeTypeSimilarShape  EdgeType = "similar_shape"
	EdgeTypeSameFamily   EdgeType = "same_family"
	EdgeTypeCoOccurred          EdgeType = "co_occurred"
	EdgeTypeDeploymentAdjacent  EdgeType = "deployment_adjacent"
	EdgeTypeRemediationSequence EdgeType = "remediation_sequence"
	EdgeTypeInferredCausality   EdgeType = "inferred_causality"
)

var validEdgeTypes = map[EdgeType]bool{
	EdgeTypeSameTrace:       true,
	EdgeTypeSameIncident:   true,
	EdgeTypeServiceRenamedTo: true,
	EdgeTypeDependsOn:     true,
	EdgeTypeDeployedBefore: true,
	EdgeTypeMetricSpikedAfter: true,
	EdgeTypeLogErrorAfter:  true,
	EdgeTypeCausedProbably: true,
	EdgeTypeRemediatedBy:   true,
	EdgeTypeSimilarShape:   true,
	EdgeTypeSameFamily:    true,
	EdgeTypeCoOccurred:         true,
	EdgeTypeDeploymentAdjacent:  true,
	EdgeTypeRemediationSequence: true,
	EdgeTypeInferredCausality:   true,
}

type Edge struct {
	ID         string                 `json:"id" db:"id"`
	FromNodeID string                 `json:"from_node_id" db:"from_node_id"`
	ToNodeID   string                 `json:"to_node_id" db:"to_node_id"`
	Type      EdgeType               `json:"type" db:"type"`
	Weight   float64                `json:"weight" db:"weight"`
	Attributes map[string]interface{} `json:"attributes" db:"attributes"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
}

func (e *Edge) Validate() error {
	if e.ID == "" {
		return errors.New("edge ID cannot be empty")
	}
	if e.FromNodeID == "" {
		return errors.New("from_node_id cannot be empty")
	}
	if e.ToNodeID == "" {
		return errors.New("to_node_id cannot be empty")
	}
	if !validEdgeTypes[e.Type] {
		return errors.New("invalid edge type")
	}
	if e.Weight < 0.0 || e.Weight > 1.0 {
		return errors.New("edge weight must be between 0.0 and 1.0")
	}
	return nil
}

type TraversalOptions struct {
	MaxDepth      int        `json:"max_depth"`
	EdgeTypes    []EdgeType `json:"edge_types,omitempty"`
	TimeWindowMin *time.Time `json:"time_window_min,omitempty"`
	TimeWindowMax *time.Time `json:"time_window_max,omitempty"`
}

type GraphView struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}