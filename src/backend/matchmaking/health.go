package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"voice/backend/matchmaking/internal/queue"
)

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Redis   string `json:"redis,omitempty"`
}

func healthHandler(service string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(healthResponse{Service: service, Status: "ok"}); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
	return mux
}

func healthWithRedis(base http.Handler, q *queue.RedisQueue) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		redisStatus := "ok"
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := q.Ping(ctx); err != nil {
			redisStatus = "degraded"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Service: serviceName,
			Status:  "ok",
			Redis:   redisStatus,
		})
	}))
	mux.Handle("/", base)
	return mux
}
