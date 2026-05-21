package main

import (
	"strings"
)

func (s *collectorState) processorEnabled(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, candidate := range s.cfg.processorRegistry {
		if strings.EqualFold(strings.TrimSpace(candidate), name) {
			return true
		}
	}
	return false
}

func (s *collectorState) schemaValidationEnabled() bool {
	return s.processorEnabled("validate")
}

func (s *collectorState) redactionEnabled() bool {
	return s.processorEnabled("redact")
}

func (s *collectorState) identityEnabled() bool {
	return s.processorEnabled("identity") || s.processorEnabled("tenant_resolve") || s.processorEnabled("enrich")
}

func (s *collectorState) dedupeEnabled() bool {
	return s.cfg.dedupeEnabled && s.processorEnabled("dedupe")
}

func (s *collectorState) limitsEnabled() bool {
	return s.processorEnabled("size_limit") || s.processorEnabled("cardinality_limit")
}

func (s *collectorState) memoryLimiterEnabled() bool {
	return s.processorEnabled("memory_limiter")
}

func (s *collectorState) acquireIngestCapacity(eventCount int, totalBytes int64) (*governanceError, func()) {
	if !s.memoryLimiterEnabled() {
		return nil, func() {}
	}

	if s.cfg.maxInflightRequests > 0 && s.metrics.inflightRequests.Load() >= int64(s.cfg.maxInflightRequests) {
		return &governanceError{
			Code:      "backpressure",
			Message:   "collector max_inflight_requests exceeded",
			Retryable: true,
		}, nil
	}
	if s.cfg.maxInflightEvents > 0 && s.metrics.inflightEvents.Load()+int64(eventCount) > int64(s.cfg.maxInflightEvents) {
		return &governanceError{
			Code:      "backpressure",
			Message:   "collector max_inflight_events exceeded",
			Retryable: true,
		}, nil
	}
	if s.cfg.maxQueueBytes > 0 && s.metrics.queueBytes.Load()+totalBytes > s.cfg.maxQueueBytes {
		return &governanceError{
			Code:      "queue_full",
			Message:   "collector max_queue_bytes exceeded",
			Retryable: true,
		}, nil
	}

	s.metrics.inflightRequests.Add(1)
	s.metrics.inflightEvents.Add(int64(eventCount))
	return nil, func() {
		s.metrics.inflightRequests.Add(-1)
		s.metrics.inflightEvents.Add(-int64(eventCount))
	}
}
