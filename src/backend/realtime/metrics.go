package main

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	grpcmw "voice/backend/pkg/grpcmw"
)

type realtimeMetrics struct {
	connectionsActive prometheus.Gauge
	connectTotal      *prometheus.CounterVec
	helloDuration     prometheus.Histogram
	natsConsumeLag    *prometheus.GaugeVec
}

var rtMetrics *realtimeMetrics

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
	}
	reg.MustRegister(
		m.connectionsActive,
		m.connectTotal,
		m.helloDuration,
		m.natsConsumeLag,
	)
	rtMetrics = m
	return m
}

func observeWSConnectFail() {
	if rtMetrics == nil {
		return
	}
	rtMetrics.connectTotal.WithLabelValues("fail").Inc()
}

func observeWSConnectSuccess() {
	if rtMetrics == nil {
		return
	}
	rtMetrics.connectTotal.WithLabelValues("success").Inc()
	rtMetrics.connectionsActive.Inc()
}

func observeWSDisconnect() {
	if rtMetrics == nil {
		return
	}
	rtMetrics.connectionsActive.Dec()
}

func observeWSHelloDuration(start time.Time) {
	if rtMetrics == nil {
		return
	}
	rtMetrics.helloDuration.Observe(time.Since(start).Seconds())
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
	}
}

func runNatsConsumeLagPoller(ctx context.Context, js nats.JetStreamContext, instanceID string) {
	if rtMetrics == nil || js == nil {
		return
	}
	consumers := realtimeJetStreamConsumers(instanceID)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	update := func() {
		for _, c := range consumers {
			info, err := js.ConsumerInfo(c.stream, c.consumer)
			if err != nil {
				rtMetrics.natsConsumeLag.DeleteLabelValues(c.stream, c.consumer)
				continue
			}
			rtMetrics.natsConsumeLag.WithLabelValues(c.stream, c.consumer).Set(float64(info.NumPending))
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
