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

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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

	dmLister := dialDMChatLister()

	hub := newWSHub()
	instanceID := strings.TrimSpace(os.Getenv("REALTIME_INSTANCE_ID"))
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	log.Printf("%s instance_id=%s", serviceName, instanceID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var rf *redisFanout
	if redisAddr := strings.TrimSpace(os.Getenv("REALTIME_REDIS_ADDR")); redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: strings.TrimSpace(os.Getenv("REALTIME_REDIS_PASSWORD")),
		})
		defer func() { _ = rdb.Close() }()
		rf = newRedisFanout(redisFanoutConfig{
			Client:     rdb,
			Hub:        hub,
			InstanceID: instanceID,
		})
		go func() {
			err := rf.runSubscriber(ctx)
			if err != nil && err != context.Canceled {
				log.Printf("realtime redis subscriber exited: %v", err)
			}
		}()
	}

	if natsURL := strings.TrimSpace(os.Getenv("NATS_URL")); natsURL != "" {
		go func() {
			err := runMessageEventsConsumer(ctx, hub, natsURL, instanceID)
			if err != nil && err != context.Canceled {
				log.Printf("realtime message.events consumer exited: %v", err)
			}
		}()
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           newServiceHandler(serviceName, tv, dmLister, hub, rf, instanceID),
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
		cancel()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop:
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := server.Shutdown(shutCtx); err != nil {
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

func dialDMChatLister() dmChatLister {
	addr := strings.TrimSpace(os.Getenv("REALTIME_CHAT_GRPC_ADDR"))
	if addr == "" {
		return nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("realtime: REALTIME_CHAT_GRPC_ADDR NewClient %q: %v", addr, err)
		return nil
	}
	return newGRPCDMChatLister(conn)
}
