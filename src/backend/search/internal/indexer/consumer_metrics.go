package indexer

import "github.com/prometheus/client_golang/prometheus"

const (
	consumerOutcomeAck                    = "ack"
	consumerOutcomeRetry                  = "retry"
	consumerOutcomeTerminalMalformed      = "terminal_malformed"
	consumerOutcomeTerminalSemanticPoison = "terminal_semantic_poison"
	consumerOutcomeDispositionFailure     = "disposition_failure"
)

// ConsumerMetrics exposes the durable-consumer delivery outcome taxonomy.
// There is no configured dead-letter stream for Search; terminal outcomes are
// therefore observable here and in logs rather than published elsewhere.
type ConsumerMetrics struct {
	outcomes *prometheus.CounterVec
}

// NewConsumerMetrics registers Search indexer consumer outcome metrics.
func NewConsumerMetrics(reg prometheus.Registerer) *ConsumerMetrics {
	return newConsumerMetrics(reg)
}

func newConsumerMetrics(reg prometheus.Registerer) *ConsumerMetrics {
	m := &ConsumerMetrics{outcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "search_indexer_consume_outcomes_total",
		Help: "JetStream delivery outcomes for Search index projections.",
	}, []string{"stream", "outcome"})}
	if reg != nil {
		reg.MustRegister(m.outcomes)
	}
	return m
}

func (m *ConsumerMetrics) observe(stream, outcome string) {
	if m != nil {
		m.outcomes.WithLabelValues(stream, outcome).Inc()
	}
}
