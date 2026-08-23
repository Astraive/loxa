package storage

import (
	"context"
	"errors"
	"time"

	"github.com/astraive/loza/cortex/internal/models"
)

var ErrNotFound = errors.New("storage record not found")

// LifecycleData contains the extracted lifecycle primitives for indexing
type LifecycleData struct {
	EventID         string
	EventName       string
	Service         string
	Outcome         string
	DurationMs      float64
	TraceID         string
	SpanID          string
	Level           string
	Environment     string
	Release         string
	CheckpointCount int
	ProcessCount    int
	GroupCount      int
	TimerCount      int
	LinkCount       int
	Checkpoints     []*models.EventCheckpoint
	Processes       []*models.EventProcess
	Groups          []*models.EventGroup
	Timers          []*models.EventTimer
	Links           []*models.EventLink
	Attrs           map[string]interface{}
}

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
	// Core CRUD
	Save(ctx context.Context, event *models.Event, lifecycle *LifecycleData) error
	SaveBatch(ctx context.Context, events []*models.Event, lifecycles []*LifecycleData) error
	Get(ctx context.Context, id string) (*models.Event, error)
	List(ctx context.Context, limit, offset int) ([]*models.Event, error)

	// Trace and incident queries
	FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error)
	FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error)
	FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error)

	// Lifecycle-aware queries
	FindByEventName(ctx context.Context, eventName string, limit int, offset int) ([]*models.Event, error)
	FindByOutcome(ctx context.Context, outcome string, limit int, offset int) ([]*models.Event, error)
	FindByLevel(ctx context.Context, level string, limit int, offset int) ([]*models.Event, error)
	FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit int, offset int) ([]*models.Event, error)
	FindByEnvironment(ctx context.Context, env string, limit int, offset int) ([]*models.Event, error)
	FindByRelease(ctx context.Context, release string, limit int, offset int) ([]*models.Event, error)

	// Aggregate queries
	CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error)
	CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error)
	AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error)
	PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error)
	DistinctServices(ctx context.Context) ([]string, error)
	DistinctEventNames(ctx context.Context) ([]string, error)

	// Lifecycle summary
	ListLifecycleSummaries(ctx context.Context, filter *LifecycleFilter) ([]*models.LifecycleSummary, int, error)
}

// LifecycleFilter provides filtering for lifecycle-aware queries
type LifecycleFilter struct {
	Service        string
	EventName      string
	Outcome        string
	Level          string
	Environment    string
	TraceID        string
	From           time.Time
	To             time.Time
	Limit          int
	Offset         int
	MinDuration    float64
	MaxDuration    float64
	HasCheckpoints *bool
	HasProcesses   *bool
	HasGroups      *bool
	HasTimers      *bool
	HasLinks       *bool
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
