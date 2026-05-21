package storage

import (
	"context"

	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

type Storage interface {
	Init(ctx context.Context) error
	Close() error

	Events() EventStore
	Topology() TopologyStore
	Graph() GraphStore
	Incidents() IncidentStore
	Signatures() SignatureStore
	Remediations() RemediationStore
	Feedback() FeedbackStore
}

type EventStore interface {
	Save(ctx context.Context, event *models.Event) error
	SaveBatch(ctx context.Context, events []*models.Event) error
	Get(ctx context.Context, id string) (*models.Event, error)
	List(ctx context.Context, limit, offset int) ([]*models.Event, error)
	FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error)
	FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error)
	FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error)
}

type TopologyStore interface {
	SaveAlias(ctx context.Context, alias *models.ServiceAlias) error
	GetAlias(ctx context.Context, alias string, timestamp string) (*models.ServiceAlias, error)
	GetHistory(ctx context.Context, service string) ([]*models.ServiceAlias, error)
}

type GraphStore interface {
	SaveNode(ctx context.Context, node *models.Node) error
	GetNode(ctx context.Context, id string) (*models.Node, error)
	ListNodes(ctx context.Context, nodeType string, limit int) ([]*models.Node, error)

	SaveEdge(ctx context.Context, edge *models.Edge) error
	GetEdges(ctx context.Context, nodeID string, edgeType string) ([]*models.Edge, error)
	Traverse(ctx context.Context, startNodeID string, opts models.TraversalOptions) (*models.GraphView, error)
}

type IncidentStore interface {
	Save(ctx context.Context, incident *models.Incident) error
	Get(ctx context.Context, id string) (*models.Incident, error)
	GetBySignature(ctx context.Context, signatureID string) (*models.Incident, error)
	List(ctx context.Context, limit, offset int) ([]*models.Incident, error)
}

type SignatureStore interface {
	Save(ctx context.Context, sig *models.IncidentSignature) error
	Get(ctx context.Context, id string) (*models.IncidentSignature, error)
	List(ctx context.Context, limit int) ([]*models.IncidentSignature, error)
	FindSimilar(ctx context.Context, sig *models.IncidentSignature, limit int) ([]*models.SimilarIncident, error)
	FindByBehavioralHash(ctx context.Context, hash string) ([]*models.IncidentSignature, error)
	UpdateDecay(ctx context.Context, signatureID string, factor float64) error
	ArchiveStale(ctx context.Context, threshold float64) (int, error)
	UpdateLastMatched(ctx context.Context, signatureID string) error
}

type SalienceStore interface {
	Save(ctx context.Context, score *models.SalienceScore) error
	Get(ctx context.Context, eventType string) (float64, error)
	List(ctx context.Context, limit int) ([]*models.SalienceScore, error)
}

type RemediationStore interface {
	Save(ctx context.Context, rem *models.Remediation) error
	Get(ctx context.Context, id string) (*models.Remediation, error)
	ListByIncident(ctx context.Context, incidentID string) ([]*models.Remediation, error)
	ListBySignature(ctx context.Context, signatureID string, limit int) ([]*models.RemediationStats, error)
}

type FeedbackStore interface {
	Save(ctx context.Context, fb *models.RemediationFeedback) error
	GetByRemediation(ctx context.Context, remediationID string) ([]*models.RemediationFeedback, error)
	GetSuccessRate(ctx context.Context, action string, signatureID string) (float64, error)
}