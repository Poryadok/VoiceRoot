package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/roleoverrideprincipal"
)

type roleOverrideTargetStore interface {
	ValidateRoleOverrideTarget(context.Context, uuid.UUID, *uuid.UUID) error
}

func (s *SpaceGRPC) ValidateRoleOverrideTarget(ctx context.Context, req *spacev1.ValidateRoleOverrideTargetRequest) (*spacev1.ValidateRoleOverrideTargetResponse, error) {
	if err := roleoverrideprincipal.Require(ctx, "space", req); err != nil {
		return nil, err
	}
	spaceID, err := roleoverrideprincipal.CanonicalID(req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	var roomID *uuid.UUID
	if req.VoiceRoomId != nil {
		room, err := roleoverrideprincipal.CanonicalID(*req.VoiceRoomId)
		if err != nil {
			return nil, err
		}
		roomID = &room
	}
	if s == nil {
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	targetStore := s.roleOverrideTargets
	if targetStore == nil {
		if s.Store == nil {
			return nil, status.Error(codes.Unavailable, "target validation unavailable")
		}
		targetStore = s.Store
	}
	if err := targetStore.ValidateRoleOverrideTarget(ctx, spaceID, roomID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "target not found")
		}
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "target validation unavailable")
	}
	return &spacev1.ValidateRoleOverrideTargetResponse{}, nil
}
