package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/voice/internal/spacelifecycle"
)

type CanonicalVoiceRoomAccess struct {
	SpaceID                    string
	Member                     bool
	Active                     bool
	AccessEpoch                uint64
	CommunicationStateResolved bool
	TimedOut                   bool
	DecisionDeadline           time.Time
}

type AuthoritativeVoiceRoomAccessResolver interface {
	ResolveVoiceRoomAccess(context.Context, string, string, string) (CanonicalVoiceRoomAccess, error)
}

type CanonicalVoiceRoomGrants struct {
	PolicyEpoch     uint64
	CanJoin         bool
	CanPublishAudio bool
	CanSubscribe    bool
}

type SpaceVoiceRoomGrantResolver interface {
	ResolveVoiceRoomGrants(context.Context, string, string, string) (CanonicalVoiceRoomGrants, error)
}

func (s *VoiceGRPC) resolveCanonicalVoiceRoomAccess(ctx context.Context, expectedSpaceID, voiceRoomID, profileID string) (CanonicalVoiceRoomAccess, error) {
	if s == nil || s.VoiceRoomAccessResolver == nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "canonical voice room resolver not configured")
	}
	deadline := time.Now().Add(2 * time.Second)
	decisionCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	access, err := s.VoiceRoomAccessResolver.ResolveVoiceRoomAccess(decisionCtx, expectedSpaceID, voiceRoomID, profileID)
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
	if access.SpaceID != expectedSpaceID {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.PermissionDenied, "canonical voice room space does not match expected Space")
	}
	if !access.CommunicationStateResolved || access.AccessEpoch == 0 {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space communication authority incomplete")
	}
	if access.TimedOut {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.PermissionDenied, "member is timed out in this Space")
	}
	if err := decisionCtx.Err(); err != nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space admission decision expired")
	}
	access.DecisionDeadline = deadline
	if s.SpaceLifecycle == nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space lifecycle admission unavailable")
	}
	if err := s.SpaceLifecycle.CheckAdmission(decisionCtx, access.SpaceID); err != nil {
		if errors.Is(err, spacelifecycle.ErrSpaceFrozen) {
			return CanonicalVoiceRoomAccess{}, status.Error(codes.FailedPrecondition, "Space voice admission is fenced")
		}
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space lifecycle admission unavailable")
	}
	if decisionCtx.Err() != nil {
		return CanonicalVoiceRoomAccess{}, status.Error(codes.Unavailable, "Space admission decision expired")
	}
	return access, nil
}
