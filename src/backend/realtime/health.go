package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type readinessDeps struct {
	Redis   *redis.Client
	NatsURL string
}

func healthOnly(service string) http.Handler {
	return readinessHandler(service, readinessDeps{})
}

func readinessHandler(service string, deps readinessDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		status, reason := checkReadiness(r.Context(), deps)
		w.Header().Set("Content-Type", "application/json")
		if status != "ok" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		if err := json.NewEncoder(w).Encode(healthResponse{
			Service: service,
			Status:  status,
			Reason:  reason,
		}); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
}

func checkReadiness(ctx context.Context, deps readinessDeps) (status, reason string) {
	if deps.Redis != nil {
		pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := deps.Redis.Ping(pctx).Err()
		cancel()
		if err != nil {
			return "degraded", "redis_unavailable"
		}
	}
	if url := strings.TrimSpace(deps.NatsURL); url != "" {
		nc, err := nats.Connect(url, natsConnectOptions("voice-realtime-readiness")...)
		if err != nil {
			return "degraded", "nats_unavailable"
		}
		_ = nc.Drain()
	}
	return "ok", ""
}
