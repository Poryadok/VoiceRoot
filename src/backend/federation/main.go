package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
)

const serviceName = "federation"

func main() {
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 30*time.Second)
	authority, pool, err := authorityRuntime(connectCtx)
	connectCancel()
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	httpserver.ApplyHTTPServerTimeouts(authority)
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	metricsReg := prometheus.NewRegistry()
	health := http.NewServeMux()
	health.Handle("/", healthHandler(serviceName))
	health.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		var version int
		if err := pool.QueryRow(ctx, "SELECT version FROM federation_schema_versions WHERE version=1").Scan(&version); err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	server := &http.Server{
		Addr:    addr,
		Handler: voiceprom.MountMetricsOnHealth(health, metricsReg),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 2)
	go func() { errCh <- authority.ListenAndServeTLS("", "") }()
	log.Printf("%s listening on %s", serviceName, addr)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), runtimeconfig.ShutdownTimeoutFromEnv())
		defer cancel()
		if err := authority.Shutdown(ctx); err != nil {
			log.Print(err)
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
