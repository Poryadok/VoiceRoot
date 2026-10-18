package main

import (
	"context"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/grpcsvc"
	"voice/backend/moderation/internal/intakeprincipal"
	"voice/backend/pkg/grpcmw"
)

func startReportIntake(ctx context.Context, svc *grpcsvc.ModerationGRPC, logger *slog.Logger) (func(), error) {
	cfg, enabled, err := intakeprincipal.FromEnv()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return func() {}, nil
	}
	runtime, err := intakeprincipal.New(ctx, cfg)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}
	options := append(grpcmw.ServerOptions(logger), runtime.ServerOptions()...)
	server := grpc.NewServer(options...)
	moderationv1.RegisterModerationServiceServer(server, svc)
	go func() {
		if err := server.Serve(listener); err != nil {
			logger.Error("report intake listener stopped", slog.Any("error", err))
		}
	}()
	return func() { server.Stop(); _ = listener.Close(); _ = runtime.Close() }, nil
}
