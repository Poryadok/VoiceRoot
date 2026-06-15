package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/file/internal/clamav"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"
	grpcsvc "voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/imgproc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/s2s"
	"voice/backend/file/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

const serviceName = "file"

func waitForGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return context.Canceled
		}
		if !conn.WaitForStateChange(ctx, st) {
			return context.Cause(ctx)
		}
	}
}

func main() {
	logger := httpserver.NewLogger(serviceName)
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
			log.Printf("%s: R2 config incomplete; uploads disabled: %v", serviceName, err)
		}

		var chatGuard grpcsvc.ChatGuard
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), grpcclient.DialTimeoutFromEnv())
			if err := waitForGRPCReady(waitCtx, cconn); err != nil {
				waitCancel()
				log.Fatalf("chat grpc dial: %v", err)
			}
			waitCancel()
			chatGuard = s2s.NewGRPCChatGuard(chatv1.NewChatServiceClient(cconn))
		}
		var scanner grpcsvc.Scanner
		if clamAddr := strings.TrimSpace(os.Getenv("CLAMAV_ADDR")); clamAddr != "" {
			scanner = clamav.Scanner{Addr: clamAddr}
		}
		var reader grpcsvc.ObjectReader
		var processor grpcsvc.ImageProcessor
		if presigner != nil {
			reader = presigner
			processor = imgproc.Processor{Reader: presigner, Writer: presigner}
		}
		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		filev1.RegisterFileServiceServer(grpcSrv, grpcsvc.New(grpcsvc.Deps{
			Files:     store.NewFilesStore(pool),
			Presigner: presigner,
			ChatGuard: chatGuard,
			Reader:    reader,
			Processor: processor,
			Scanner:   scanner,
		}))
		go func() {
			logger.Info("gRPC listening", slog.String("addr", grpcListen))
			if err := grpcSrv.Serve(lis); err != nil {
				log.Fatalf("grpc serve: %v", err)
			}
		}()
	} else {
		logger.Warn("DATABASE_URL not set; gRPC disabled (health only)")
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpserver.Wrap(healthHandler(serviceName), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", addr))
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
