package gcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	collectorevent "github.com/astraive/loxa-collector/internal/event"
	"github.com/astraive/loxa-collector/internal/sinks/internal/atrest"
	"github.com/astraive/loxa-collector/internal/sinks/internal/projection"
	storagepath "github.com/astraive/loxa-collector/internal/storage"
)

// Config controls GCS sink behavior.
type Config struct {
	Client      *storage.Client
	Bucket      string
	Prefix      string
	ObjectKeyFn func() string
	// Schema projects output payload keys from encoded events.
	Schema        map[string]string
	BatchSize     int
	FlushInterval time.Duration
	EncryptKey    string
}

type sink struct {
	cfg       Config
	mu        sync.Mutex
	buf       bytes.Buffer
	batchSize int
	columns   []string
	count     int
	closed    bool
	stop      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// New creates a GCS sink.
func New(cfg Config) (collectorevent.Sink, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("gcs: Client is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("gcs: Bucket is required")
	}
	if cfg.ObjectKeyFn == nil {
		prefix := cfg.Prefix
		cfg.ObjectKeyFn = func() string {
			return storagepath.NDJSONArchiveKey(prefix, time.Now().UTC())
		}
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	s := &sink{
		cfg:       cfg,
		batchSize: cfg.BatchSize,
		stop:      make(chan struct{}),
	}
	if len(cfg.Schema) > 0 {
		s.cfg.Schema = make(map[string]string, len(cfg.Schema))
		for col, path := range cfg.Schema {
			col = strings.TrimSpace(col)
			if col == "" {
				return nil, fmt.Errorf("gcs: schema column cannot be empty")
			}
			path = strings.TrimSpace(path)
			if path == "" {
				return nil, fmt.Errorf("gcs: empty schema path for column %q", col)
			}
			if err := projection.ValidatePath(path); err != nil {
				return nil, fmt.Errorf("gcs: invalid schema path for column %q: %w", col, err)
			}
			s.cfg.Schema[col] = path
		}
		s.columns = projection.SortedColumns(s.cfg.Schema)
	}
	s.wg.Add(1)
	go s.flushLoop()
	return s, nil
}

func (s *sink) Name() string { return "gcs" }

func (s *sink) WriteEvent(ctx context.Context, encoded []byte, _ *collectorevent.Event) error {
	if len(s.columns) > 0 {
		var err error
		encoded, err = projection.ProjectEncoded(encoded, s.cfg.Schema, s.columns)
		if err != nil {
			return err
		}
	}
	payload, n, err := s.appendLocked(encoded)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	if err := s.putObject(ctx, payload); err != nil {
		s.restore(payload, n)
		return err
	}
	return nil
}

func (s *sink) Flush(ctx context.Context) error { return s.flushBatch(ctx) }

func (s *sink) Close(ctx context.Context) error {
	var closeErr error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		close(s.stop)
		s.wg.Wait()
		if err := s.flushBatch(ctx); err != nil {
			closeErr = err
		}
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
			_ = s.flushBatch(context.Background())
		case <-s.stop:
			return
		}
	}
}

func (s *sink) appendLocked(encoded []byte) ([]byte, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, 0, errors.New("gcs: sink closed")
	}
	s.buf.Write(encoded)
	if len(encoded) == 0 || encoded[len(encoded)-1] != '\n' {
		s.buf.WriteByte('\n')
	}
	s.count++
	if s.count < s.batchSize {
		return nil, 0, nil
	}
	payload := append([]byte(nil), s.buf.Bytes()...)
	n := s.count
	s.buf.Reset()
	s.count = 0
	return payload, n, nil
}

func (s *sink) flushBatch(ctx context.Context) error {
	payload, n := s.drain()
	if len(payload) == 0 {
		return nil
	}
	if err := s.putObject(ctx, payload); err != nil {
		s.restore(payload, n)
		return err
	}
	return nil
}

func (s *sink) drain() ([]byte, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count == 0 {
		return nil, 0
	}
	payload := append([]byte(nil), s.buf.Bytes()...)
	n := s.count
	s.buf.Reset()
	s.count = 0
	return payload, n
}

func (s *sink) restore(payload []byte, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf.Bytes()) > 0 {
		existing := append([]byte(nil), s.buf.Bytes()...)
		s.buf.Reset()
		s.buf.Write(payload)
		s.buf.Write(existing)
	} else {
		s.buf.Reset()
		s.buf.Write(payload)
	}
	s.count += n
}

func (s *sink) putObject(ctx context.Context, payload []byte) error {
	key := s.cfg.ObjectKeyFn()
	if strings.TrimSpace(s.cfg.EncryptKey) != "" {
		var err error
		payload, err = atrest.EncryptBytes(payload, s.cfg.EncryptKey)
		if err != nil {
			return err
		}
	}
	w := s.cfg.Client.Bucket(s.cfg.Bucket).Object(key).NewWriter(ctx)
	w.ContentType = "application/x-ndjson"
	if _, err := w.Write(payload); err != nil {
		return joinWriteCloseError(key, err, w.Close())
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs: close writer for object %q: %w", key, err)
	}
	return nil
}

func joinWriteCloseError(key string, writeErr, closeErr error) error {
	if writeErr == nil {
		return closeErr
	}
	wrappedWrite := fmt.Errorf("gcs: write object %q: %w", key, writeErr)
	if closeErr == nil {
		return wrappedWrite
	}
	return errors.Join(
		wrappedWrite,
		fmt.Errorf("gcs: close writer for object %q after write failure: %w", key, closeErr),
	)
}
