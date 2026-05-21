package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	IngestEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_ingest_events_total",
			Help: "Total number of events ingested",
		},
		[]string{"kind", "status"},
	)

	IngestValidationErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cortex_ingest_validation_errors_total",
			Help: "Total number of validation errors during ingestion",
		},
	)

	ReconstructDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cortex_reconstruct_duration_seconds",
			Help:    "Time taken for incident reconstruction",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"mode"},
	)

	ReconstructTimeouts = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cortex_reconstruct_timeouts_total",
			Help: "Total number of graph traversal timeouts during reconstruction",
		},
	)

	MatcherErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cortex_matcher_errors_total",
			Help: "Total number of signature matcher errors",
		},
	)

	MatcherCacheRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_matcher_cache_requests_total",
			Help: "Total number of matcher cache requests",
		},
		[]string{"result"},
	)

	MatcherCandidateBatch = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "cortex_matcher_candidate_batch_size",
			Help:    "Number of candidate signatures evaluated per similar-incident lookup",
			Buckets: prometheus.DefBuckets,
		},
	)

	AuthFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_auth_failures_total",
			Help: "Total number of authentication failures",
		},
		[]string{"reason"},
	)

	GraphTraversalDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "cortex_graph_traversal_duration_seconds",
			Help:    "Time taken for graph traversal",
			Buckets: prometheus.DefBuckets,
		},
	)

	EventsProcessed = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "cortex_events_batch_size",
			Help:    "Number of events processed in a batch",
			Buckets: prometheus.DefBuckets,
		},
	)

	CollectorBridgeLagSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cortex_collector_bridge_lag_seconds",
			Help: "Current lag between wall clock and the newest successfully processed collector event",
		},
	)

	CollectorBridgeReconnects = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_collector_bridge_reconnects_total",
			Help: "Number of collector bridge reconnect cycles",
		},
		[]string{"transport", "reason"},
	)

	CollectorBridgeFlushes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_collector_bridge_flushes_total",
			Help: "Number of collector bridge batch flush attempts",
		},
		[]string{"result"},
	)

	CollectorBridgeBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "cortex_collector_bridge_batch_size",
			Help:    "Batch size flushed by the collector source-of-truth bridge",
			Buckets: prometheus.DefBuckets,
		},
	)

	CollectorBridgeBackpressure = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cortex_collector_bridge_backpressure_total",
			Help: "Number of times the collector bridge had to wait because the live tail buffer was full",
		},
	)
)
