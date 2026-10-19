package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"voice/backend/pkg/principal"
)

func moderationJWKSHandler(keys []principal.JWKSKey) http.Handler {
	public := principal.JWKSHandlerKeys(keys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "verified workload certificate required", http.StatusForbidden)
			return
		}
		if r.URL.Path != "/internal/principals/jwks" {
			http.NotFound(w, r)
			return
		}
		public.ServeHTTP(w, r)
	})
}

type moderationJWKSRuntime struct {
	server   *http.Server
	listener net.Listener
	errors   chan error
	done     chan struct{}
}

func startModerationJWKSFromEnv(ctx context.Context, keys []principal.JWKSKey) (*moderationJWKSRuntime, error) {
	listen := strings.TrimSpace(os.Getenv("MODERATION_PRINCIPAL_JWKS_LISTEN"))
	if listen == "" {
		listen = ":8443"
	}
	return startModerationJWKS(ctx, keys, listen, os.Getenv("MODERATION_PRINCIPAL_TLS_CERT_FILE"), os.Getenv("MODERATION_PRINCIPAL_TLS_KEY_FILE"), os.Getenv("MODERATION_PRINCIPAL_TLS_CLIENT_CA_FILE"))
}

// This P0 listener requires only local public signing keys and workload TLS.
// It must be started before reciprocal private authority calls/public readiness.
func startModerationJWKS(ctx context.Context, keys []principal.JWKSKey, listen, certificate, key, clientCA string) (*moderationJWKSRuntime, error) {
	if ctx == nil || len(keys) != 2 {
		return nil, errors.New("complete Moderation issuer and bootstrap context required")
	}
	if _, err := principal.PublicJWKSKeys(keys); err != nil {
		return nil, err
	}
	if strings.TrimSpace(certificate) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(clientCA) == "" {
		return nil, errors.New("Moderation JWKS workload TLS files required")
	}
	pair, err := tls.LoadX509KeyPair(certificate, key)
	if err != nil {
		return nil, errors.New("invalid Moderation JWKS workload keypair")
	}
	anchors, err := os.ReadFile(clientCA)
	if err != nil || len(anchors) > 1048576 {
		return nil, errors.New("invalid Moderation JWKS workload client CA")
	}
	trusted := x509.NewCertPool()
	if !trusted.AppendCertsFromPEM(anchors) {
		return nil, errors.New("invalid Moderation JWKS workload client CA")
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}, ClientCAs: trusted, ClientAuth: tls.RequireAndVerifyClientCert}
	listener, err := tls.Listen("tcp", listen, config)
	if err != nil {
		return nil, err
	}
	runtime := &moderationJWKSRuntime{listener: listener, errors: make(chan error, 1), done: make(chan struct{})}
	runtime.server = &http.Server{Handler: moderationJWKSHandler(keys), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		defer close(runtime.done)
		if err := runtime.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			runtime.errors <- err
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			runtime.Close()
		case <-runtime.done:
		}
	}()
	return runtime, nil
}

func (r *moderationJWKSRuntime) Close() {
	if r == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.server.Shutdown(ctx); err != nil {
		_ = r.server.Close()
	}
}
