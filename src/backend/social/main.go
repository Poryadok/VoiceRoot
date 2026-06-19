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
	"google.golang.org/grpc/credentials/insecure"

	grpcsvc "voice/backend/social/internal/grpcsvc"
	socials2s "voice/backend/social/internal/s2s"
	"voice/backend/social/internal/store"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/grpcmw"
	"voice/backend/pkg/httpserver"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

const serviceName = "social"

func main() {
	logger := httpserver.NewLogger(serviceName)
	httpAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		httpAddr = v
	}
	grpcListen := ":9090"
	if v := strings.TrimSpace(os.Getenv("SOCIAL_GRPC_LISTEN")); v != "" {
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

		var privacy grpcsvc.FriendRequestPrivacyChecker
		var phoneSearchPrivacy grpcsvc.PhoneSearchPrivacyChecker
		var phoneHashes grpcsvc.PhoneHashLookup
		var spaceCoMembership grpcsvc.SpaceCoMembershipChecker
		if userAddr := strings.TrimSpace(os.Getenv("USER_GRPC_ADDR")); userAddr != "" {
			uconn, err := grpc.NewClient(grpcclient.DialTarget(userAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("user grpc: %v", err)
			}
			defer func() { _ = uconn.Close() }()
			userPrivacy := &socials2s.GRPCUserPrivacy{Client: userv1.NewUserServiceClient(uconn)}
			privacy = userPrivacy
			phoneSearchPrivacy = userPrivacy
		}
		if authAddr := strings.TrimSpace(os.Getenv("AUTH_GRPC_ADDR")); authAddr != "" {
			aconn, err := grpc.NewClient(grpcclient.DialTarget(authAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("auth grpc: %v", err)
			}
			defer func() { _ = aconn.Close() }()
			phoneHashes = socials2s.NewGRPCAuthPhoneHashLookup(aconn)
		}
		if spaceAddr := strings.TrimSpace(os.Getenv("SPACE_GRPC_ADDR")); spaceAddr != "" {
			spconn, err := grpc.NewClient(grpcclient.DialTarget(spaceAddr), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("space grpc: %v", err)
			}
			defer func() { _ = spconn.Close() }()
			spaceCoMembership = socials2s.NewGRPCSpaceCoMembership(spconn)
		}

		lis, err := net.Listen("tcp", grpcListen)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		grpcSrv = grpc.NewServer(grpcmw.ServerOptions(logger)...)
		socialv1.RegisterSocialServiceServer(grpcSrv, &grpcsvc.SocialGRPC{
			Friends:            &store.FriendshipStore{Pool: pool},
			Blocks:             &store.BlockStore{Pool: pool},
			Privacy:            privacy,
			PhoneSearchPrivacy: phoneSearchPrivacy,
			PhoneHashes:        phoneHashes,
			SpaceCoMembership:  spaceCoMembership,
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
