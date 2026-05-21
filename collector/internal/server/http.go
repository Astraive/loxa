package server

import (
	"context"
	"crypto/subtle"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

type HTTPServer struct {
	cfg     HTTPConfig
	state   State
	server  *http.Server
	ready   atomic.Bool
	limiter *rate.Limiter
}

func NewHTTPServer(cfg HTTPConfig, state State) *HTTPServer {
	return &HTTPServer{
		cfg:   cfg,
		state: state,
	}
}

func (s *HTTPServer) Name() string { return "http" }

func (s *HTTPServer) Addr() string { return s.cfg.Addr }

func (s *HTTPServer) IsReady() bool { return s.ready.Load() }

func (s *HTTPServer) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}

	s.limiter = rate.NewLimiter(rate.Inf, 0)
	if s.cfg.RateLimitEnabled {
		s.limiter = rate.NewLimiter(rate.Limit(s.cfg.RateLimitRPS), s.cfg.RateLimitBurst)
	}

	mux := http.NewServeMux()
	if s.cfg.IngestPath != "" {
		mux.HandleFunc("POST "+s.cfg.IngestPath, s.handleIngest)
	}
	if s.cfg.HealthPath != "" {
		mux.HandleFunc("GET "+s.cfg.HealthPath, s.handleHealth)
	}
	if s.cfg.ReadyPath != "" {
		mux.HandleFunc("GET "+s.cfg.ReadyPath, s.handleReady)
	}
	if s.cfg.MetricsEnabled && s.cfg.MetricsPath != "" {
		mux.Handle(s.cfg.MetricsPath, promhttp.Handler())
	}

	maxHeaderBytes := 1 << 20
	if s.cfg.MaxHeaderBytes > 0 {
		maxHeaderBytes = int(s.cfg.MaxHeaderBytes)
	}

	s.server = &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		MaxHeaderBytes:   maxHeaderBytes,
		IdleTimeout:      s.cfg.IdleTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	s.ready.Store(true)
	return s.server.ListenAndServe()
}

func (s *HTTPServer) Stop(ctx context.Context) error {
	s.ready.Store(false)
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) handleIngest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AuthEnabled {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get(s.cfg.AuthHeader)), []byte(s.cfg.AuthValue)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if s.cfg.RateLimitEnabled && !s.limiter.Allow() {
		http.Error(w, "Rate Limited", http.StatusTooManyRequests)
		return
	}

	if r.ContentLength > s.cfg.MaxBodyBytes && s.cfg.MaxBodyBytes > 0 {
		http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
		return
	}

	events, err := parseNDJSONEvents(r.Body, s.cfg.MaxBodyBytes)
	if err != nil {
		http.Error(w, "Bad Request: "+err.Error(), http.StatusBadRequest)
		return
	}

	accepted, err := s.state.Ingest(r.Context(), events)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"accepted":` + itoa(accepted) + `}`))
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.state.IsHealthy() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy"}`))
	}
}

func (s *HTTPServer) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.state.IsReady() {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
	}
}