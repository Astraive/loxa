package api

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/eventbus"
	"github.com/astraive/loza/cortex/internal/eventconv"
	"github.com/astraive/loza/cortex/internal/graph"
	"github.com/astraive/loza/cortex/internal/learner"
	"github.com/astraive/loza/cortex/internal/matcher"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/processor"
	"github.com/astraive/loza/cortex/internal/reconstructor"
	"github.com/astraive/loza/cortex/internal/redaction"
	"github.com/astraive/loza/cortex/internal/storage"
	"github.com/astraive/loza/cortex/internal/topology"
	lozav1 "github.com/astraive/loza/gen/go/loza/core"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	lozav1.UnimplementedCortexServiceServer
	config      *config.Config
	processor   *processor.EventProcessor
	bus         *eventbus.EventBus
	topology    *topology.Resolver
	graph       *graph.Builder
	match       matcher.SignatureService
	remediation *learner.Learner
	recon       *reconstructor.IncidentReconstructor
	incidents   storage.IncidentStore
	signatures  storage.SignatureStore
	ready       bool
}

func NewGRPCServer(cfg *config.Config, stor storage.Storage, redactCfg redaction.Config) *GRPCServer {
	topology := topology.NewResolver(stor.Topology())
	graphBuilder := graph.NewBuilder(stor.Graph())
	matching, err := matcher.NewConfiguredSignatureMatcher(stor.Signatures(), cfg.Matcher)
	if err != nil {
		log.Warn().Err(err).Str("mode", cfg.Matcher.Mode).Msg("Matcher mode unavailable, falling back to Go matcher")
		matching = matcher.NewSignatureMatcher(stor.Signatures())
	}
	remediation := learner.NewLearner(stor.Remediations(), stor.Feedback(), stor.Signatures())
	recon := reconstructor.NewIncidentReconstructor(graphBuilder, matching, remediation, stor.Incidents())
	eventProc := processor.NewEventProcessor(stor.Events(), stor.Topology(), stor.Graph())
	if cfg.PIIRedaction.Enabled {
		eventProc = eventProc.WithConfigurableRedaction(redactCfg)
	}

	return &GRPCServer{
		config:      cfg,
		processor:   eventProc,
		bus:         eventbus.New(),
		topology:    topology,
		graph:       graphBuilder,
		match:       matching,
		remediation: remediation,
		recon:       recon,
		incidents:   stor.Incidents(),
		signatures:  stor.Signatures(),
		ready:       true,
	}
}

func (s *GRPCServer) RegisterServer(server *grpc.Server) {
	lozav1.RegisterCortexServiceServer(server, s)
}

func (s *GRPCServer) Healthz(ctx context.Context, req *lozav1.HealthzRequest) (*lozav1.HealthzResponse, error) {
	return &lozav1.HealthzResponse{Status: "OK"}, nil
}

// IngestEvent processes a single event from the full Event proto message.
func (s *GRPCServer) IngestEvent(ctx context.Context, req *lozav1.IngestEventRequest) (*lozav1.IngestEventResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "ingest event request is required")
	}

	event, warnings, err := protoEventToModel(req.Event)
	if err != nil {
		return nil, err
	}
	if event.Provenance == "" {
		event.Provenance = eventconv.NormalizeProvenance("grpc")
	}

	if err := s.processor.ProcessEvent(ctx, event); err != nil {
		return nil, err
	}

	return &lozav1.IngestEventResponse{
		Status:   "accepted",
		EventId:  event.ID,
		Warnings: warnings,
	}, nil
}

// IngestBatch processes multiple events from the full Event proto messages.
func (s *GRPCServer) IngestBatch(ctx context.Context, req *lozav1.IngestBatchRequest) (*lozav1.IngestBatchResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "ingest batch request is required")
	}

	events := make([]*models.Event, 0, len(req.Events))
	var allWarnings []string

	for _, e := range req.Events {
		event, warnings, err := protoEventToModel(e)
		if err != nil {
			log.Warn().Err(err).Msg("Skipping invalid event in batch")
			allWarnings = append(allWarnings, err.Error())
			continue
		}
		if event.Provenance == "" {
			event.Provenance = eventconv.NormalizeProvenance("grpc")
		}
		events = append(events, event)
		allWarnings = append(allWarnings, warnings...)
	}

	if len(events) == 0 {
		return &lozav1.IngestBatchResponse{Status: "empty", Count: 0}, nil
	}

	if err := s.processor.ProcessBatch(ctx, events); err != nil {
		return nil, err
	}

	return &lozav1.IngestBatchResponse{
		Status:   "accepted",
		Count:    int32(len(events)),
		Warnings: allWarnings,
	}, nil
}

// protoEventToModel converts a proto Event message to a models.Event, extracting all lifecycle fields.
func protoEventToModel(pe *lozav1.Event) (*models.Event, []string, error) {
	if pe == nil {
		return nil, nil, status.Error(codes.InvalidArgument, "event is required")
	}

	event := &models.Event{
		ID:            pe.EventId,
		EventID:       pe.EventId,
		Service:       pe.Service,
		SchemaVersion: pe.SchemaVersion,
		EventVersion:  pe.EventVersion,
		Version:       pe.Version,
		Environment:   pe.Environment,
		Event:         pe.Event,
		Level:         levelFromProto(pe.Level),
		Outcome:       outcomeFromProto(pe.Outcome),
		DurationMs:    pe.DurationMs,
		RequestID:     pe.RequestId,
		TraceID:       pe.TraceId,
		SpanID:        pe.SpanId,
		TraceFlags:    pe.TraceFlags,
		Release:       pe.Release,
		Kind:          kindFromProto(pe.Kind),
	}

	if pe.Timestamp != nil {
		event.Timestamp = pe.Timestamp.AsTime()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	// HTTP context
	if pe.Http != nil {
		event.HTTP = &models.HttpContext{
			Method:     pe.Http.Method,
			Path:       pe.Http.Path,
			Route:      pe.Http.Route,
			StatusCode: int(pe.Http.StatusCode),
			ClientIP:   pe.Http.ClientIp,
			UserAgent:  pe.Http.UserAgent,
			URL:        pe.Http.Url,
			Host:       pe.Http.Host,
		}
	}

	// IncidentID
	event.IncidentID = pe.IncidentId

	// Error
	if pe.Error != nil {
		event.Error = &models.EventError{
			Type:    pe.Error.Type,
			Message: pe.Error.Message,
			Code:    pe.Error.Code,
			Cause:   pe.Error.Cause,
			Stack:   pe.Error.Stack,
		}
	}

	// User, Tenant
	if pe.User != nil {
		event.User = pe.User.AsMap()
	}
	if pe.Tenant != nil {
		event.Tenant = pe.Tenant.AsMap()
	}

	// Attrs
	if pe.Attrs != nil {
		event.Attrs = pe.Attrs.AsMap()
	}

	// Lifecycle: Checkpoints
	if len(pe.Checkpoints) > 0 {
		event.Checkpoints = make([]*models.EventCheckpoint, 0, len(pe.Checkpoints))
		for _, cp := range pe.Checkpoints {
			c := &models.EventCheckpoint{Name: cp.Name, AtMs: cp.AtMs}
			if cp.Attrs != nil {
				c.Attrs = cp.Attrs.AsMap()
			}
			event.Checkpoints = append(event.Checkpoints, c)
		}
	}

	// Lifecycle: Processes
	if len(pe.Process) > 0 {
		event.Processes = make([]*models.EventProcess, 0, len(pe.Process))
		for _, p := range pe.Process {
			proc := &models.EventProcess{
				Step:        int(p.Step),
				Name:        p.Name,
				StatusCode:  int(p.StatusCode),
				StartedAtMs: p.StartedAtMs,
				EndedAtMs:   p.EndedAtMs,
				DurationMs:  p.DurationMs,
				Outcome:     p.Outcome,
			}
			if p.Attrs != nil {
				proc.Attrs = p.Attrs.AsMap()
			}
			if p.Error != nil {
				proc.Error = &models.EventError{
					Type:    p.Error.Type,
					Message: p.Error.Message,
					Code:    p.Error.Code,
					Cause:   p.Error.Cause,
					Stack:   p.Error.Stack,
				}
			}
			event.Processes = append(event.Processes, proc)
		}
	}

	// Lifecycle: Groups
	if len(pe.Groups) > 0 {
		event.Groups = make([]*models.EventGroup, 0, len(pe.Groups))
		for _, g := range pe.Groups {
			grp := &models.EventGroup{
				Name:        g.Name,
				StatusCode:  int(g.StatusCode),
				StartedAtMs: g.StartedAtMs,
				EndedAtMs:   g.EndedAtMs,
				DurationMs:  g.DurationMs,
				Outcome:     g.Outcome,
			}
			if g.Attrs != nil {
				grp.Attrs = g.Attrs.AsMap()
			}
			if g.Error != nil {
				grp.Error = &models.EventError{
					Type:    g.Error.Type,
					Message: g.Error.Message,
					Code:    g.Error.Code,
					Cause:   g.Error.Cause,
					Stack:   g.Error.Stack,
				}
			}
			event.Groups = append(event.Groups, grp)
		}
	}

	// Lifecycle: Timers
	if len(pe.Timers) > 0 {
		event.Timers = make([]*models.EventTimer, 0, len(pe.Timers))
		for _, t := range pe.Timers {
			timer := &models.EventTimer{
				Name:       t.Name,
				DurationMs: t.DurationMs,
				StatusCode: int(t.StatusCode),
			}
			if t.Attrs != nil {
				timer.Attrs = t.Attrs.AsMap()
			}
			event.Timers = append(event.Timers, timer)
		}
	}

	// Lifecycle: Links
	if len(pe.Links) > 0 {
		event.Links = make([]*models.EventLink, 0, len(pe.Links))
		for _, l := range pe.Links {
			link := &models.EventLink{
				Type:   l.Type,
				Target: l.Target,
			}
			if l.Attrs != nil {
				link.Attrs = l.Attrs.AsMap()
			}
			event.Links = append(event.Links, link)
		}
	}

	// Store raw fields if present
	if pe.Timestamp != nil || pe.Event != "" || len(pe.Checkpoints) > 0 {
		event.Raw = make(map[string]interface{})
		if pe.Event != "" {
			event.Raw["event"] = pe.Event
		}
		if !event.Timestamp.IsZero() {
			event.Raw["timestamp"] = event.Timestamp.Format(time.RFC3339Nano)
		}
		if len(pe.Checkpoints) > 0 {
			event.Raw["checkpoints"] = pe.Checkpoints
		}
	}

	return event, nil, nil
}

func (s *GRPCServer) Reconstruct(ctx context.Context, req *lozav1.ReconstructRequest) (*lozav1.ReconstructResponse, error) {
	incident, err := s.incidents.Get(ctx, req.IncidentId)
	if err != nil {
		return nil, err
	}

	graphView, _ := s.graph.GetIncidentGraph(ctx, req.IncidentId, 3)

	nodes := make([]*lozav1.GraphNode, 0)
	edges := make([]*lozav1.GraphEdge, 0)

	if graphView != nil {
		for _, n := range graphView.Nodes {
			nodes = append(nodes, &lozav1.GraphNode{
				Id:       n.ID,
				Service:  n.Label,
				NodeType: string(n.Type),
			})
		}
		for _, e := range graphView.Edges {
			edges = append(edges, &lozav1.GraphEdge{
				From:     e.FromNodeID,
				To:       e.ToNodeID,
				EdgeType: string(e.Type),
			})
		}
	}

	return &lozav1.ReconstructResponse{
		IncidentId:       incident.ID,
		Status:           incident.Status,
		Severity:         incident.Severity,
		PrimaryService:   incident.PrimaryService,
		AffectedServices: incident.AffectedServices,
		Nodes:            nodes,
		Edges:            edges,
	}, nil
}

func (s *GRPCServer) GetGraph(ctx context.Context, req *lozav1.GetGraphRequest) (*lozav1.GetGraphResponse, error) {
	var graphView *models.GraphView
	var err error

	if req.Service != "" {
		graphView, err = s.graph.GetServiceGraph(ctx, req.Service, int(req.Depth))
	} else if req.IncidentId != "" {
		graphView, err = s.graph.GetIncidentGraph(ctx, req.IncidentId, int(req.Depth))
	} else {
		return &lozav1.GetGraphResponse{}, nil
	}

	if err != nil {
		return nil, err
	}

	nodes := make([]*lozav1.GraphNode, 0, len(graphView.Nodes))
	edges := make([]*lozav1.GraphEdge, 0, len(graphView.Edges))

	for _, n := range graphView.Nodes {
		nodes = append(nodes, &lozav1.GraphNode{
			Id:       n.ID,
			Service:  n.Label,
			NodeType: string(n.Type),
		})
	}

	for _, e := range graphView.Edges {
		edges = append(edges, &lozav1.GraphEdge{
			From:     e.FromNodeID,
			To:       e.ToNodeID,
			EdgeType: string(e.Type),
		})
	}

	return &lozav1.GetGraphResponse{Nodes: nodes, Edges: edges}, nil
}

func (s *GRPCServer) RecordFeedback(ctx context.Context, req *lozav1.RecordFeedbackRequest) (*lozav1.RecordFeedbackResponse, error) {
	outcomeCode := 200
	if !req.Success {
		outcomeCode = 500
	}

	feedback := &models.RemediationFeedback{
		FeedbackID:      "",
		RemediationID:   req.RemediationId,
		IncidentID:      req.IncidentId,
		OutcomeCode:     outcomeCode,
		OutcomeCategory: models.OutcomeCategory(outcomeCode),
		Notes:           req.Notes,
		Timestamp:       time.Now(),
	}

	if err := s.remediation.RecordFeedback(ctx, feedback); err != nil {
		return nil, err
	}

	return &lozav1.RecordFeedbackResponse{Status: "recorded"}, nil
}

func (s *GRPCServer) StreamEvents(req *lozav1.StreamEventsRequest, stream lozav1.CortexService_StreamEventsServer) error {
	filter := req.IncidentId

	log.Info().Str("filter", filter).Msg("Starting event stream")

	ch := s.bus.Subscribe(filter)
	defer s.bus.Unsubscribe(filter, ch)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			pbEvent := modelToProtoEvent(event)
			if err := stream.Send(pbEvent); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (s *GRPCServer) StartGRPC(addr string, server *grpc.Server) error {
	log.Info().Str("addr", addr).Msg("Starting Cortex gRPC server")
	return nil
}

// modelToProtoEvent converts a models.Event to a proto Event for streaming.
func modelToProtoEvent(event *models.Event) *lozav1.Event {
	pe := &lozav1.Event{
		EventId:       event.ID,
		Event:         event.Event,
		Service:       event.Service,
		SchemaVersion: event.SchemaVersion,
		EventVersion:  event.EventVersion,
		Version:       event.Version,
		Environment:   event.Environment,
		Level:         protoLevel(event.Level),
		Outcome:       protoOutcome(event.Outcome),
		Kind:          protoKind(event.Kind),
		DurationMs:    event.DurationMs,
		RequestId:     event.RequestID,
		TraceId:       event.TraceID,
		SpanId:        event.SpanID,
		TraceFlags:    event.TraceFlags,
		Release:       event.Release,
		IncidentId:    event.IncidentID,
	}

	if !event.Timestamp.IsZero() {
		pe.Timestamp = timestamppb.New(event.Timestamp)
	}

	// HTTP context
	if event.HTTP != nil {
		pe.Http = &lozav1.HttpContext{
			Method:     event.HTTP.Method,
			Path:       event.HTTP.Path,
			Route:      event.HTTP.Route,
			StatusCode: int32(event.HTTP.StatusCode),
			ClientIp:   event.HTTP.ClientIP,
			UserAgent:  event.HTTP.UserAgent,
			Url:        event.HTTP.URL,
			Host:       event.HTTP.Host,
		}
	}

	// Error
	if event.Error != nil {
		pe.Error = &lozav1.ErrorContext{
			Type:    event.Error.Type,
			Message: event.Error.Message,
			Code:    event.Error.Code,
			Cause:   event.Error.Cause,
			Stack:   event.Error.Stack,
		}
	}

	// Build Raw struct for user/tenant/attrs and lifecycle data
	rawMap := make(map[string]interface{})

	if event.User != nil {
		rawMap["user"] = event.User
	}
	if event.Tenant != nil {
		rawMap["tenant"] = event.Tenant
	}
	if event.Attrs != nil {
		rawMap["attrs"] = event.Attrs
	}

	// Lifecycle primitives
	if len(event.Checkpoints) > 0 {
		cps := make([]interface{}, 0, len(event.Checkpoints))
		for _, cp := range event.Checkpoints {
			cpMap := map[string]interface{}{"name": cp.Name, "at_ms": cp.AtMs}
			if cp.Attrs != nil {
				cpMap["attrs"] = cp.Attrs
			}
			cps = append(cps, cpMap)
		}
		rawMap["checkpoints"] = cps
	}
	if len(event.Processes) > 0 {
		procs := make([]interface{}, 0, len(event.Processes))
		for _, p := range event.Processes {
			procMap := map[string]interface{}{
				"step": p.Step, "name": p.Name, "started_at_ms": p.StartedAtMs,
			}
			if p.StatusCode > 0 {
				procMap["status_code"] = p.StatusCode
			}
			if p.EndedAtMs > 0 {
				procMap["ended_at_ms"] = p.EndedAtMs
			}
			if p.DurationMs > 0 {
				procMap["duration_ms"] = p.DurationMs
			}
			if p.Outcome != "" {
				procMap["outcome"] = p.Outcome
			}
			if p.Attrs != nil {
				procMap["attrs"] = p.Attrs
			}
			if p.Error != nil {
				procMap["error"] = map[string]interface{}{
					"type": p.Error.Type, "message": p.Error.Message,
					"code": p.Error.Code, "cause": p.Error.Cause, "stack": p.Error.Stack,
				}
			}
			procs = append(procs, procMap)
		}
		rawMap["processes"] = procs
	}
	if len(event.Groups) > 0 {
		grps := make([]interface{}, 0, len(event.Groups))
		for _, g := range event.Groups {
			grpMap := map[string]interface{}{"name": g.Name, "started_at_ms": g.StartedAtMs}
			if g.StatusCode > 0 {
				grpMap["status_code"] = g.StatusCode
			}
			if g.EndedAtMs > 0 {
				grpMap["ended_at_ms"] = g.EndedAtMs
			}
			if g.DurationMs > 0 {
				grpMap["duration_ms"] = g.DurationMs
			}
			if g.Outcome != "" {
				grpMap["outcome"] = g.Outcome
			}
			if g.Attrs != nil {
				grpMap["attrs"] = g.Attrs
			}
			if g.Error != nil {
				grpMap["error"] = map[string]interface{}{
					"type": g.Error.Type, "message": g.Error.Message,
					"code": g.Error.Code, "cause": g.Error.Cause, "stack": g.Error.Stack,
				}
			}
			grps = append(grps, grpMap)
		}
		rawMap["groups"] = grps
	}
	if len(event.Timers) > 0 {
		timers := make([]interface{}, 0, len(event.Timers))
		for _, t := range event.Timers {
			timerMap := map[string]interface{}{"name": t.Name, "duration_ms": t.DurationMs}
			if t.StatusCode > 0 {
				timerMap["status_code"] = t.StatusCode
			}
			if t.Attrs != nil {
				timerMap["attrs"] = t.Attrs
			}
			timers = append(timers, timerMap)
		}
		rawMap["timers"] = timers
	}
	if len(event.Links) > 0 {
		links := make([]interface{}, 0, len(event.Links))
		for _, l := range event.Links {
			linkMap := map[string]interface{}{"type": l.Type, "target": l.Target}
			if l.Attrs != nil {
				linkMap["attrs"] = l.Attrs
			}
			links = append(links, linkMap)
		}
		rawMap["links"] = links
	}

	if len(rawMap) > 0 {
		st, err := structpb.NewStruct(rawMap)
		if err != nil {
			log.Warn().Err(err).Int("fields", len(rawMap)).Msg("Failed to convert lifecycle/attrs to structpb for streaming")
		} else {
			pe.Attrs = st
		}
	}

	return pe
}

// protoLevel converts a level string to the proto enum.
func protoLevel(level string) lozav1.EventLevel {
	switch level {
	case "debug":
		return lozav1.EventLevel_EVENT_LEVEL_DEBUG
	case "info":
		return lozav1.EventLevel_EVENT_LEVEL_INFO
	case "warn", "warning":
		return lozav1.EventLevel_EVENT_LEVEL_WARN
	case "error":
		return lozav1.EventLevel_EVENT_LEVEL_ERROR
	case "notice":
		return lozav1.EventLevel_EVENT_LEVEL_NOTICE
	case "fatal":
		return lozav1.EventLevel_EVENT_LEVEL_FATAL
	default:
		return lozav1.EventLevel_EVENT_LEVEL_UNSPECIFIED
	}
}

// levelFromProto converts a proto enum back to a lowercase level string.
func levelFromProto(level lozav1.EventLevel) string {
	switch level {
	case lozav1.EventLevel_EVENT_LEVEL_DEBUG:
		return "debug"
	case lozav1.EventLevel_EVENT_LEVEL_INFO:
		return "info"
	case lozav1.EventLevel_EVENT_LEVEL_WARN:
		return "warn"
	case lozav1.EventLevel_EVENT_LEVEL_ERROR:
		return "error"
	case lozav1.EventLevel_EVENT_LEVEL_NOTICE:
		return "notice"
	case lozav1.EventLevel_EVENT_LEVEL_FATAL:
		return "fatal"
	default:
		return ""
	}
}

// outcomeFromProto converts a proto enum back to a lowercase outcome string.
func outcomeFromProto(outcome lozav1.EventOutcome) string {
	switch outcome {
	case lozav1.EventOutcome_EVENT_OUTCOME_SUCCESS:
		return "success"
	case lozav1.EventOutcome_EVENT_OUTCOME_ERROR:
		return "error"
	case lozav1.EventOutcome_EVENT_OUTCOME_PARTIAL:
		return "partial"
	case lozav1.EventOutcome_EVENT_OUTCOME_ABANDONED:
		return "abandoned"
	case lozav1.EventOutcome_EVENT_OUTCOME_RETRIED:
		return "retried"
	case lozav1.EventOutcome_EVENT_OUTCOME_CANCELLED:
		return "cancelled"
	case lozav1.EventOutcome_EVENT_OUTCOME_TIMEOUT:
		return "timeout"
	case lozav1.EventOutcome_EVENT_OUTCOME_SKIPPED:
		return "skipped"
	case lozav1.EventOutcome_EVENT_OUTCOME_REJECTED:
		return "rejected"
	case lozav1.EventOutcome_EVENT_OUTCOME_QUARANTINED:
		return "quarantined"
	default:
		return ""
	}
}

// kindFromProto converts a proto EventKind enum to a models.EventKind.
func kindFromProto(kind lozav1.EventKind) models.EventKind {
	switch kind {
	case lozav1.EventKind_EVENT_KIND_LOG:
		return models.EventKindLog
	case lozav1.EventKind_EVENT_KIND_HTTP:
		return models.EventKindHTTP
	case lozav1.EventKind_EVENT_KIND_JOB:
		return models.EventKindJob
	case lozav1.EventKind_EVENT_KIND_QUEUE:
		return models.EventKindQueue
	case lozav1.EventKind_EVENT_KIND_CLI:
		return models.EventKindCLI
	case lozav1.EventKind_EVENT_KIND_CRON:
		return models.EventKindCron
	case lozav1.EventKind_EVENT_KIND_AGENT:
		return models.EventKindAgent
	case lozav1.EventKind_EVENT_KIND_AI:
		return models.EventKindAI
	default:
		return models.EventKindLozaEvent
	}
}

// protoKind converts a models.EventKind to the proto EventKind enum.
func protoKind(kind models.EventKind) lozav1.EventKind {
	switch kind {
	case models.EventKindLog:
		return lozav1.EventKind_EVENT_KIND_LOG
	case models.EventKindHTTP:
		return lozav1.EventKind_EVENT_KIND_HTTP
	case models.EventKindJob:
		return lozav1.EventKind_EVENT_KIND_JOB
	case models.EventKindQueue:
		return lozav1.EventKind_EVENT_KIND_QUEUE
	case models.EventKindCLI:
		return lozav1.EventKind_EVENT_KIND_CLI
	case models.EventKindCron:
		return lozav1.EventKind_EVENT_KIND_CRON
	case models.EventKindAgent:
		return lozav1.EventKind_EVENT_KIND_AGENT
	case models.EventKindAI:
		return lozav1.EventKind_EVENT_KIND_AI
	default:
		return lozav1.EventKind_EVENT_KIND_EVENT
	}
}

// protoOutcome converts an outcome string to the proto enum.
func protoOutcome(outcome string) lozav1.EventOutcome {
	switch outcome {
	case "success":
		return lozav1.EventOutcome_EVENT_OUTCOME_SUCCESS
	case "error":
		return lozav1.EventOutcome_EVENT_OUTCOME_ERROR
	case "partial":
		return lozav1.EventOutcome_EVENT_OUTCOME_PARTIAL
	case "abandoned":
		return lozav1.EventOutcome_EVENT_OUTCOME_ABANDONED
	case "retried":
		return lozav1.EventOutcome_EVENT_OUTCOME_RETRIED
	case "cancelled":
		return lozav1.EventOutcome_EVENT_OUTCOME_CANCELLED
	case "timeout":
		return lozav1.EventOutcome_EVENT_OUTCOME_TIMEOUT
	case "skipped":
		return lozav1.EventOutcome_EVENT_OUTCOME_SKIPPED
	case "rejected":
		return lozav1.EventOutcome_EVENT_OUTCOME_REJECTED
	case "quarantined":
		return lozav1.EventOutcome_EVENT_OUTCOME_QUARANTINED
	default:
		return lozav1.EventOutcome_EVENT_OUTCOME_UNSPECIFIED
	}
}
