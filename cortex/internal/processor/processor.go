package processor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/eventconv"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
)

type EventProcessor struct {
	eventStore storage.EventStore
	topology   storage.TopologyStore
	graph      storage.GraphStore
	redactor   *Redactor
}

func NewEventProcessor(eventStore storage.EventStore, topology storage.TopologyStore, graph storage.GraphStore) *EventProcessor {
	return &EventProcessor{
		eventStore: eventStore,
		topology:   topology,
		graph:      graph,
	}
}

// WithRedactor adds PII redaction to the processor.
func (p *EventProcessor) WithRedactor(r *Redactor) *EventProcessor {
	p.redactor = r
	return p
}

func (p *EventProcessor) ProcessEvent(ctx context.Context, event *models.Event) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	normalized := p.normalizeEvent(event)

	if err := p.eventStore.Save(ctx, normalized); err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	if err := p.createGraphNodes(ctx, normalized); err != nil {
		return fmt.Errorf("failed to create graph nodes: %w", err)
	}

	return nil
}

func (p *EventProcessor) ProcessBatch(ctx context.Context, events []*models.Event) error {
	var validEvents []*models.Event
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("event validation failed for %s: %w", e.ID, err)
		}
		validEvents = append(validEvents, p.normalizeEvent(e))
	}

	if err := p.eventStore.SaveBatch(ctx, validEvents); err != nil {
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
			return fmt.Errorf("line %d: failed to parse JSON: %w", lineNum, err)
		}

		event, err := eventconv.FromRawMap(rawEvent, "jsonl")
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
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

// Redactor applies PII redaction to event data.
type Redactor struct {
	Mode string // "enforce" or "log"
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
		normalized.Raw = p.redactPII(normalized.Raw)
	}
	return &normalized
}

func (p *EventProcessor) redactPII(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(data))
	for k, v := range data {
		if s, ok := v.(string); ok {
			result[k] = p.redactString(k, s)
		} else if m, ok := v.(map[string]interface{}); ok {
			result[k] = p.redactPII(m)
		} else {
			result[k] = v
		}
	}
	return result
}

func (p *EventProcessor) redactString(key, value string) string {
	sensitive := []string{"password", "secret", "token", "api_key", "authorization", "credit_card", "ssn", "email"}
	lowerKey := key
	for _, s := range sensitive {
		if contains(lowerKey, s) {
			return "[REDACTED]"
		}
	}
	return value
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && (s[0] == sub[0] && contains(s[1:], sub[1:])))
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
					p.graph.SaveEdge(ctx, edge)
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
					p.graph.SaveEdge(ctx, edge)
				}
			}
		}
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

	for _, key := range []string{"message", "error", "level", "severity", "metric_name", "metric", "anomaly", "is_anomaly", "value", "status_code", "outcome", "route", "method", "deployment_id", "version"} {
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

// AsyncProcessor wraps EventProcessor with async batch ingestion for high throughput.
type AsyncProcessor struct {
	processor *EventProcessor
	ch        chan *models.Event
	batchSize int
	workers   int
	done      chan struct{}
	flushCh   chan chan struct{}
}

// NewAsyncProcessor creates an async ingestion pipeline.
func NewAsyncProcessor(processor *EventProcessor, workers, channelSize, batchSize int) *AsyncProcessor {
	if workers <= 0 {
		workers = 4
	}
	if channelSize <= 0 {
		channelSize = 8192
	}
	if batchSize <= 0 {
		batchSize = 256
	}


	ap := &AsyncProcessor{
		processor: processor,
		ch:        make(chan *models.Event, channelSize),
		batchSize: batchSize,
		workers:   workers,
		done:      make(chan struct{}),
		flushCh:   make(chan chan struct{}),
	}

	for i := 0; i < workers; i++ {
		go ap.worker()
	}

	return ap
}

// IngestChan returns the channel for fire-and-forget event submission.
func (ap *AsyncProcessor) IngestChan() chan<- *models.Event {
	return ap.ch
}

// Ingest sends an event to the async pipeline.
func (ap *AsyncProcessor) Ingest(event *models.Event) {
	ap.ch <- event
}

// Sync flushes all pending events and waits for completion.
func (ap *AsyncProcessor) Sync() error {
	ch := make(chan struct{})
	ap.flushCh <- ch
	<-ch
	return nil
}

// Stop gracefully shuts down the async processor.
func (ap *AsyncProcessor) Stop() {
	close(ap.done)
}

func (ap *AsyncProcessor) worker() {
	batch := make([]*models.Event, 0, ap.batchSize)
	flushTimer := time.NewTicker(500 * time.Millisecond)
	defer flushTimer.Stop()

	for {
		select {
		case <-ap.done:
			return
		case event := <-ap.ch:
			batch = append(batch, event)
			if len(batch) >= ap.batchSize {
				ap.flushBatch(batch)
				batch = batch[:0]
			}
		case <-flushTimer.C:
			if len(batch) > 0 {
				ap.flushBatch(batch)
				batch = batch[:0]
			}
		case syncCh := <-ap.flushCh:
			// Drain channel
			for {
				select {
				case event := <-ap.ch:
					batch = append(batch, event)
					if len(batch) >= ap.batchSize {
						ap.flushBatch(batch)
						batch = batch[:0]
					}
				default:
					goto done
				}
			}
		done:
			if len(batch) > 0 {
				ap.flushBatch(batch)
				batch = batch[:0]
			}
			close(syncCh)
		}
	}
}

func (ap *AsyncProcessor) flushBatch(batch []*models.Event) {
	if len(batch) == 0 {
		return
	}
	ctx := context.Background()
	_ = ap.processor.ProcessBatch(ctx, batch)
}
