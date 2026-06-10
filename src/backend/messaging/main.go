package main

import (
	"context"
	"fmt"
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

	grpcsvc "voice/backend/messaging/internal/grpcsvc"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "messaging"

func waitForGRPCReady(ctx context.Context, conn *grpc.ClientConn) error {
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return nil
		}
		if st == connectivity.Shutdown {
			return fmt.Errorf("grpc connection shutdown")
		}
		if !conn.WaitForStateChange(ctx, st) {
			return fmt.Errorf("grpc dial: %w", context.Cause(ctx))
		}
	}
}

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		var chatGuard grpcsvc.ChatGuard
		if chatAddr := strings.TrimSpace(os.Getenv("CHAT_GRPC_ADDR")); chatAddr != "" {
			cconn, err := grpc.NewClient(grpcclient.DialTarget(chatAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("chat grpc: %v", err)
			}
			defer func() { _ = cconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := waitForGRPCReady(waitCtx, cconn); err != nil {
				waitCancel()
				log.Fatalf("chat grpc dial: %v", err)
			}
			waitCancel()
			chatGuard = s2s.NewGRPCChatGuard(chatv1.NewChatServiceClient(cconn))
		} else {
			chatDB := strings.TrimSpace(os.Getenv("CHAT_DATABASE_URL"))
			if chatDB != "" {
				cctx, ccancel := context.WithTimeout(context.Background(), 15*time.Second)
				chatPool, err := pgxpool.New(cctx, chatDB)
				ccancel()
				if err != nil {
					log.Fatalf("chat postgres: %v", err)
				}
				defer chatPool.Close()
				chatGuard = &store.SQLChatGuard{Pool: chatPool}
			} else {
				chatGuard = &store.SQLChatGuard{Pool: pool}
			}
		}

		var blocks grpcsvc.AccountPairBlockChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(grpcclient.DialTarget(socialAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			defer func() { _ = sconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := waitForGRPCReady(waitCtx, sconn); err != nil {
				waitCancel()
				log.Fatalf("social grpc dial: %v", err)
			}
			waitCancel()
			blocks = s2s.NewSocialGRPCBlocks(sconn)
		}

		var profiles grpcsvc.ProfileAccountLookup
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = uconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := waitForGRPCReady(waitCtx, uconn); err != nil {
				waitCancel()
				log.Fatalf("user grpc dial: %v", err)
			}
			waitCancel()
			profiles = &s2s.UserGRPCProfiles{Client: userv1.NewUserServiceClient(uconn)}
		}

		var files grpcsvc.FileMetadataLookup
		if fileAddr := strings.TrimSpace(os.Getenv("FILE_GRPC_ADDR")); fileAddr != "" {
			fconn, err := grpc.NewClient(grpcclient.DialTarget(fileAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("file grpc: %v", err)
			}
			defer func() { _ = fconn.Close() }()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := waitForGRPCReady(waitCtx, fconn); err != nil {
				waitCancel()
				log.Fatalf("file grpc dial: %v", err)
			}
			waitCancel()
			files = s2s.NewFileGRPCMetadata(filev1.NewFileServiceClient(fconn))
		}

		var msgEvents messageevents.MessageEventsPublisher
		if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
			jsPub, err := messageevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			if err := jsPub.EnsureStream(); err != nil {
				log.Fatalf("nats ensure message_events stream: %v", err)
			}
			jsPub.Logger = logger
			msgEvents = jsPub
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		messagingv1.RegisterMessagingServiceServer(grpcSrv, &grpcsvc.MessagingGRPC{
			Messages:      &store.MessagesStore{Pool: pool},
			Reactions:     &store.ReactionsStore{Pool: pool},
			ChatGuard:     chatGuard,
			Blocks:        blocks,
			UserProfiles:  profiles,
			Files:         files,
			MessageEvents: msgEvents,
		})
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
		Addr:              httpAddr,
		Handler:           httpserver.Wrap(healthHandler(serviceName), logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	logger.Info("listening", slog.String("addr", httpAddr))
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
