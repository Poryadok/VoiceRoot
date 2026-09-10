package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func ownershipRoleTLSFromEnv() (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate roots: %w", err)
	}
	if path := strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_CA_FILE")); path != "" {
		encoded, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read Role principal CA: %w", err)
		}
		if !roots.AppendCertsFromPEM(encoded) {
			return nil, errors.New("role principal CA contains no certificates")
		}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: strings.TrimSpace(os.Getenv("ROLE_PRINCIPAL_TLS_SERVER_NAME"))}, nil
}

func spaceHTTPHandler(service string, jwks principalJWKS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", healthHandler(service))
	if len(jwks.Keys) > 0 {
		mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=30")
			if err := json.NewEncoder(w).Encode(jwks); err != nil {
				http.Error(w, "encode public keys", http.StatusInternalServerError)
			}
		})
	}
	return mux
}
