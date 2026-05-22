package httpbatch

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/astraive/loxa/sdks/go"
)

// Config controls HTTP batch sink behavior.
type Config struct {
	URL           string
	Method        string
	Headers       map[string]string
	BatchSize     int
	FlushInterval time.Duration
	Gzip          bool
	Client        *http.Client
}

type sink struct {
	cfg       Config
	mu        sync.Mutex
	buf       [][]byte
	timer     *time.Ticker
	stop      chan struct{}
	closeOnce sync.Once
}

// New creates an HTTP batch sink.
func New(cfg Config) (loxa.Sink, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("httpbatch: URL is required")
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 5 * time.Second}
	}

	s := &sink{
		cfg:   cfg,
		buf:   make([][]byte, 0, cfg.BatchSize),
		timer: time.NewTicker(cfg.FlushInterval),
		stop:  make(chan struct{}),
	}
	go s.loop()
	return s, nil
}

func (s *sink) Name() string { return "httpbatch" }

func (s *sink) WriteEvent(_ context.Context, encoded []byte, _ *loxa.Event) error {
	s.mu.Lock()
	cp := make([]byte, len(encoded))
	copy(cp, encoded)
	s.buf = append(s.buf, cp)
	flush := len(s.buf) >= s.cfg.BatchSize
	s.mu.Unlock()
	if flush {
		return s.Flush(context.Background())
	}
	return nil
}

func (s *sink) WriteBatch(ctx context.Context, encoded [][]byte, _ []*loxa.Event) error {
	if len(encoded) == 0 {
		return nil
	}
	payload := bytes.Join(encoded, nil)
	return s.send(ctx, payload)
}

func (s *sink) Flush(ctx context.Context) error {
	s.mu.Lock()
	if len(s.buf) == 0 {
		s.mu.Unlock()
		return nil
	}
	payload := bytes.Join(s.buf, nil)
	s.buf = s.buf[:0]
	s.mu.Unlock()
	return s.send(ctx, payload)
}

func (s *sink) send(ctx context.Context, payload []byte) error {
	body := payload
	if s.cfg.Gzip {
		var b bytes.Buffer
		zw := gzip.NewWriter(&b)
		if _, err := zw.Write(payload); err != nil {
			return err
		}
		if err := zw.Close(); err != nil {
			return err
		}
		body = b.Bytes()
	}

	req, err := http.NewRequestWithContext(ctx, s.cfg.Method, s.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if s.cfg.Gzip {
		req.Header.Set("Content-Encoding", "gzip")
	}
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("httpbatch: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (s *sink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.timer.Stop()
	})
	return s.Flush(ctx)
}

func (s *sink) loop() {
	for {
		select {
		case <-s.timer.C:
			_ = s.Flush(context.Background())
		case <-s.stop:
			return
		}
	}
}
