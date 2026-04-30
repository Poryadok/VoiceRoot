package main

import (
	"net/http"
	"strings"
)

type corsConfig struct {
	AllowedOrigins []string
	AllowedHeaders []string
	AllowedMethods []string
}

func (g *gateway) applyCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if !originAllowed(origin, g.config.cors.AllowedOrigins) {
		if r.Method == http.MethodOptions {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cors_origin_denied"})
			return true
		}
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(corsValues(g.config.cors.AllowedMethods, defaultCORSMethods()), ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(corsValues(g.config.cors.AllowedHeaders, defaultCORSHeaders()), ", "))
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func originAllowed(origin string, allowed []string) bool {
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == origin {
			return true
		}
	}
	return false
}

func corsValues(values, defaults []string) []string {
	if len(values) == 0 {
		return defaults
	}
	return values
}

func defaultCORSMethods() []string {
	return []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
}

func defaultCORSHeaders() []string {
	return []string{"Authorization", "Content-Type", "X-Request-Id", "X-Voice-Client-Platform", "X-Voice-Client-Version"}
}
