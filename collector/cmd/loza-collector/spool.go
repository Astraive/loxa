package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxSpoolReplayRecordBytes = 16 * 1024 * 1024

var (
	errSpoolCapacity         = errors.New("spool capacity exceeded")
	errDeliveryQueueCapacity = errors.New("delivery queue bytes exceeded")
)

type spoolDelivery struct {
	payload     []byte
	startOffset int64
	endOffset   int64
	skip        bool
}

func (d spoolDelivery) storedBytes() int64 {
	return d.endOffset - d.startOffset
}

func (s *collectorState) initReliability() error {
	if s.cfg.reliabilityMode != "spool" && s.cfg.reliabilityMode != "hybrid" {
		return nil
	}
	s.reliabilityCtx, s.reliabilityCancel = context.WithCancel(context.Background())
	if err := os.MkdirAll(s.cfg.spoolDir, 0o755); err != nil {
		return fmt.Errorf("mkdir spool dir: %w", err)
	}

	spoolPath := filepath.Join(s.cfg.spoolDir, s.cfg.spoolFile)
	f, err := os.OpenFile(spoolPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open spool file: %w", err)
	}
	s.spoolFile = f

	posFilePath := spoolPath + ".pos"
	s.spoolPosFile = posFilePath
	s.spoolBadFile = spoolPath + ".bad.ndjson"

	if err := s.loadSpoolPosition(); err != nil {
		logJSON("warn", "spool_position_load_failed", map[string]any{"error": err.Error()})
		s.spoolProcessedPos = 0
	}

	if st, err := f.Stat(); err == nil {
		currentSize := st.Size()
		s.metrics.spoolBytes.Store(currentSize)
		if currentSize > s.cfg.maxSpoolBytes {
			s.spoolHealthy.Store(false)
		} else if s.spoolProcessedPos >= currentSize {
			s.metrics.spoolBytes.Store(0)
			s.spoolHealthy.Store(true)
		} else {
			s.spoolHealthy.Store(true)
		}
	}

	s.deliveryQueue = make(chan spoolDelivery, s.cfg.deliveryQueueSize)
	s.deliverySpace = make(chan struct{}, 1)
	s.deliveryWG.Add(1)
	go s.deliveryWorker()

	if err := s.replaySpool(); err != nil {
		logJSON("error", "spool_replay_failed", map[string]any{"error": err.Error()})
	}

	return nil
}

func (s *collectorState) loadSpoolPosition() error {
	posData, err := os.ReadFile(s.spoolPosFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var pos struct {
		ProcessedPos int64 `json:"processed_pos"`
		EventCount   int64 `json:"event_count"`
	}
	if err := json.Unmarshal(posData, &pos); err != nil {
		return err
	}
	s.spoolProcessedPos = pos.ProcessedPos
	s.metrics.spoolReplayCount.Store(pos.EventCount)
	return nil
}

func (s *collectorState) saveSpoolPosition() error {
	if s.spoolPosFile == "" {
		return nil
	}
	pos := struct {
		ProcessedPos int64 `json:"processed_pos"`
		EventCount   int64 `json:"event_count"`
	}{
		ProcessedPos: s.spoolProcessedPos,
		EventCount:   s.metrics.spoolReplayCount.Load(),
	}
	data, err := json.Marshal(pos)
	if err != nil {
		return err
	}
	tmpPath := s.spoolPosFile + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	f, err := os.OpenFile(tmpPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_ = os.Remove(s.spoolPosFile)
	return os.Rename(tmpPath, s.spoolPosFile)
}

func (s *collectorState) closeReliability() {
	s.closeOnce.Do(func() {
		if s.deliveryQueue != nil {
			s.spoolMu.Lock()
			close(s.deliveryQueue)
			s.spoolMu.Unlock()
			s.deliveryWG.Wait()
			s.deliveryQueue = nil
			s.deliverySpace = nil
		}
		if s.reliabilityCancel != nil {
			s.reliabilityCancel()
			s.reliabilityCancel = nil
		}
		if s.spoolFile != nil {
			if err := s.spoolFile.Close(); err != nil {
				logJSON("error", "spool_close_failed", map[string]any{"error": err.Error()})
			}
			s.spoolFile = nil
		}
		if s.processor != nil {
			if err := s.processor.Close(); err != nil {
				logJSON("error", "processor_close_failed", map[string]any{"error": err.Error()})
			}
			s.processor = nil
		}
	})
}

func (s *collectorState) appendSpool(raw []byte) (spoolDelivery, error) {
	s.spoolMu.Lock()
	defer s.spoolMu.Unlock()
	return s.appendSpoolLocked(raw)
}

func (s *collectorState) appendAndEnqueueSpool(raw []byte) error {
	if err := s.validateDeliveryPayload(raw); err != nil {
		return err
	}

	s.spoolMu.Lock()
	defer s.spoolMu.Unlock()

	item, err := s.appendSpoolLocked(raw)
	if err != nil {
		return err
	}
	if err := s.enqueueDelivery(item); err != nil {
		return fmt.Errorf("enqueue durable spool record: %w", err)
	}
	return nil
}

func (s *collectorState) appendSpoolLocked(raw []byte) (spoolDelivery, error) {
	if s.spoolFile == nil {
		return spoolDelivery{}, errors.New("spool file is not initialized")
	}

	record := append([]byte(nil), raw...)
	if encryptionEnabled(s.cfg.storageEncryptionKey) {
		var err error
		record, err = encryptBlob(record, s.cfg.storageEncryptionKey)
		if err != nil {
			return spoolDelivery{}, err
		}
	}
	stored := append(record, '\n')

	startOffset, err := s.spoolFile.Seek(0, io.SeekEnd)
	if err != nil {
		return spoolDelivery{}, err
	}
	unprocessedBytes := startOffset - s.spoolProcessedPos
	if unprocessedBytes < 0 {
		unprocessedBytes = startOffset
	}
	projectedBytes := unprocessedBytes + int64(len(stored))
	if s.cfg.maxSpoolBytes > 0 && projectedBytes > s.cfg.maxSpoolBytes {
		return spoolDelivery{}, fmt.Errorf(
			"%w: current=%d record=%d max=%d",
			errSpoolCapacity,
			unprocessedBytes,
			len(stored),
			s.cfg.maxSpoolBytes,
		)
	}

	n, err := s.spoolFile.Write(stored)
	if err != nil {
		_ = s.spoolFile.Truncate(startOffset)
		return spoolDelivery{}, err
	}
	if n != len(stored) {
		_ = s.spoolFile.Truncate(startOffset)
		return spoolDelivery{}, io.ErrShortWrite
	}
	if s.cfg.spoolFsync {
		if err := s.spoolFile.Sync(); err != nil {
			_ = s.spoolFile.Truncate(startOffset)
			_, _ = s.spoolFile.Seek(0, io.SeekEnd)
			return spoolDelivery{}, err
		}
	}

	s.metrics.spoolBytes.Store(projectedBytes)
	s.spoolHealthy.Store(true)
	return spoolDelivery{
		payload:     append([]byte(nil), raw...),
		startOffset: startOffset,
		endOffset:   startOffset + int64(n),
	}, nil
}

func (s *collectorState) validateDeliveryPayload(raw []byte) error {
	if !s.memoryLimiterEnabled() || s.cfg.maxQueueBytes <= 0 || int64(len(raw)) <= s.cfg.maxQueueBytes {
		return nil
	}
	s.metrics.requestsThrottled.Add(1)
	s.metrics.sinkWriteErrors.Add(1)
	logJSON("warn", "delivery_queue_bytes_exceeded", map[string]any{
		"event_bytes": len(raw),
		"queue_bytes": s.metrics.queueBytes.Load(),
		"max_bytes":   s.cfg.maxQueueBytes,
	})
	err := fmt.Errorf("%w: event=%d max=%d", errDeliveryQueueCapacity, len(raw), s.cfg.maxQueueBytes)
	s.maybeWriteDLQ(raw, err)
	return err
}

func (s *collectorState) enqueueDelivery(item spoolDelivery) error {
	item.payload = append([]byte(nil), item.payload...)
	if err := s.validateDeliveryPayloadForQueue(item); err != nil {
		return err
	}
	if s.deliveryQueue == nil {
		return errors.New("delivery queue is not initialized")
	}

	size := int64(len(item.payload))
	ctx := s.reliabilityCtx
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		current := s.metrics.queueBytes.Load()
		withinLimit := !s.memoryLimiterEnabled() ||
			s.cfg.maxQueueBytes <= 0 ||
			current+size <= s.cfg.maxQueueBytes
		durableOversize := item.endOffset > item.startOffset && current == 0
		if withinLimit || durableOversize {
			if s.metrics.queueBytes.CompareAndSwap(current, current+size) {
				break
			}
			continue
		}

		select {
		case <-s.deliverySpace:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	select {
	case s.deliveryQueue <- item:
		return nil
	case <-ctx.Done():
		s.releaseDeliveryBytes(size)
		return ctx.Err()
	}
}

func (s *collectorState) validateDeliveryPayloadForQueue(item spoolDelivery) error {
	if item.endOffset > item.startOffset {
		return nil
	}
	return s.validateDeliveryPayload(item.payload)
}

func (s *collectorState) releaseDeliveryBytes(size int64) {
	s.metrics.queueBytes.Add(-size)
	if s.deliverySpace == nil {
		return
	}
	select {
	case s.deliverySpace <- struct{}{}:
	default:
	}
}

func (s *collectorState) deliveryWorker() {
	defer s.deliveryWG.Done()
	for item := range s.deliveryQueue {
		s.releaseDeliveryBytes(int64(len(item.payload)))
		if item.skip {
			s.markSpoolDelivered(item)
			continue
		}
		s.processSpoolEvent(item)
	}
}

func (s *collectorState) processSpoolEvent(item spoolDelivery) {
	raw := item.payload
	if err := s.ensureProcessor(); err != nil {
		s.metrics.sinkWriteErrors.Add(1)
		s.sinkHealthy.Store(false)
		logJSON("error", "collector_pipeline_not_initialized", map[string]any{"error": err.Error()})
		// DLQ fallback for pipeline init failure
		s.maybeWriteDLQ(raw, err)
		return
	}

	ctx := s.reliabilityCtx
	if ctx == nil {
		ctx = context.Background()
	}
	s.processorMu.RLock()
	result := s.processor.Process(ctx, raw)
	s.processorMu.RUnlock()
	if failures := result.Outcome.FailureCount(); failures > 0 {
		s.metrics.sinkWriteErrors.Add(int64(failures))
	}

	if result.Err != nil {
		s.sinkHealthy.Store(false)
		logJSON("error", "spool_delivery_failed", map[string]any{"error": result.Err.Error()})
		s.maybeWriteDLQ(raw, result.Err)
		return
	}

	s.sinkHealthy.Store(true)
	s.markSpoolDelivered(item)
}

func (s *collectorState) maybeWriteDLQ(raw []byte, err error) {
	s.processorMu.RLock()
	proc := s.processor
	s.processorMu.RUnlock()
	if proc == nil {
		if initErr := s.ensureProcessor(); initErr != nil {
			logJSON("error", "collector_dlq_processor_not_initialized", map[string]any{"error": initErr.Error()})
			return
		}
		s.processorMu.RLock()
		proc = s.processor
		s.processorMu.RUnlock()
	}
	proc.WriteDLQ(raw, err)
}

func (s *collectorState) markSpoolDelivered(item spoolDelivery) {
	if item.endOffset <= item.startOffset {
		return
	}

	s.spoolMu.Lock()
	defer s.spoolMu.Unlock()

	if s.spoolFile == nil {
		return
	}
	if item.startOffset != s.spoolProcessedPos {
		logJSON("warn", "spool_checkpoint_gap", map[string]any{
			"expected_offset": s.spoolProcessedPos,
			"record_start":    item.startOffset,
			"record_end":      item.endOffset,
		})
		return
	}

	currentSize, err := s.spoolFile.Seek(0, io.SeekEnd)
	if err != nil {
		logJSON("error", "spool_truncate_seek_failed", map[string]any{"error": err.Error()})
		return
	}
	if item.endOffset > currentSize {
		logJSON("error", "spool_checkpoint_out_of_bounds", map[string]any{
			"record_end":   item.endOffset,
			"current_size": currentSize,
		})
		return
	}

	s.spoolProcessedPos = item.endOffset
	if s.spoolProcessedPos < currentSize {
		s.metrics.spoolBytes.Store(currentSize - s.spoolProcessedPos)
		if err := s.saveSpoolPosition(); err != nil {
			logJSON("error", "spool_position_save_failed", map[string]any{"error": err.Error()})
		}
		return
	}

	if err := s.spoolFile.Truncate(0); err != nil {
		logJSON("error", "spool_truncate_failed", map[string]any{"error": err.Error()})
		return
	}
	if _, err := s.spoolFile.Seek(0, io.SeekStart); err != nil {
		logJSON("error", "spool_rewind_failed", map[string]any{"error": err.Error()})
		return
	}
	s.spoolProcessedPos = 0
	s.metrics.spoolBytes.Store(0)
	if err := s.saveSpoolPosition(); err != nil {
		logJSON("error", "spool_position_save_failed", map[string]any{"error": err.Error()})
	}
}

func (s *collectorState) replaySpool() error {
	if s.spoolFile == nil {
		return nil
	}

	fileInfo, err := s.spoolFile.Stat()
	if err != nil {
		return err
	}
	currentSize := fileInfo.Size()

	if s.spoolProcessedPos >= currentSize {
		if currentSize > 0 {
			s.spoolMu.Lock()
			if err := s.spoolFile.Truncate(0); err == nil {
				_, _ = s.spoolFile.Seek(0, io.SeekStart)
				s.spoolProcessedPos = 0
				_ = s.saveSpoolPosition()
			}
			s.spoolMu.Unlock()
		}
		s.metrics.spoolBytes.Store(0)
		logJSON("info", "spool_already_processed", map[string]any{
			"processed_pos": s.spoolProcessedPos,
			"current_size":  currentSize,
		})
		return nil
	}

	if s.spoolProcessedPos > 0 {
		if _, err := s.spoolFile.Seek(s.spoolProcessedPos, io.SeekStart); err != nil {
			return err
		}
	} else {
		if _, err := s.spoolFile.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	replayStart := s.spoolProcessedPos
	nextOffset := replayStart
	replayCount := int64(0)
	skippedCount := int64(0)
	sc := bufio.NewScanner(s.spoolFile)
	maxRecordBytes := int(s.cfg.maxEventBytes)
	if maxRecordBytes <= 0 || maxRecordBytes > maxSpoolReplayRecordBytes {
		maxRecordBytes = maxSpoolReplayRecordBytes
	}
	buf := make([]byte, 0, min(maxRecordBytes, 1024*1024))
	sc.Buffer(buf, maxRecordBytes)

	for sc.Scan() {
		rawLine := sc.Bytes()
		storedBytes := int64(len(rawLine) + 1)
		if remaining := currentSize - nextOffset; storedBytes > remaining {
			storedBytes = remaining
		}
		item := spoolDelivery{
			startOffset: nextOffset,
			endOffset:   nextOffset + storedBytes,
		}
		nextOffset = item.endOffset

		line := bytes.TrimSpace(rawLine)
		if len(line) == 0 {
			item.skip = true
			if err := s.enqueueDelivery(item); err != nil {
				return fmt.Errorf("enqueue skipped spool record: %w", err)
			}
			skippedCount++
			continue
		}

		decoded := append([]byte(nil), line...)
		if encryptionEnabled(s.cfg.storageEncryptionKey) {
			if plain, err := decryptBlob(decoded, s.cfg.storageEncryptionKey); err == nil {
				decoded = plain
			}
		}
		if !json.Valid(decoded) {
			s.quarantineBadSpoolLine(decoded)
			item.skip = true
			if err := s.enqueueDelivery(item); err != nil {
				return fmt.Errorf("enqueue invalid spool record: %w", err)
			}
			skippedCount++
			continue
		}

		item.payload = decoded
		if err := s.enqueueDelivery(item); err != nil {
			return fmt.Errorf("enqueue spool replay: %w", err)
		}
		replayCount++
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan spool replay: %w", err)
	}

	totalReplayCount := s.metrics.spoolReplayCount.Add(replayCount)
	logJSON("info", "spool_replay_completed", map[string]any{
		"replayed":    replayCount,
		"skipped":     skippedCount,
		"from_pos":    replayStart,
		"total_count": totalReplayCount,
	})

	_, _ = s.spoolFile.Seek(0, io.SeekEnd)
	return nil
}

func (s *collectorState) quarantineBadSpoolLine(raw []byte) {
	if s.spoolBadFile == "" || len(bytes.TrimSpace(raw)) == 0 {
		return
	}
	entry := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"reason":    "invalid_spool_record",
		"raw":       string(raw),
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		logJSON("error", "spool_quarantine_encode_failed", map[string]any{"error": err.Error()})
		return
	}
	f, err := os.OpenFile(s.spoolBadFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		logJSON("error", "spool_quarantine_open_failed", map[string]any{"error": err.Error()})
		return
	}
	defer f.Close()
	if _, err := f.Write(append(encoded, '\n')); err != nil {
		logJSON("error", "spool_quarantine_write_failed", map[string]any{"error": err.Error()})
	}
}
