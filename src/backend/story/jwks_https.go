package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"voice/backend/pkg/httpserver"
)

// newStoryJWKSHTTPSServer exposes the already generated public Story key set
// on the separately configured TLS endpoint. Partial configuration fails closed.
func newStoryJWKSHTTPSServer(lookup func(string) (string, bool), document []byte) (*http.Server, bool, error) {
	names := []string{"STORY_PRINCIPAL_JWKS_HTTPS_LISTEN", "STORY_PRINCIPAL_JWKS_TLS_CERT_FILE", "STORY_PRINCIPAL_JWKS_TLS_KEY_FILE"}
	values := make([]string, len(names))
	enabled := false
	for i, name := range names {
		value, present := lookup(name)
		enabled = enabled || present
		values[i] = strings.TrimSpace(value)
	}
	if !enabled {
		return nil, false, nil
	}
	if values[0] == "" || values[1] == "" || values[2] == "" || len(document) == 0 {
		return nil, true, errors.New("Story HTTPS JWKS address, TLS certificate, key and public keys are required")
	}
	certificate, err := tls.LoadX509KeyPair(values[1], values[2])
	if err != nil {
		return nil, true, errors.New("invalid Story HTTPS JWKS TLS certificate/key")
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, true, errors.New("invalid Story HTTPS JWKS TLS certificate")
	}
	now := time.Now()
	serverAuth := false
	for _, usage := range leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth || usage == x509.ExtKeyUsageAny {
			serverAuth = true
		}
	}
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) || !serverAuth || leaf.IsCA {
		return nil, true, errors.New("Story HTTPS JWKS certificate is not currently valid for a server")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	})
	server := &http.Server{Addr: values[0], Handler: mux, TLSConfig: &tls.Config{
		MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{certificate},
	}}
	httpserver.ApplyHTTPServerTimeouts(server)
	return server, true, nil
}

func listenStoryJWKSHTTPS(server *http.Server) (net.Listener, error) {
	return net.Listen("tcp", server.Addr)
}
