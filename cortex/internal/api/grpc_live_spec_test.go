package api

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	loxav1 "github.com/astraive/loxa/gen/go/loxa/core"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/redaction"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGRPCServerAcceptsCollectorStyleBatch(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Storage.Backend = "duckdb"
	cfg.Storage.DuckDB.Path = filepath.Join(t.TempDir(), "cortex.duckdb")
	cfg.Collector.SourceOfTruth = false

	stor, err := storage.NewStorage(cfg)
	require.NoError(t, err)
	defer stor.Close()

	srv := grpc.NewServer()
	NewGRPCServer(cfg, stor, redaction.Config{}).RegisterServer(srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
	go func() { _ = srv.Serve(lis) }()
	defer srv.GracefulStop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := loxav1.NewCortexServiceClient(conn)
	ts := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	resp, err := client.IngestBatch(context.Background(), &loxav1.IngestBatchRequest{
		Events: []*loxav1.Event{{
			EventId:   "evt-live-1",
			Timestamp: timestamppb.New(ts),
			Kind:      loxav1.EventKind_EVENT_KIND_EVENT,
			Service:   "checkout",
			Event:     "payment.completed",
			TraceId:   "tr_live_1",
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "accepted", resp.Status)
	require.Equal(t, int32(1), resp.Count)

	event, err := stor.Events().Get(context.Background(), "evt-live-1")
	require.NoError(t, err)
	require.Equal(t, models.EventKindLoxaEvent, event.Kind)
	require.Equal(t, "checkout", event.Service)
	require.Equal(t, "tr_live_1", event.TraceID)
	require.Equal(t, "grpc", event.Provenance, "gRPC-ingested events should have 'grpc' provenance")
}

func TestGRPCServerProvenanceIsGrpc(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Storage.Backend = "duckdb"
	cfg.Storage.DuckDB.Path = filepath.Join(t.TempDir(), "cortex.duckdb")
	cfg.Collector.SourceOfTruth = false

	stor, err := storage.NewStorage(cfg)
	require.NoError(t, err)
	defer stor.Close()

	srv := grpc.NewServer()
	NewGRPCServer(cfg, stor, redaction.Config{}).RegisterServer(srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()
	go func() { _ = srv.Serve(lis) }()
	defer srv.GracefulStop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := loxav1.NewCortexServiceClient(conn)
	ts := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	// Test single IngestEvent
	resp, err := client.IngestEvent(context.Background(), &loxav1.IngestEventRequest{
		Event: &loxav1.Event{
			EventId:   "evt-provenance-single",
			Timestamp: timestamppb.New(ts),
			Kind:      loxav1.EventKind_EVENT_KIND_HTTP,
			Service:   "checkout",
			Event:     "checkout.started",
			IncidentId: "inc-1234",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "accepted", resp.Status)

	event, err := stor.Events().Get(context.Background(), "evt-provenance-single")
	require.NoError(t, err)
	require.Equal(t, models.EventKindHTTP, event.Kind)
	require.Equal(t, "grpc", event.Provenance, "single gRPC-ingested event should have 'grpc' provenance")
	require.Equal(t, "inc-1234", event.IncidentID, "IncidentID should be preserved from proto")

	// Test batch IngestBatch
	respBatch, err := client.IngestBatch(context.Background(), &loxav1.IngestBatchRequest{
		Events: []*loxav1.Event{{
			EventId:    "evt-provenance-batch",
			Timestamp:  timestamppb.New(ts),
			Kind:       loxav1.EventKind_EVENT_KIND_EVENT,
			Service:    "billing",
			Event:      "invoice.sent",
			IncidentId: "inc-5678",
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "accepted", respBatch.Status)
	require.Equal(t, int32(1), respBatch.Count)

	event, err = stor.Events().Get(context.Background(), "evt-provenance-batch")
	require.NoError(t, err)
	require.Equal(t, "grpc", event.Provenance, "batch gRPC-ingested event should have 'grpc' provenance")
	require.Equal(t, "inc-5678", event.IncidentID, "IncidentID should be preserved from proto in batch")
}
