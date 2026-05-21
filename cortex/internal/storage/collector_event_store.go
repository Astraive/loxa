package storage

import (
	"context"

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

func (s *collectorBackedEventStore) Save(context.Context, *models.Event) error {
	return nil
}

func (s *collectorBackedEventStore) SaveBatch(context.Context, []*models.Event) error {
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
