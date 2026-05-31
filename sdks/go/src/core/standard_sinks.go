package core

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	speccontract "github.com/astraive/loxa/spec/generated/go/contract"
)

// ── stdout / stderr sinks ─────────────────────────────────────────────────────

type writerSink struct {
	name string
	mu   sync.Mutex
	w    io.Writer
	bw   *bufio.Writer
}

func newWriterSink(name string, w io.Writer) *writerSink {
	return &writerSink{
		name: name,
		w:    w,
		bw:   bufio.NewWriterSize(w, 32*1024),
	}
}

func (s *writerSink) Name() string { return s.name }

func (s *writerSink) WriteEvent(_ context.Context, encoded []byte, _ *Event) error {
	s.mu.Lock()
	_, err := s.bw.Write(encoded)
	s.mu.Unlock()
	return err
}

func (s *writerSink) Flush(_ context.Context) error {
	s.mu.Lock()
	err := s.bw.Flush()
	s.mu.Unlock()
	return err
}

func (s *writerSink) Close(ctx context.Context) error { return s.Flush(ctx) }

// StdoutSink returns a Sink that writes NDJSON to os.Stdout.
func StdoutSink() Sink { return newWriterSink("stdout", os.Stdout) }

// StderrSink returns a Sink that writes NDJSON to os.Stderr.
func StderrSink() Sink { return newWriterSink("stderr", os.Stderr) }

// ── file sink ─────────────────────────────────────────────────────────────────

type fileSink struct {
	writerSink
	f *os.File
}

// FileSink returns a Sink that appends NDJSON to the file at path.
func FileSink(path string) (Sink, error) {
	// #nosec G304
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("loxa: open file sink %q: %w", path, err)
	}
	s := &fileSink{f: f}
	s.writerSink = *newWriterSink("file:"+path, f)
	return s, nil
}

func (s *fileSink) Close(ctx context.Context) error {
	if err := s.Flush(ctx); err != nil {
		return err
	}
	return s.f.Close()
}

// ── rotating file sink ────────────────────────────────────────────────────────

// RotatingFileConfig configures the rotating file sink.
type RotatingFileConfig struct {
	Path     string
	MaxBytes int64
	MaxAge   time.Duration
}

type rotatingFileSink struct {
	mu      sync.Mutex
	cfg     RotatingFileConfig
	f       *os.File
	written int64
	opened  time.Time
}

// RotatingFileSink returns a Sink that rotates log files.
func RotatingFileSink(cfg RotatingFileConfig) (Sink, error) {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 100 * 1024 * 1024
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 24 * time.Hour
	}
	s := &rotatingFileSink{cfg: cfg}
	if err := s.open(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *rotatingFileSink) open() error {
	// #nosec G304
	f, err := os.OpenFile(s.cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("loxa: open rotating file %q: %w", s.cfg.Path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("loxa: stat rotating file %q: %w", s.cfg.Path, err)
	}
	s.f = f
	s.opened = time.Now()
	s.written = info.Size()
	return nil
}

func (s *rotatingFileSink) Name() string { return "rotatingfile:" + s.cfg.Path }

func (s *rotatingFileSink) WriteEvent(_ context.Context, encoded []byte, _ *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.written >= s.cfg.MaxBytes || time.Since(s.opened) >= s.cfg.MaxAge {
		if err := s.rotate(); err != nil {
			return err
		}
	}
	n, err := s.f.Write(encoded)
	s.written += int64(n)
	return err
}

func (s *rotatingFileSink) rotate() error {
	if s.f == nil {
		return fmt.Errorf("loxa: rotating file sink %q not open", s.cfg.Path)
	}
	if err := s.f.Close(); err != nil {
		return fmt.Errorf("loxa: close rotating file %q: %w", s.cfg.Path, err)
	}
	ts := time.Now().Format("20060102-150405")
	rotated := s.cfg.Path + "." + ts
	if err := os.Rename(s.cfg.Path, rotated); err != nil {
		reopenErr := s.open()
		if reopenErr != nil {
			return errors.Join(
				fmt.Errorf("loxa: rotate %q to %q: %w", s.cfg.Path, rotated, err),
				fmt.Errorf("loxa: reopen %q after failed rotate: %w", s.cfg.Path, reopenErr),
			)
		}
		return fmt.Errorf("loxa: rotate %q to %q: %w", s.cfg.Path, rotated, err)
	}
	if err := s.open(); err != nil {
		return fmt.Errorf("loxa: reopen %q after rotate: %w", s.cfg.Path, err)
	}
	return nil
}

func (s *rotatingFileSink) Flush(_ context.Context) error {
	s.mu.Lock()
	err := s.f.Sync()
	s.mu.Unlock()
	return err
}

func (s *rotatingFileSink) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

// ── memory sink ───────────────────────────────────────────────────────────────

type MemorySinkStore struct {
	mu     sync.Mutex
	events []*Event
	raw    [][]byte
}

func (m *MemorySinkStore) Events() []*Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Event, len(m.events))
	copy(out, m.events)
	return out
}

func (m *MemorySinkStore) Raw() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]byte, len(m.raw))
	copy(out, m.raw)
	return out
}

func (m *MemorySinkStore) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func (m *MemorySinkStore) Clear() {
	m.mu.Lock()
	m.events = m.events[:0]
	m.raw = m.raw[:0]
	m.mu.Unlock()
}

type memorySink struct{ store *MemorySinkStore }

func (s *memorySink) Name() string { return "memory" }

func (s *memorySink) WriteEvent(_ context.Context, encoded []byte, ev *Event) error {
	cp := make([]byte, len(encoded))
	copy(cp, encoded)
	s.store.mu.Lock()
	s.store.events = append(s.store.events, ev)
	s.store.raw = append(s.store.raw, cp)
	s.store.mu.Unlock()
	return nil
}

func (s *memorySink) Flush(_ context.Context) error { return nil }
func (s *memorySink) Close(_ context.Context) error { return nil }

func MemorySink() (Sink, *MemorySinkStore) {
	store := &MemorySinkStore{}
	return &memorySink{store: store}, store
}

// ── noop sink ─────────────────────────────────────────────────────────────────

type noopSink struct{}

func (noopSink) Name() string                                           { return "noop" }
func (noopSink) WriteEvent(_ context.Context, _ []byte, _ *Event) error { return nil }
func (noopSink) Flush(_ context.Context) error                          { return nil }
func (noopSink) Close(_ context.Context) error                          { return nil }

func NoopSink() Sink { return noopSink{} }

// CollectorSinkConfig configures the lightweight HTTP batch collector sink.
type CollectorSinkConfig struct {
	Endpoint          string
	Headers           map[string]string
	Client            *http.Client
	Transport         *HTTPTransport
	Metrics           *MetricsCollector
	MaxRetries        int
	MaxBackoff        time.Duration
	Timeout           time.Duration
	ConnectionTimeout time.Duration
	SDKName           string
	SDKVersion        string
	Service           string
	EnableCompression bool
}

type collectorSink struct {
	cfg CollectorSinkConfig
}

func CollectorSink(cfg CollectorSinkConfig) (Sink, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("loxa: collector endpoint is required")
	}
	if cfg.Transport == nil {
		cfg.Transport = NewHTTPTransport(HTTPTransportConfig{
			MaxRetries:        cfg.MaxRetries,
			MaxBackoff:        cfg.MaxBackoff,
			Timeout:           cfg.Timeout,
			ConnectionTimeout: cfg.ConnectionTimeout,
			Client:            cfg.Client,
			Metrics:           cfg.Metrics,
		})
	}
	if cfg.Client == nil {
		cfg.Client = cfg.Transport.Client()
	}
	if cfg.SDKName == "" {
		cfg.SDKName = "loxa-go"
	}
	if cfg.SDKVersion == "" {
		cfg.SDKVersion = SDKVersion()
	}
	return &collectorSink{cfg: cfg}, nil
}

// LegacyHTTPBatchSink is a convenience wrapper that creates a CollectorSink.
// Deprecated: Use HTTPBatchSink with HTTPBatchSinkConfig for real batching.
func LegacyHTTPBatchSink(endpoint string) (Sink, error) {
	return CollectorSink(CollectorSinkConfig{Endpoint: endpoint})
}

func (s *collectorSink) Name() string { return "collector" }

func (s *collectorSink) WriteEvent(ctx context.Context, encoded []byte, ev *Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	body := encoded
	contentType := "application/x-ndjson"
	if envelope, ok := s.envelope(encoded, ev); ok {
		body = envelope
		contentType = "application/json"
	}
	if contentType == "application/json" {
		if err := ValidateIngestEnvelopeBytes(body, false); err != nil {
			return err
		}
	}
	if s.cfg.EnableCompression {
		compressed, err := gzipBody(body)
		if err != nil {
			return err
		}
		body = compressed
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if s.cfg.EnableCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}
	for k, v := range s.cfg.Headers {
		req.Header.Set(k, v)
	}
	if s.cfg.Transport != nil {
		var transportResp *HTTPResponse
		transportResp, err = s.cfg.Transport.Do(ctx, req)
		if err != nil {
			return err
		}
		if transportResp.StatusCode >= 300 {
			return fmt.Errorf("loxa: collector returned status %d: %s", transportResp.StatusCode, truncateErrorBody(string(transportResp.Body)))
		}
		if retryable, message := collectorResponseHasRetryableError(transportResp.Body); retryable {
			return fmt.Errorf("loxa: collector reported retryable error: %s", message)
		}
		if failed, message := collectorResponseHasPermanentBatchFailure(transportResp.Body); failed {
			return fmt.Errorf("loxa: collector reported batch failure: %s", message)
		}
		return nil
	}
	resp, err := s.cfg.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		// Drain response body to allow HTTP keep-alive reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	raw, readErr := readSinkResponse(resp.Body)
	if readErr != nil {
		return readErr
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("loxa: collector returned status %d: %s", resp.StatusCode, truncateErrorBody(string(raw)))
	}
	if retryable, message := collectorResponseHasRetryableError(raw); retryable {
		return fmt.Errorf("loxa: collector reported retryable error: %s", message)
	}
	if failed, message := collectorResponseHasPermanentBatchFailure(raw); failed {
		return fmt.Errorf("loxa: collector reported batch failure: %s", message)
	}
	return nil
}

func (s *collectorSink) envelope(encoded []byte, ev *Event) ([]byte, bool) {
	var raw json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(encoded), &raw); err != nil {
		return nil, false
	}
	service := strings.TrimSpace(s.cfg.Service)
	if service == "" && ev != nil {
		service = strings.TrimSpace(ev.Service)
	}
	if service == "" {
		service = "unknown"
	}
	envelope := map[string]any{
		"api_version": LOXA_INGEST_API_VERSION,
		"source": map[string]any{
			"sdk":     s.cfg.SDKName,
			"version": s.cfg.SDKVersion,
			"service": service,
		},
		"events": []json.RawMessage{raw},
	}
	out, err := json.Marshal(envelope)
	return out, err == nil
}

func collectorResponseHasRetryableError(raw []byte) (bool, string) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false, ""
	}
	resp, err := speccontract.ParseCollectorResponse(raw)
	if err != nil {
		return false, ""
	}
	return resp.RetryableError()
}

func collectorResponseHasPermanentBatchFailure(raw []byte) (bool, string) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return false, ""
	}
	resp, err := speccontract.ParseCollectorResponse(raw)
	if err != nil {
		return false, ""
	}
	failed, message := resp.PermanentFailure()
	if !failed {
		return false, ""
	}
	if strings.TrimSpace(message) == "" || message == "schema_invalid" {
		for _, ack := range resp.Acks {
			if (ack.Status == "invalid" || ack.Status == "rejected") && strings.TrimSpace(ack.Message) != "" {
				return true, ack.Message
			}
		}
	}
	return true, message
}

func (s *collectorSink) Flush(context.Context) error { return nil }
func (s *collectorSink) Close(context.Context) error { return nil }

// truncateErrorBody limits error messages to prevent credential leaks in logs
func truncateErrorBody(body string) string {
	if len(body) > 512 {
		return body[:512] + "... (truncated)"
	}
	return strings.TrimSpace(body)
}

func readSinkResponse(body io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(body, maxCollectorClientResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxCollectorClientResponseBytes {
		return raw[:maxCollectorClientResponseBytes], fmt.Errorf("response exceeds %d bytes", maxCollectorClientResponseBytes)
	}
	return raw, nil
}

func gzipBody(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		// Return the (possibly incomplete) compressed data along with the error
		// so callers can decide whether to use it or fall back to uncompressed.
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

// ── HTTPBatchSink ────────────────────────────────────────────────────────────

// HTTPBatchSinkConfig configures the HTTP batch sink.
type HTTPBatchSinkConfig struct {
	Endpoint      string
	Headers       map[string]string
	BatchSize     int
	FlushInterval time.Duration
	Gzip          bool
	Client        *http.Client
}

type httpBatchSink struct {
	cfg       HTTPBatchSinkConfig
	mu        sync.Mutex
	buf       [][]byte
	timer     *time.Ticker
	stop      chan struct{}
	closeOnce sync.Once
}

// HTTPBatchSink creates a sink that batches events as NDJSON and flushes
// to the endpoint when BatchSize is reached or FlushInterval elapses.
func HTTPBatchSink(cfg HTTPBatchSinkConfig) (Sink, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("loxa: httpbatch endpoint is required")
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
	s := &httpBatchSink{
		cfg:   cfg,
		buf:   make([][]byte, 0, cfg.BatchSize),
		timer: time.NewTicker(cfg.FlushInterval),
		stop:  make(chan struct{}),
	}
	go s.loop()
	return s, nil
}

func (s *httpBatchSink) Name() string { return "httpbatch" }

func (s *httpBatchSink) WriteEvent(_ context.Context, encoded []byte, _ *Event) error {
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

func (s *httpBatchSink) WriteBatch(ctx context.Context, encoded [][]byte, _ []*Event) error {
	if len(encoded) == 0 {
		return nil
	}
	payload := bytes.Join(encoded, nil)
	return s.send(ctx, payload)
}

func (s *httpBatchSink) Flush(ctx context.Context) error {
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

func (s *httpBatchSink) send(ctx context.Context, payload []byte) error {
	body := payload
	if s.cfg.Gzip {
		var b bytes.Buffer
		zw := gzip.NewWriter(&b)
		if _, err := zw.Write(payload); err != nil {
			_ = zw.Close()
			return err
		}
		if err := zw.Close(); err != nil {
			return fmt.Errorf("gzip compression error: %w", err)
		}
		body = b.Bytes()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
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
	defer func() {
		// Drain response body to allow HTTP keep-alive reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		bodyBytes, _ := readSinkResponse(resp.Body)
		return fmt.Errorf("httpbatch: unexpected status %d: %s", resp.StatusCode, truncateErrorBody(string(bodyBytes)))
	}
	return nil
}

func (s *httpBatchSink) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		close(s.stop)
		s.timer.Stop()
	})
	return s.Flush(ctx)
}

func (s *httpBatchSink) loop() {
	for {
		select {
		case <-s.timer.C:
			_ = s.Flush(context.Background())
		case <-s.stop:
			return
		}
	}
}

// ── MultiSink ─────────────────────────────────────────────────────────────────

// MultiSink fans out events to multiple sinks.
func MultiSink(sinks ...Sink) Sink {
	return &multiSink{sinks: sinks}
}

type multiSink struct {
	sinks []Sink
}

func (s *multiSink) Name() string { return "multi" }

func (s *multiSink) WriteEvent(ctx context.Context, encoded []byte, ev *Event) error {
	var errs []error
	for _, snk := range s.sinks {
		if snk != nil {
			if err := snk.WriteEvent(ctx, encoded, ev); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (s *multiSink) Flush(ctx context.Context) error {
	var errs []error
	for _, snk := range s.sinks {
		if snk != nil {
			if err := snk.Flush(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (s *multiSink) Close(ctx context.Context) error {
	var errs []error
	for _, snk := range s.sinks {
		if snk != nil {
			if err := snk.Close(ctx); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// ── OTLSink ───────────────────────────────────────────────────────────────────

// OTLSink sends events to an OpenTelemetry-compatible endpoint.
// OTLSink creates an OTLP-compatible sink that forwards events via HTTP batch.
func OTLSink(endpoint string) (Sink, error) {
	return HTTPBatchSink(HTTPBatchSinkConfig{Endpoint: endpoint})
}

// ── Drain / Pause / Resume / QueueSize / Health ──────────────────────────────

// Drain empties the sink's buffer.
type Drainable interface {
	Drain(ctx context.Context) error
}

// Pauseable is a sink that can be paused and resumed.
type Pauseable interface {
	Pause()
	Resume()
}

// Sized is a sink that reports its queue size.
type Sized interface {
	QueueSize() int
}

// Checkable is a sink that reports health.
type Checkable interface {
	Health(ctx context.Context) error
}

// Drain calls Drain on a sink if it implements Drainable.
func Drain(ctx context.Context, s Sink) error {
	if d, ok := s.(Drainable); ok {
		return d.Drain(ctx)
	}
	return s.Flush(ctx)
}

// Pause pauses a sink if it implements Pauseable.
func Pause(s Sink) {
	if p, ok := s.(Pauseable); ok {
		p.Pause()
	}
}

// Resume resumes a paused sink if it implements Pauseable.
func Resume(s Sink) {
	if p, ok := s.(Pauseable); ok {
		p.Resume()
	}
}

// QueueSize returns the sink's queue size if it implements Sized, or 0.
func QueueSize(s Sink) int {
	if sq, ok := s.(Sized); ok {
		return sq.QueueSize()
	}
	return 0
}

// Health checks sink health if it implements Checkable.
func Health(ctx context.Context, s Sink) error {
	if h, ok := s.(Checkable); ok {
		return h.Health(ctx)
	}
	return nil
}
