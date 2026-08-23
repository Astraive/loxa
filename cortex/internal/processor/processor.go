package processor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/astraive/loza/cortex/internal/eventconv"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/redaction"
	"github.com/astraive/loza/cortex/internal/storage"
	"github.com/rs/zerolog/log"
)

// ErrInvalidEvent classifies malformed event input independently from storage
// and processing failures.
var ErrInvalidEvent = errors.New("invalid event")

type EventProcessor struct {
	eventStore storage.EventStore
	topology   storage.TopologyStore
	graph      storage.GraphStore
	redactor   *redaction.Redactor
}

func NewEventProcessor(eventStore storage.EventStore, topology storage.TopologyStore, graph storage.GraphStore) *EventProcessor {
	return &EventProcessor{
		eventStore: eventStore,
		topology:   topology,
		graph:      graph,
	}
}

// WithRedactor adds PII redaction to the processor.
func (p *EventProcessor) WithRedactor(r *redaction.Redactor) *EventProcessor {
	p.redactor = r
	return p
}

// WithConfigurableRedaction creates a Redactor from config and attaches it.
func (p *EventProcessor) WithConfigurableRedaction(cfg redaction.Config) *EventProcessor {
	p.redactor = redaction.NewWithConfig(cfg)
	return p
}

func (p *EventProcessor) ProcessEvent(ctx context.Context, event *models.Event) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidEvent, err)
	}

	normalized := p.normalizeEvent(event)

	// Extract lifecycle primitives before saving
	lifecycle := p.extractLifecycle(normalized)

	// Save event with lifecycle data
	if err := p.eventStore.Save(ctx, normalized, lifecycle); err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	if err := p.createGraphNodes(ctx, normalized); err != nil {
		return fmt.Errorf("failed to create graph nodes: %w", err)
	}

	return nil
}

func (p *EventProcessor) ProcessBatch(ctx context.Context, events []*models.Event) error {
	var validEvents []*models.Event
	var lifecycles []*storage.LifecycleData

	for _, e := range events {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("%w %s: %w", ErrInvalidEvent, e.ID, err)
		}
		normalized := p.normalizeEvent(e)
		validEvents = append(validEvents, normalized)
		lifecycles = append(lifecycles, p.extractLifecycle(normalized))
	}

	if err := p.eventStore.SaveBatch(ctx, validEvents, lifecycles); err != nil {
		return fmt.Errorf("failed to save batch: %w", err)
	}

	if err := p.createGraphNodesBatch(ctx, validEvents); err != nil {
		return fmt.Errorf("failed to create graph nodes: %w", err)
	}

	return nil
}

func (p *EventProcessor) ProcessJSONL(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	lineNum := 0
	var events []*models.Event

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rawEvent map[string]interface{}
		if err := json.Unmarshal(line, &rawEvent); err != nil {
			return fmt.Errorf("%w at line %d: failed to parse JSON: %w", ErrInvalidEvent, lineNum, err)
		}

		event, err := eventconv.FromRawMap(rawEvent, "jsonl")
		if err != nil {
			return fmt.Errorf("%w at line %d: %w", ErrInvalidEvent, lineNum, err)
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	if len(events) > 0 {
		return p.ProcessBatch(ctx, events)
	}

	return nil
}

func (p *EventProcessor) normalizeEvent(event *models.Event) *models.Event {
	normalized := *event
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = time.Now()
	}
	if normalized.Raw == nil {
		normalized.Raw = make(map[string]interface{})
	}
	if p.redactor != nil {
		normalized.Raw = p.redactor.RedactMap(normalized.Raw)
	}
	return &normalized
}

// extractLifecycle extracts lifecycle primitives from the event for indexing
func (p *EventProcessor) extractLifecycle(event *models.Event) *storage.LifecycleData {
	return &storage.LifecycleData{
		EventID:         event.ID,
		EventName:       event.Event,
		Service:         event.Service,
		Outcome:         event.Outcome,
		DurationMs:      event.DurationMs,
		TraceID:         event.TraceID,
		SpanID:          event.SpanID,
		Level:           event.Level,
		Environment:     event.Environment,
		Release:         event.Release,
		CheckpointCount: len(event.Checkpoints),
		ProcessCount:    len(event.Processes),
		GroupCount:      len(event.Groups),
		TimerCount:      len(event.Timers),
		LinkCount:       len(event.Links),
		Checkpoints:     event.Checkpoints,
		Processes:       event.Processes,
		Groups:          event.Groups,
		Timers:          event.Timers,
		Links:           event.Links,
		Attrs:           event.Attrs,
	}
}

func (p *EventProcessor) createLinkEdges(ctx context.Context, event *models.Event) {
	for _, link := range event.Links {
		edge := &models.Edge{
			ID:         fmt.Sprintf("%s->%s:%s", event.ID, link.Target, link.Type),
			FromNodeID: event.ID,
			ToNodeID:   link.Target,
			Type:       models.EdgeType(link.Type),
			Weight:     1.0,
			CreatedAt:  time.Now(),
		}
		if saveErr := p.graph.SaveEdge(ctx, edge); saveErr != nil {
			log.Warn().Err(saveErr).Str("edge_id", edge.ID).Msg("failed to save link edge")
		}
	}
}

func (p *EventProcessor) createGraphNodes(ctx context.Context, event *models.Event) error {
	canonicalService := event.Service

	if alias, err := p.topology.GetAlias(ctx, event.Service, event.Timestamp.Format(time.RFC3339)); err == nil {
		canonicalService = alias.Canonical
	}

	if err := p.saveCoreGraphArtifacts(ctx, event, canonicalService); err != nil {
		return err
	}
	if event.TraceID != "" {
		relatedEvents, err := p.eventStore.FindByTraceID(ctx, event.TraceID)
		if err == nil {
			for _, related := range relatedEvents {
				if related.ID != event.ID {
					edge := &models.Edge{
						ID:         fmt.Sprintf("%s->%s", event.ID, related.ID),
						FromNodeID: event.ID,
						ToNodeID:   related.ID,
						Type:       models.EdgeTypeSameTrace,
						Weight:     1.0,
						CreatedAt:  time.Now(),
					}
					if saveErr := p.graph.SaveEdge(ctx, edge); saveErr != nil {
						log.Warn().Err(saveErr).Str("edge_id", edge.ID).Msg("failed to save trace edge")
					}
				}
			}
		}
	}

	if event.IncidentID != "" {
		relatedEvents, err := p.eventStore.FindByIncidentID(ctx, event.IncidentID)
		if err == nil {
			for _, related := range relatedEvents {
				if related.ID != event.ID {
					edge := &models.Edge{
						ID:         fmt.Sprintf("%s->%s", event.ID, related.ID),
						FromNodeID: event.ID,
						ToNodeID:   related.ID,
						Type:       models.EdgeTypeSameIncident,
						Weight:     1.0,
						CreatedAt:  time.Now(),
					}
					if saveErr := p.graph.SaveEdge(ctx, edge); saveErr != nil {
						log.Warn().Err(saveErr).Str("edge_id", edge.ID).Msg("failed to save incident edge")
					}
				}
			}
		}
	}

	if event.Links != nil {
		p.createLinkEdges(ctx, event)
	}

	return nil
}

func (p *EventProcessor) createGraphNodesBatch(ctx context.Context, events []*models.Event) error {
	canonicalServices := make(map[string]string, len(events))
	traceGroups := make(map[string][]*models.Event)
	incidentGroups := make(map[string][]*models.Event)
	seenEdges := make(map[string]struct{})

	for _, event := range events {
		canonicalService := event.Service
		if alias, err := p.topology.GetAlias(ctx, event.Service, event.Timestamp.Format(time.RFC3339)); err == nil {
			canonicalService = alias.Canonical
		}
		canonicalServices[event.ID] = canonicalService

		if err := p.saveCoreGraphArtifacts(ctx, event, canonicalService); err != nil {
			return err
		}

		if event.TraceID != "" {
			traceGroups[event.TraceID] = append(traceGroups[event.TraceID], event)
		}
		if event.IncidentID != "" {
			incidentGroups[event.IncidentID] = append(incidentGroups[event.IncidentID], event)
		}

		if event.Links != nil {
			p.createLinkEdges(ctx, event)
		}
	}

	for traceID, group := range traceGroups {
		if err := p.linkGroupedEvents(ctx, group, seenEdges, models.EdgeTypeSameTrace); err != nil {
			return err
		}
		if len(group) <= 1 {
			relatedEvents, err := p.eventStore.FindByTraceID(ctx, traceID)
			if err == nil {
				for _, event := range group {
					if err := p.linkExternalEvents(ctx, event, relatedEvents, seenEdges, models.EdgeTypeSameTrace); err != nil {
						return err
					}
				}
			}
		}
	}

	for incidentID, group := range incidentGroups {
		if err := p.linkGroupedEvents(ctx, group, seenEdges, models.EdgeTypeSameIncident); err != nil {
			return err
		}
		if len(group) <= 1 {
			relatedEvents, err := p.eventStore.FindByIncidentID(ctx, incidentID)
			if err == nil {
				for _, event := range group {
					if err := p.linkExternalEvents(ctx, event, relatedEvents, seenEdges, models.EdgeTypeSameIncident); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (p *EventProcessor) saveCoreGraphArtifacts(ctx context.Context, event *models.Event, canonicalService string) error {
	serviceNode := &models.Node{
		ID:        canonicalService,
		Type:      models.NodeTypeService,
		Label:     canonicalService,
		CreatedAt: time.Now(),
	}
	if err := p.graph.SaveNode(ctx, serviceNode); err != nil {
		return fmt.Errorf("failed to save service node: %w", err)
	}

	nodeType := models.NodeTypeLog
	if event.Kind == models.EventKindMetric {
		nodeType = models.NodeTypeMetric
	} else if event.Kind == models.EventKindDeploy {
		nodeType = models.NodeTypeDeployment
	}

	eventNode := &models.Node{
		ID:         event.ID,
		Type:       nodeType,
		Label:      string(event.Kind),
		Attributes: graphAttributesForEvent(event),
		CreatedAt:  event.Timestamp,
	}
	if err := p.graph.SaveNode(ctx, eventNode); err != nil {
		return fmt.Errorf("failed to save event node: %w", err)
	}

	if err := p.graph.SaveEdge(ctx, &models.Edge{
		ID:         fmt.Sprintf("%s->%s", event.ID, canonicalService),
		FromNodeID: event.ID,
		ToNodeID:   canonicalService,
		Type:       models.EdgeTypeDependsOn,
		Weight:     1.0,
		CreatedAt:  time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to save event-service edge: %w", err)
	}

	if event.Kind == models.EventKindDeploy {
		deployNode := &models.Node{
			ID:         fmt.Sprintf("%s-deploy", event.ID),
			Type:       models.NodeTypeDeployment,
			Label:      "deployment",
			Attributes: graphAttributesForEvent(event),
			CreatedAt:  event.Timestamp,
		}
		if err := p.graph.SaveNode(ctx, deployNode); err != nil {
			return fmt.Errorf("failed to save deployment node: %w", err)
		}

		if err := p.graph.SaveEdge(ctx, &models.Edge{
			ID:         fmt.Sprintf("%s->%s", deployNode.ID, canonicalService),
			FromNodeID: deployNode.ID,
			ToNodeID:   canonicalService,
			Type:       models.EdgeTypeDeployedBefore,
			Weight:     1.0,
			CreatedAt:  time.Now(),
		}); err != nil {
			return fmt.Errorf("failed to save deployment edge: %w", err)
		}
	}

	return nil
}

func graphAttributesForEvent(event *models.Event) map[string]any {
	attrs := map[string]any{
		"service": event.Service,
	}
	if event.TraceID != "" {
		attrs["trace_id"] = event.TraceID
	}
	if event.IncidentID != "" {
		attrs["incident_id"] = event.IncidentID
	}
	if event.Event != "" {
		attrs["event"] = event.Event
	}
	if event.Outcome != "" {
		attrs["outcome"] = event.Outcome
	}
	if event.DurationMs > 0 {
		attrs["duration_ms"] = event.DurationMs
	}
	if event.Level != "" {
		attrs["level"] = event.Level
	}

	for _, key := range []string{"message", "error", "severity", "metric_name", "anomaly", "is_anomaly", "value", "status_code", "route", "method", "deployment_id", "version"} {
		if value, ok := event.Raw[key]; ok {
			attrs[key] = value
		}
	}
	return attrs
}

func (p *EventProcessor) linkGroupedEvents(ctx context.Context, events []*models.Event, seen map[string]struct{}, edgeType models.EdgeType) error {
	if len(events) < 2 {
		return nil
	}

	sorted := append([]*models.Event(nil), events...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			edgeID := fmt.Sprintf("%s->%s:%s", sorted[i].ID, sorted[j].ID, edgeType)
			if _, ok := seen[edgeID]; ok {
				continue
			}
			seen[edgeID] = struct{}{}
			if err := p.graph.SaveEdge(ctx, &models.Edge{
				ID:         edgeID,
				FromNodeID: sorted[i].ID,
				ToNodeID:   sorted[j].ID,
				Type:       edgeType,
				Weight:     1.0,
				CreatedAt:  time.Now(),
			}); err != nil {
				return fmt.Errorf("failed to save grouped edge: %w", err)
			}
		}
	}
	return nil
}

func (p *EventProcessor) linkExternalEvents(ctx context.Context, event *models.Event, related []*models.Event, seen map[string]struct{}, edgeType models.EdgeType) error {
	for _, other := range related {
		if other == nil || other.ID == event.ID {
			continue
		}
		edgeID := fmt.Sprintf("%s->%s:%s", event.ID, other.ID, edgeType)
		if _, ok := seen[edgeID]; ok {
			continue
		}
		seen[edgeID] = struct{}{}
		if err := p.graph.SaveEdge(ctx, &models.Edge{
			ID:         edgeID,
			FromNodeID: event.ID,
			ToNodeID:   other.ID,
			Type:       edgeType,
			Weight:     1.0,
			CreatedAt:  time.Now(),
		}); err != nil {
			return fmt.Errorf("failed to save external edge: %w", err)
		}
	}
	return nil
}
