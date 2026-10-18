package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/reporttargetclient"
	"voice/backend/moderation/internal/store"
)

// newReportTargetRuntime creates all four exact transports and serves only the
// current+next public JWKS on HTTPS. It never falls back to ordinary clients.
func newReportTargetRuntime(ctx context.Context, logger *slog.Logger) (*reporttargetclient.Client, func(), error) {
	const prefix = "MODERATION_REPORT_TARGETS_"
	enabled := false
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, prefix) {
			enabled = true
			break
		}
	}
	if !enabled {
		return nil, func() {}, nil
	}
	issuer, document, err := loadModerationReportTargetIssuerFromEnv()
	if err != nil {
		return nil, nil, err
	}
	if issuer == nil {
		return nil, nil, errors.New("report target signing identity required")
	}
	client, closeClient, err := reporttargetclient.DialFromEnv(issuer)
	if err != nil {
		return nil, nil, err
	}
	cert, err := tls.LoadX509KeyPair(os.Getenv(prefix+"JWKS_TLS_CERT_FILE"), os.Getenv(prefix+"JWKS_TLS_KEY_FILE"))
	if err != nil {
		closeClient()
		return nil, nil, err
	}
	addr := os.Getenv(prefix + "JWKS_LISTEN")
	if addr == "" || addr != strings.TrimSpace(addr) {
		closeClient()
		return nil, nil, errors.New("report target HTTPS JWKS listener required")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		closeClient()
		return nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(document)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := server.Serve(tls.NewListener(listener, &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}})); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("report target JWKS listener stopped")
		}
	}()
	return client, func() { _ = server.Close(); _ = listener.Close(); closeClient() }, nil
}

// startReportValidation owns shutdown order: worker stops before publisher,
// owner transports and the caller's database pool are closed.
func startReportValidation(ctx context.Context, reports *store.ReportStore, logger *slog.Logger) (func(), error) {
	targets, closeTargets, err := newReportTargetRuntime(ctx, logger)
	if err != nil {
		return nil, err
	}
	if targets == nil {
		return closeTargets, nil
	}
	publisher, err := reportintake.NewPublisher(strings.TrimSpace(os.Getenv("NATS_URL")))
	if err != nil {
		closeTargets()
		return nil, err
	}
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	worker := &reportintake.Worker{Reports: reports, Targets: targets, Publish: publisher.Publish}
	// BE-046 injects its sole VALID hook on its combined source; nil stays manual.
	go func() { defer close(done); worker.Run(workerCtx, logger) }()
	return func() { cancel(); <-done; publisher.Close(); closeTargets() }, nil
}
