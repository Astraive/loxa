package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/astraive/loxa-collector/internal/otlpconv"
	loxav1 "github.com/astraive/loxa/spec/proto/loxa/v1"
	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	cfg   GRPCConfig
	state State
	ready atomic.Bool
	mu    sync.Mutex
	lis   net.Listener
	srv   *grpc.Server
}

func NewGRPCServer(cfg GRPCConfig, state State) *GRPCServer {
	return &GRPCServer{
		cfg:   cfg,
		state: state,
	}
}

func (s *GRPCServer) Name() string { return "grpc" }

func (s *GRPCServer) Addr() string { return s.cfg.Port }

func (s *GRPCServer) IsReady() bool { return s.ready.Load() }

func (s *GRPCServer) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}

	var opts []grpc.ServerOption

	opts = append(opts, grpc.MaxConcurrentStreams(uint32(s.cfg.MaxConcurrentStreams)))
	opts = append(opts, grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize))
	opts = append(opts, grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize))

	if s.cfg.MaxConnectionAge.d > 0 || s.cfg.KeepaliveTime.d > 0 {
		kep := keepalive.EnforcementPolicy{
			MinTime: s.cfg.KeepaliveTime.d,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(kep))
	}

	kvp := keepalive.ServerParameters{
		MaxConnectionAge:      s.cfg.MaxConnectionAge.d,
		MaxConnectionAgeGrace: s.cfg.MaxConnectionAgeGrace.d,
		Time:                  s.cfg.KeepaliveTime.d,
		Timeout:               s.cfg.KeepaliveTimeout.d,
	}
	opts = append(opts, grpc.KeepaliveParams(kvp))

	if s.cfg.TLSEnabled {
		creds, err := credentials.NewServerTLSFromFile(s.cfg.TLSCertFile, s.cfg.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certs: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	s.srv = grpc.NewServer(opts...)

	loxav1.RegisterCollectorServiceServer(s.srv, &collectorSvcServer{state: s.state})
	loxav1.RegisterLogIngestServer(s.srv, &logIngestSvcServer{state: s.state})
	loxav1.RegisterLoxaIngestServer(s.srv, &loxaIngestSvcServer{state: s.state})
	collectorlogsv1.RegisterLogsServiceServer(s.srv, &otlpLogsServiceServer{state: s.state})

	var err error
	s.lis, err = net.Listen("tcp", s.cfg.Port)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.Port, err)
	}

	go func() {
		<-ctx.Done()
		s.srv.GracefulStop()
	}()

	s.ready.Store(true)
	return s.srv.Serve(s.lis)
}

func (s *GRPCServer) Stop(ctx context.Context) error {
	s.ready.Store(false)
	if s.srv == nil {
		return nil
	}
	s.srv.GracefulStop()
	return nil
}

type collectorSvcServer struct {
	loxav1.UnimplementedCollectorServiceServer
	state State
}

func (s *collectorSvcServer) Health(ctx context.Context, req *loxav1.CollectorStatusRequest) (*loxav1.CollectorStatusResponse, error) {
	if s.state.IsHealthy() {
		return &loxav1.CollectorStatusResponse{Status: "ok"}, nil
	}
	return &loxav1.CollectorStatusResponse{Status: "unhealthy"}, nil
}

func (s *collectorSvcServer) Ready(ctx context.Context, req *loxav1.CollectorStatusRequest) (*loxav1.CollectorStatusResponse, error) {
	if s.state.IsReady() {
		return &loxav1.CollectorStatusResponse{Status: "ready"}, nil
	}
	return &loxav1.CollectorStatusResponse{Status: "not_ready"}, nil
}

type logIngestSvcServer struct {
	loxav1.UnimplementedLogIngestServer
	state State
}

type loxaIngestSvcServer struct {
	loxav1.UnimplementedLoxaIngestServer
	state State
}

type otlpLogsServiceServer struct {
	collectorlogsv1.UnimplementedLogsServiceServer
	state State
}

func (s *logIngestSvcServer) Push(ctx context.Context, batch *loxav1.RawEventBatch) (*loxav1.PushResponse, error) {
	return ingestGRPCBatch(ctx, s.state, batch)
}

func (s *loxaIngestSvcServer) Ingest(ctx context.Context, batch *loxav1.RawEventBatch) (*loxav1.PushResponse, error) {
	return ingestGRPCBatch(ctx, s.state, batch)
}

func (s *loxaIngestSvcServer) IngestStream(stream loxav1.LoxaIngest_IngestStreamServer) error {
	var (
		totalAccepted int64
		totalRejected int64
		totalInvalid  int64
		totalDeduped  int64
		acks          []*loxav1.EventAck
	)

	for {
		batch, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&loxav1.PushResponse{
					Accepted: totalAccepted,
					Rejected: totalRejected,
					Invalid:  totalInvalid,
					Deduped:  totalDeduped,
					Acks:     acks,
				})
			}
			return status.Errorf(codes.Internal, "stream recv failed: %v", err)
		}
		resp, err := ingestGRPCBatch(stream.Context(), s.state, batch)
		if err != nil {
			return err
		}
		totalAccepted += resp.Accepted
		totalRejected += resp.Rejected
		totalInvalid += resp.Invalid
		totalDeduped += resp.Deduped
		acks = append(acks, resp.Acks...)
	}
}

func ingestGRPCBatch(ctx context.Context, state State, batch *loxav1.RawEventBatch) (*loxav1.PushResponse, error) {
	if batch == nil || batch.Events == nil {
		return &loxav1.PushResponse{Accepted: 0}, nil
	}

	events := make([][]byte, 0, len(batch.Events))
	for _, event := range batch.Events {
		if event != nil && event.RawJson != "" {
			events = append(events, []byte(event.RawJson))
		}
	}

	if len(events) == 0 {
		return &loxav1.PushResponse{Accepted: 0}, nil
	}

	accepted, err := state.Ingest(ctx, events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}

	return &loxav1.PushResponse{Accepted: int64(accepted)}, nil
}

func (s *otlpLogsServiceServer) Export(ctx context.Context, req *collectorlogsv1.ExportLogsServiceRequest) (*collectorlogsv1.ExportLogsServiceResponse, error) {
	events, err := otlpconv.ConvertExportLogsRequest(req, otlpconv.Config{
		SchemaVersion:  "v1",
		EventVersion:   "v1",
		DefaultService: "otlp",
	})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "otlp conversion failed: %v", err)
	}
	if _, err := s.state.Ingest(ctx, events); err != nil {
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}
	return &collectorlogsv1.ExportLogsServiceResponse{}, nil
}
