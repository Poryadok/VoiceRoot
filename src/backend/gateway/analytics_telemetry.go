package main

import (
	"context"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"voice/backend/pkg/analyticsevents"
	"voice/backend/pkg/correlation"
)

type gatewayAnalyticsTelemetry struct {
	pub        analyticsevents.Publisher
	sampleRate float64
}

func gatewayAnalyticsFromEnv() gatewayAnalyticsTelemetry {
	rate := 0.01
	if v := strings.TrimSpace(os.Getenv("GATEWAY_ANALYTICS_SAMPLE_RATE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			rate = f
		}
	}
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" || rate <= 0 {
		return gatewayAnalyticsTelemetry{sampleRate: rate}
	}
	pub, err := analyticsevents.NewJetStreamPublisher(natsURL)
	if err != nil {
		return gatewayAnalyticsTelemetry{sampleRate: rate}
	}
	return gatewayAnalyticsTelemetry{pub: pub, sampleRate: rate}
}

func (t gatewayAnalyticsTelemetry) maybePublish(r *http.Request, group string, status int, duration time.Duration) {
	if t.pub == nil || t.sampleRate <= 0 || r == nil {
		return
	}
	if group == "health" || group == "metrics" || group == "ws" {
		return
	}
	if rand.Float64() > t.sampleRate {
		return
	}
	ctx := r.Context()
	if rid := r.Header.Get(correlation.RequestIDHeader); rid != "" {
		ctx = correlation.WithGRPC(ctx, rid)
	}
	_ = t.pub.Publish(ctx, "analytics.gateway.request", "gateway", "api_request", map[string]any{
		"route_group": group,
		"method":      r.Method,
		"path":        r.URL.Path,
		"status":      status,
		"duration_ms": duration.Milliseconds(),
	})
}
