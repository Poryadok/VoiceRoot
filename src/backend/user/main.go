package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

const serviceName = "user"

func main() {
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcAddr := ":9090"
	if v := os.Getenv("USER_GRPC_ADDR"); v != "" {
		grpcAddr = v
	}

	dbURL := os.Getenv("DATABASE_URL")
	var pool *pgxpool.Pool
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		p, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		pool = p
		defer pool.Close()
	}

	if pool != nil {
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		srv := grpc.NewServer()
		userv1.RegisterUserServiceServer(srv, &grpcsvc.UserGRPC{Profiles: store.NewProfileStore(pool)})
		go func() {
			log.Printf("%s gRPC listening on %s", serviceName, grpcAddr)
			if err := srv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	} else {
		log.Printf("%s: DATABASE_URL not set; gRPC disabled (health only)", serviceName)
	}

	server := &http.Server{
		Addr:              httpAddr,
		Handler:           healthHandler(serviceName),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	log.Printf("%s listening on %s", serviceName, httpAddr)
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
