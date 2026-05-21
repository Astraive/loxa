package collectorsync

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/collectorbridge"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	cortexmetrics "github.com/astraive/loxa/loxa-cortex/internal/metrics"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/rs/zerolog/log"
)

type BatchProcessor interface {
	ProcessBatch(ctx context.Context, events []*models.Event) error
}

type cursorState struct {
	mu     sync.RWMutex
	cursor collectorbridge.Cursor
}

func (s *cursorState) Current() collectorbridge.Cursor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cursor
}

func (s *cursorState) Update(cur collectorbridge.Cursor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursor = cur
}

func RunSourceOfTruthSync(ctx context.Context, cfg config.CollectorConfig, proc BatchProcessor) {
	client := collectorbridge.NewClient(cfg)
	cursor, err := client.LoadCursor()
	if err != nil {
		log.Warn().Err(err).Str("cursor_path", cfg.CursorPath).Msg("Failed to load collector cursor; starting from zero cursor")
	}
	state := &cursorState{cursor: cursor}

	for {
		if ctx.Err() != nil {
			return
		}

		if err := runPollCatchup(ctx, cfg, client, proc, state); err != nil {
			if ctx.Err() != nil {
				return
			}
			cortexmetrics.CollectorBridgeReconnects.WithLabelValues(normalizedTailTransport(cfg), "poll_error").Inc()
			log.Warn().Err(err).Str("collector_url", cfg.URL).Msg("Collector catch-up poll failed")
			if !sleepWithContext(ctx, normalizedReconnectBackoff(cfg)) {
				return
			}
			continue
		}

		if !cfg.TailEnabled {
			if !sleepWithContext(ctx, normalizedPollInterval(cfg)) {
				return
			}
			continue
		}

		err := runTailSession(ctx, cfg, client, proc, state)
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			cortexmetrics.CollectorBridgeReconnects.WithLabelValues(normalizedTailTransport(cfg), "tail_error").Inc()
			log.Warn().Err(err).Str("collector_url", cfg.URL).Msg("Collector tail stream interrupted; reverting to poll catch-up")
		} else {
			cortexmetrics.CollectorBridgeReconnects.WithLabelValues(normalizedTailTransport(cfg), "tail_closed").Inc()
			log.Warn().Str("collector_url", cfg.URL).Msg("Collector tail stream closed; reverting to poll catch-up")
		}
		if !sleepWithContext(ctx, normalizedReconnectBackoff(cfg)) {
			return
		}
	}
}

func runPollCatchup(ctx context.Context, cfg config.CollectorConfig, client *collectorbridge.Client, proc BatchProcessor, state *cursorState) error {
	limit := cfg.BatchSize
	if limit <= 0 {
		limit = 1000
	}

	for {
		current := state.Current()
		events, next, err := client.FetchEventsSince(ctx, current, limit)
		if err != nil {
			return err
		}
		events = filterEventsAfterCursor(events, current)
		if len(events) == 0 {
			return nil
		}
		if err := flushBatch(ctx, client, proc, state, events); err != nil {
			return err
		}
		if sameCursor(next, current) || len(events) < limit {
			return nil
		}
	}
}

func runTailSession(ctx context.Context, cfg config.CollectorConfig, client *collectorbridge.Client, proc BatchProcessor, state *cursorState) error {
	batchSize := normalizedTailBatchSize(cfg)
	flushInterval := normalizedTailFlushInterval(cfg)
	bufferSize := normalizedTailBufferSize(cfg)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventsCh := make(chan *models.Event, bufferSize)
	errCh := make(chan error, 1)

	go func() {
		errCh <- client.StreamTail(streamCtx, func(event *models.Event) error {
			if !isEventAfterCursor(event, state.Current()) {
				return nil
			}
			select {
			case eventsCh <- event:
				return nil
			default:
				cortexmetrics.CollectorBridgeBackpressure.Inc()
			}
			select {
			case eventsCh <- event:
				return nil
			case <-streamCtx.Done():
				return streamCtx.Err()
			}
		})
	}()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []*models.Event
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-eventsCh:
			if event == nil {
				continue
			}
			batch = append(batch, event)
			if len(batch) >= batchSize {
				if err := flushBatch(ctx, client, proc, state, batch); err != nil {
					return err
				}
				batch = nil
			}
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			if err := flushBatch(ctx, client, proc, state, batch); err != nil {
				return err
			}
			batch = nil
		case err := <-errCh:
			if len(batch) > 0 {
				if flushErr := flushBatch(ctx, client, proc, state, batch); flushErr != nil {
					return flushErr
				}
			}
			if err == nil {
				return fmt.Errorf("collector tail stream closed")
			}
			return err
		}
	}
}

func flushBatch(ctx context.Context, client *collectorbridge.Client, proc BatchProcessor, state *cursorState, events []*models.Event) error {
	ordered := sortAndDedupeEvents(events, state.Current())
	if len(ordered) == 0 {
		return nil
	}
	cortexmetrics.CollectorBridgeBatchSize.Observe(float64(len(ordered)))
	if err := proc.ProcessBatch(ctx, ordered); err != nil {
		cortexmetrics.CollectorBridgeFlushes.WithLabelValues("error").Inc()
		return err
	}
	next := cursorForEvents(ordered)
	if err := client.SaveCursor(next); err != nil {
		cortexmetrics.CollectorBridgeFlushes.WithLabelValues("cursor_error").Inc()
		return fmt.Errorf("save collector cursor: %w", err)
	}
	state.Update(next)
	cortexmetrics.CollectorBridgeFlushes.WithLabelValues("success").Inc()
	if !next.Timestamp.IsZero() {
		cortexmetrics.CollectorBridgeLagSeconds.Set(time.Since(next.Timestamp).Seconds())
	}
	return nil
}

func filterEventsAfterCursor(events []*models.Event, cursor collectorbridge.Cursor) []*models.Event {
	filtered := make([]*models.Event, 0, len(events))
	for _, event := range events {
		if isEventAfterCursor(event, cursor) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func sortAndDedupeEvents(events []*models.Event, cursor collectorbridge.Cursor) []*models.Event {
	filtered := filterEventsAfterCursor(events, cursor)
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Timestamp.Equal(filtered[j].Timestamp) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	unique := make([]*models.Event, 0, len(filtered))
	seen := make(map[string]struct{}, len(filtered))
	for _, event := range filtered {
		key := event.ID
		if key == "" {
			key = event.Timestamp.UTC().Format(time.RFC3339Nano)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, event)
	}
	return unique
}

func cursorForEvents(events []*models.Event) collectorbridge.Cursor {
	var current collectorbridge.Cursor
	for _, event := range events {
		if event == nil {
			continue
		}
		candidate := collectorbridge.Cursor{Timestamp: event.Timestamp, EventID: event.ID}
		if cursorLess(current, candidate) {
			current = candidate
		}
	}
	return current
}

func isEventAfterCursor(event *models.Event, cursor collectorbridge.Cursor) bool {
	if event == nil {
		return false
	}
	candidate := collectorbridge.Cursor{Timestamp: event.Timestamp, EventID: event.ID}
	return cursorLess(cursor, candidate)
}

func cursorLess(a, b collectorbridge.Cursor) bool {
	if a.Timestamp.IsZero() {
		return !b.Timestamp.IsZero() || stringsGreaterThanEmpty(b.EventID)
	}
	if b.Timestamp.IsZero() {
		return false
	}
	if a.Timestamp.Before(b.Timestamp) {
		return true
	}
	if a.Timestamp.After(b.Timestamp) {
		return false
	}
	return a.EventID < b.EventID
}

func sameCursor(a, b collectorbridge.Cursor) bool {
	return a.Timestamp.Equal(b.Timestamp) && a.EventID == b.EventID
}

func normalizedPollInterval(cfg config.CollectorConfig) time.Duration {
	if cfg.PollInterval > 0 {
		return cfg.PollInterval
	}
	return 30 * time.Second
}

func normalizedTailBufferSize(cfg config.CollectorConfig) int {
	if cfg.TailBufferSize > 0 {
		return cfg.TailBufferSize
	}
	return 1
}

func normalizedTailBatchSize(cfg config.CollectorConfig) int {
	if cfg.TailBatchSize > 0 {
		return cfg.TailBatchSize
	}
	if cfg.BatchSize > 0 {
		return cfg.BatchSize
	}
	return 1
}

func normalizedTailFlushInterval(cfg config.CollectorConfig) time.Duration {
	if cfg.TailFlushInterval > 0 {
		return cfg.TailFlushInterval
	}
	return 500 * time.Millisecond
}

func normalizedReconnectBackoff(cfg config.CollectorConfig) time.Duration {
	if cfg.TailReconnectBackoff > 0 {
		return cfg.TailReconnectBackoff
	}
	return 2 * time.Second
}

func normalizedTailTransport(cfg config.CollectorConfig) string {
	if cfg.TailTransport == "websocket" {
		return "websocket"
	}
	return "http"
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func stringsGreaterThanEmpty(v string) bool {
	return v != ""
}
