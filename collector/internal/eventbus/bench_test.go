package eventbus_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/astraive/loxa-collector/internal/eventbus"
	_ "github.com/astraive/loxa-collector/internal/eventbus/memory"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newMemBus(b *testing.B) eventbus.Bus {
	b.Helper()
	bus, err := eventbus.New(context.Background(), eventbus.Config{
		Type:   "memory",
		Topic:  "bench",
		Memory: eventbus.MemoryConfig{BufferSize: 100000},
	})
	if err != nil {
		b.Fatalf("create bus: %v", err)
	}
	return bus
}

func makeEnvelope(id string) eventbus.Envelope {
	return eventbus.Envelope{
		ID:        id,
		TenantID:  "bench-tenant",
		Service:   "bench-service",
		Event:     "bench.event",
		Timestamp: time.Now(),
		Headers:   map[string]string{"source": "benchmark"},
		Body:      []byte(`{"key":"value","count":42,"nested":{"a":"b"}}`),
	}
}

func makeBatch(n int) []eventbus.Envelope {
	batch := make([]eventbus.Envelope, n)
	for i := range batch {
		batch[i] = makeEnvelope(fmt.Sprintf("bench-%d", i))
	}
	return batch
}

// drainHandler returns a handler that signals wg for each consumed message.
// This keeps the channel drained so publish-only benchmarks do not overflow.
func drainHandler(wg *sync.WaitGroup) eventbus.Handler {
	return func(_ context.Context, _ eventbus.Message) error {
		wg.Done()
		return nil
	}
}

// ---------------------------------------------------------------------------
// 1. Publish single event
// ---------------------------------------------------------------------------
// Measures publish throughput when a fast consumer keeps the channel drained.
// The publish path: closed-check -> getOrCreateTopic -> channel send.
// The drain handler is intentionally minimal (no ack) to isolate publish cost.

func BenchmarkEventBusPublish(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()

	var wg sync.WaitGroup
	if err := bus.Subscribe(ctx, "bench", "drain", drainHandler(&wg)); err != nil {
		b.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond) // let consumeLoop start

	env := makeEnvelope("pub")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 2. Publish batch of 10
// ---------------------------------------------------------------------------

func BenchmarkEventBusPublishBatch10(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()

	var wg sync.WaitGroup
	if err := bus.Subscribe(ctx, "bench", "drain", drainHandler(&wg)); err != nil {
		b.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	envs := makeBatch(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(10)
		if err := bus.Publish(ctx, "bench", envs); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 3. Publish batch of 100
// ---------------------------------------------------------------------------

func BenchmarkEventBusPublishBatch100(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()

	var wg sync.WaitGroup
	if err := bus.Subscribe(ctx, "bench", "drain", drainHandler(&wg)); err != nil {
		b.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	envs := makeBatch(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(100)
		if err := bus.Publish(ctx, "bench", envs); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 4. Subscribe + dispatch (publish -> consume -> ack)
// ---------------------------------------------------------------------------

func BenchmarkEventBusSubscribeDispatch(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	done := make(chan struct{}, 1)

	handler := func(_ context.Context, msg eventbus.Message) error {
		_ = msg.Ack(ctx)
		select {
		case done <- struct{}{}:
		default:
		}
		return nil
	}
	if err := bus.Subscribe(ctx, "bench", "dispatch-group", handler); err != nil {
		b.Fatal(err)
	}

	env := makeEnvelope("dispatch")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		<-done
	}
}

// ---------------------------------------------------------------------------
// 5. Fanout to 10 subscribers
// ---------------------------------------------------------------------------

func BenchmarkEventBusFanout10(b *testing.B) {
	benchmarkFanout(b, 10)
}

// ---------------------------------------------------------------------------
// 6. Fanout to 100 subscribers
// ---------------------------------------------------------------------------

func BenchmarkEventBusFanout100(b *testing.B) {
	benchmarkFanout(b, 100)
}

func benchmarkFanout(b *testing.B, n int) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		group := fmt.Sprintf("fanout-%d", i)
		if err := bus.Subscribe(ctx, "bench", group,
			func(_ context.Context, msg eventbus.Message) error {
				_ = msg.Ack(ctx)
				wg.Done()
				return nil
			}); err != nil {
			b.Fatalf("subscribe group %q: %v", group, err)
		}
	}

	env := makeEnvelope("fanout")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(n)
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 7. Queue enqueue (publish into channel buffer)
// ---------------------------------------------------------------------------

func BenchmarkEventBusQueueEnqueue(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()

	var wg sync.WaitGroup
	if err := bus.Subscribe(ctx, "bench", "drain", drainHandler(&wg)); err != nil {
		b.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)

	envs := makeBatch(1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		if err := bus.Publish(ctx, "bench", envs); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 8. Queue dequeue (consume from channel buffer)
// ---------------------------------------------------------------------------

func BenchmarkEventBusQueueDequeue(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	msgCh := make(chan eventbus.Message, 1)

	handler := func(_ context.Context, msg eventbus.Message) error {
		msgCh <- msg
		return nil
	}
	if err := bus.Subscribe(ctx, "bench", "dequeue-group", handler); err != nil {
		b.Fatal(err)
	}

	env := makeEnvelope("dequeue")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		<-msgCh
	}
}

// ---------------------------------------------------------------------------
// 9. Acknowledge event
// ---------------------------------------------------------------------------

func BenchmarkEventBusAck(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	msgCh := make(chan eventbus.Message, 1)

	handler := func(_ context.Context, msg eventbus.Message) error {
		msgCh <- msg
		return nil
	}
	if err := bus.Subscribe(ctx, "bench", "ack-group", handler); err != nil {
		b.Fatal(err)
	}

	env := makeEnvelope("ack")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		msg := <-msgCh
		if err := msg.Ack(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// 10. Retry scheduling (handler error -> DLQ path)
// ---------------------------------------------------------------------------

func BenchmarkEventBusRetry(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	errDLQ := fmt.Errorf("retry test error")

	var wg sync.WaitGroup

	handler := func(_ context.Context, msg eventbus.Message) error {
		wg.Done()
		return errDLQ
	}
	if err := bus.Subscribe(ctx, "bench", "retry-group", handler); err != nil {
		b.Fatal(err)
	}

	env := makeEnvelope("retry")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}

// ---------------------------------------------------------------------------
// 11. Dead-letter routing (PublishDLQ)
// ---------------------------------------------------------------------------

func BenchmarkEventBusDeadLetter(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	dlqBus, ok := bus.(eventbus.DeadLetterPublisher)
	if !ok {
		b.Fatal("memory bus does not implement DeadLetterPublisher")
	}

	b.ReportAllocs()
	ctx := context.Background()
	env := makeEnvelope("dlq")
	reason := fmt.Errorf("dead letter reason")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := dlqBus.PublishDLQ(ctx, env, reason); err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// 12. Backpressure handling (buffer-full rejection time)
// ---------------------------------------------------------------------------

func BenchmarkEventBusBackpressure(b *testing.B) {
	bus, err := eventbus.New(context.Background(), eventbus.Config{
		Type:   "memory",
		Memory: eventbus.MemoryConfig{BufferSize: 100},
	})
	if err != nil {
		b.Fatalf("create bus: %v", err)
	}
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()

	// Fill the buffer to capacity so every subsequent publish is rejected.
	if err := bus.Publish(ctx, "bench", makeBatch(100)); err != nil {
		b.Fatalf("fill buffer: %v", err)
	}

	env := makeEnvelope("bp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Expected to return an error (buffer full) -- this is the fast path we measure.
		_ = bus.Publish(ctx, "bench", []eventbus.Envelope{env})
	}
}

// ---------------------------------------------------------------------------
// 13. Event serialization (JSON marshal)
// ---------------------------------------------------------------------------

func BenchmarkEventBusSerialization(b *testing.B) {
	env := makeEnvelope("serial")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eventbus.MarshalEnvelope(env)
	}
}

// ---------------------------------------------------------------------------
// 14. Event deserialization (JSON unmarshal)
// ---------------------------------------------------------------------------

func BenchmarkEventBusDeserialization(b *testing.B) {
	env := makeEnvelope("serial")
	data, _ := eventbus.MarshalEnvelope(env)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eventbus.UnmarshalEnvelope(data)
	}
}

// ---------------------------------------------------------------------------
// 15. End-to-end publish to consume
// ---------------------------------------------------------------------------

func BenchmarkEventBusE2E(b *testing.B) {
	bus := newMemBus(b)
	defer bus.Close(context.Background())

	b.ReportAllocs()
	ctx := context.Background()
	var wg sync.WaitGroup

	handler := func(_ context.Context, msg eventbus.Message) error {
		_ = msg.Ack(ctx)
		wg.Done()
		return nil
	}
	if err := bus.Subscribe(ctx, "bench", "e2e-group", handler); err != nil {
		b.Fatal(err)
	}

	env := makeEnvelope("e2e")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		if err := bus.Publish(ctx, "bench", []eventbus.Envelope{env}); err != nil {
			b.Fatal(err)
		}
		wg.Wait()
	}
}
