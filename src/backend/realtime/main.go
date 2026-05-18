package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	voicejwt "voice/backend/pkg/jwt"
)

const serviceName = "realtime"

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}

	jwksURL := strings.TrimSpace(os.Getenv("REALTIME_JWKS_URL"))
	if jwksURL == "" {
		jwksURL = strings.TrimSpace(os.Getenv("GATEWAY_JWKS_URL"))
	}
	var tv tokenValidator
	if jwksURL != "" {
		tv = voicejwt.NewJWKSValidator(
			jwksURL,
			firstNonEmpty(os.Getenv("REALTIME_JWT_ISSUER"), os.Getenv("GATEWAY_JWT_ISSUER")),
			firstNonEmpty(os.Getenv("REALTIME_JWT_AUDIENCE"), os.Getenv("GATEWAY_JWT_AUDIENCE")),
		)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           newServiceHandler(serviceName, tv),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}
