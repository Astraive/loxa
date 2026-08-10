package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/astraive/loza/collector/internal/auth"
	"github.com/astraive/loza/collector/internal/otlpconv"
	"github.com/astraive/loza/collector/internal/version"
	lozav1 "github.com/astraive/loza/gen/go/loza/core"
	collectorlogsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	cfg               GRPCConfig
	state             State
	ready             atomic.Bool
	lis               net.Listener
	srv               *grpc.Server
	authEnabled       bool
	allowLocalDevKeys bool
	trustedProxies    []*net.IPNet
	keyStore          auth.KeyStore
	keyCache          *auth.MemoryKeyCache
	serverSecret      []byte
}

func NewGRPCServer(cfg GRPCConfig, state State) *GRPCServer {
	return &GRPCServer{
		cfg:   cfg,
		state: state,
	}
}

// WithAuth configures API key authentication for the gRPC server.
// When set, UnaryAuthInterceptor and StreamAuthInterceptor are added as server options.
func (s *GRPCServer) WithAuth(store auth.KeyStore, cache *auth.MemoryKeyCache, serverSecret []byte) *GRPCServer {
	s.authEnabled = true
	s.keyStore = store
	s.keyCache = cache
	s.serverSecret = serverSecret
	return s
}

// WithAllowLocalDevKeys enables lx_local_dev_* key acceptance on this server.
func (s *GRPCServer) WithAllowLocalDevKeys(v bool) *GRPCServer {
	s.allowLocalDevKeys = v
	return s
}

// WithTrustedProxies sets the trusted proxy CIDRs for IP-based ABAC checks.
func (s *GRPCServer) WithTrustedProxies(proxies []*net.IPNet) *GRPCServer {
	s.trustedProxies = proxies
	return s
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

	if s.authEnabled {
		grpcOpts := []auth.GRPCAuthOption{auth.GRPCWithAllowLocalDevKeys(s.allowLocalDevKeys)}
		if len(s.trustedProxies) > 0 {
			grpcOpts = append(grpcOpts, auth.GRPCWithTrustedProxies(s.trustedProxies))
		}
		opts = append(opts,
			grpc.ChainUnaryInterceptor(auth.UnaryAuthInterceptor(s.keyStore, s.keyCache, s.serverSecret, grpcOpts...)),
			grpc.ChainStreamInterceptor(auth.StreamAuthInterceptor(s.keyStore, s.keyCache, s.serverSecret, grpcOpts...)),
		)
	}

	s.srv = grpc.NewServer(opts...)

	lozav1.RegisterCollectorServiceServer(s.srv, &collectorSvcServer{state: s.state})
	lozav1.RegisterLogIngestServer(s.srv, &logIngestSvcServer{state: s.state})
	lozav1.RegisterLozaIngestServer(s.srv, &lozaIngestSvcServer{state: s.state})
	lozav1.RegisterCollectorIngestServer(s.srv, &collectorIngestServer{state: s.state})
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
	lozav1.UnimplementedCollectorServiceServer
	state State
}

func (s *collectorSvcServer) Health(ctx context.Context, req *lozav1.CollectorStatusRequest) (*lozav1.CollectorStatusResponse, error) {
	if s.state.IsHealthy() {
		return &lozav1.CollectorStatusResponse{Status: "ok"}, nil
	}
	return &lozav1.CollectorStatusResponse{Status: "unhealthy"}, nil
}

func (s *collectorSvcServer) Ready(ctx context.Context, req *lozav1.CollectorStatusRequest) (*lozav1.CollectorStatusResponse, error) {
	if s.state.IsReady() {
		return &lozav1.CollectorStatusResponse{Status: "ready"}, nil
	}
	return &lozav1.CollectorStatusResponse{Status: "not_ready"}, nil
}

type logIngestSvcServer struct {
	lozav1.UnimplementedLogIngestServer
	state State
}

type lozaIngestSvcServer struct {
	lozav1.UnimplementedLozaIngestServer
	state State
}

type otlpLogsServiceServer struct {
	collectorlogsv1.UnimplementedLogsServiceServer
	state State
}

func (s *logIngestSvcServer) Push(ctx context.Context, batch *lozav1.RawEventBatch) (*lozav1.PushResponse, error) {
	return ingestGRPCBatch(ctx, s.state, batch)
}

func (s *lozaIngestSvcServer) Ingest(ctx context.Context, batch *lozav1.RawEventBatch) (*lozav1.PushResponse, error) {
	return ingestGRPCBatch(ctx, s.state, batch)
}

func (s *lozaIngestSvcServer) IngestStream(stream lozav1.LozaIngest_IngestStreamServer) error {
	var (
		totalAccepted int64
		totalRejected int64
		totalInvalid  int64
		totalDeduped  int64
		acks          []*lozav1.EventAck
	)

	for {
		batch, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&lozav1.PushResponse{
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

func ingestGRPCBatch(ctx context.Context, state State, batch *lozav1.RawEventBatch) (*lozav1.PushResponse, error) {
	if batch == nil || batch.Events == nil {
		return &lozav1.PushResponse{Accepted: 0}, nil
	}

	events := make([][]byte, 0, len(batch.Events))
	for _, event := range batch.Events {
		if event != nil && event.RawJson != "" {
			events = append(events, []byte(event.RawJson))
		}
	}

	if len(events) == 0 {
		return &lozav1.PushResponse{Accepted: 0}, nil
	}

	accepted, err := state.Ingest(ctx, events)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}

	return &lozav1.PushResponse{Accepted: int64(accepted)}, nil
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

// collectorIngestServer implements the CollectorIngest gRPC service for SDK lightweight connectors.
type collectorIngestServer struct {
	lozav1.UnimplementedCollectorIngestServer
	state State
}

func (s *collectorIngestServer) Ingest(ctx context.Context, batch *lozav1.IngestBatch) (*lozav1.IngestResponse, error) {
	if batch == nil || len(batch.Events) == 0 {
		return &lozav1.IngestResponse{}, nil
	}

	rawEvents := make([][]byte, 0, len(batch.Events))
	accepted := int64(0)
	rejected := int64(0)
	invalid := int64(0)
	acks := make([]*lozav1.EventAck, 0, len(batch.Events))

	for _, event := range batch.Events {
		if event == nil {
			continue
		}
		// Convert proto Event to raw JSON for the collector's existing ingest pipeline
		rawMap, err := eventProtoToMap(event)
		if err != nil {
			invalid++
			acks = append(acks, &lozav1.EventAck{
				EventId: event.EventId,
				Status:  "invalid",
				Reason:  err.Error(),
			})
			continue
		}
		rawJSON, err := json.Marshal(rawMap)
		if err != nil {
			invalid++
			acks = append(acks, &lozav1.EventAck{
				EventId: event.EventId,
				Status:  "invalid",
				Reason:  err.Error(),
			})
			continue
		}
		rawEvents = append(rawEvents, rawJSON)
	}

	if len(rawEvents) > 0 {
		n, err := s.state.Ingest(ctx, rawEvents)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
		}
		accepted = int64(n)
		rejected = int64(len(rawEvents)) - accepted
	}

	return &lozav1.IngestResponse{
		Accepted: accepted,
		Rejected: rejected,
		Invalid:  invalid,
		Acks:     acks,
	}, nil
}

func (s *collectorIngestServer) IngestStream(stream lozav1.CollectorIngest_IngestStreamServer) error {
	var totalAccepted int64
	var totalRejected int64
	var totalInvalid int64
	var allAcks []*lozav1.EventAck

	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&lozav1.IngestResponse{
					Accepted: totalAccepted,
					Rejected: totalRejected,
					Invalid:  totalInvalid,
					Acks:     allAcks,
				})
			}
			return status.Errorf(codes.Internal, "stream recv failed: %v", err)
		}

		batch := &lozav1.IngestBatch{Events: []*lozav1.Event{event}}
		resp, err := s.Ingest(stream.Context(), batch)
		if err != nil {
			return err
		}
		totalAccepted += resp.Accepted
		totalRejected += resp.Rejected
		totalInvalid += resp.Invalid
		allAcks = append(allAcks, resp.Acks...)
	}
}

func (s *collectorIngestServer) Ping(ctx context.Context, req *lozav1.PingRequest) (*lozav1.PingResponse, error) {
	return &lozav1.PingResponse{
		Status:  "ok",
		Version: version.CollectorVersion(),
	}, nil
}

// eventProtoToMap converts a proto Event message to a map[string]interface{} for JSON serialization.
func eventProtoToMap(event *lozav1.Event) (map[string]interface{}, error) {
	rawMap := make(map[string]interface{})

	if event.EventId != "" {
		rawMap["id"] = event.EventId
		rawMap["event_id"] = event.EventId
	}
	if event.Timestamp != nil {
		rawMap["timestamp"] = event.Timestamp.AsTime().Format(time.RFC3339Nano)
	}
	if event.Service != "" {
		rawMap["service"] = event.Service
	}
	if event.SchemaVersion != "" {
		rawMap["schema_version"] = event.SchemaVersion
	}
	if event.EventVersion != "" {
		rawMap["event_version"] = event.EventVersion
	}
	if event.Version != "" {
		rawMap["version"] = event.Version
	}
	if event.Environment != "" {
		rawMap["environment"] = event.Environment
	}
	if event.Event != "" {
		rawMap["event"] = event.Event
	}
	if event.Level != lozav1.EventLevel_EVENT_LEVEL_UNSPECIFIED {
		rawMap["level"] = event.Level.String()
	}
	if event.Outcome != lozav1.EventOutcome_EVENT_OUTCOME_UNSPECIFIED {
		rawMap["outcome"] = event.Outcome.String()
	}
	if event.DurationMs != 0 {
		rawMap["duration_ms"] = event.DurationMs
	}
	if event.RequestId != "" {
		rawMap["request_id"] = event.RequestId
	}
	if event.TraceId != "" {
		rawMap["trace_id"] = event.TraceId
	}
	if event.SpanId != "" {
		rawMap["span_id"] = event.SpanId
	}
	if event.TraceFlags != "" {
		rawMap["trace_flags"] = event.TraceFlags
	}
	if event.Release != "" {
		rawMap["release"] = event.Release
	}

	// HTTP context
	if event.Http != nil {
		httpMap := make(map[string]interface{})
		if event.Http.Method != "" {
			httpMap["method"] = event.Http.Method
		}
		if event.Http.Path != "" {
			httpMap["path"] = event.Http.Path
		}
		if event.Http.Route != "" {
			httpMap["route"] = event.Http.Route
		}
		if event.Http.StatusCode != 0 {
			httpMap["status_code"] = event.Http.StatusCode
		}
		if event.Http.ClientIp != "" {
			httpMap["client_ip"] = event.Http.ClientIp
		}
		if event.Http.UserAgent != "" {
			httpMap["user_agent"] = event.Http.UserAgent
		}
		if event.Http.Url != "" {
			httpMap["url"] = event.Http.Url
		}
		if event.Http.Host != "" {
			httpMap["host"] = event.Http.Host
		}
		rawMap["http"] = httpMap
	}

	// Error
	if event.Error != nil {
		errMap := make(map[string]interface{})
		if event.Error.Type != "" {
			errMap["type"] = event.Error.Type
		}
		if event.Error.Message != "" {
			errMap["message"] = event.Error.Message
		}
		if event.Error.Code != "" {
			errMap["code"] = event.Error.Code
		}
		if event.Error.Cause != "" {
			errMap["cause"] = event.Error.Cause
		}
		if event.Error.Stack != "" {
			errMap["stack"] = event.Error.Stack
		}
		rawMap["error"] = errMap
	}

	// User, Tenant, Resource
	if event.User != nil {
		rawMap["user"] = event.User.AsMap()
	}
	if event.Tenant != nil {
		rawMap["tenant"] = event.Tenant.AsMap()
	}
	if event.Resource != nil {
		rawMap["resource"] = event.Resource.AsMap()
	}

	// Attrs
	if event.Attrs != nil {
		rawMap["attrs"] = event.Attrs.AsMap()
	}

	// Errors (repeated)
	if len(event.Errors) > 0 {
		errors := make([]interface{}, 0, len(event.Errors))
		for _, e := range event.Errors {
			errMap := make(map[string]interface{})
			if e.Type != "" {
				errMap["type"] = e.Type
			}
			if e.Message != "" {
				errMap["message"] = e.Message
			}
			if e.Code != "" {
				errMap["code"] = e.Code
			}
			if e.Retriable {
				errMap["retriable"] = true
			}
			if e.Cause != "" {
				errMap["cause"] = e.Cause
			}
			errors = append(errors, errMap)
		}
		rawMap["errors"] = errors
	}

	// Lifecycle: Checkpoints
	if len(event.Checkpoints) > 0 {
		cps := make([]interface{}, 0, len(event.Checkpoints))
		for _, cp := range event.Checkpoints {
			cpMap := map[string]interface{}{"name": cp.Name, "at_ms": cp.AtMs}
			if cp.Attrs != nil {
				cpMap["attrs"] = cp.Attrs.AsMap()
			}
			cps = append(cps, cpMap)
		}
		rawMap["checkpoints"] = cps
	}

	// Lifecycle: Processes
	if len(event.Process) > 0 {
		procs := make([]interface{}, 0, len(event.Process))
		for _, p := range event.Process {
			procMap := map[string]interface{}{
				"step":          p.Step,
				"name":          p.Name,
				"started_at_ms": p.StartedAtMs,
			}
			if p.StatusCode != 0 {
				procMap["status_code"] = p.StatusCode
			}
			if p.EndedAtMs != 0 {
				procMap["ended_at_ms"] = p.EndedAtMs
			}
			if p.DurationMs != 0 {
				procMap["duration_ms"] = p.DurationMs
			}
			if p.Attrs != nil {
				procMap["attrs"] = p.Attrs.AsMap()
			}
			if p.Outcome != "" {
				procMap["outcome"] = p.Outcome
			}
			if p.Error != nil {
				errMap := map[string]interface{}{
					"type":    p.Error.Type,
					"message": p.Error.Message,
				}
				if p.Error.Code != "" {
					errMap["code"] = p.Error.Code
				}
				procMap["error"] = errMap
			}
			procs = append(procs, procMap)
		}
		rawMap["processes"] = procs
	}

	// Lifecycle: Groups
	if len(event.Groups) > 0 {
		grps := make([]interface{}, 0, len(event.Groups))
		for _, g := range event.Groups {
			grpMap := map[string]interface{}{
				"name":          g.Name,
				"started_at_ms": g.StartedAtMs,
			}
			if g.StatusCode != 0 {
				grpMap["status_code"] = g.StatusCode
			}
			if g.EndedAtMs != 0 {
				grpMap["ended_at_ms"] = g.EndedAtMs
			}
			if g.DurationMs != 0 {
				grpMap["duration_ms"] = g.DurationMs
			}
			if g.Attrs != nil {
				grpMap["attrs"] = g.Attrs.AsMap()
			}
			if g.Outcome != "" {
				grpMap["outcome"] = g.Outcome
			}
			if g.Error != nil {
				errMap := map[string]interface{}{
					"type":    g.Error.Type,
					"message": g.Error.Message,
				}
				if g.Error.Code != "" {
					errMap["code"] = g.Error.Code
				}
				grpMap["error"] = errMap
			}
			grps = append(grps, grpMap)
		}
		rawMap["groups"] = grps
	}

	// Lifecycle: Timers
	if len(event.Timers) > 0 {
		timers := make([]interface{}, 0, len(event.Timers))
		for _, t := range event.Timers {
			timerMap := map[string]interface{}{
				"name":        t.Name,
				"duration_ms": t.DurationMs,
			}
			if t.StatusCode != 0 {
				timerMap["status_code"] = t.StatusCode
			}
			if t.Attrs != nil {
				timerMap["attrs"] = t.Attrs.AsMap()
			}
			timers = append(timers, timerMap)
		}
		rawMap["timers"] = timers
	}

	// Lifecycle: Links
	if len(event.Links) > 0 {
		links := make([]interface{}, 0, len(event.Links))
		for _, l := range event.Links {
			linkMap := map[string]interface{}{
				"type":   l.Type,
				"target": l.Target,
			}
			if l.Attrs != nil {
				linkMap["attrs"] = l.Attrs.AsMap()
			}
			links = append(links, linkMap)
		}
		rawMap["links"] = links
	}

	return rawMap, nil
}
