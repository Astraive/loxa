package api

import (
	"context"
	"time"

	loxav1 "github.com/astraive/loxa-spec/proto/loxa/v1"
	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/eventbus"
	"github.com/astraive/loxa/loxa-cortex/internal/eventconv"
	"github.com/astraive/loxa/loxa-cortex/internal/graph"
	"github.com/astraive/loxa/loxa-cortex/internal/learner"
	"github.com/astraive/loxa/loxa-cortex/internal/matcher"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/processor"
	"github.com/astraive/loxa/loxa-cortex/internal/reconstructor"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
	"github.com/astraive/loxa/loxa-cortex/internal/topology"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
)

type GRPCServer struct {
	loxav1.UnimplementedCortexServiceServer
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

func NewGRPCServer(cfg *config.Config, stor storage.Storage) *GRPCServer {
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

	return &GRPCServer{
		config:      cfg,
		processor:   eventProc,
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
	loxav1.RegisterCortexServiceServer(server, s)
}

func (s *GRPCServer) Healthz(ctx context.Context, req *loxav1.HealthzRequest) (*loxav1.HealthzResponse, error) {
	return &loxav1.HealthzResponse{Status: "OK"}, nil
}

func (s *GRPCServer) IngestEvent(ctx context.Context, req *loxav1.IngestEventRequest) (*loxav1.IngestEventResponse, error) {
	event := &models.Event{
		ID:         req.Id,
		Timestamp:  req.Timestamp.AsTime(),
		Kind:       eventconv.NormalizeEventKind(req.Kind),
		Service:    req.Service,
		TraceID:    req.TraceId,
		IncidentID: req.IncidentId,
		Provenance: eventconv.NormalizeProvenance(req.Provenance),
		CreatedAt:  time.Now(),
	}

	if req.Raw != nil {
		event.Raw = req.Raw.AsMap()
	}

	if err := s.processor.ProcessEvent(ctx, event); err != nil {
		return nil, err
	}

	return &loxav1.IngestEventResponse{Status: "accepted"}, nil
}

func (s *GRPCServer) IngestBatch(ctx context.Context, req *loxav1.IngestBatchRequest) (*loxav1.IngestBatchResponse, error) {
	events := make([]*models.Event, len(req.Events))
	for i, e := range req.Events {
		event := &models.Event{
			ID:         e.Id,
			Timestamp:  e.Timestamp.AsTime(),
			Kind:       eventconv.NormalizeEventKind(e.Kind),
			Service:    e.Service,
			TraceID:    e.TraceId,
			IncidentID: e.IncidentId,
			Provenance: eventconv.NormalizeProvenance(e.Provenance),
			CreatedAt:  time.Now(),
		}
		if e.Raw != nil {
			event.Raw = e.Raw.AsMap()
		}
		events[i] = event
	}

	if err := s.processor.ProcessBatch(ctx, events); err != nil {
		return nil, err
	}

	return &loxav1.IngestBatchResponse{
		Status: "accepted",
		Count:  int32(len(events)),
	}, nil
}

func (s *GRPCServer) Reconstruct(ctx context.Context, req *loxav1.ReconstructRequest) (*loxav1.ReconstructResponse, error) {
	incident, err := s.incidents.Get(ctx, req.IncidentId)
	if err != nil {
		return nil, err
	}

	graphView, _ := s.graph.GetIncidentGraph(ctx, req.IncidentId, 3)

	nodes := make([]*loxav1.GraphNode, 0)
	edges := make([]*loxav1.GraphEdge, 0)

	if graphView != nil {
		for _, n := range graphView.Nodes {
			nodes = append(nodes, &loxav1.GraphNode{
				Id:       n.ID,
				Service:  n.Label,
				NodeType: string(n.Type),
			})
		}
		for _, e := range graphView.Edges {
			edges = append(edges, &loxav1.GraphEdge{
				From:     e.FromNodeID,
				To:       e.ToNodeID,
				EdgeType: string(e.Type),
			})
		}
	}

	return &loxav1.ReconstructResponse{
		IncidentId:       incident.ID,
		Status:           incident.Status,
		Severity:         incident.Severity,
		PrimaryService:   incident.PrimaryService,
		AffectedServices: incident.AffectedServices,
		Nodes:            nodes,
		Edges:            edges,
	}, nil
}

func (s *GRPCServer) GetGraph(ctx context.Context, req *loxav1.GetGraphRequest) (*loxav1.GetGraphResponse, error) {
	var graphView *models.GraphView
	var err error

	if req.Service != "" {
		graphView, err = s.graph.GetServiceGraph(ctx, req.Service, int(req.Depth))
	} else if req.IncidentId != "" {
		graphView, err = s.graph.GetIncidentGraph(ctx, req.IncidentId, int(req.Depth))
	} else {
		return &loxav1.GetGraphResponse{}, nil
	}

	if err != nil {
		return nil, err
	}

	nodes := make([]*loxav1.GraphNode, 0, len(graphView.Nodes))
	edges := make([]*loxav1.GraphEdge, 0, len(graphView.Edges))

	for _, n := range graphView.Nodes {
		nodes = append(nodes, &loxav1.GraphNode{
			Id:       n.ID,
			Service:  n.Label,
			NodeType: string(n.Type),
		})
	}

	for _, e := range graphView.Edges {
		edges = append(edges, &loxav1.GraphEdge{
			From:     e.FromNodeID,
			To:       e.ToNodeID,
			EdgeType: string(e.Type),
		})
	}

	return &loxav1.GetGraphResponse{Nodes: nodes, Edges: edges}, nil
}

func (s *GRPCServer) RecordFeedback(ctx context.Context, req *loxav1.RecordFeedbackRequest) (*loxav1.RecordFeedbackResponse, error) {
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

	return &loxav1.RecordFeedbackResponse{Status: "recorded"}, nil
}

func (s *GRPCServer) StreamEvents(req *loxav1.StreamEventsRequest, stream loxav1.CortexService_StreamEventsServer) error {
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
			pbEvent := &loxav1.IngestEventRequest{
				Id:         event.ID,
				Timestamp:  nil, // would need timestamppb.New()
				Kind:       string(event.Kind),
				Service:    event.Service,
				TraceId:    event.TraceID,
				IncidentId: event.IncidentID,
				Provenance: event.Provenance,
			}
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
