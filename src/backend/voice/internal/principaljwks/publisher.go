package principaljwks

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"

	"voice/backend/pkg/principal"
)

// Publisher owns only the service's public signing-key endpoint. It exposes no
// mutation handlers and is independent of admission readiness.
type Publisher struct {
	server   *http.Server
	listener net.Listener
	errors   chan error
}

// Start validates TLS and binds synchronously, then publishes the same current
// and next public keys used by the service's configured signer. Callers must
// observe Errors throughout startup as well as during normal serving.
func Start(cfg Config, wrap func(http.Handler) http.Handler) (*Publisher, error) {
	if cfg.SigningKey == nil || cfg.NextSigningKey == nil || cfg.KeyID == "" || cfg.NextKeyID == "" || cfg.KeyID == cfg.NextKeyID || cfg.SigningKey.PublicKey.Equal(&cfg.NextSigningKey.PublicKey) {
		return nil, errors.New("Voice public JWKS requires distinct current and next signing keys")
	}
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/internal/v1/principal/jwks.json", principal.JWKSHandlerKeys([]principal.JWKSKey{
		{KeyID: cfg.KeyID, PublicKey: &cfg.SigningKey.PublicKey},
		{KeyID: cfg.NextKeyID, PublicKey: &cfg.NextSigningKey.PublicKey},
	}))
	var handler http.Handler = mux
	if wrap != nil {
		handler = wrap(handler)
	}
	publisher := &Publisher{
		server:   &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second},
		listener: tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}}),
		errors:   make(chan error, 1),
	}
	go func() {
		publisher.errors <- publisher.server.Serve(publisher.listener)
		close(publisher.errors)
	}()
	return publisher, nil
}

func (p *Publisher) Errors() <-chan error               { return p.errors }
func (p *Publisher) Addr() net.Addr                     { return p.listener.Addr() }
func (p *Publisher) Close() error                       { return p.server.Close() }
func (p *Publisher) Shutdown(ctx context.Context) error { return p.server.Shutdown(ctx) }
