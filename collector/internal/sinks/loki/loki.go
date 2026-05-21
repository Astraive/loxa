package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	collectorevent "github.com/astraive/loxa-collector/internal/event"
)

// Config controls Loki sink behavior.
type Config struct {
	URL string
	// TenantID sets X-Scope-OrgID for multi-tenant Loki deployments.
	TenantID string
	// Labels are attached to each pushed Loki stream.
	Labels map[string]string
	// Headers are added to each Loki push request.
	Headers       map[string]string
	BatchSize     int
	FlushInterval time.Duration
	Timeout       time.Duration
	Client        *http.Client
}

type sink struct {
	cfg Config

	mu        sync.Mutex
	batch     []lokiEntry
	closed    bool
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

type lokiEntry struct {
	Timestamp string
	Line      string
}

type lokiPushPayload struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

var labelKeyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// New creates a Loki sink.
func New(cfg Config) (collectorevent.Sink, error) {
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return nil, fmt.Errorf("loki: URL is required")
	}
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("loki: invalid URL: %w", err)
	}
	if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return nil, fmt.Errorf("loki: URL must include scheme and host")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.Timeout < 0 {
		return nil, fmt.Errorf("loki: timeout must be >= 0")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: cfg.Timeout}
	}
	cfg.Labels = normalizedLabels(cfg.Labels)
	if len(cfg.Labels) == 0 {
		cfg.Labels = map[string]string{"source": "loxa"}
	}
	cfg.Headers = copyStringMap(cfg.Headers)

	s := &sink{
		cfg:   cfg,
		batch: make([]lokiEntry, 0, cfg.BatchSize),
		stop:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.flushLoop()
	return s, nil
}

func (s *sink) Name() string { return "loki" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, ev *collectorevent.Event) error {
	line := strings.TrimSpace(string(encoded))
	if line == "" {
		return fmt.Errorf("loki: empty encoded event")
	}

	ts := time.Now().UTC()
	if ev != nil && !ev.Timestamp.IsZero() {
		ts = ev.Timestamp.UTC()
	}
	entry := lokiEntry{
		Timestamp: fmt.Sprintf("%d", ts.UnixNano()),
		Line:      line,
	}

	batch, err := s.appendEntry(entry)
	if err != nil {
		return err
	}
	if len(batch) == 0 {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.pushBatch(ctx, batch); err != nil {
		s.restore(batch)
		return err
	}
	return nil
}

func (s *sink) Flush(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	batch := s.drain()
	if len(batch) == 0 {
		return nil
	}
	if err := s.pushBatch(ctx, batch); err != nil {
		s.restore(batch)
		return err
	}
	return nil
}

func (s *sink) Close(ctx context.Context) error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.stop)
		s.wg.Wait()
		closeErr = s.Flush(ctx)
	})
	return closeErr
}

func (s *sink) flushLoop() {
	defer s.wg.Done()
	t := time.NewTicker(s.cfg.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = s.Flush(context.Background())
		case <-s.stop:
			return
		}
	}
}

func (s *sink) appendEntry(entry lokiEntry) ([]lokiEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fmt.Errorf("loki: sink closed")
	}
	s.batch = append(s.batch, entry)
	if len(s.batch) < s.cfg.BatchSize {
		return nil, nil
	}
	batch := make([]lokiEntry, len(s.batch))
	copy(batch, s.batch)
	s.batch = s.batch[:0]
	return batch, nil
}

func (s *sink) drain() []lokiEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batch) == 0 {
		return nil
	}
	batch := make([]lokiEntry, len(s.batch))
	copy(batch, s.batch)
	s.batch = s.batch[:0]
	return batch
}

func (s *sink) restore(batch []lokiEntry) {
	if len(batch) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batch) > 0 {
		s.batch = append(append(make([]lokiEntry, 0, len(batch)+len(s.batch)), batch...), s.batch...)
	} else {
		s.batch = append(make([]lokiEntry, 0, len(batch)), batch...)
	}
}

func (s *sink) pushBatch(ctx context.Context, batch []lokiEntry) error {
	if len(batch) == 0 {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
		}

		err := s.doPushBatch(ctx, batch)
		if err == nil {
			return nil
		}
		lastErr = err

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("loki: push batch failed after 3 attempts: %w", lastErr)
}

func (s *sink) doPushBatch(ctx context.Context, batch []lokiEntry) error {
	values := make([][]string, 0, len(batch))
	for _, entry := range batch {
		values = append(values, []string{entry.Timestamp, entry.Line})
	}

	payload := lokiPushPayload{
		Streams: []lokiStream{
			{
				Stream: copyStringMap(s.cfg.Labels),
				Values: values,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("loki: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("loki: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.cfg.TenantID) != "" {
		req.Header.Set("X-Scope-OrgID", s.cfg.TenantID)
	}
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("loki: push batch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if trimmed := strings.TrimSpace(string(msg)); trimmed != "" {
			return fmt.Errorf("loki: unexpected status %d: %s", resp.StatusCode, trimmed)
		}
		return fmt.Errorf("loki: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func normalizedLabels(labels map[string]string) map[string]string {
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		key := strings.TrimSpace(k)
		if !labelKeyPattern.MatchString(key) {
			continue
		}
		out[key] = strings.TrimSpace(v)
	}
	return out
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
