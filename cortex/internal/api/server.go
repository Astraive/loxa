package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/astraive/loxa/loxa-cortex/internal/config"
	"github.com/astraive/loxa/loxa-cortex/internal/correlation"
	"github.com/astraive/loxa/loxa-cortex/internal/graph"
	"github.com/astraive/loxa/loxa-cortex/internal/learner"
	"github.com/astraive/loxa/loxa-cortex/internal/matcher"
	"github.com/astraive/loxa/loxa-cortex/internal/middleware"
	"github.com/astraive/loxa/loxa-cortex/internal/models"
	"github.com/astraive/loxa/loxa-cortex/internal/processor"
	"github.com/astraive/loxa/loxa-cortex/internal/reconstructor"
	"github.com/astraive/loxa/loxa-cortex/internal/redaction"
	"github.com/astraive/loxa/loxa-cortex/internal/storage"
	"github.com/astraive/loxa/loxa-cortex/internal/topology"
	"github.com/go-chi/chi/v5"
	httpmiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

type Server struct {
	config      *config.Config
	processor   *processor.EventProcessor
	asyncProc   *processor.AsyncProcessor
	analyzer    *correlation.Analyzer
	topology    *topology.Resolver
	graph       *graph.Builder
	match       matcher.SignatureService
	remediation *learner.Learner
	recon       *reconstructor.IncidentReconstructor
	graphql     *GraphQLServer
	incidents   storage.IncidentStore
	signatures  storage.SignatureStore
	auth        *middleware.AuthMiddleware
	rateLimit   *middleware.RateLimiter
	ready       bool
}

func NewServer(cfg *config.Config, stor storage.Storage) *Server {
	topology := topology.NewResolver(stor.Topology())
	graphBuilder := graph.NewBuilder(stor.Graph())
	matching, err := matcher.NewConfiguredSignatureMatcher(stor.Signatures(), cfg.Matcher)
	if err != nil {
		log.Warn().Err(err).Str("mode", cfg.Matcher.Mode).Msg("Matcher mode unavailable, falling back to Go matcher")
		matching = matcher.NewSignatureMatcher(stor.Signatures())
	}
	remediation := learner.NewLearner(stor.Remediations(), stor.Feedback(), stor.Signatures()).
		WithConfig(cfg.Learner.LearningRate, cfg.Learner.FeatureWeightMin, cfg.Learner.FeatureWeightMax)

	recon := reconstructor.NewIncidentReconstructor(graphBuilder, matching, remediation, stor.Incidents()).
		WithConfig(
			cfg.Reconstructor.Fast.MaxDepth, cfg.Reconstructor.Fast.MaxEvents, cfg.Reconstructor.Fast.TimeWindow,
			cfg.Reconstructor.Deep.MaxDepth, cfg.Reconstructor.Deep.MaxEvents, cfg.Reconstructor.Deep.TimeWindow,
			reconstructor.ConfidenceWeights{
				CausalChainBonus:  cfg.Reconstructor.Confidence.CausalChainBonus,
				SymptomBonus:      cfg.Reconstructor.Confidence.SymptomBonus,
				SimilarityWeight:  cfg.Reconstructor.Confidence.SimilarityWeight,
				RemediationWeight: cfg.Reconstructor.Confidence.RemediationWeight,
				MaxConfidence:     cfg.Reconstructor.Confidence.MaxConfidence,
				MinConfidence:     cfg.Reconstructor.Confidence.MinConfidence,
			},
		)

	graphBuilder.WithDefaultMaxDepth(cfg.Reconstructor.Graph.DefaultMaxDepth)

	eventProc := processor.NewEventProcessor(stor.Events(), stor.Topology(), stor.Graph())
	if cfg.PIIRedaction.Enabled {
		eventProc.WithRedactor(redaction.NewWithConfig(redaction.Config{
			Mode:      redaction.Mode(cfg.PIIRedaction.Mode),
			Blocklist: cfg.PIIRedaction.Blocklist,
			Allowlist: cfg.PIIRedaction.Allowlist,
		}))
	}

	authMiddleware := middleware.NewAuthMiddleware(cfg.Authentication)
	graphqlServer := NewGraphQLServer(cfg, stor)

	asyncProc := processor.NewAsyncProcessor(eventProc,
		cfg.Ingestion.AsyncWorkers, cfg.Ingestion.ChannelSize, cfg.Ingestion.MicroBatchSize)

	corrCfg := correlation.FromConfig(
		cfg.Correlation.Enabled,
		cfg.Correlation.AnalysisInterval,
		cfg.Correlation.CoOccurrenceWindow,
		cfg.Correlation.DeploymentAdjacencyWindow,
		cfg.Correlation.MinCoOccurrenceCount,
	)
	analyzer := correlation.NewAnalyzer(corrCfg, stor.Events(), stor.Graph())

	var rateLimit *middleware.RateLimiter
	if cfg.RateLimit.Enabled {
		rateLimit = middleware.NewRateLimiter(cfg.RateLimit.PerAPIKeyRPM, cfg.RateLimit.PerIPRPM)
	}

	s := &Server{
		config:      cfg,
		processor:   eventProc,
		asyncProc:   asyncProc,
		analyzer:    analyzer,
		topology:    topology,
		graph:       graphBuilder,
		match:       matching,
		remediation: remediation,
		recon:       recon,
		graphql:     graphqlServer,
		incidents:   stor.Incidents(),
		signatures:  stor.Signatures(),
		auth:        authMiddleware,
		rateLimit:   rateLimit,
		ready:       true,
	}

	return s
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(httpmiddleware.RequestID)
	r.Use(httpmiddleware.RealIP)
	r.Use(httpmiddleware.Logger)
	r.Use(httpmiddleware.Recoverer)

	if s.config.Server.MaxBodyBytes > 0 {
		r.Use(bodySizeLimit(s.config.Server.MaxBodyBytes))
	}

	r.Get("/healthz", s.Healthz)
	r.Get("/readyz", s.Readyz)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	if s.config.Authentication.Enabled {
		r.Use(s.auth.Middleware)
	}

	if s.rateLimit != nil {
		r.Use(s.rateLimit.Middleware)
	}

	r.Route("/v1", func(r chi.Router) {
		r.Post("/events", s.IngestEvent)
		r.Post("/events/batch", s.IngestBatch)
		r.Post("/events/jsonl", s.IngestJSONL)

		r.Post("/reconstruct", s.Reconstruct)
		r.Post("/incidents/{incident_id}/reconstruct", s.ReconstructIncident)

		r.Post("/feedback/remediation", s.RecordRemediation)
		r.Post("/feedback/incident", s.RecordIncidentFeedback)

		r.Get("/graph/service/{service}", s.ServiceGraph)
		r.Get("/graph/incident/{incident_id}", s.IncidentGraph)
	})

	r.Handle("/graphql", s.graphql.Handler())
	r.Handle("/v1/ws", s.WebSocketHandler())

	return r
}

func (s *Server) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Warn().Err(err).Msg("failed to write healthz response")
	}
}

func (s *Server) Readyz(w http.ResponseWriter, r *http.Request) {
	if s.ready {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Warn().Err(err).Msg("failed to write readyz response")
		}
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err := w.Write([]byte("NOT READY")); err != nil {
			log.Warn().Err(err).Msg("failed to write not ready response")
		}
	}
}

func (s *Server) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.processor.ProcessEvent(r.Context(), &event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "accepted"}); err != nil {
		log.Error().Err(err).Msg("failed to encode ingest response")
	}
}

func (s *Server) IngestBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Events []models.Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	events := make([]*models.Event, len(req.Events))
	for i := range req.Events {
		events[i] = &req.Events[i]
	}

	if err := s.processor.ProcessBatch(r.Context(), events); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "accepted"}); err != nil {
		log.Error().Err(err).Msg("failed to encode ingest response")
	}
}

func (s *Server) IngestJSONL(w http.ResponseWriter, r *http.Request) {
	if err := s.processor.ProcessJSONL(r.Context(), r.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "accepted"}); err != nil {
		log.Error().Err(err).Msg("failed to encode ingest response")
	}
}

type ReconstructRequest struct {
	IncidentID string `json:"incident_id"`
	Mode       string `json:"mode"`
}

func (s *Server) Reconstruct(w http.ResponseWriter, r *http.Request) {
	var req ReconstructRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var ctx *models.IncidentContext
	var err error

	if req.Mode == "deep" {
		ctx, err = s.recon.ReconstructDeep(r.Context(), req.IncidentID)
	} else {
		ctx, err = s.recon.ReconstructFast(r.Context(), req.IncidentID)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ctx); err != nil {
		log.Error().Err(err).Msg("failed to encode reconstruct response")
	}
}

func (s *Server) ReconstructIncident(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "incident_id")

	mode := r.URL.Query().Get("mode")
	var ctx *models.IncidentContext
	var err error

	if mode == "deep" {
		ctx, err = s.recon.ReconstructDeep(r.Context(), incidentID)
	} else {
		ctx, err = s.recon.ReconstructFast(r.Context(), incidentID)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(ctx); err != nil {
		log.Error().Err(err).Msg("failed to encode reconstruct response")
	}
}

func (s *Server) RecordRemediation(w http.ResponseWriter, r *http.Request) {
	var remediation models.Remediation
	if err := json.NewDecoder(r.Body).Decode(&remediation); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.remediation.RecordRemediation(r.Context(), &remediation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "recorded"}); err != nil {
		log.Error().Err(err).Msg("failed to encode remediation response")
	}
}

func (s *Server) RecordIncidentFeedback(w http.ResponseWriter, r *http.Request) {
	var feedback models.RemediationFeedback
	if err := json.NewDecoder(r.Body).Decode(&feedback); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.remediation.RecordFeedback(r.Context(), &feedback); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "recorded"}); err != nil {
		log.Error().Err(err).Msg("failed to encode remediation response")
	}
}

func (s *Server) ServiceGraph(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	depth := 3
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			depth = parsed
		}
	}

	graphView, err := s.graph.GetServiceGraph(r.Context(), service, depth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(graphView); err != nil {
		log.Error().Err(err).Msg("failed to encode graph response")
	}
}

func (s *Server) IncidentGraph(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "incident_id")
	depth := 3
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			depth = parsed
		}
	}

	graphView, err := s.graph.GetIncidentGraph(r.Context(), incidentID, depth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(graphView); err != nil {
		log.Error().Err(err).Msg("failed to encode graph response")
	}
}

func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Msg("Starting Cortex server")

	// Start correlation analyzer as background goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if s.analyzer != nil {
		go s.analyzer.Run(ctx)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return srv.ListenAndServe()
}

func StartServer(cfg *config.Config, stor storage.Storage) error {
	srv := NewServer(cfg, stor)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	return srv.Start(addr)
}

func bodySizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

var _ io.Closer = (*Server)(nil)

func (s *Server) Close() error {
	if s.asyncProc != nil {
		s.asyncProc.Stop()
	}
	return nil
}
