package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"

	"voice/backend/pkg/httpserver"
	voiceprom "voice/backend/pkg/promhttp"
	"voice/backend/pkg/runtimeconfig"
)

const serviceName = "federation"

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	metricsReg := prometheus.NewRegistry()
	server := &http.Server{
		Addr:    addr,
		Handler: voiceprom.MountMetricsOnHealth(healthHandler(serviceName), metricsReg),
	}
	httpserver.ApplyHTTPServerTimeouts(server)
	errCh := make(chan error, 1)
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
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
