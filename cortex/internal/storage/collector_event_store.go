package storage

import (
	"context"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/collectorbridge"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
)

type collectorBackedEventStore struct {
	client *collectorbridge.Client
}

func newCollectorBackedEventStore(cfg config.CollectorConfig) EventStore {
	return &collectorBackedEventStore{client: collectorbridge.NewClient(cfg)}
}

func (s *collectorBackedEventStore) Save(ctx context.Context, event *models.Event, lifecycle *LifecycleData) error {
	// Collector-backed: events are stored in collector, not in cortex
	return nil
}

func (s *collectorBackedEventStore) SaveBatch(ctx context.Context, events []*models.Event, lifecycles []*LifecycleData) error {
	// Collector-backed: events are stored in collector, not in cortex
	return nil
}

func (s *collectorBackedEventStore) Get(ctx context.Context, id string) (*models.Event, error) {
	return s.client.GetByID(ctx, id)
}

func (s *collectorBackedEventStore) List(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	return s.client.ListRecent(ctx, limit)
}

func (s *collectorBackedEventStore) FindByTraceID(ctx context.Context, traceID string) ([]*models.Event, error) {
	return s.client.FindByTraceID(ctx, traceID, 1000)
}

func (s *collectorBackedEventStore) FindByIncidentID(ctx context.Context, incidentID string) ([]*models.Event, error) {
	return s.client.FindByIncidentID(ctx, incidentID, 1000)
}

func (s *collectorBackedEventStore) FindByService(ctx context.Context, service string, from, to string) ([]*models.Event, error) {
	return s.client.FindByService(ctx, service, from, to, 1000)
}

// Lifecycle-aware query methods (delegate to collector bridge)
func (s *collectorBackedEventStore) FindByEventName(ctx context.Context, eventName string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByEventName(ctx, eventName, limit)
}

func (s *collectorBackedEventStore) FindByOutcome(ctx context.Context, outcome string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByOutcome(ctx, outcome, limit)
}

func (s *collectorBackedEventStore) FindByLevel(ctx context.Context, level string, limit, offset int) ([]*models.Event, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit, offset int) ([]*models.Event, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) FindByEnvironment(ctx context.Context, env string, limit, offset int) ([]*models.Event, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) FindByRelease(ctx context.Context, release string, limit, offset int) ([]*models.Event, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error) {
	return 0, nil
}

func (s *collectorBackedEventStore) PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error) {
	return 0, nil
}

func (s *collectorBackedEventStore) DistinctServices(ctx context.Context) ([]string, error) {
	return s.client.DistinctServices(ctx)
}

func (s *collectorBackedEventStore) DistinctEventNames(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (s *collectorBackedEventStore) ListLifecycleSummaries(ctx context.Context, filter *LifecycleFilter) ([]*models.LifecycleSummary, int, error) {
	return nil, 0, nil
}

type storageWithExternalEvents struct {
	base   Storage
	events EventStore
}

func (s *storageWithExternalEvents) Init(ctx context.Context) error { return s.base.Init(ctx) }
func (s *storageWithExternalEvents) Close() error                   { return s.base.Close() }
func (s *storageWithExternalEvents) Events() EventStore             { return s.events }
func (s *storageWithExternalEvents) Topology() TopologyStore        { return s.base.Topology() }
func (s *storageWithExternalEvents) Graph() GraphStore              { return s.base.Graph() }
func (s *storageWithExternalEvents) Incidents() IncidentStore       { return s.base.Incidents() }
func (s *storageWithExternalEvents) Signatures() SignatureStore     { return s.base.Signatures() }
func (s *storageWithExternalEvents) Remediations() RemediationStore { return s.base.Remediations() }
func (s *storageWithExternalEvents) Feedback() FeedbackStore        { return s.base.Feedback() }
