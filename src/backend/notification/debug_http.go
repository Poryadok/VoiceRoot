package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"voice/backend/notification/internal/fcm"
)

func notificationHTTPHandler(serviceName string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler(serviceName))
	mux.HandleFunc("/debug/recorded-pushes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
		if profileID == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"profile_id required"}`))
			return
		}
		pid, err := uuid.Parse(profileID)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		rec, ok := fcm.GlobalPushRecorder.LastForProfile(pid)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rec)
	})
	return mux
}
