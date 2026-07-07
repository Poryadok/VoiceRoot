package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsIngested = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_ingest_events_total",
		Help: "Total analytics events accepted for buffering",
	})
	FlushBatches = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_flush_batches_total",
		Help: "Total ClickHouse batch flushes",
	})
	FlushErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "analytics_flush_errors_total",
		Help: "Failed ClickHouse batch flushes",
	})
	InsertLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "analytics_clickhouse_insert_latency_seconds",
		Help:    "ClickHouse batch insert latency",
		Buckets: prometheus.DefBuckets,
	})
	BufferDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "analytics_buffer_depth",
		Help: "Pending events in memory buffer",
	})
	IngestLag = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "analytics_ingest_lag_seconds",
		Help:    "Lag from event timestamp to ingest",
		Buckets: []float64{0.1, 0.5, 1, 5, 15, 60, 120},
	})
)
