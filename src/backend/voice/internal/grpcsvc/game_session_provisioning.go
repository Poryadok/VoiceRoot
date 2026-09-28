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

type GameSessionProvisioner interface {
	Provision(context.Context, *callsv1.ProvisionGameSessionRoomRequest) (*callsv1.ProvisionGameSessionRoomResponse, error)
}

type GameSessionProvisioningGRPC struct {
	callsv1.UnimplementedGameSessionProvisioningServiceServer
	Store GameSessionProvisioner
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
