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

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	grpcsvc "voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceevents"

	callsv1 "voice.app/voice/calls/v1"
)

const serviceName = "voice"

func main() {
	addr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("VOICE_GRPC_LISTEN")); v != "" {
		grpcListen = v
	}

	var grpcSrv *grpc.Server
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	var callStore voicestore.CallStore
	if redisAddr := strings.TrimSpace(os.Getenv("VOICE_REDIS_ADDR")); redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: strings.TrimSpace(os.Getenv("VOICE_REDIS_PASSWORD")),
		})
		defer func() { _ = rdb.Close() }()
		callStore = voicestore.NewRedisCallStore(rdb, strings.TrimSpace(os.Getenv("VOICE_REDIS_PREFIX")))
	} else {
		callStore = voicestore.NewMemoryCallStore()
		log.Printf("%s: VOICE_REDIS_ADDR not set; using in-memory call store", serviceName)
	}

	var events voiceevents.Publisher
	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		jsPub, err := voiceevents.NewJetStreamPublisher(natsURL)
		if err != nil {
			log.Fatalf("nats jetstream publisher: %v", err)
		}
		defer func() { _ = jsPub.Close() }()
		events = jsPub
	}

	tokenTTL := time.Hour
	voiceSvc := &grpcsvc.VoiceGRPC{
		Calls: callStore,
		Tokens: livekit.NewHS256TokenIssuer(
			strings.TrimSpace(os.Getenv("LIVEKIT_API_KEY")),
			strings.TrimSpace(os.Getenv("LIVEKIT_API_SECRET")),
			strings.TrimSpace(os.Getenv("LIVEKIT_URL")),
			tokenTTL,
		),
		Events:      events,
		RingTimeout: 30 * time.Second,
	}
	lis, err := net.Listen("tcp", grpcListen)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	grpcSrv = grpc.NewServer()
	callsv1.RegisterVoiceServiceServer(grpcSrv, voiceSvc)
	go func() {
		log.Printf("%s gRPC listening on %s", serviceName, grpcListen)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()
	go runMissedCallSweeper(runCtx, voiceSvc)

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
		runCancel()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		runCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
		if grpcSrv != nil {
			grpcSrv.GracefulStop()
		}
	}
}

func runMissedCallSweeper(ctx context.Context, svc *grpcsvc.VoiceGRPC) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := svc.MarkExpiredCallsMissed(ctx); err != nil {
				log.Printf("voice missed-call sweeper: %v", err)
			}
		}
	}
}
