package grpcsvc

import "github.com/prometheus/client_golang/prometheus"

// ScanMetrics records process-lifetime sample outcomes, not a durable scan ledger.
type ScanMetrics struct {
	outcomes *prometheus.CounterVec
}

var defaultScanMetrics = NewScanMetrics(prometheus.DefaultRegisterer)

// NewScanMetrics uses the service's metrics registry. A private registry isolates tests.
func NewScanMetrics(reg prometheus.Registerer) *ScanMetrics {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "moderation_automod_scan_samples_total",
		Help: "Terminal persistence outcomes for eligible AutoMod scan samples in this process.",
	}, []string{"outcome"})
	if err := reg.Register(c); err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			c = existing.ExistingCollector.(*prometheus.CounterVec)
		} else {
			panic(err)
		}
	}
	for _, outcome := range []string{"acknowledged", "unconfirmed", "skipped_canceled", "skipped_budget"} {
		c.WithLabelValues(outcome)
	}
	return &ScanMetrics{outcomes: c}
}
