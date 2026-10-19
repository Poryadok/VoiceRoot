package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

func healthHandler(service string) http.Handler {
	return healthHandlerWithRealtime(service, nil)
}

func healthHandlerWithRealtime(service string, ready func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		status := "ok"
		if ready != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			err := ready(ctx)
			cancel()
			if err != nil {
				status = "degraded"
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		}
		if err := json.NewEncoder(w).Encode(healthResponse{Service: service, Status: status}); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	})
	return mux
}
