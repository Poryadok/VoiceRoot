package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/ordinarycall"
)

// ordinaryIdentity resolves account ownership from the trusted User authority.
// Invited peers have no authenticated admission epoch until their own request.
func (s *VoiceGRPC) ordinaryIdentity(ctx context.Context, profileID string, actor bool) (ordinarycall.Identity, error) {
	profile, err := uuid.Parse(profileID)
	if err != nil || profile == uuid.Nil || profile.String() != profileID {
		return ordinarycall.Identity{}, status.Error(codes.InvalidArgument, "invalid voice profile identity")
	}
	if s == nil || s.AccountVoiceProfiles == nil {
		return ordinarycall.Identity{}, status.Error(codes.Unavailable, "trusted profile account authority unavailable")
	}
	account, err := s.AccountVoiceProfiles.AccountIDByProfileID(ctx, profile)
	if err != nil || account == uuid.Nil {
		if status.Code(err) == codes.NotFound {
			return ordinarycall.Identity{}, status.Error(codes.PermissionDenied, "voice profile has no trusted account")
		}
		return ordinarycall.Identity{}, status.Error(codes.Unavailable, "trusted profile account lookup unavailable")
	}
	id := ordinarycall.Identity{AccountID: account, ProfileID: profile}
	if !actor {
		return id, nil
	}
	claimed, ok := accountVoiceCaller(ctx, profileID)
	if !ok || claimed != account.String() {
		return ordinarycall.Identity{}, status.Error(codes.PermissionDenied, "authenticated account does not own voice profile")
	}
	if _, delegated := principal.FromContext(ctx); delegated {
		verifiedProfile, verifiedAccount, epoch, err := callerVoiceUser(ctx)
		if err != nil {
			return ordinarycall.Identity{}, err
		}
		if verifiedProfile != profileID || verifiedAccount != account.String() || epoch <= 0 {
			return ordinarycall.Identity{}, status.Error(codes.Unauthenticated, "invalid ordinary admission identity")
		}
		id.SessionEpoch = epoch
	}
	return id, nil
}

func (s *VoiceGRPC) requireOrdinaryEpoch(ctx context.Context, id ordinarycall.Identity) error {
	// The legacy listener supplies trusted account/profile but no signed epoch.
	// Keep that provenance explicitly unknown; A3 must reject it. Never infer a
	// positive epoch from Auth floor, another device, or the invited peer.
	if id.SessionEpoch == 0 {
		return nil
	}
	if s.SessionEpochChecker == nil {
		return status.Error(codes.Unavailable, "session epoch authority unavailable")
	}
	return s.SessionEpochChecker.RequireCurrent(ctx, id.AccountID.String(), id.SessionEpoch)
}
