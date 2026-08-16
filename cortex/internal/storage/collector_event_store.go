package storage

import (
	"context"
	"time"

	"github.com/astraive/loza/cortex/internal/collectorbridge"
	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/rs/zerolog/log"
)

type collectorBackedEventStore struct {
	client *collectorbridge.Client
}

func newCollectorBackedEventStore(cfg config.CollectorConfig) EventStore {
	return &collectorBackedEventStore{client: collectorbridge.NewClient(cfg)}
}

func (s *collectorBackedEventStore) Save(ctx context.Context, event *models.Event, lifecycle *LifecycleData) error {
	// Collector-backed: events are stored in collector, not in cortex
	log.Warn().Str("event_id", event.ID).Msg("Save called on collector-backed event store; event discarded (should be routed to collector)")
	return nil
}

func (s *collectorBackedEventStore) SaveBatch(ctx context.Context, events []*models.Event, lifecycles []*LifecycleData) error {
	// Collector-backed: events are stored in collector, not in cortex
	log.Warn().Int("count", len(events)).Msg("SaveBatch called on collector-backed event store; events discarded (should be routed to collector)")
	return nil
}

func (s *collectorBackedEventStore) Get(ctx context.Context, id string) (*models.Event, error) {
	return s.client.GetByID(ctx, id)
}

func (s *collectorBackedEventStore) List(ctx context.Context, limit, offset int) ([]*models.Event, error) {
	return s.client.ListRecentPage(ctx, limit, offset)
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

func (s *collectorBackedEventStore) FindByEventName(ctx context.Context, eventName string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByEventNamePage(ctx, eventName, limit, offset)
}

func (s *collectorBackedEventStore) FindByOutcome(ctx context.Context, outcome string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByOutcomePage(ctx, outcome, limit, offset)
}

func (s *collectorBackedEventStore) FindByLevel(ctx context.Context, level string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByLevelPage(ctx, level, limit, offset)
}

func (s *collectorBackedEventStore) FindByDurationRange(ctx context.Context, minMs, maxMs float64, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByDurationRangePage(ctx, minMs, maxMs, limit, offset)
}

func (s *collectorBackedEventStore) FindByEnvironment(ctx context.Context, env string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByEnvironmentPage(ctx, env, limit, offset)
}

func (s *collectorBackedEventStore) FindByRelease(ctx context.Context, release string, limit, offset int) ([]*models.Event, error) {
	return s.client.FindByReleasePage(ctx, release, limit, offset)
}

func (s *collectorBackedEventStore) CountByOutcome(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return s.client.CountByOutcome(ctx, service, from, to)
}

func (s *collectorBackedEventStore) CountByEventName(ctx context.Context, service string, from, to time.Time) (map[string]int64, error) {
	return s.client.CountByEventName(ctx, service, from, to)
}

func (s *collectorBackedEventStore) AverageDuration(ctx context.Context, eventName string, from, to time.Time) (float64, error) {
	return s.client.AverageDuration(ctx, eventName, from, to)
}

func (s *collectorBackedEventStore) PercentileDuration(ctx context.Context, eventName string, percentile float64, from, to time.Time) (float64, error) {
	return s.client.PercentileDuration(ctx, eventName, percentile, from, to)
}

func (s *collectorBackedEventStore) DistinctServices(ctx context.Context) ([]string, error) {
	return s.client.DistinctServices(ctx)
}

func (s *collectorBackedEventStore) DistinctEventNames(ctx context.Context) ([]string, error) {
	return s.client.DistinctEventNames(ctx)
}

func (s *collectorBackedEventStore) ListLifecycleSummaries(ctx context.Context, filter *LifecycleFilter) ([]*models.LifecycleSummary, int, error) {
	// Build a filter map for the bridge client
	filterMap := make(map[string]any)
	if filter != nil {
		if filter.Service != "" {
			filterMap["service"] = filter.Service
		}
		if filter.EventName != "" {
			filterMap["event_name"] = filter.EventName
		}
		if filter.Outcome != "" {
			filterMap["outcome"] = filter.Outcome
		}
	}
	rows, total, err := s.client.ListLifecycleSummaries(ctx, filterMap, 100, 0)
	if err != nil {
		return nil, 0, err
	}
	summaries := make([]*models.LifecycleSummary, 0, len(rows))
	for _, row := range rows {
		summary := &models.LifecycleSummary{}
		if id, ok := row["event_id"].(string); ok {
			summary.EventID = id
		} else if id, ok := row["id"].(string); ok {
			summary.EventID = id
		}
		if ev, ok := row["event"].(string); ok {
			summary.Event = ev
		}
		if svc, ok := row["service"].(string); ok {
			summary.Service = svc
		}
		if outcome, ok := row["outcome"].(string); ok {
			summary.Outcome = outcome
		}
		if dur, ok := row["duration_ms"].(float64); ok {
			summary.DurationMs = dur
		}
		summaries = append(summaries, summary)
	}
	return summaries, total, nil
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
