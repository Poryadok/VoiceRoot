package main

import (
	"encoding/json"
	"net/http"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/promhttp"
)

type healthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
}

func healthHandler(service string) http.Handler {
	return healthHandlerWithGate(service, nil)
}

func healthHandlerWithGate(service string, gate *automod.BootGate) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		if gate != nil {
			if _, err := gate.AcquireEffects(r.Context()); err != nil {
				http.Error(w, "Moderation reconciliation incomplete", http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{Service: service, Status: "ok"})
	})
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
	return promhttp.MountMetricsOnHealth(mux, nil)
}
