package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"

	filev1 "voice.app/voice/file/v1"
)

const serviceName = "file"

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("FILE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	var grpcSrv *grpc.Server
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		presigner, err := r2file.NewS3R2Presigner(r2file.EnvConfigFromOSEnv())
		if err != nil {
			log.Printf("%s: R2 config incomplete; gRPC disabled: %v", serviceName, err)
		} else {
			lis, err := net.Listen("tcp", grpcListen)
			if err != nil {
				log.Fatalf("grpc listen: %v", err)
			}
			grpcSrv = grpc.NewServer()
			filev1.RegisterFileServiceServer(grpcSrv, grpcsvc.New(grpcsvc.Deps{
				Files:     store.NewFilesStore(pool),
				Presigner: presigner,
			}))
			go func() {
				log.Printf("%s gRPC listening on %s", serviceName, grpcListen)
				if err := grpcSrv.Serve(lis); err != nil {
					log.Fatalf("grpc serve: %v", err)
				}
			}()
		}
	} else {
		log.Printf("%s: DATABASE_URL not set; gRPC disabled (health only)", serviceName)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           healthHandler(serviceName),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
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
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}
}
