package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	loxav1 "github.com/astraive/loxa/spec/proto/loxa/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type cortexBridgeClient struct {
	cfg       collectorConfig
	metrics   *collectorMetrics
	conn      *grpc.ClientConn
	client    loxav1.CortexServiceClient
	queue     chan []byte
	wg        sync.WaitGroup
	closeOnce sync.Once
}

func newCortexBridgeClient(cfg collectorConfig, metrics *collectorMetrics) (*cortexBridgeClient, error) {
	if !cfg.cortexBridgeEnabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.cortexBridgeEndpoint) == "" {
		return nil, errors.New("cortex bridge endpoint is empty")
	}

	var creds credentials.TransportCredentials
	if cfg.cortexBridgeInsecure {
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}

	conn, err := grpc.NewClient(cfg.cortexBridgeEndpoint, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}

	bridge := &cortexBridgeClient{
		cfg:     cfg,
		metrics: metrics,
		conn:    conn,
		client:  loxav1.NewCortexServiceClient(conn),
	}
	if bridge.mode() == "async" {
		queueSize := cfg.cortexBridgeQueueSize
		if queueSize <= 0 {
			queueSize = 4096
		}
		bridge.queue = make(chan []byte, queueSize)
		bridge.wg.Add(1)
		go bridge.run()
	}
	return bridge, nil
}

func (c *cortexBridgeClient) mode() string {
	if strings.TrimSpace(c.cfg.cortexBridgeMode) == "" {
		return "async"
	}
	return c.cfg.cortexBridgeMode
}

func (c *cortexBridgeClient) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		if c.queue != nil {
			close(c.queue)
		}
		c.wg.Wait()
		if c.conn != nil {
			_ = c.conn.Close()
		}
	})
	return nil
}

func (c *cortexBridgeClient) Deliver(ctx context.Context, raws [][]byte) error {
	if c == nil || len(raws) == 0 {
		return nil
	}
	if c.mode() == "sync" {
		return c.sendBatch(ctx, raws)
	}
	for _, raw := range raws {
		cp := append([]byte(nil), raw...)
		select {
		case c.queue <- cp:
			if c.metrics != nil {
				c.metrics.cortexBridgeQueueDepth.Add(1)
			}
		default:
			if c.metrics != nil {
				c.metrics.cortexBridgeErrors.Add(1)
			}
			return errors.New("cortex bridge queue full")
		}
	}
	return nil
}

func (c *cortexBridgeClient) run() {
	defer c.wg.Done()
	flushEvery := c.cfg.cortexBridgeFlushIntvl
	if flushEvery <= 0 {
		flushEvery = 500 * time.Millisecond
	}
	batchSize := c.cfg.cortexBridgeBatchSize
	if batchSize <= 0 {
		batchSize = 128
	}
	ticker := time.NewTicker(flushEvery)
	defer ticker.Stop()

	batch := make([][]byte, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		timeout := c.cfg.cortexBridgeTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := c.sendBatch(ctx, batch); err != nil {
			if c.metrics != nil {
				c.metrics.cortexBridgeErrors.Add(1)
			}
			logJSON("warn", "collector_cortex_bridge_flush_failed", map[string]any{"error": err.Error(), "count": len(batch)})
		} else if c.metrics != nil {
			c.metrics.cortexBridgeFlushes.Add(1)
			c.metrics.cortexBridgeEvents.Add(int64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case raw, ok := <-c.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, raw)
			if c.metrics != nil {
				c.metrics.cortexBridgeQueueDepth.Add(-1)
			}
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (c *cortexBridgeClient) sendBatch(ctx context.Context, raws [][]byte) error {
	req := &loxav1.IngestBatchRequest{Events: make([]*loxav1.Event, 0, len(raws))}
	for _, raw := range raws {
		event, err := rawToCortexEvent(raw)
		if err != nil {
			logJSON("warn", "collector_cortex_bridge_decode_failed", map[string]any{"error": err.Error()})
			continue
		}
		req.Events = append(req.Events, event.Event)
	}
	if len(req.Events) == 0 {
		return nil
	}
	if c.cfg.cortexBridgeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.cortexBridgeTimeout)
		defer cancel()
	}
	if strings.TrimSpace(c.cfg.cortexBridgeAPIKey) != "" && strings.TrimSpace(c.cfg.cortexBridgeHeader) != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, c.cfg.cortexBridgeHeader, c.cfg.cortexBridgeAPIKey)
	}
	_, err := c.client.IngestBatch(ctx, req)
	if err == nil && c.metrics != nil && c.mode() == "sync" {
		c.metrics.cortexBridgeFlushes.Add(1)
		c.metrics.cortexBridgeEvents.Add(int64(len(req.Events)))
	}
	return err
}

func rawToCortexEvent(raw []byte) (*loxav1.IngestEventRequest, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if eventType, ok := payload["event_type"]; ok {
		if _, has := payload["event"]; !has {
			payload["event"] = eventType
		}
	}

	pe := &loxav1.Event{
		EventId: stringValue(payload["event_id"]),
		Service: stringValue(payload["service"]),
		Release: stringValue(payload["release"]),
		TraceId: stringValue(payload["trace_id"]),
		SpanId:  stringValue(payload["span_id"]),
	}

	kind := strings.ToLower(stringValue(payload["kind"]))
	switch kind {
	case "log":
		pe.Kind = loxav1.EventKind_EVENT_KIND_LOG
	case "ingest", "metric", "span", "trace":
		pe.Kind = loxav1.EventKind_EVENT_KIND_EVENT
	default:
		pe.Kind = loxav1.EventKind_EVENT_KIND_EVENT
	}

	if evt, ok := payload["event"]; ok {
		if s, ok := evt.(string); ok {
			pe.Event = s
		}
	}

	if ts := parseTimestampValue(payload["timestamp"]); ts != nil {
		pe.Timestamp = ts
	} else {
		pe.Timestamp = timestamppb.Now()
	}

	// Store remaining raw payload as attrs
	rawStruct, err := structpb.NewStruct(payload)
	if err == nil {
		pe.Attrs = rawStruct
	}

	return &loxav1.IngestEventRequest{
		Event: pe,
	}, nil
}

func parseTimestampValue(v any) *timestamppb.Timestamp {
	raw := stringValue(v)
	if raw == "" {
		return nil
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return timestamppb.New(ts)
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return timestamppb.New(ts)
	}
	return nil
}

func stringValue(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func (s *collectorState) forwardAcceptedToCortex(ctx context.Context, raws [][]byte) {
	if s == nil || s.cortexBridge == nil || len(raws) == 0 {
		return
	}
	if err := s.cortexBridge.Deliver(ctx, raws); err != nil {
		logJSON("warn", "collector_cortex_bridge_delivery_failed", map[string]any{"error": err.Error(), "count": len(raws), "mode": s.cfg.cortexBridgeMode})
	}
}
