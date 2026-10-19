package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	grpcmw "voice/backend/pkg/grpcmw"
)

type realtimeMetrics struct {
	connectionsActive  prometheus.Gauge
	connectTotal       *prometheus.CounterVec
	helloDuration      prometheus.Histogram
	natsConsumeLag     *prometheus.GaugeVec
	fanoutEnqueueTotal *prometheus.CounterVec
}

var rtMetrics atomic.Pointer[realtimeMetrics]

func initRealtimeMetrics(reg *prometheus.Registry) *realtimeMetrics {
	m := &realtimeMetrics{
		connectionsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "realtime_ws_connections_active",
			Help: "Active WebSocket connections on this realtime instance.",
		}),
		connectTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "realtime_ws_connect_total",
			Help: "Total WebSocket connection attempts.",
		}, []string{"code"}),
		helloDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "realtime_ws_hello_duration_seconds",
			Help:    "Time from successful WebSocket upgrade to first outbound hello frame.",
			Buckets: grpcmw.DefaultHistogramBuckets,
		}),
		natsConsumeLag: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "realtime_nats_consume_lag",
			Help: "JetStream consumer pending messages (stream lag).",
		}, []string{"stream", "consumer"}),
		fanoutEnqueueTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "realtime_ws_fanout_enqueue_total",
			Help: "Profile fan-out enqueue outcomes; labels are a bounded policy set.",
		}, []string{"outcome"}),
	}
	reg.MustRegister(
		m.connectionsActive,
		m.connectTotal,
		m.helloDuration,
		m.natsConsumeLag,
		m.fanoutEnqueueTotal,
	)
	// CounterVec/GaugeVec children are omitted from /metrics until label sets exist.
	m.connectTotal.WithLabelValues("success")
	m.connectTotal.WithLabelValues("fail")
	m.fanoutEnqueueTotal.WithLabelValues("enqueued")
	m.fanoutEnqueueTotal.WithLabelValues("dropped")
	m.fanoutEnqueueTotal.WithLabelValues("disconnect_on_overflow")
	rtMetrics.Store(m)
	return m
}

func observeWSFanoutEnqueue(result fanoutEnqueueResult) {
	m := rtMetrics.Load()
	if m == nil {
		return
	}
	switch result {
	case fanoutEnqueued:
		m.fanoutEnqueueTotal.WithLabelValues("enqueued").Inc()
	case fanoutDisconnectOnOverflow:
		m.fanoutEnqueueTotal.WithLabelValues("disconnect_on_overflow").Inc()
	default:
		m.fanoutEnqueueTotal.WithLabelValues("dropped").Inc()
	}
}

func observeWSConnectFail() {
	m := rtMetrics.Load()
	if m == nil {
		return
	}
	m.connectTotal.WithLabelValues("fail").Inc()
}

func (m *realtimeMetrics) observeWSConnectSuccess() {
	if m == nil {
		return
	}
	m.connectTotal.WithLabelValues("success").Inc()
	m.connectionsActive.Inc()
}

func (m *realtimeMetrics) observeWSDisconnect() {
	if m == nil {
		return
	}
	m.connectionsActive.Dec()
}

func (m *realtimeMetrics) observeWSHelloDuration(start time.Time) {
	if m == nil {
		return
	}
	m.helloDuration.Observe(time.Since(start).Seconds())
}

type jetStreamConsumerRef struct {
	stream   string
	consumer string
}

func realtimeJetStreamConsumers(instanceID string) []jetStreamConsumerRef {
	return []jetStreamConsumerRef{
		{stream: jsStreamMessageEvents, consumer: consumerDurableName(instanceID)},
		{stream: jsStreamVoiceEvents, consumer: voiceConsumerDurableName(instanceID)},
		{stream: jsStreamChatEvents, consumer: chatConsumerDurableName(instanceID)},
		{stream: jsStreamMatchmakingEvents, consumer: matchmakingConsumerDurableName(instanceID)},
		{stream: jsStreamRoleEvents, consumer: roleConsumerDurableName(instanceID)},
		{stream: jsStreamUserEvents, consumer: userConsumerDurableName(instanceID)},
	}
}

func runNatsConsumeLagPoller(ctx context.Context, js nats.JetStreamContext, instanceID string) {
	m := rtMetrics.Load()
	if m == nil || js == nil {
		return
	}
	consumers := realtimeJetStreamConsumers(instanceID)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	update := func() {
		for _, c := range consumers {
			info, err := js.ConsumerInfo(c.stream, c.consumer)
			if err != nil {
				m.natsConsumeLag.DeleteLabelValues(c.stream, c.consumer)
				continue
			}
			m.natsConsumeLag.WithLabelValues(c.stream, c.consumer).Set(float64(info.NumPending))
		}
	}
	update()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
