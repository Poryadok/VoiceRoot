package grpcsvc

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CanonicalVoiceRoomAccess struct {
	SpaceID string
	Member  bool
	Active  bool
}

type AuthoritativeVoiceRoomAccessResolver interface {
	ResolveVoiceRoomAccess(context.Context, string, string) (CanonicalVoiceRoomAccess, error)
}

func (s *VoiceGRPC) resolveCanonicalVoiceRoomAccess(ctx context.Context, voiceRoomID, profileID string) (CanonicalVoiceRoomAccess, error) {
	if s == nil || s.VoiceRoomAccessResolver == nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.FailedPrecondition, "canonical voice room resolver not configured")
	}
	access, err := s.VoiceRoomAccessResolver.ResolveVoiceRoomAccess(ctx, voiceRoomID, profileID)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound, codes.Unavailable:
			return CanonicalVoiceRoomAccess{}, err
		}
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "canonical voice room resolver unavailable")
	}
	if !access.Active || !access.Member || strings.TrimSpace(access.SpaceID) == "" {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.PermissionDenied, "canonical voice room access denied")
	}
	if _, err := uuid.Parse(access.SpaceID); err != nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.PermissionDenied, "canonical voice room access denied")
	}
	return access, nil
}
