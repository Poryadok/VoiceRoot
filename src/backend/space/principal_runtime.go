package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func ownershipRoleTLSFromEnv() (*tls.Config, error) {
	config, err := ownershipTLSFromEnv("ROLE_PRINCIPAL_TLS_CA_FILE", "ROLE_PRINCIPAL_TLS_SERVER_NAME", "Role")
	if err != nil {
		return nil, err
	}
	certificate := strings.TrimSpace(os.Getenv("SPACE_ROLE_CLIENT_CERT_FILE"))
	key := strings.TrimSpace(os.Getenv("SPACE_ROLE_CLIENT_KEY_FILE"))
	if certificate == "" || key == "" {
		return nil, fmt.Errorf("space Role client certificate and key are required for mutual TLS")
	}
	identity, err := tls.LoadX509KeyPair(certificate, key)
	if err != nil {
		return nil, fmt.Errorf("load Space Role client identity: %w", err)
	}
	config.Certificates = []tls.Certificate{identity}
	return config, nil
}

func ownershipAuthTLSFromEnv() (*tls.Config, error) {
	return ownershipTLSFromEnv("AUTH_PRINCIPAL_TLS_CA_FILE", "AUTH_PRINCIPAL_TLS_SERVER_NAME", "Auth")
}

func ownershipTLSFromEnv(caEnv, serverNameEnv, peer string) (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system certificate roots: %w", err)
	}
	if path := strings.TrimSpace(os.Getenv(caEnv)); path != "" {
		encoded, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s principal CA: %w", peer, err)
		}
		if !roots.AppendCertsFromPEM(encoded) {
			return nil, fmt.Errorf("%s principal CA contains no certificates", strings.ToLower(peer))
		}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: strings.TrimSpace(os.Getenv(serverNameEnv))}, nil
}

func spaceHTTPHandler(service string, jwks principalJWKS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", healthHandler(service))
	if len(jwks.Keys) > 0 {
		mux.Handle("/.well-known/jwks.json", spaceJWKSHandler(jwks))
	}
	return mux
}

func spaceJWKSHandler(jwks principalJWKS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(jwks.Keys) == 0 {
			http.NotFound(w, r)
			return
		}
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
