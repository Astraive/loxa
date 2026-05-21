package api

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	loxav1 "github.com/astraive/loxa-spec/proto/loxa/v1"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
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
	NewGRPCServer(cfg, stor).RegisterServer(srv)

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
	rawStruct, err := structpb.NewStruct(map[string]any{
		"event_id":  "evt-live-1",
		"service":   "checkout",
		"kind":      "http",
		"event":     "payment.completed",
		"trace_id":  "tr_live_1",
		"timestamp": ts.Format(time.RFC3339),
	})
	require.NoError(t, err)

	resp, err := client.IngestBatch(context.Background(), &loxav1.IngestBatchRequest{
		Events: []*loxav1.IngestEventRequest{{
			Id:         "evt-live-1",
			Timestamp:  timestamppb.New(ts),
			Kind:       "http",
			Service:    "checkout",
			TraceId:    "tr_live_1",
			Provenance: "collector",
			Raw:        rawStruct,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "accepted", resp.Status)
	require.Equal(t, int32(1), resp.Count)

	event, err := stor.Events().Get(context.Background(), "evt-live-1")
	require.NoError(t, err)
	require.Equal(t, models.EventKindLoxaEvent, event.Kind)
	require.Equal(t, "collector", event.Provenance)
	require.Equal(t, "checkout", event.Service)
	require.Equal(t, "tr_live_1", event.TraceID)
}
