package core

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector implements Prometheus metrics for the SDK.
// Requirements: 49.1, 49.2, 49.3, 49.4, 49.5, 49.6, 49.7, 49.8, 49.9, 49.10
type MetricsCollector struct {
	// Counters
	eventsCreated  prometheus.Counter
	eventsFinished prometheus.Counter
	eventsEmitted  *prometheus.CounterVec
	eventsDropped  *prometheus.CounterVec
	retryTotal     *prometheus.CounterVec
	backpressure   prometheus.Counter

	// Gauges
	bufferSize     prometheus.Gauge
	bufferCapacity prometheus.Gauge

	// Histograms
	emitDuration prometheus.Histogram

	// Registry
	registry *prometheus.Registry

	// Internal state for buffer tracking
	currentBufferSize atomic.Int64
	maxBufferSize     int64
}

// NewMetricsCollector creates a new Prometheus metrics collector for the SDK.
// Requirements: 49.10
func NewMetricsCollector(namespace string, maxBufferSize int) *MetricsCollector {
	if namespace == "" {
		namespace = "loxa_sdk"
	}

	registry := prometheus.NewRegistry()

	mc := &MetricsCollector{
		registry:      registry,
		maxBufferSize: int64(maxBufferSize),
	}

	// Counter: events_created_total
	// Requirement: 49.1
	mc.eventsCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "events_created_total",
		Help:      "Total number of events created via StartEvent",
	})

	// Counter: events_finished_total
	// Requirement: 49.2
	mc.eventsFinished = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "events_finished_total",
		Help:      "Total number of events finished via Finish/FinishError",
	})

	// Counter: events_emitted_total with status label
	// Requirement: 49.3
	mc.eventsEmitted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_emitted_total",
			Help:      "Total number of events emitted to collector",
		},
		[]string{"status"},
	)

	// Counter: events_dropped_total with reason label
	// Requirement: 49.4
	mc.eventsDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "events_dropped_total",
			Help:      "Total number of events dropped",
		},
		[]string{"reason"},
	)

	// Histogram: emit_duration_seconds
	// Requirement: 49.5
	mc.emitDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "emit_duration_seconds",
		Help:      "Duration of Emit operations in seconds",
		Buckets:   prometheus.DefBuckets,
	})

	// Gauge: buffer_size
	// Requirement: 49.6
	mc.bufferSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "buffer_size",
		Help:      "Current number of events in buffer",
	})

	// Gauge: buffer_capacity
	// Requirement: 49.7
	mc.bufferCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "buffer_capacity",
		Help:      "Maximum buffer size",
	})
	mc.bufferCapacity.Set(float64(maxBufferSize))

	// Counter: retry_total with attempt label
	// Requirement: 49.8
	mc.retryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retry_total",
			Help:      "Total number of retry attempts",
		},
		[]string{"attempt"},
	)

	// Counter: backpressure_total
	// Requirement: 49.9
	mc.backpressure = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "backpressure_total",
		Help:      "Total number of backpressure events (429/503 responses)",
	})

	// Register all metrics
	registry.MustRegister(
		mc.eventsCreated,
		mc.eventsFinished,
		mc.eventsEmitted,
		mc.eventsDropped,
		mc.emitDuration,
		mc.bufferSize,
		mc.bufferCapacity,
		mc.retryTotal,
		mc.backpressure,
	)

	return mc
}

// OnEventCreated increments the events_created_total counter.
// Requirement: 49.1
func (mc *MetricsCollector) OnEventCreated() {
	if mc != nil && mc.eventsCreated != nil {
		mc.eventsCreated.Inc()
		mc.incrementBufferSize()
	}
}

// OnEventFinished increments the events_finished_total counter.
// Requirement: 49.2
func (mc *MetricsCollector) OnEventFinished() {
	if mc != nil && mc.eventsFinished != nil {
		mc.eventsFinished.Inc()
	}
}

// OnEventEmitted increments the events_emitted_total counter with status label.
// Requirement: 49.3
func (mc *MetricsCollector) OnEventEmitted(success bool) {
	if mc != nil && mc.eventsEmitted != nil {
		status := "success"
		if !success {
			status = "failure"
		}
		mc.eventsEmitted.WithLabelValues(status).Inc()
		mc.decrementBufferSize()
	}
}

// OnEventDropped increments the events_dropped_total counter with reason label.
// Requirement: 49.4
func (mc *MetricsCollector) OnEventDropped(reason string) {
	if mc != nil && mc.eventsDropped != nil {
		mc.eventsDropped.WithLabelValues(reason).Inc()
		mc.decrementBufferSize()
	}
}

// ObserveEmitDuration records the duration of an Emit operation.
// Requirement: 49.5
func (mc *MetricsCollector) ObserveEmitDuration(duration time.Duration) {
	if mc != nil && mc.emitDuration != nil {
		mc.emitDuration.Observe(duration.Seconds())
	}
}

// OnRetry increments the retry_total counter with attempt label.
// Requirement: 49.8
func (mc *MetricsCollector) OnRetry(attempt int) {
	if mc != nil && mc.retryTotal != nil {
		mc.retryTotal.WithLabelValues(strconv.Itoa(attempt)).Inc()
	}
}

// OnBackpressure increments the backpressure_total counter.
// Requirement: 49.9
func (mc *MetricsCollector) OnBackpressure() {
	if mc != nil && mc.backpressure != nil {
		mc.backpressure.Inc()
	}
}

// incrementBufferSize increments the buffer size gauge.
// Requirement: 49.6
func (mc *MetricsCollector) incrementBufferSize() {
	if mc != nil && mc.bufferSize != nil {
		newSize := mc.currentBufferSize.Add(1)
		mc.bufferSize.Set(float64(newSize))
	}
}

// decrementBufferSize decrements the buffer size gauge.
// Requirement: 49.6
func (mc *MetricsCollector) decrementBufferSize() {
	if mc != nil && mc.bufferSize != nil {
		newSize := mc.currentBufferSize.Add(-1)
		if newSize < 0 {
			newSize = 0
			mc.currentBufferSize.Store(0)
		}
		mc.bufferSize.Set(float64(newSize))
	}
}

// SetBufferSize sets the current buffer size directly.
// Requirement: 49.6
func (mc *MetricsCollector) SetBufferSize(size int) {
	if mc != nil && mc.bufferSize != nil {
		mc.currentBufferSize.Store(int64(size))
		mc.bufferSize.Set(float64(size))
	}
}

// Handler returns an HTTP handler for Prometheus metrics endpoint.
// Requirement: 49.10
func (mc *MetricsCollector) Handler() http.Handler {
	if mc == nil || mc.registry == nil {
		return http.NotFoundHandler()
	}
	return promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{})
}

// Registry returns the Prometheus registry for custom registration.
// Requirement: 49.11
func (mc *MetricsCollector) Registry() *prometheus.Registry {
	if mc == nil {
		return nil
	}
	return mc.registry
}

// PrometheusStatsHandler wraps MetricsCollector to implement StatsHandler interface.
// Requirements: 34.3, 34.4
type PrometheusStatsHandler struct {
	metrics *MetricsCollector
}

// NewPrometheusStatsHandler creates a new StatsHandler backed by Prometheus metrics.
func NewPrometheusStatsHandler(namespace string, maxBufferSize int) *PrometheusStatsHandler {
	return &PrometheusStatsHandler{
		metrics: NewMetricsCollector(namespace, maxBufferSize),
	}
}

// OnEmit is called when an event is successfully emitted.
// Requirement: 34.3
func (h *PrometheusStatsHandler) OnEmit(ev *Event) {
	if h != nil && h.metrics != nil {
		h.metrics.OnEventEmitted(true)
	}
}

// OnDrop is called when an event is dropped.
// Requirement: 34.4
func (h *PrometheusStatsHandler) OnDrop(reason string) {
	if h != nil && h.metrics != nil {
		h.metrics.OnEventDropped(reason)
	}
}

// OnError is called when an error occurs during asynchronous operations.
// Requirement: 34.3, 34.4
func (h *PrometheusStatsHandler) OnError(err error) {
	_ = err
}

// OnDeliveryFailed records an explicit delivery failure without counting it as a success emit.
func (h *PrometheusStatsHandler) OnDeliveryFailed(_ *Event, _ error) {
	if h != nil && h.metrics != nil {
		h.metrics.OnEventEmitted(false)
	}
}

// OnEventCreated increments the events_created_total counter.
func (h *PrometheusStatsHandler) OnEventCreated() {
	if h != nil && h.metrics != nil {
		h.metrics.OnEventCreated()
	}
}

// OnEventFinished increments the events_finished_total counter.
func (h *PrometheusStatsHandler) OnEventFinished() {
	if h != nil && h.metrics != nil {
		h.metrics.OnEventFinished()
	}
}

// ObserveEmitDuration records the duration of an emit operation.
func (h *PrometheusStatsHandler) ObserveEmitDuration(d time.Duration) {
	if h != nil && h.metrics != nil {
		h.metrics.ObserveEmitDuration(d)
	}
}

// Metrics returns the underlying MetricsCollector for direct access.
func (h *PrometheusStatsHandler) Metrics() *MetricsCollector {
	if h == nil {
		return nil
	}
	return h.metrics
}

// Handler returns an HTTP handler for the Prometheus metrics endpoint.
func (h *PrometheusStatsHandler) Handler() http.Handler {
	if h == nil || h.metrics == nil {
		return http.NotFoundHandler()
	}
	return h.metrics.Handler()
}
