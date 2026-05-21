package main

import (
	"context"
	"math/rand"
	"strings"
	"time"

	processing "github.com/astraive/loxa-collector/internal/processing"
)

func (s *collectorState) isReady() bool {
	return s.ready.Load() && s.effectiveSinkHealthy() && s.effectiveSpoolHealthy() && s.effectiveDiskHealthy()
}

func (s *collectorState) effectiveSinkHealthy() bool {
	if s.sinkHealthy.Load() {
		return true
	}
	return s.metrics.sinkWriteErrors.Load() == 0
}

func (s *collectorState) effectiveSpoolHealthy() bool {
	if s.cfg.reliabilityMode != "spool" {
		return true
	}
	if s.spoolHealthy.Load() {
		return true
	}
	return s.metrics.spoolBytes.Load() == 0
}

func (s *collectorState) effectiveDiskHealthy() bool {
	if s.diskHealthy.Load() {
		return true
	}
	return s.metrics.sinkWriteErrors.Load() == 0
}

func (s *collectorState) ensureProcessor() error {
	s.processorMu.Lock()
	defer s.processorMu.Unlock()
	if s.processor != nil {
		return nil
	}
	if s.rng == nil {
		s.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	processor, err := processing.New(processing.Config{
		DeliveryPolicy:          s.cfg.deliveryPolicy,
		RetryEnabled:            s.cfg.retryEnabled,
		RetryMaxAttempts:        s.cfg.retryMaxAttempts,
		RetryInitialBackoff:     s.cfg.retryInitialBackoff,
		RetryMaxBackoff:         s.cfg.retryMaxBackoff,
		RetryJitter:             s.cfg.retryJitter,
		FallbackEnabled:         s.cfg.fallbackEnabled,
		FallbackOnPrimaryFail:   s.cfg.fallbackOnPrimaryFail,
		FallbackOnSecondaryFail: s.cfg.fallbackOnSecondaryFail,
		FallbackOnPolicyFail:    s.cfg.fallbackOnPolicyFail,
		DLQEnabled:              s.cfg.dlqEnabled,
		DLQPath:                 s.cfg.dlqPath,
		DLQOnPrimaryFail:        s.cfg.dlqOnPrimaryFail,
		DLQOnSecondaryFail:      s.cfg.dlqOnSecondaryFail,
		DLQOnFallbackFail:       s.cfg.dlqOnFallbackFail,
		DLQOnPolicyFail:         s.cfg.dlqOnPolicyFail,
		DedupeEnabled:           s.dedupeEnabled(),
		DedupeKey:               s.cfg.dedupeKey,
		DedupeWindow:            s.cfg.dedupeWindow,
		OnDiskFull: func() {
			s.diskHealthy.Store(false)
		},
		Schema: processing.SchemaConfig{
			Mode:           schemaModeForProcessor(s),
			SchemaVersion:  s.cfg.schemaSchemaVersion,
			EventVersion:   s.cfg.schemaEventVersion,
			QuarantinePath: s.cfg.schemaQuarantinePath,
			Registry:       s.convertSchemaRegistry(),
		},
		Privacy: processing.PrivacyConfig{
			Mode:       s.cfg.privacyMode,
			Blocklist:  append([]string(nil), s.cfg.privacyBlocklist...),
			Allowlist:  append([]string(nil), s.cfg.privacyAllowlist...),
			SecretScan: s.cfg.secretScan,
		},
		LogFunc: func(level string, message string, fields map[string]any) {
			logJSON(level, message, fields)
		},
	}, s.ingestSink, s.secondarySinks, s.fallbackSink, s.rng)
	if err != nil {
		return err
	}
	s.processor = processor
	return nil
}

func schemaModeForProcessor(s *collectorState) string {
	if !s.schemaValidationEnabled() {
		return "off"
	}
	return s.cfg.schemaMode
}

func (s *collectorState) convertSchemaRegistry() []processing.SchemaRegistryEntry {
	if len(s.cfg.schemaRegistry) == 0 {
		return nil
	}
	out := make([]processing.SchemaRegistryEntry, len(s.cfg.schemaRegistry))
	for i, e := range s.cfg.schemaRegistry {
		out[i] = processing.SchemaRegistryEntry{
			SchemaVersion:  e.SchemaVersion,
			EventVersion:   e.EventVersion,
			RequiredFields: e.RequiredFields,
		}
	}
	return out
}

func (s *collectorState) isDuplicate(value string) (bool, error) {
	if strings.TrimSpace(value) == "" {
		return false, nil
	}
	if s.dedupeStore != nil {
		seen, err := s.dedupeStore.SeenBefore(context.Background(), value, s.cfg.dedupeWindow)
		if err != nil {
			return false, err
		}
		return seen, nil
	}
	s.dedupeMu.Lock()
	defer s.dedupeMu.Unlock()
	now := time.Now()
	for k, ts := range s.dedupeSeenAt {
		if now.Sub(ts) > s.cfg.dedupeWindow {
			delete(s.dedupeSeenAt, k)
		}
	}
	if ts, ok := s.dedupeSeenAt[value]; ok && now.Sub(ts) <= s.cfg.dedupeWindow {
		return true, nil
	}
	s.dedupeSeenAt[value] = now
	return false, nil
}
