package main

import (
	"context"
	"errors"
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
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"voice/backend/chat/internal/chatevents"
	grpcsvc "voice/backend/chat/internal/grpcsvc"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "chat"

func main() {
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("CHAT_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		defer pool.Close()

		var blocks grpcsvc.AccountBlockChecker
		if socialAddr := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_ADDR")); socialAddr != "" {
			sconn, err := grpc.NewClient(socialAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("social grpc: %v", err)
			}
			sconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			for {
				st := sconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc: unexpected shutdown")
				}
				if !sconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = sconn.Close()
					log.Fatalf("social grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = sconn.Close() }()
			blocks = grpcsvc.NewSocialGRPCBlocks(sconn)
		}

		var profiles grpcsvc.UserProfileLookup
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(userAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			uconn.Connect()
			waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			for {
				st := uconn.GetState()
				if st == connectivity.Ready {
					break
				}
				if st == connectivity.Shutdown {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc: unexpected shutdown")
				}
				if !uconn.WaitForStateChange(waitCtx, st) {
					waitCancel()
					_ = uconn.Close()
					log.Fatalf("user grpc dial: %v", context.Cause(waitCtx))
				}
			}
			waitCancel()
			defer func() { _ = uconn.Close() }()
			profiles = &grpcsvc.UserGRPCProfiles{Client: userv1.NewUserServiceClient(uconn)}
		}

		var listEnrich grpcsvc.ListChatsEnrichment
		if msgAddr := strings.TrimSpace(os.Getenv("MESSAGING_GRPC_ADDR")); msgAddr != "" {
			mconn, err := grpc.NewClient(msgAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("messaging grpc: %v", err)
			}
			defer func() { _ = mconn.Close() }()
			listEnrich = grpcsvc.NewMessagingListEnricher(messagingv1.NewMessagingServiceClient(mconn))
		}

		dmStore := &store.DMStore{Pool: pool}
		natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
		var chatEvents chatevents.Publisher
		if natsURL != "" {
			jsPub, err := chatevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				log.Fatalf("nats jetstream publisher: %v", err)
			}
			defer func() { _ = jsPub.Close() }()
			chatEvents = jsPub
			go func() {
				if err := runMessageActivityConsumer(runCtx, natsURL, os.Getenv("HOSTNAME"), dmStore); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("chat: message activity consumer stopped: %v", err)
				}
			}()
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer()
		chatv1.RegisterChatServiceServer(grpcSrv, &grpcsvc.ChatGRPC{
			DM:         dmStore,
			Profiles:   profiles,
			Blocks:     blocks,
			ListEnrich: listEnrich,
			ChatEvents: chatEvents,
		})
		go func() {
			log.Printf("%s gRPC listening on %s", serviceName, grpcListen)
			if err := grpcSrv.Serve(lis); err != nil {
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
		runCancel()
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
