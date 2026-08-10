package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	lozav1 "github.com/astraive/loza/gen/go/loza/core"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeCortexBridgeServer struct {
	lozav1.UnimplementedCortexServiceServer
	mu      sync.Mutex
	batches []*lozav1.IngestBatchRequest
}

func (f *fakeCortexBridgeServer) IngestBatch(_ context.Context, req *lozav1.IngestBatchRequest) (*lozav1.IngestBatchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, req)
	return &lozav1.IngestBatchResponse{Status: "accepted", Count: int32(len(req.Events))}, nil
}

func (f *fakeCortexBridgeServer) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

func (f *fakeCortexBridgeServer) lastBatch() *lozav1.IngestBatchRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.batches) == 0 {
		return nil
	}
	return f.batches[len(f.batches)-1]
}

func startFakeCortexBridge(t *testing.T) (string, *fakeCortexBridgeServer, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	fake := &fakeCortexBridgeServer{}
	lozav1.RegisterCortexServiceServer(srv, fake)
	go func() {
		_ = srv.Serve(lis)
	}()
	return lis.Addr().String(), fake, func() {
		srv.GracefulStop()
		_ = lis.Close()
	}
}

func TestCortexBridgeSyncDelivery(t *testing.T) {
	endpoint, fake, stop := startFakeCortexBridge(t)
	defer stop()

	bridge, err := newCortexBridgeClient(collectorConfig{
		cortexBridgeEnabled:  true,
		cortexBridgeMode:     "sync",
		cortexBridgeEndpoint: endpoint,
		cortexBridgeInsecure: true,
		cortexBridgeTimeout:  2 * time.Second,
	}, nil)
	require.NoError(t, err)
	defer bridge.Close()

	raw := []byte(`{"event_id":"evt-sync","timestamp":"2026-05-17T12:00:00Z","service":"checkout","kind":"http","trace_id":"tr_1"}`)
	require.NoError(t, bridge.Deliver(context.Background(), [][]byte{raw}))

	require.Eventually(t, func() bool { return fake.batchCount() == 1 }, 3*time.Second, 20*time.Millisecond)
	batch := fake.lastBatch()
	require.NotNil(t, batch)
	require.Len(t, batch.Events, 1)
	require.Equal(t, "evt-sync", batch.Events[0].EventId)
	require.Equal(t, "checkout", batch.Events[0].Service)
	require.Equal(t, lozav1.EventKind_EVENT_KIND_EVENT, batch.Events[0].Kind)
	require.Equal(t, "tr_1", batch.Events[0].TraceId)
	require.NotNil(t, batch.Events[0].Attrs)
}

func TestCortexBridgeAsyncFlush(t *testing.T) {
	endpoint, fake, stop := startFakeCortexBridge(t)
	defer stop()

	bridge, err := newCortexBridgeClient(collectorConfig{
		cortexBridgeEnabled:    true,
		cortexBridgeMode:       "async",
		cortexBridgeEndpoint:   endpoint,
		cortexBridgeInsecure:   true,
		cortexBridgeTimeout:    2 * time.Second,
		cortexBridgeBatchSize:  2,
		cortexBridgeFlushIntvl: 25 * time.Millisecond,
		cortexBridgeQueueSize:  8,
	}, nil)
	require.NoError(t, err)
	defer bridge.Close()

	rawA := []byte(`{"event_id":"evt-a","timestamp":"2026-05-17T12:00:00Z","service":"billing","kind":"event"}`)
	rawB := []byte(`{"event_id":"evt-b","timestamp":"2026-05-17T12:00:01Z","service":"billing","kind":"event"}`)
	require.NoError(t, bridge.Deliver(context.Background(), [][]byte{rawA, rawB}))

	require.Eventually(t, func() bool { return fake.batchCount() >= 1 }, 3*time.Second, 20*time.Millisecond)
	batch := fake.lastBatch()
	require.NotNil(t, batch)
	require.Len(t, batch.Events, 2)
}
