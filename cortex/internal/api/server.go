package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	lozacortex "github.com/astraive/loza/cortex"
	"github.com/astraive/loza/cortex/internal/config"
	"github.com/astraive/loza/cortex/internal/correlation"
	"github.com/astraive/loza/cortex/internal/graph"
	"github.com/astraive/loza/cortex/internal/learner"
	"github.com/astraive/loza/cortex/internal/matcher"
	"github.com/astraive/loza/cortex/internal/middleware"
	"github.com/astraive/loza/cortex/internal/models"
	"github.com/astraive/loza/cortex/internal/processor"
	"github.com/astraive/loza/cortex/internal/reconstructor"
	"github.com/astraive/loza/cortex/internal/redaction"
	"github.com/astraive/loza/cortex/internal/storage"
	"github.com/astraive/loza/cortex/internal/topology"
	"github.com/go-chi/chi/v5"
	httpmiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// cortexVersion is the current version of the cortex service.
var cortexVersion = lozacortex.Version

// maxGraphDepth caps the depth parameter for graph queries to prevent abuse.
const maxGraphDepth = 100

// defaultMaxBodyBytes is the fallback body size limit when config.MaxBodyBytes is 0.
const defaultMaxBodyBytes = 10 * 1024 * 1024 // 10MB

type Server struct {
	config      *config.Config
	processor   *processor.EventProcessor
	analyzer    *correlation.Analyzer
	topology    *topology.Resolver
	graph       *graph.Builder
	match       matcher.SignatureService
	remediation *learner.Learner
	recon       *reconstructor.IncidentReconstructor
	graphql     *GraphQLServer
	storage     storage.Storage
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
		var rlOpts []middleware.RateLimiterOption
		if len(cfg.Server.TrustedProxies) > 0 {
			var cidrs []*net.IPNet
			for _, cidr := range cfg.Server.TrustedProxies {
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					log.Fatal().Err(err).Msgf("invalid trusted_proxies CIDR %q", cidr)
				}
				cidrs = append(cidrs, ipNet)
			}
			rlOpts = append(rlOpts, middleware.WithTrustedProxies(cidrs))
		}
		rateLimit = middleware.NewRateLimiter(cfg.RateLimit.PerAPIKeyRPM, cfg.RateLimit.PerIPRPM, rlOpts...)
	}

	s := &Server{
		config:      cfg,
		processor:   eventProc,
		analyzer:    analyzer,
		topology:    topology,
		graph:       graphBuilder,
		match:       matching,
		remediation: remediation,
		recon:       recon,
		graphql:     graphqlServer,
		storage:     stor,
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

	r.Use(middleware.SecurityHeaders)
	r.Use(httpmiddleware.RequestID)
	r.Use(httpmiddleware.RealIP)
	r.Use(httpmiddleware.Logger)
	r.Use(httpmiddleware.Recoverer)

	bodyLimit := s.config.Server.MaxBodyBytes
	if bodyLimit <= 0 {
		bodyLimit = defaultMaxBodyBytes
	}
	r.Use(bodySizeLimit(bodyLimit))

	// Public endpoints (no auth) -- registered directly on the router before
	// any auth middleware so they are never blocked by authentication.
	r.Get("/healthz", s.Healthz)
	r.Get("/readyz", s.Readyz)
	r.Get("/version", s.Version)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// All protected routes live in a group with auth + rate-limit middleware.
	r.Group(func(rg chi.Router) {
		if s.config.Authentication.Enabled {
			rg.Use(s.auth.Middleware)
		}

		if s.rateLimit != nil {
			rg.Use(s.rateLimit.Middleware)
		}

		// Per-route role checks (pass-through when auth is nil)
		requireWriter := func(next http.Handler) http.Handler { return next }
		requireReader := func(next http.Handler) http.Handler { return next }
		if s.auth != nil {
			requireWriter = s.auth.RequireRole("writer")
			requireReader = s.auth.RequireRole("reader")
		}

		// Ingest endpoints: writer role required
		rg.With(requireWriter).Post("/events", s.IngestEvent)
		rg.With(requireWriter).Post("/events/batch", s.IngestBatch)
		rg.With(requireWriter).Post("/events/jsonl", s.IngestJSONL)

		// Reconstruct endpoints: reader role required
		rg.With(requireReader).Post("/reconstruct", s.Reconstruct)
		rg.With(requireReader).Post("/incidents/{incident_id}/reconstruct", s.ReconstructIncident)

		// Feedback endpoints: writer role required
		rg.With(requireWriter).Post("/feedback/remediation", s.RecordRemediation)
		rg.With(requireWriter).Post("/feedback/incident", s.RecordIncidentFeedback)

		// Graph endpoints: reader role required
		rg.With(requireReader).Get("/graph/service/{service}", s.ServiceGraph)
		rg.With(requireReader).Get("/graph/incident/{incident_id}", s.IncidentGraph)

		// GraphQL and WebSocket: reader role required
		rg.With(requireReader).Handle("/graphql", s.graphql.Handler())
		rg.With(requireReader).Handle("/ws", s.WebSocketHandler())
	})

	return r
}

func (s *Server) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		log.Warn().Err(err).Msg("failed to write healthz response")
	}
}

func (s *Server) Readyz(w http.ResponseWriter, r *http.Request) {
	ready := true
	checks := map[string]string{}

	// Check if graph engine is initialized
	if s.graph == nil {
		ready = false
		checks["graph"] = "not initialized"
	}

	// Check if processor is initialized
	if s.processor == nil {
		ready = false
		checks["processor"] = "not initialized"
	}

	// Verify storage is responsive with a lightweight query.
	if s.incidents == nil {
		ready = false
		checks["storage"] = "not initialized"
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if _, err := s.incidents.List(ctx, 1, 0); err != nil {
			ready = false
			checks["storage"] = "unavailable"
			log.Error().
				Err(err).
				Str("event.name", "cortex.readiness").
				Str("event.kind", "request").
				Str("event.outcome", "error").
				Msg("readiness storage check failed")
		}
	}

	status := "ready"
	code := http.StatusOK
	if !ready {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"checks": checks,
	}); err != nil {
		log.Warn().Err(err).Msg("failed to write readyz response")
	}
}

func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"version": cortexVersion}); err != nil {
		log.Warn().Err(err).Msg("failed to write version response")
	}
}

func (s *Server) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.processor.ProcessEvent(r.Context(), &event); err != nil {
		log.Warn().Err(err).Msg("event ingestion failed")
		writeIngestionError(w, err)
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
		log.Error().Err(err).Msg("batch ingestion failed")
		writeIngestionError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "accepted"}); err != nil {
		log.Error().Err(err).Msg("failed to encode ingest response")
	}
}

func (s *Server) IngestJSONL(w http.ResponseWriter, r *http.Request) {
	if err := s.processor.ProcessJSONL(r.Context(), r.Body); err != nil {
		log.Error().Err(err).Msg("JSONL ingestion failed")
		writeIngestionError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "accepted"}); err != nil {
		log.Error().Err(err).Msg("failed to encode ingest response")
	}
}

func writeIngestionError(w http.ResponseWriter, err error) {
	if errors.Is(err, processor.ErrInvalidEvent) {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
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
		log.Warn().Err(err).Msg("reconstruct failed")
		http.Error(w, "not found", http.StatusNotFound)
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
		log.Warn().Err(err).Str("incident_id", incidentID).Msg("reconstruct incident failed")
		http.Error(w, "not found", http.StatusNotFound)
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
		log.Error().Err(err).Msg("record remediation failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
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
		log.Error().Err(err).Msg("record incident feedback failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "recorded"}); err != nil {
		log.Error().Err(err).Msg("failed to encode remediation response")
	}
}

func graphLookupStatus(err error) int {
	if errors.Is(err, storage.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func graphDepth(r *http.Request) (int, error) {
	const defaultDepth = 3
	raw := r.URL.Query().Get("depth")
	if raw == "" {
		return defaultDepth, nil
	}
	depth, err := strconv.Atoi(raw)
	if err != nil || depth < 1 || depth > maxGraphDepth {
		return 0, fmt.Errorf("depth must be between 1 and %d", maxGraphDepth)
	}
	return depth, nil
}

func (s *Server) ServiceGraph(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	depth, err := graphDepth(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	graphView, err := s.graph.GetServiceGraph(r.Context(), service, depth)
	if err != nil {
		if graphLookupStatus(err) == http.StatusNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Error().
			Err(err).
			Str("event.name", "cortex.service_graph").
			Str("event.kind", "request").
			Str("event.outcome", "error").
			Str("service", service).
			Msg("service graph lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(graphView); err != nil {
		log.Error().Err(err).Msg("failed to encode graph response")
	}
}

func (s *Server) IncidentGraph(w http.ResponseWriter, r *http.Request) {
	incidentID := chi.URLParam(r, "incident_id")
	depth, err := graphDepth(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	graphView, err := s.graph.GetIncidentGraph(r.Context(), incidentID, depth)
	if err != nil {
		if graphLookupStatus(err) == http.StatusNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Error().
			Err(err).
			Str("event.name", "cortex.incident_graph").
			Str("event.kind", "request").
			Str("event.outcome", "error").
			Str("incident.id", incidentID).
			Msg("incident graph lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
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
