package main

import (
	"context"
	"errors"
	"net"

	"google.golang.org/grpc"
	socialv1 "voice.app/voice/social/v1"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/social/internal/grpcsvc"
)

func loadMessagingReportConfig() (socialprincipal.Config, bool, error) {
	return socialprincipal.LoadFromEnvWithAudience("social", "social-messaging-report", "SOCIAL_REPORT_MESSAGING_", ":9092")
}

func newMessagingReportServer(ctx context.Context, cfg socialprincipal.Config, service *grpcsvc.SocialGRPC) (*grpc.Server, func(), error) {
	if service == nil || service.Blocks == nil || service.Blocks.Pool == nil || service.ProfileAccounts == nil {
		return nil, nil, errors.New("Social report policy requires native storage and signed User resolver")
	}
	runtime, err := socialprincipal.New(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	socialv1.RegisterSocialServiceServer(server, &grpcsvc.MessagingReportBlocksGRPC{Social: service})
	return server, func() { server.Stop(); _ = runtime.Close() }, nil
}

func startMessagingReportListener(ctx context.Context, cfg socialprincipal.Config, service *grpcsvc.SocialGRPC, errorsOut chan<- error) (func(), error) {
	server, closeServer, err := newMessagingReportServer(ctx, cfg, service)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		closeServer()
		return nil, err
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			errorsOut <- err
		}
	}()
	return closeServer, nil
}
