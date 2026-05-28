package eventbus

import (
	"sync"

	"github.com/astraive/loxa/cortex/internal/models"
	"github.com/rs/zerolog/log"
)

// EventBus provides in-memory pub/sub for events.
// Used by the processor to publish ingested events and by
// gRPC StreamEvents to subscribe.
type EventBus struct {
	subscribers    map[string][]chan *models.Event
	mu             sync.RWMutex
	bufferSize     int
}

// New creates a new EventBus with default buffer size (128).
func New() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan *models.Event),
		bufferSize:  128,
	}
}

// WithBufferSize sets the subscriber channel buffer size.
func (b *EventBus) WithBufferSize(size int) *EventBus {
	if size > 0 {
		b.bufferSize = size
	}
	return b
}

// Subscribe creates a new subscription with an optional filter.
// filter="" means all events. Returns a channel that receives events.
func (b *EventBus) Subscribe(filter string) <-chan *models.Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan *models.Event, b.bufferSize)
	b.subscribers[filter] = append(b.subscribers[filter], ch)

	log.Info().Str("filter", filter).Msg("EventBus: new subscriber")
	return ch
}

// Unsubscribe removes a subscription and closes its channel.
func (b *EventBus) Unsubscribe(filter string, ch <-chan *models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[filter]
	for i, sub := range subs {
		// Compare the underlying channel
		if sub == ch {
			b.subscribers[filter] = append(subs[:i], subs[i+1:]...)
			close(sub)
			break
		}
	}
}

// Publish sends an event to all matching subscribers.
func (b *EventBus) Publish(event *models.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Send to wildcard subscribers
	for _, ch := range b.subscribers[""] {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is full (non-blocking)
		}
	}

	// Send to incident-specific subscribers
	if event.IncidentID != "" {
		for _, ch := range b.subscribers[event.IncidentID] {
			select {
			case ch <- event:
			default:
			}
		}
	}

	// Send to service-specific subscribers
	if event.Service != "" {
		for _, ch := range b.subscribers[event.Service] {
			select {
			case ch <- event:
			default:
			}
		}
	}
}

// SubscriberCount returns the number of active subscribers.
func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	for _, subs := range b.subscribers {
		count += len(subs)
	}
	return count
}
