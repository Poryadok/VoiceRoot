package main

import "github.com/prometheus/client_golang/prometheus"

func newOwnershipOutboxDeliveryAlertGauge(registry prometheus.Registerer) prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "space_ownership_outbox_delivery_alerting_events",
		Help: "Number of retryable Space ownership outbox events with at least ten consecutive delivery failures.",
	})
	registry.MustRegister(gauge)
	return gauge
}
