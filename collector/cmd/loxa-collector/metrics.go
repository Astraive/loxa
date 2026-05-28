package main

import (
	"net/http"

	collectormetrics "github.com/astraive/loxa/collector/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *collectorState) metricsHandler() http.Handler {
	s.metricsInit.Do(func() {
		reg := prometheus.NewRegistry()
		reg.MustRegister(
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.RequestsTotalName,
				Help: "Total ingest requests received.",
			}, func() float64 { return float64(s.metrics.requestsTotal.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.RequestsAuthErrorsName,
				Help: "Total ingest requests rejected by auth.",
			}, func() float64 { return float64(s.metrics.requestsAuthErr.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.RequestsLimitedName,
				Help: "Total ingest requests rejected by rate limiting.",
			}, func() float64 { return float64(s.metrics.requestsLimited.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.RequestsThrottledName,
				Help: "Total ingest requests rejected by collector backpressure limits.",
			}, func() float64 { return float64(s.metrics.requestsThrottled.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.EventsAcceptedName,
				Help: "Total events accepted by collector.",
			}, func() float64 { return float64(s.metrics.eventsAccepted.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.EventsInvalidName,
				Help: "Total events rejected as invalid JSON objects.",
			}, func() float64 { return float64(s.metrics.eventsInvalid.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.EventsRejectedName,
				Help: "Total events rejected before sink persistence.",
			}, func() float64 { return float64(s.metrics.eventsRejected.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.EventsDedupedName,
				Help: "Total duplicate events short-circuited by dedupe.",
			}, func() float64 { return float64(s.metrics.eventsDeduped.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.SinkWriteErrorsName,
				Help: "Total sink write failures.",
			}, func() float64 { return float64(s.metrics.sinkWriteErrors.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.SpoolBytesName,
				Help: "Current spool file size in bytes.",
			}, func() float64 { return float64(s.metrics.spoolBytes.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.QueueBytesName,
				Help: "Current in-memory delivery queue bytes.",
			}, func() float64 { return float64(s.metrics.queueBytes.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.InflightRequestsName,
				Help: "Current ingest requests being processed.",
			}, func() float64 { return float64(s.metrics.inflightRequests.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.InflightEventsName,
				Help: "Current ingest events being processed.",
			}, func() float64 { return float64(s.metrics.inflightEvents.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.SinkHealthName,
				Help: "Collector sink health state (1=healthy,0=unhealthy).",
			}, func() float64 { return float64(boolMetric(s.effectiveSinkHealthy())) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.DiskHealthName,
				Help: "Collector disk health state (1=healthy,0=unhealthy).",
			}, func() float64 { return float64(boolMetric(s.effectiveDiskHealthy())) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.SpoolHealthName,
				Help: "Collector spool health state (1=healthy,0=unhealthy).",
			}, func() float64 { return float64(boolMetric(s.effectiveSpoolHealthy())) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.ReadyName,
				Help: "Collector readiness state (1=ready,0=not ready).",
			}, func() float64 { return float64(boolMetric(s.isReady())) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.CortexBridgeFlushesName,
				Help: "Total successful collector-to-cortex bridge flushes.",
			}, func() float64 { return float64(s.metrics.cortexBridgeFlushes.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.CortexBridgeEventsName,
				Help: "Total events forwarded through the collector-to-cortex bridge.",
			}, func() float64 { return float64(s.metrics.cortexBridgeEvents.Load()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: collectormetrics.CortexBridgeErrorsName,
				Help: "Total collector-to-cortex bridge delivery errors.",
			}, func() float64 { return float64(s.metrics.cortexBridgeErrors.Load()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: collectormetrics.CortexBridgeQueueDepthName,
				Help: "Current async collector-to-cortex bridge queue depth.",
			}, func() float64 { return float64(s.metrics.cortexBridgeQueueDepth.Load()) }),
		)
		s.metricsHTTP = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	})
	return s.metricsHTTP
}
