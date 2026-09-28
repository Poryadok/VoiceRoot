package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/voice/internal/gameprincipal"
	"voice/backend/voice/internal/gameprovision"
)

type ManagedGameSessionRoomLookup interface {
	GetRoom(context.Context, string) (gameprovision.Room, error)
}

type GameSessionProvisioner interface {
	Provision(context.Context, *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error)
}

type GameSessionCloser interface {
	CloseGameSessionRoom(context.Context, *callsv1.CloseGameSessionRoomRequest, gameprovision.ManagedGameSessionMediaFencer) (*callsv1.CloseGameSessionRoomResponse, error)
}

type GameSessionProvisioningGRPC struct {
	callsv1.UnimplementedGameSessionProvisioningServiceServer
	Store  GameSessionProvisioner
	Closer GameSessionCloser
	Fencer gameprovision.ManagedGameSessionMediaFencer
}

func (s *GameSessionProvisioningGRPC) CloseGameSessionRoom(ctx context.Context, request *callsv1.CloseGameSessionRoomRequest) (*callsv1.CloseGameSessionRoomResponse, error) {
	if err := gameprincipal.RequireClose(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Closer == nil || s.Fencer == nil {
		return nil, status.Error(codes.Unavailable, "game session close unavailable")
	}
	response, err := s.Closer.CloseGameSessionRoom(ctx, request, s.Fencer)
	if errors.Is(err, gameprovision.ErrConflict) {
		return nil, status.Error(codes.AlreadyExists, "close operation or room conflicts")
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid game session close request")
	}
	if errors.Is(err, gameprovision.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "managed game session room not found")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "game session close failed")
	}
	return response, nil
}

func (s *GameSessionProvisioningGRPC) ProvisionGameSessionRoom(ctx context.Context, request *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error) {
	if err := gameprincipal.RequireProvisioning(ctx, request); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "game session provisioning unavailable")
	}
	response, err := s.Store.Provision(ctx, request)
	if errors.Is(err, gameprovision.ErrConflict) {
		return nil, status.Error(codes.AlreadyExists, "operation or resource binding conflicts")
	}
	if errors.Is(err, gameprovision.ErrInvalidRequest) {
		return nil, status.Error(codes.InvalidArgument, "invalid game session provisioning request")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "game session provisioning failed")
	}
	return response, nil
}
