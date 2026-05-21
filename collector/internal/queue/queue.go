package queue

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrQueueFull       = errors.New("queue is full, applying backpressure")
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrQueueClosed     = errors.New("queue is closed")
	ErrSinkUnavailable = errors.New("sink is currently unavailable")
)

type Event struct {
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   []byte         `json:"payload"`
	Attempts  int            `json:"attempts"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type QueueConfig struct {
	Directory               string
	MaxSizeBytes            int64
	MaxEvents               int
	BatchSize               int
	BatchTimeout            time.Duration
	FlushInterval           time.Duration
	Sink                    Sink
	SinkName                string
	RetryEnabled            bool
	RetryMaxAttempts        int
	RetryInitialBackoff     time.Duration
	RetryMaxBackoff         time.Duration
	RetryJitter             bool
	CircuitBreakerThreshold int
	CircuitBreakerTimeout   time.Duration
	CircuitBreakerHalfOpen  bool
}

type Sink interface {
	WriteEvent(ctx context.Context, data []byte, key []byte) error
	WriteBatch(ctx context.Context, events [][]byte) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) error
}

type State struct {
	Enqueued      int64 `json:"enqueued"`
	Flushed       int64 `json:"flushed"`
	Failed        int64 `json:"failed"`
	Retried       int64 `json:"retried"`
	Backpressured int64 `json:"backpressured"`
	InFlight      int64 `json:"in_flight"`
	CircuitOpen   bool  `json:"circuit_open"`
}

type Queue struct {
	cfg    QueueConfig
	events []Event
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool

	inflight   map[string]Event
	inflightMu sync.Mutex

	circuitFailures int
	circuitOpenTime time.Time
	circuitMu       sync.Mutex

	state   State
	stateMu sync.RWMutex

	stopCh    chan struct{}
	flushDone chan struct{}
	wg        sync.WaitGroup
	rng       *rand.Rand
}

func NewQueue(cfg QueueConfig) (*Queue, error) {
	if cfg.Directory == "" {
		return nil, errors.New("queue directory is required")
	}
	if cfg.Sink == nil {
		return nil, errors.New("sink is required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = 5 * time.Second
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.RetryMaxAttempts <= 0 {
		cfg.RetryMaxAttempts = 3
	}
	if cfg.RetryInitialBackoff <= 0 {
		cfg.RetryInitialBackoff = 100 * time.Millisecond
	}
	if cfg.RetryMaxBackoff <= 0 {
		cfg.RetryMaxBackoff = 10 * time.Second
	}
	if cfg.CircuitBreakerThreshold <= 0 {
		cfg.CircuitBreakerThreshold = 10
	}
	if cfg.CircuitBreakerTimeout <= 0 {
		cfg.CircuitBreakerTimeout = 30 * time.Second
	}

	if err := os.MkdirAll(cfg.Directory, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir queue dir: %w", err)
	}

	q := &Queue{
		cfg:       cfg,
		events:    make([]Event, 0, cfg.BatchSize),
		inflight:  make(map[string]Event),
		stopCh:    make(chan struct{}),
		flushDone: make(chan struct{}, 1),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	q.cond = sync.NewCond(&q.mu)

	q.wg.Add(1)
	go q.flushLoop()

	return q, nil
}

func (q *Queue) Enqueue(ctx context.Context, event Event) error {
	if event.ID == "" {
		event.ID = q.generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	if q.isCircuitOpen() {
		q.stateMu.Lock()
		q.state.Backpressured++
		q.stateMu.Unlock()
		return ErrCircuitOpen
	}

	if q.cfg.MaxEvents > 0 && len(q.events) >= q.cfg.MaxEvents {
		q.stateMu.Lock()
		q.state.Backpressured++
		q.stateMu.Unlock()
		return ErrQueueFull
	}

	q.events = append(q.events, event)
	q.stateMu.Lock()
	q.state.Enqueued++
	q.state.InFlight++
	q.stateMu.Unlock()

	q.inflightMu.Lock()
	q.inflight[event.ID] = event
	q.inflightMu.Unlock()

	q.cond.Signal()

	return nil
}

func (q *Queue) EnqueueBatch(ctx context.Context, events []Event) []error {
	q.mu.Lock()
	defer q.mu.Unlock()

	errors := make([]error, len(events))

	if q.closed {
		for i := range events {
			errors[i] = ErrQueueClosed
		}
		return errors
	}

	if q.isCircuitOpen() {
		for i := range events {
			errors[i] = ErrCircuitOpen
		}
		q.stateMu.Lock()
		q.state.Backpressured += int64(len(events))
		q.stateMu.Unlock()
		return errors
	}

	for i, event := range events {
		if event.ID == "" {
			event.ID = q.generateEventID()
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}

		if q.cfg.MaxEvents > 0 && len(q.events) >= q.cfg.MaxEvents {
			errors[i] = ErrQueueFull
			q.stateMu.Lock()
			q.state.Backpressured++
			q.stateMu.Unlock()
			continue
		}

		q.events = append(q.events, event)
		errors[i] = nil

		q.inflightMu.Lock()
		q.inflight[event.ID] = event
		q.inflightMu.Unlock()
	}

	enqueued := 0
	for _, err := range errors {
		if err == nil {
			enqueued++
		}
	}

	q.stateMu.Lock()
	q.state.Enqueued += int64(enqueued)
	q.state.InFlight += int64(enqueued)
	q.stateMu.Unlock()

	q.cond.Signal()

	return errors
}

func (q *Queue) flushLoop() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopCh:
			q.mu.Lock()
			q.closed = true
			q.mu.Unlock()
			return
		case <-ticker.C:
			q.mu.Lock()
			if len(q.events) > 0 {
				q.mu.Unlock()
				if err := q.doFlush(); err != nil {
					log.Printf("queue flush error: %v", err)
				}
			} else {
				q.mu.Unlock()
			}
		default:
			q.mu.Lock()
			if len(q.events) >= q.cfg.BatchSize {
				q.mu.Unlock()
				if err := q.doFlush(); err != nil {
					log.Printf("queue flush error: %v", err)
				}
				continue
			}
			q.cond.Wait()
			q.mu.Unlock()
		}
	}
}

func (q *Queue) doFlush() error {
	q.mu.Lock()
	if len(q.events) == 0 {
		q.mu.Unlock()
		return nil
	}

	events := make([]Event, len(q.events))
	copy(events, q.events)
	q.events = q.events[:0]
	q.mu.Unlock()

	batch := make([][]byte, len(events))
	for i, e := range events {
		batch[i] = e.Payload
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var flushErr error
	err := q.cfg.Sink.WriteBatch(ctx, batch)
	if err != nil {
		flushErr = err
		q.recordFailure()
	}

	if flushErr == nil {
		q.recordSuccess()
	}

	// Collect retry events under stateMu, then re-acquire q.mu to append.
	var retryEvents []Event

	q.inflightMu.Lock()
	q.stateMu.Lock()

	if flushErr == nil {
		for _, e := range events {
			delete(q.inflight, e.ID)
		}
		q.state.Flushed += int64(len(events))
		q.state.InFlight -= int64(len(events))
	} else {
		q.state.Failed += int64(len(events))
		if q.cfg.RetryEnabled {
			retryEvents = make([]Event, 0, len(events))
			for _, e := range events {
				e.Attempts++
				if e.Attempts < q.cfg.RetryMaxAttempts {
					retryEvents = append(retryEvents, e)
					q.state.InFlight--
					delete(q.inflight, e.ID)
					q.state.Retried++
				} else {
					delete(q.inflight, e.ID)
					q.state.InFlight--
					q.state.Failed++
				}
			}
		} else {
			q.state.InFlight -= int64(len(events))
		}
	}

	q.stateMu.Unlock()
	q.inflightMu.Unlock()

	// Append retry events under q.mu (the lock that protects q.events).
	if len(retryEvents) > 0 {
		q.mu.Lock()
		q.events = append(q.events, retryEvents...)
		q.mu.Unlock()
		q.cond.Signal()
	}

	if err := q.cfg.Sink.Flush(ctx); err != nil {
		return fmt.Errorf("sink flush: %w", err)
	}
	return flushErr
}

func (q *Queue) recordFailure() {
	q.circuitMu.Lock()
	defer q.circuitMu.Unlock()

	q.circuitFailures++
	if q.circuitFailures >= q.cfg.CircuitBreakerThreshold {
		q.circuitOpenTime = time.Now()
		q.stateMu.Lock()
		q.state.CircuitOpen = true
		q.stateMu.Unlock()
	}
}

func (q *Queue) recordSuccess() {
	q.circuitMu.Lock()
	defer q.circuitMu.Unlock()

	if q.circuitFailures > 0 {
		q.circuitFailures--
		if q.circuitFailures == 0 {
			q.stateMu.Lock()
			if q.state.CircuitOpen && time.Since(q.circuitOpenTime) > q.cfg.CircuitBreakerTimeout {
				q.state.CircuitOpen = false
			}
			q.stateMu.Unlock()
		}
	}
}

func (q *Queue) isCircuitOpen() bool {
	q.circuitMu.Lock()
	defer q.circuitMu.Unlock()

	if q.circuitOpenTime.IsZero() {
		return false
	}

	if time.Since(q.circuitOpenTime) > q.cfg.CircuitBreakerTimeout {
		q.circuitOpenTime = time.Time{}
		q.circuitFailures = 0
		q.stateMu.Lock()
		q.state.CircuitOpen = false
		q.stateMu.Unlock()
		return false
	}

	return true
}

func (q *Queue) State() State {
	q.stateMu.RLock()
	defer q.stateMu.RUnlock()
	return q.state
}

func (q *Queue) Close() error {
	close(q.stopCh)
	q.wg.Wait()

	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()

	// Final flush of any remaining buffered events
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := q.doFlush(); err != nil {
		return err
	}

	return q.cfg.Sink.Close(ctx)
}

func (q *Queue) generateEventID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(q.rng, 16))
}

func randomString(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

type DiskSinkConfig struct {
	Path        string
	MaxFileSize int64
	MaxFiles    int
	EncryptKey  string
}

type DiskSink struct {
	cfg  DiskSinkConfig
	file *os.File
	mu   sync.Mutex
	pos  int64
}

func NewDiskSink(cfg DiskSinkConfig) (*DiskSink, error) {
	if cfg.Path == "" {
		return nil, errors.New("disk sink path is required")
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}
	if cfg.MaxFiles <= 0 {
		cfg.MaxFiles = 10
	}

	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(cfg.Path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	s := &DiskSink{cfg: cfg, file: f}
	if st, err := f.Stat(); err == nil {
		s.pos = st.Size()
	}

	return s, nil
}

func (s *DiskSink) WriteEvent(ctx context.Context, data []byte, key []byte) error {
	return s.WriteBatch(ctx, [][]byte{data})
}

func (s *DiskSink) WriteBatch(ctx context.Context, events [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pos >= s.cfg.MaxFileSize {
		if err := s.rotateFile(); err != nil {
			return err
		}
	}

	var payload []byte
	var marshalErrs []error
	for _, e := range events {
		event := Event{
			Timestamp: time.Now(),
			Payload:   e,
		}
		if cfgKey := s.cfg.EncryptKey; cfgKey != "" {
			encrypted, err := encryptPayload(event.Payload, cfgKey)
			if err != nil {
				marshalErrs = append(marshalErrs, fmt.Errorf("encrypt event payload: %w", err))
				continue
			}
			event.Payload = encrypted
		}
		data, err := json.Marshal(event)
		if err != nil {
			marshalErrs = append(marshalErrs, fmt.Errorf("marshal event %s: %w", event.ID, err))
			continue
		}
		payload = append(payload, data...)
		payload = append(payload, '\n')
	}
	if len(marshalErrs) > 0 {
		return errors.Join(marshalErrs...)
	}

	n, err := s.file.Write(payload)
	if err != nil {
		return err
	}
	s.pos += int64(n)

	return nil
}

func (s *DiskSink) rotateFile() error {
	filename := s.cfg.Path
	dir := filepath.Dir(filename)
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	for i := s.cfg.MaxFiles - 1; i > 0; i-- {
		oldPath := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, i, ext))
		newPath := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, i+1, ext))
		if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	currentPath := filepath.Join(dir, fmt.Sprintf("%s.0%s", name, ext))
	if err := s.file.Close(); err != nil {
		return err
	}
	if err := os.Rename(s.cfg.Path, currentPath); err != nil {
		f, openErr := os.OpenFile(s.cfg.Path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
		if openErr != nil {
			return err
		}
		s.file = f
		s.pos = 0
		return err
	}

	f, err := os.OpenFile(s.cfg.Path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	s.file = f
	s.pos = 0

	return nil
}

func (s *DiskSink) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Sync()
}

func (s *DiskSink) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

func encryptPayload(payload []byte, key string) ([]byte, error) {
	if key == "" {
		cp := make([]byte, len(payload))
		copy(cp, payload)
		return cp, nil
	}
	derived := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(cryptorand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, payload, nil)
	buf := append(nonce, sealed...)
	encoded := base64.StdEncoding.EncodeToString(buf)
	return []byte("enc:" + encoded), nil
}
