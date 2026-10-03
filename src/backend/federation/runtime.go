package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func authorityRuntime(ctx context.Context) (*http.Server, *pgxpool.Pool, error) {
	required := []string{"FEDERATION_DATABASE_URL", "FEDERATION_TLS_CERT", "FEDERATION_TLS_KEY", "FEDERATION_CLIENT_CA", "FEDERATION_SIGNING_SEED_FILE", "FEDERATION_KEY_ID", "FEDERATION_ISSUER", "FEDERATION_ENVIRONMENT", "FEDERATION_OPERATOR_CERT_SHA256"}
	for _, name := range required {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			return nil, nil, fmt.Errorf("%s is required", name)
		}
	}
	seedText, err := os.ReadFile(os.Getenv("FEDERATION_SIGNING_SEED_FILE"))
	if err != nil {
		return nil, nil, fmt.Errorf("read signing seed: %w", err)
	}
	seed, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(seedText)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("signing seed must be 32 bytes base64url")
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv("FEDERATION_TLS_CERT"), os.Getenv("FEDERATION_TLS_KEY"))
	if err != nil {
		return nil, nil, fmt.Errorf("load server certificate: %w", err)
	}
	ca, err := os.ReadFile(os.Getenv("FEDERATION_CLIENT_CA"))
	if err != nil {
		return nil, nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, nil, fmt.Errorf("invalid client CA")
	}
	pins := map[string]bool{}
	for _, pin := range strings.Split(os.Getenv("FEDERATION_OPERATOR_CERT_SHA256"), ",") {
		pin = strings.TrimSpace(pin)
		if !validPin(pin) {
			return nil, nil, fmt.Errorf("invalid operator certificate pin")
		}
		pins[pin] = true
	}
	mediaPins := map[string]bool{}
	if configured := strings.TrimSpace(os.Getenv("FEDERATION_MEDIA_ISSUER_CERT_SHA256")); configured != "" {
		for _, pin := range strings.Split(configured, ",") {
			pin = strings.TrimSpace(pin)
			if !validPin(pin) || pins[pin] || mediaPins[pin] {
				return nil, nil, fmt.Errorf("invalid media issuer certificate pin")
			}
			mediaPins[pin] = true
		}
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("FEDERATION_DATABASE_URL"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid federation database configuration")
	}
	if err = validateFederationDatabaseName(cfg.ConnConfig.Database); err != nil {
		return nil, nil, err
	}
	cfg.MaxConns = 16
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("federation database unavailable")
	}
	if err = migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("federation migration: %w", err)
	}
	store := &authorityStore{Pool: pool, Key: ed25519.NewKeyFromSeed(seed), KeyID: os.Getenv("FEDERATION_KEY_ID"), Issuer: os.Getenv("FEDERATION_ISSUER"), Environment: os.Getenv("FEDERATION_ENVIRONMENT")}
	for pin := range mediaPins {
		available, err := store.mediaIssuerPinAvailable(ctx, pin)
		if err != nil || !available {
			pool.Close()
			return nil, nil, fmt.Errorf("media issuer identity unavailable")
		}
	}
	addr := os.Getenv("FEDERATION_HTTPS_LISTEN")
	if addr == "" {
		addr = ":9443"
	}
	server := &http.Server{Addr: addr, Handler: &authorityAPI{Store: store, Operators: pins, MediaIssuers: mediaPins}, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots, Certificates: []tls.Certificate{cert}}, MaxHeaderBytes: 16 << 10}
	return server, pool, nil
}

func validateFederationDatabaseName(name string) error {
	if name != "federation_db" {
		return fmt.Errorf("federation database URL must select federation_db")
	}
	return nil
}
