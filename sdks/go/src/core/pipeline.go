package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// SinkWriter is the minimal sink interface used by the pipeline.
// This avoids importing the root loxa package from internal/core.
type SinkWriter interface {
	WriteEvent(ctx context.Context, encoded []byte, ev *Event) error
	Flush(ctx context.Context) error
	Close(ctx context.Context) error
}

// BatchSinkWriter optionally accepts a batch of pipeline items in one call.
type BatchSinkWriter interface {
	WriteBatch(ctx context.Context, items []PipelineItem) error
}

// PipelineItem carries encoded event bytes to workers.
type PipelineItem struct {
	Encoded []byte
	Event   *Event
	Level   int // 0=debug, 1=info, 2=warn, 3=error — used by drop policies
	IsError bool
}

// PipelineConfig carries the settings the pipeline needs.
type PipelineConfig struct {
	QueueSize     int
	Workers       int
	FlushInterval time.Duration
	MaxBatchBytes int
	Backpressure  BackpressurePolicy
	Sinks         []SinkWriter
	Fallback      SinkWriter
	OnDrop        func(reason string)
	OnError       func(err error)
}

// Pipeline manages the async event emission queue.
type Pipeline struct {
	cfg       PipelineConfig
	queue     chan PipelineItem
	done      chan struct{}
	stopOnce  sync.Once
	workerWG  sync.WaitGroup
	pending   atomic.Int64
	drainMu   sync.Mutex
	drainCond *sync.Cond
}

// ErrPipelineClosed is returned when enqueueing after shutdown starts.
var ErrPipelineClosed = errors.New("pipeline closed")

// NewPipeline creates and starts the async pipeline.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 8192
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = time.Second
	}
	if cfg.MaxBatchBytes <= 0 {
		cfg.MaxBatchBytes = 4 * 1024 * 1024
	}

	p := &Pipeline{
		cfg:   cfg,
		queue: make(chan PipelineItem, cfg.QueueSize),
		done:  make(chan struct{}),
	}
	p.drainCond = sync.NewCond(&p.drainMu)
	p.workerWG.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go p.worker()
	}
	go p.flusher()
	return p
}

// Enqueue sends an item to the async queue per the backpressure policy.
func (p *Pipeline) Enqueue(item PipelineItem) (bool, error) {
	select {
	case <-p.done:
		return false, ErrPipelineClosed
	default:
	}

	// Own encoded bytes so callers can safely reuse buffers after Enqueue returns.
	if len(item.Encoded) > 0 {
		item.Encoded = append([]byte(nil), item.Encoded...)
	}

	enqueued := false
	switch p.cfg.Backpressure {
	case Block:
		select {
		case p.queue <- item:
			enqueued = true
		case <-p.done:
			return false, ErrPipelineClosed
		}

	case DropNewest:
		select {
		case p.queue <- item:
			enqueued = true
		case <-p.done:
			return false, ErrPipelineClosed
		default:
			p.notifyDrop("queue_full_drop_newest")
		}

	case DropOldest:
		select {
		case p.queue <- item:
			enqueued = true
		case <-p.done:
			return false, ErrPipelineClosed
		default:
			select {
			case <-p.done:
				return false, ErrPipelineClosed
			case <-p.queue:
				p.pending.Add(-1)
			default:
			}
			select {
			case p.queue <- item:
				enqueued = true
			case <-p.done:
				return false, ErrPipelineClosed
			default:
				p.notifyDrop("queue_full_drop_oldest")
			}
		}

	case DropDebug:
		if item.Level == 0 { // debug
			select {
			case p.queue <- item:
				enqueued = true
			case <-p.done:
				return false, ErrPipelineClosed
			default:
				p.notifyDrop("queue_full_drop_debug")
			}
		} else {
			select {
			case p.queue <- item:
				enqueued = true
			case <-p.done:
				return false, ErrPipelineClosed
			}
		}

	case DropSampled:
		if item.IsError {
			select {
			case p.queue <- item:
				enqueued = true
			case <-p.done:
				return false, ErrPipelineClosed
			}
		} else {
			select {
			case p.queue <- item:
				enqueued = true
			case <-p.done:
				return false, ErrPipelineClosed
			default:
				p.notifyDrop("queue_full_drop_sampled")
			}
		}

	case SyncFallback:
		select {
		case p.queue <- item:
			enqueued = true
		case <-p.done:
			return false, ErrPipelineClosed
		default:
			if err := p.writeDirect(item); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	if enqueued {
		p.pending.Add(1)
	}
	return enqueued, nil
}

func (p *Pipeline) writeDirect(item PipelineItem) error {
	return p.writeBatch([]PipelineItem{item})
}

func (p *Pipeline) writeBatch(items []PipelineItem) error {
	ctx := context.Background()
	var last error
	failed := false
	for _, s := range p.cfg.Sinks {
		if err := writeBatchToSink(ctx, s, items); err != nil {
			failed = true
			p.notifyError(err)
			last = err
		}
	}
	if failed && p.cfg.Fallback != nil {
		if err := writeBatchToSink(ctx, p.cfg.Fallback, items); err != nil {
			p.notifyError(err)
			last = err
		}
	}
	return last
}

func writeBatchToSink(ctx context.Context, sink SinkWriter, items []PipelineItem) error {
	if len(items) == 0 {
		return nil
	}
	if batchSink, ok := sink.(BatchSinkWriter); ok {
		return batchSink.WriteBatch(ctx, items)
	}
	for _, item := range items {
		if err := sink.WriteEvent(ctx, item.Encoded, item.Event); err != nil {
			return err
		}
	}
	return nil
}

// Flush drains the queue and flushes sinks.
func (p *Pipeline) Flush(ctx context.Context) error {
	if err := p.waitForDrain(ctx); err != nil {
		return err
	}
	return p.flushSinks(ctx)
}

// Shutdown closes the pipeline and waits for drain.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() { close(p.done) })
	if err := p.waitForDrain(ctx); err != nil {
		return err
	}

	waitCh := make(chan struct{})
	go func() {
		p.workerWG.Wait()
		close(waitCh)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waitCh:
	}

	return p.flushSinks(ctx)
}

func (p *Pipeline) worker() {
	defer p.workerWG.Done()
	var carry *PipelineItem
	for {
		item, ok := p.nextItem(&carry)
		if !ok {
			return
		}
		batch := p.collectBatch(item, &carry)
		if err := p.writeBatch(batch); err != nil {
			p.notifyError(err)
		}
		p.finishItems(len(batch))

		select {
		case <-p.done:
			for {
				item, ok := p.tryNextQueuedItem(&carry)
				if !ok {
					return
				}
				batch := p.collectBatch(item, &carry)
				if err := p.writeBatch(batch); err != nil {
					p.notifyError(err)
				}
				p.finishItems(len(batch))
			}
		default:
		}
	}
}

func (p *Pipeline) nextItem(carry **PipelineItem) (PipelineItem, bool) {
	if carry != nil && *carry != nil {
		item := **carry
		*carry = nil
		return item, true
	}
	select {
	case item, ok := <-p.queue:
		return item, ok
	case <-p.done:
		return p.tryNextQueuedItem(carry)
	}
}

func (p *Pipeline) tryNextQueuedItem(carry **PipelineItem) (PipelineItem, bool) {
	if carry != nil && *carry != nil {
		item := **carry
		*carry = nil
		return item, true
	}
	select {
	case item, ok := <-p.queue:
		return item, ok
	default:
		return PipelineItem{}, false
	}
}

func (p *Pipeline) collectBatch(first PipelineItem, carry **PipelineItem) []PipelineItem {
	batch := []PipelineItem{first}
	if p.cfg.MaxBatchBytes <= 0 {
		return batch
	}
	size := len(first.Encoded)
	for {
		next, ok := p.tryNextQueuedItem(carry)
		if !ok {
			return batch
		}
		if len(batch) > 0 && size+len(next.Encoded) > p.cfg.MaxBatchBytes {
			*carry = &next
			return batch
		}
		batch = append(batch, next)
		size += len(next.Encoded)
	}
}

func (p *Pipeline) flusher() {
	t := time.NewTicker(p.cfg.FlushInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if err := p.flushSinks(context.Background()); err != nil {
				p.notifyError(err)
			}
		case <-p.done:
			return
		}
	}
}

func (p *Pipeline) isDrained() bool {
	return p.pending.Load() == 0
}

func (p *Pipeline) waitForDrain(ctx context.Context) error {
	p.drainMu.Lock()
	defer p.drainMu.Unlock()
	for !p.isDrained() {
		// Register the cancellation callback BEFORE checking ctx.Err()
		// to avoid a race where cancellation happens between the check and AfterFunc registration.
		stopBroadcast := context.AfterFunc(ctx, func() {
			p.drainMu.Lock()
			p.drainCond.Broadcast()
			p.drainMu.Unlock()
		})
		if err := ctx.Err(); err != nil {
			stopBroadcast()
			return err
		}
		p.drainCond.Wait()
		stopBroadcast()
	}
	return nil
}

func (p *Pipeline) finishItems(count int) {
	if count <= 0 {
		return
	}
	if p.pending.Add(int64(-count)) == 0 {
		p.drainMu.Lock()
		p.drainCond.Broadcast()
		p.drainMu.Unlock()
	}
}

func (p *Pipeline) flushSinks(ctx context.Context) error {
	var last error
	for _, s := range p.cfg.Sinks {
		if err := s.Flush(ctx); err != nil {
			p.notifyError(err)
			last = err
		}
	}
	if p.cfg.Fallback != nil {
		if err := p.cfg.Fallback.Flush(ctx); err != nil {
			p.notifyError(err)
			last = err
		}
	}
	return last
}

func (p *Pipeline) notifyDrop(reason string) {
	if p.cfg.OnDrop != nil {
		p.cfg.OnDrop(reason)
	}
}

func (p *Pipeline) notifyError(err error) {
	if err != nil && p.cfg.OnError != nil {
		p.cfg.OnError(err)
	}
}
